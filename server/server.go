package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"

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

// Run 開始監聽並接受連線（阻塞）。
func (srv *Server) Run() error {
	ln, err := net.Listen("tcp", srv.addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", srv.addr, err)
	}
	defer ln.Close()
	fmt.Printf("✅ 伺服器啟動成功，正在監聽 %s 端口...\n", srv.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("❌ 接受連線錯誤:", err)
			continue
		}
		fmt.Println("✅ 客戶端已連接:", conn.RemoteAddr())
		go srv.handleConn(conn)
	}
}

// handleConn 在獨立 goroutine 中處理單一 client 連線。
func (srv *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		args, err := resp.Parse(reader)
		if err != nil {
			// 連線關閉或讀取錯誤，靜默退出
			return
		}
		if len(args) == 0 {
			continue
		}

		cmd := strings.ToUpper(args[0])

		switch cmd {
		case "PING":
			resp.WriteSimpleString(conn, "PONG")

		case "SET":
			if len(args) < 3 {
				resp.WriteError(conn, "wrong number of arguments for 'SET'")
				continue
			}
			srv.store.Set(args[1], args[2])
			resp.WriteSimpleString(conn, "OK")

		case "GET":
			if len(args) < 2 {
				resp.WriteError(conn, "wrong number of arguments for 'GET'")
				continue
			}
			val, ok := srv.store.Get(args[1])
			if !ok {
				resp.WriteNullBulk(conn)
			} else {
				resp.WriteBulkString(conn, val)
			}

		case "DEL":
			if len(args) < 2 {
				resp.WriteError(conn, "wrong number of arguments for 'DEL'")
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
			if len(args) < 2 {
				resp.WriteError(conn, "wrong number of arguments for 'EXISTS'")
				continue
			}
			count := int64(0)
			for _, key := range args[1:] {
				if srv.store.Exists(key) {
					count++
				}
			}
			resp.WriteInteger(conn, count)

		default:
			resp.WriteError(conn, fmt.Sprintf("unknown command '%s'", strings.ToLower(cmd)))
		}
	}
}
