# 專案 Roadmap

## 總覽

這份文件記錄整個專案從最簡版本到具備生產級概念的完整規劃。每個 Stage 都有明確的學習目標、實作範圍與完成標準。

| Stage   | 主題                        | 狀態      |
| ------- | --------------------------- | --------- |
| Stage 1 | TCP Server + Inline 指令    | ✅ 已完成 |
| Stage 2 | RESP2 協議 + 模組化         | ✅ 已完成 |
| Stage 3 | TTL / 過期機制              | 🔲 待實作 |
| Stage 4 | 持久化（AOF）               | 🔲 待實作 |
| Stage 5 | 更多資料型態（List / Hash） | 🔲 待實作 |
| Stage 6 | 單元測試與整合測試          | 🔲 待實作 |

---

## Stage 1｜TCP Server + Inline 指令

**目標：** 建立最小可運作的 TCP Key-Value Server。

**實作內容：**

- `net.Listen` 建立 TCP listener
- goroutine 處理每個 client 連線
- `sync.RWMutex` 保護共享 map
- `bufio.Scanner` 逐行讀取純文字指令
- 支援 `PING`、`SET`、`GET`

**完成標準：** 可用 `nc localhost 6380` 輸入指令並收到正確回應。

📄 詳見 `docs/stage1.md`

---

## Stage 2｜RESP2 協議 + 模組化

**目標：** 升級為標準 Redis 協議，將程式碼拆分成獨立模組。

**實作內容：**

- `resp` package：解析 RESP2 Array 格式、支援 inline fallback
- `store` package：封裝 thread-safe KV store
- `server` package：封裝 TCP 監聽與指令路由
- `main.go` 精簡為入口點
- 新增 `DEL`、`EXISTS` 指令
- 錯誤回應改為標準 `-ERR` 格式

**完成標準：** 可用 `redis-cli -p 6380` 連線，PING / SET / GET / DEL / EXISTS 行為與真實 Redis 相同。

📄 詳見 `docs/stage2.md`

---

## Stage 3｜TTL / 過期機制

**目標：** 讓 key 可以設定存活時間，時間到自動消失。

**實作內容：**

- `store` 新增 expiry 欄位：`map[string]time.Time`
- `SET key value EX seconds` 語法支援
- `EXPIRE key seconds`：設定既有 key 的過期時間
- `TTL key`：查詢剩餘秒數（`-1` 表示永久，`-2` 表示不存在）
- `PERSIST key`：移除過期時間
- 背景 goroutine 定期清除過期 key（lazy expiration + active expiration）

**學習重點：**

- `time.Time`、`time.Duration` 的使用
- 背景 goroutine 的生命週期管理
- lazy delete vs active cleanup 的取捨

**完成標準：** `SET k v EX 5` 後等 5 秒，`GET k` 回傳 `$-1`。

📄 詳見 `docs/stage3.md`（待建立）

---

## Stage 4｜持久化（AOF）

**目標：** 讓資料在 server 重啟後仍能恢復。

**實作內容：**

- Append-Only File（AOF）格式：每次寫入指令就 append 到 `.aof` 檔案
- 啟動時 replay AOF 恢復資料狀態
- 支援寫入的指令：`SET`、`DEL`、`EXPIRE`
- `BGREWRITEAOF`：壓縮 AOF 檔案（移除冗餘指令）

**學習重點：**

- 檔案 I/O：`os.OpenFile`、`bufio.Writer`
- Write-Ahead Log 概念
- 啟動時的 recovery 流程
- AOF rewrite 的必要性（檔案會無限成長）

**完成標準：** `SET name redis` 後重啟 server，`GET name` 仍回傳 `redis`。

📄 詳見 `docs/stage4.md`（待建立）

---

## Stage 5｜更多資料型態（List / Hash）

**目標：** 讓 store 支援除 String 以外的資料結構。

### List

- `LPUSH key val`、`RPUSH key val`
- `LPOP key`、`RPOP key`
- `LRANGE key start stop`
- `LLEN key`

### Hash

- `HSET key field value`
- `HGET key field`
- `HDEL key field`
- `HGETALL key`

**學習重點：**

- store 從 `map[string]string` 升級為支援多種型態
- Go interface 設計（`type Value interface{}`）
- type assertion 與 type switch
- RESP Array 回應格式（`WriteArray`）

**完成標準：** `LPUSH list a b c` 後 `LRANGE list 0 -1` 回傳正確順序的陣列。

📄 詳見 `docs/stage5.md`（待建立）

---

## Stage 6｜單元測試與整合測試

**目標：** 補上各模組的測試，確保重構安全性與行為正確性。

**實作內容：**

- `resp` package：解析器與序列化的 unit test
- `store` package：Set / Get / Del / Exists / TTL 的 unit test（含並發測試）
- `server` package：整合測試，模擬 TCP 連線送出 RESP 封包並驗證回應
- Table-driven test 風格

**學習重點：**

- Go testing 慣例：`_test.go`、`t.Run`、`t.Fatal`
- Table-driven test 模式
- `net.Pipe()` 模擬 TCP 連線
- `go test -race` 偵測 race condition

**完成標準：** `go test ./... -race` 全部通過，coverage > 80%。

📄 詳見 `docs/stage6.md`（待建立）
