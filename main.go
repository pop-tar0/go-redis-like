package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go-redis-like/aof"
	"go-redis-like/server"
	"go-redis-like/store"
)

func main() {
	s := store.New()

	// 開啟（或建立）AOF 檔案
	a, err := aof.New("redis.aof")
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ AOF 開啟失敗:", err)
		os.Exit(1)
	}
	defer a.Close()

	// 啟動前先 replay AOF，恢復上次的資料
	fmt.Println("🔄 正在從 AOF 恢復資料...")
	if err := a.Replay(func(args []string) {
		if len(args) == 0 {
			return
		}
		cmd := strings.ToUpper(args[0])
		switch cmd {
		case "SET":
			if len(args) < 3 {
				return
			}
			var ttl time.Duration
			if len(args) >= 5 && strings.ToUpper(args[3]) == "EX" {
				secs := int64(0)
				fmt.Sscanf(args[4], "%d", &secs)
				if secs > 0 {
					ttl = time.Duration(secs) * time.Second
				}
			}
			s.Set(args[1], args[2], ttl)
		case "DEL":
			for _, key := range args[1:] {
				s.Del(key)
			}
		case "EXPIRE":
			if len(args) < 3 {
				return
			}
			secs := int64(0)
			fmt.Sscanf(args[2], "%d", &secs)
			if secs > 0 {
				s.Expire(args[1], time.Duration(secs)*time.Second)
			}
		case "PERSIST":
			if len(args) < 2 {
				return
			}
			s.Persist(args[1])
		}
	}); err != nil {
		fmt.Fprintln(os.Stderr, "❌ AOF replay 失敗:", err)
		os.Exit(1)
	}
	fmt.Println("✅ AOF 恢復完成")

	// 啟動伺服器
	srv := server.New(":6380", s, a)

	if err := srv.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
