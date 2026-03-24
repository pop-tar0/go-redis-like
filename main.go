package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

func main() {
	// 開一個 TCP 監聽器，監聽在本地的 6380 端口，: 前面不寫 host，表是接受本機的這個 port
	// redis 真正常用的 port 是 6379，這裡用 6380 是為了避免和本機的 redis 衝突
	listener, err := net.Listen("tcp", ":6380")
	if err != nil {
		fmt.Println("❌ 伺服器啟動錯誤:", err)
		return
	}

	// 程式結束前關閉監聽器
	defer listener.Close()

	fmt.Println("✅ 伺服器啟動成功，正在監聽 6380 端口...")

	// 不斷接受客戶端連接
	for {
		// 接受一個客戶端連接，這裡會阻塞直到有客戶端連接進來
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("❌ 接受客戶端連接錯誤:", err)
			continue
		}

		fmt.Println("✅ 客戶端已連接:", conn.RemoteAddr())

		// 為每個客戶端連接啟動一個新的 goroutine 來處理，這樣可以同時處理多個客戶端的連接
		// 開一個新的 goroutine 來處理這個連接，這樣主程式就可以繼續接受其他客戶端的連接
		// goroutine 是 Go 語言中的輕量級執行緒，使用 go 關鍵字來啟動一個新的 goroutine
		go handleConnection(conn)
	}
}

var (
	// db 是一個簡單的字串到字串的映射，用來存儲 SET 命令設置的 key-value 資料
	db = make(map[string]string)
	// dbLock 是一個讀寫鎖，用來保護對 db 的並發訪問，確保在多個 goroutine 同時訪問 db 時不會出現競爭條件
	dbLock sync.RWMutex
)

/**
 * handleConnection 處理每個客戶端連接的函式
 * 這裡會不斷讀取客戶端發送的訊息，直到連線關閉或出現錯誤
 */
func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		input := scanner.Text()         // .Text() 會返回掃描到的當前行的文字，這裡我們假設每個命令都是一行
		fields := strings.Fields(input) // 將輸入的命令按空格分割成多個部分，第一部分是命令名稱，後面是參數
		if len(fields) == 0 {
			continue // 如果沒有輸入任何命令，繼續等待下一個命令
		}

		cmd := strings.ToUpper(fields[0]) // 將命令名稱轉換為大寫，這樣用戶輸入的命令不區分大小寫

		switch cmd {
		case "SET":
			if len(fields) < 3 {
				conn.Write([]byte("命令錯誤，正確格式: SET key value\r\n"))
				continue
			}

			// [重點] 寫入時加「全域寫鎖」，確保同一時間只有一個 goroutine 可以修改資料庫
			dbLock.Lock()
			db[fields[1]] = fields[2]
			dbLock.Unlock()

			conn.Write([]byte("+OK\r\n"))

		case "GET":
			if len(fields) < 2 {
				conn.Write([]byte("命令錯誤，正確格式: GET key\r\n"))
				continue
			}

			// [重點] 讀取時加「全域讀鎖」，效能比寫鎖好
			dbLock.RLock()
			val, exists := db[fields[1]]
			dbLock.RUnlock()

			if !exists {
				conn.Write([]byte("$-1\r\n"))
			} else {
				conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
			}

		case "PING":
			conn.Write([]byte("+PONG\r\n"))

		default:
			conn.Write([]byte("命令錯誤，未知的命令\r\n"))
		}
	}
}
