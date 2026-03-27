package server

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go-redis-like/resp"
	"go-redis-like/store"
)

// Server 封裝 TCP 監聽與連線管理邏輯。
type Server struct {
	addr  string
	store *store.Store
}

// New 建立一個新的 Server，addr 例如 ":6380"。
func New(addr string, s *store.Store) *Server {
	return &Server{addr: addr, store: s}
}

/**
 * Run 啟動 TCP 伺服器，持續接受並處理客戶端連線。
 * 這個方法會阻塞，直到伺服器遇到錯誤或被外部中斷。
 */
func (srv *Server) Run() error {
	// net.Listen 會在指定的地址和端口上開始監聽 TCP 連線。
	ln, err := net.Listen("tcp", srv.addr)

	// 如果監聽失敗，返回錯誤並包裝上下文訊息。
	if err != nil {
		return fmt.Errorf("伺服器啟動失敗: %w", err)
	}
	defer ln.Close()
	fmt.Printf("✅ 伺服器啟動成功，正在監聽 %s 端口...\n", srv.addr)

	for {
		// ln.Accept() 會阻塞直到有新的客戶端連線進來，然後返回一個 net.Conn 物件和可能的錯誤。
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("❌ 接受連線錯誤:", err)
			continue
		}
		fmt.Println("✅ 客戶端已連接:", conn.RemoteAddr())
		go srv.handleConn(conn)
	}
}

/**
 * handleConn 處理單一客戶端連線，讀取命令並回應。
 * 這個方法會在獨立的 goroutine 中執行，以允許同時處理多個客戶端。
 */
func (srv *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// bufio.NewReader(conn) 會創建一個緩衝區讀取器，從 TCP 連線中讀取資料。
	reader := bufio.NewReader(conn)

	for {
		// resp.Parse 會解析 RESP 格式的命令，返回一個字串切片和可能的錯誤。
		args, err := resp.Parse(reader)
		if err != nil {
			// 連線關閉或讀取錯誤，靜默退出
			return
		}
		if len(args) == 0 {
			continue
		}

		// 將命令轉為大寫以便比較，這樣客戶端可以使用大小寫混合的命令。
		cmd := strings.ToUpper(args[0])

		// 依據命令執行對應的邏輯，並使用 resp 包來回應客戶端。
		switch cmd {
		case "PING":
			// PING 命令通常用於測試連線是否正常，回應 "PONG"。
			resp.WriteSimpleString(conn, "PONG")

		case "SET":
			// SET 命令需要至少兩個參數：key 和 value，如果參數不足則回應錯誤。
			if len(args) < 3 {
				resp.WriteError(conn, "錯誤的參數數量，SET 命令需要 key 和 value")
				continue
			}
			var ttl time.Duration
			if len(args) >= 5 && strings.ToUpper(args[3]) == "EX" {
				secs, err := strconv.ParseInt(args[4], 10, 64)
				if err != nil || secs <= 0 {
					resp.WriteError(conn, "invalid expire time in 'SET'")
					continue
				}
				ttl = time.Duration(secs) * time.Second
			}
			srv.store.Set(args[1], args[2], ttl)
			resp.WriteSimpleString(conn, "OK")

		case "GET":
			// GET 命令需要至少一個參數：key，如果參數不足則回應錯誤。
			if len(args) < 2 {
				resp.WriteError(conn, "錯誤的參數數量，GET 命令需要 key")
				continue
			}
			val, ok := srv.store.Get(args[1])
			if !ok {
				resp.WriteNullBulk(conn)
			} else {
				resp.WriteBulkString(conn, val)
			}

		case "DEL":
			// DEL 命令需要至少一個參數：key，如果參數不足則回應錯誤。它會嘗試刪除所有指定的 key，並回應實際刪除的 key 數量。
			if len(args) < 2 {
				resp.WriteError(conn, "錯誤的參數數量，DEL 命令需要至少一個 key")
				continue
			}
			deleted := int64(0)
			for _, key := range args[1:] {
				if srv.store.Del(key) {
					deleted++
				}
			}
			resp.WriteInteger(conn, deleted)

		case "EXISTS":
			// EXISTS 命令需要至少一個參數：key，如果參數不足則回應錯誤。它會檢查所有指定的 key 是否存在，並回應存在的 key 數量。
			if len(args) < 2 {
				resp.WriteError(conn, "錯誤的參數數量，EXISTS 命令需要至少一個 key")
				continue
			}
			count := int64(0)
			for _, key := range args[1:] {
				if srv.store.Exists(key) {
					count++
				}
			}
			resp.WriteInteger(conn, count)

		case "EXPIRE":
			if len(args) < 3 {
				resp.WriteError(conn, "wrong number of arguments for 'EXPIRE'")
				continue
			}
			secs, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil || secs <= 0 {
				resp.WriteError(conn, "value is not an integer or out of range")
				continue
			}
			if srv.store.Expire(args[1], time.Duration(secs)*time.Second) {
				resp.WriteInteger(conn, 1)
			} else {
				resp.WriteInteger(conn, 0)
			}

		case "TTL":
			if len(args) < 2 {
				resp.WriteError(conn, "wrong number of arguments for 'TTL'")
				continue
			}
			resp.WriteInteger(conn, srv.store.TTL(args[1]))

		case "PERSIST":
			if len(args) < 2 {
				resp.WriteError(conn, "wrong number of arguments for 'PERSIST'")
				continue
			}
			if srv.store.Persist(args[1]) {
				resp.WriteInteger(conn, 1)
			} else {
				resp.WriteInteger(conn, 0)
			}

		default:
			resp.WriteError(conn, fmt.Sprintf("未知的命令 '%s'", strings.ToUpper(cmd)))
		}
	}
}
