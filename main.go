package main

import (
	"fmt"
	// Go 的函式庫，用於處理操作系統相關的功能，例如輸出錯誤訊息和退出程式
	"os"

	"go-redis-like/server"
	"go-redis-like/store"
)

func main() {
	// 初始化資料庫和伺服器
	s := store.New()

	// 啟動伺服器
	srv := server.New(":6380", s)

	if err := srv.Run(); err != nil {
		// Stderr 是標準錯誤輸出，通常用於輸出錯誤訊息
		fmt.Fprintln(os.Stderr, "❌", err)
		// 發生錯誤時退出程式，1 通常表示一般錯誤
		os.Exit(1)
	}
}
