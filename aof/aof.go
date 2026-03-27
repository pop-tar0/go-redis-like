package aof

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"

	"go-redis-like/resp"
	"go-redis-like/store"
)

// AOF 負責將指令持久化到檔案，並在啟動時重播。
type AOF struct {
	// mu 是一個互斥鎖，用於保護對 file 和 writer 的訪問，確保線程安全。
	mu sync.Mutex

	// file 是 AOF 檔案的 os.File 物件，用於讀寫 AOF 檔案。
	file *os.File

	// writer 是一個 bufio.Writer，用於緩衝寫入 AOF 檔案，提高寫入效率。
	writer *bufio.Writer
}

/**
 * New 函式用於創建一個新的 AOF 實例，接受一個檔案路徑作為參數。如果檔案不存在則會被創建，如果存在則會以讀寫和追加模式打開。它返回一個 AOF 實例和可能的錯誤。
 * 這個函式使用 os.OpenFile 來打開檔案，並使用 bufio.NewWriter 來創建一個緩衝區寫入器，以提高寫入效率。
 */
func New(path string) (*AOF, error) {
	// os.O_CREATE: 如果檔案不存在則創建
	// os.O_RDWR: 以讀寫模式打開檔案
	// os.O_APPEND: 以追加模式打開檔案，寫入的資料會被追加到檔案末尾
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("AOF 檔案打開失敗: %w", err)
	}
	return &AOF{
		file:   f,
		writer: bufio.NewWriter(f),
	}, nil
}

/**
 * Write 將指令以 RESP 格式寫入 AOF 檔案。它接受一個字串切片作為參數，代表指令和其參數，並將其序列化為 RESP Array 格式寫入檔案。這個方法使用互斥鎖來確保在多線程環境下的線程安全。
 * 寫入的格式如下：
 *   *<元素數量>\r\n
 *   $<元素1長度>\r\n
 *   <元素1內容>\r\n
 *   ...
 *   $<元素N長度>\r\n
 *   <元素N內容>\r\n
 */
func (a *AOF) Write(args []string) error {
	// 使用互斥鎖來保護對 file 和 writer 的訪問，確保在多線程環境下不會發生競爭條件。
	a.mu.Lock()
	defer a.mu.Unlock()

	// 寫入 RESP Array header
	fmt.Fprintf(a.writer, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(a.writer, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return a.writer.Flush()
}

/**
 * Replay 從 AOF 檔案中讀取指令並執行。它接受一個 handler 函式作為參數，這個函式會被呼叫來處理每一條從 AOF 中讀取的指令。Replay 方法會從頭開始讀取 AOF 檔案，解析每一條指令，並將其傳遞給 handler 進行處理。
 * 這個方法使用 bufio.Reader 來緩衝讀取檔案，提高讀取效率。當讀到檔案末尾時，會正常結束並返回 nil。
 */
func (a *AOF) Replay(handler func(args []string)) error {
	// 從頭開始讀
	if _, err := a.file.Seek(0, 0); err != nil {
		return err
	}

	// 使用 bufio.Reader 來緩衝讀取檔案，提高讀取效率。
	reader := bufio.NewReader(a.file)
	for {
		args, err := resp.Parse(reader)
		if err != nil {
			// 檔案讀完，正常結束
			break
		}
		if len(args) > 0 {
			handler(args)
		}
	}
	return nil
}

/**
 * Close 關閉 AOF 檔案。這個方法使用互斥鎖來確保在多線程環境下的線程安全，並且在關閉檔案之前會先刷新緩衝區中的資料。
 */
func (a *AOF) Close() error {
	// 使用互斥鎖來保護對 file 和 writer 的訪問，確保在多線程環境下不會發生競爭條件。
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.writer.Flush(); err != nil {
		return err
	}
	return a.file.Close()
}

// Rewrite 將 store 目前的快照重寫成乾淨的 AOF 檔案，清除冗餘指令。
// 流程：先寫入暫存檔 → 替換原始檔案，確保原子性。
func (a *AOF) Rewrite(s *store.Store, path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tmpPath := path + ".tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("aof: create tmp: %w", err)
	}

	w := bufio.NewWriter(tmp)
	entries := s.Snapshot()
	for _, e := range entries {
		if !e.Expiry.IsZero() {
			// 有 TTL：計算剩餘秒數，寫成 SET key value EX secs
			remaining := int64(time.Until(e.Expiry).Seconds())
			if remaining <= 0 {
				continue // 已過期，跳過
			}
			secs := fmt.Sprintf("%d", remaining)
			args := []string{"SET", e.Key, e.Value, "EX", secs}
			fmt.Fprintf(w, "*%d\r\n", len(args))
			for _, arg := range args {
				fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg)
			}
		} else {
			// 永久 key：寫成 SET key value
			args := []string{"SET", e.Key, e.Value}
			fmt.Fprintf(w, "*%d\r\n", len(args))
			for _, arg := range args {
				fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg)
			}
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	// 原子替換：把暫存檔改名為正式檔案
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("aof: rename: %w", err)
	}

	// 重新開啟檔案，讓後續的 Write 繼續 append
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("aof: reopen: %w", err)
	}
	a.file = f
	a.writer = bufio.NewWriter(f)
	return nil
}
