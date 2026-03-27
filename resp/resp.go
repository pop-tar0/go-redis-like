// Package resp 實作 Redis Serialization Protocol（RESP2）的解析與序列化。
//
// RESP2 資料型態：
//   - Simple String: +OK\r\n
//   - Error:         -ERR message\r\n
//   - Integer:       :1000\r\n
//   - Bulk String:   $6\r\nfoobar\r\n  |  $-1\r\n（null）
//   - Array:         *2\r\n$3\r\nGET\r\n$5\r\nhello\r\n
package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ---------- 型態常數 ----------

const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

// ---------- 解析 ----------

// Parse 從 reader 讀取一個完整的 RESP2 訊息，回傳解析後的指令 token 陣列。
//
// 支援兩種輸入模式：
//  1. RESP Array（redis-cli 發送的格式）：  *3\r\n$3\r\nSET\r\n...
//  2. Inline command（nc / telnet 純文字）： SET key value\r\n
func Parse(reader *bufio.Reader) ([]string, error) {
	// 偷看第一個 byte 決定走哪條路
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if b == TypeArray {
		return parseArray(reader)
	}

	// Inline fallback：把第一個 byte 推回去，再讀整行
	if err := reader.UnreadByte(); err != nil {
		return nil, err
	}
	return parseInline(reader)
}

// parseArray 解析 RESP Array 格式，假設開頭的 '*' 已被讀走。
func parseArray(reader *bufio.Reader) ([]string, error) {
	// 讀取元素數量
	countLine, err := readLine(reader)
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(countLine)
	if err != nil {
		return nil, fmt.Errorf("resp: invalid array length %q", countLine)
	}
	if count < 0 {
		return nil, nil // null array
	}

	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// 每個元素應為 Bulk String
		typeByte, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if typeByte != TypeBulkString {
			return nil, fmt.Errorf("resp: expected bulk string, got %q", typeByte)
		}

		s, err := parseBulkString(reader)
		if err != nil {
			return nil, err
		}
		args = append(args, s)
	}
	return args, nil
}

// parseBulkString 解析 Bulk String，假設開頭的 '$' 已被讀走。
func parseBulkString(reader *bufio.Reader) (string, error) {
	lenLine, err := readLine(reader)
	if err != nil {
		return "", err
	}
	length, err := strconv.Atoi(lenLine)
	if err != nil {
		return "", fmt.Errorf("resp: invalid bulk string length %q", lenLine)
	}
	if length < 0 {
		return "", nil // null bulk string
	}

	// 讀取 length 個 byte + 結尾 \r\n
	buf := make([]byte, length+2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return string(buf[:length]), nil
}

// parseInline 解析純文字 inline 指令（例如：SET key value\r\n）。
func parseInline(reader *bufio.Reader) ([]string, error) {
	line, err := readLine(reader)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}

// readLine 讀取一行並去除結尾的 \r\n 或 \n。
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	return line, nil
}

// ---------- 序列化（寫回 client）----------

// WriteSimpleString 寫入 RESP Simple String：+msg\r\n
func WriteSimpleString(w io.Writer, msg string) error {
	_, err := fmt.Fprintf(w, "+%s\r\n", msg)
	return err
}

// WriteError 寫入 RESP Error：-ERR msg\r\n
func WriteError(w io.Writer, msg string) error {
	_, err := fmt.Fprintf(w, "-ERR %s\r\n", msg)
	return err
}

// WriteInteger 寫入 RESP Integer：:n\r\n
func WriteInteger(w io.Writer, n int64) error {
	_, err := fmt.Fprintf(w, ":%d\r\n", n)
	return err
}

// WriteBulkString 寫入 RESP Bulk String：$len\r\ndata\r\n
func WriteBulkString(w io.Writer, s string) error {
	_, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(s), s)
	return err
}

// WriteNullBulk 寫入 RESP Null Bulk String：$-1\r\n
func WriteNullBulk(w io.Writer) error {
	_, err := fmt.Fprint(w, "$-1\r\n")
	return err
}
