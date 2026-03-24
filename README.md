# Handcrafted Redis Go

這是一個使用 Go 從零開始實作的簡化版 Redis-like TCP Key-Value Server，目標是透過實作理解 Redis 最核心的幾個概念：

- TCP 連線與 client handling
- goroutine 併發模型
- 記憶體中的 key-value 儲存
- `sync.RWMutex` 的讀寫鎖保護
- 類 Redis 指令回應格式

目前專案聚焦在學習用途，因此實作刻意保持簡單、清楚，方便閱讀與延伸。

## 專案現況

目前伺服器會監聽本機 `6380` port，支援最基本的：

- `PING`
- `SET key value`
- `GET key`

資料儲存在記憶體中，程式結束後不會保留。

## 核心特色

- 使用 Go 原生 `net` 套件建立 TCP server
- 每個 client connection 以 goroutine 獨立處理
- 透過 `map[string]string` 當作簡易資料庫
- 使用 `sync.RWMutex` 避免多連線同時讀寫造成 race condition
- 使用 `bufio.Scanner` 逐行讀取指令，適合用 `nc` / `telnet` 測試

## 專案結構

```text
.
├── main.go
├── README.md
├── go.mod
├── server/
│   └── server.go
└── store/
    └── store.go
```

目前主要邏輯集中在 `main.go`，`server/` 與 `store/` 目錄可作為後續模組化重構的延伸位置。

## 執行方式

確認已安裝 Go，然後在專案根目錄執行：

```bash
go run main.go
```

成功啟動後，終端機會看到類似：

```text
✅ 伺服器啟動成功，正在監聽 6380 端口...
```

## 連線測試

你可以用 `nc` 連進伺服器：

```bash
nc localhost 6380
```

接著輸入以下指令：

```text
PING
SET user:1 taro
GET user:1
GET not_found
```

預期回應：

```text
+PONG
+OK
$10
taro
$-1
```

也可以直接用 pipe 測試：

```bash
printf "PING\r\nSET name redis\r\nGET name\r\n" | nc localhost 6380
```

## 支援指令

| 指令   | 說明         | 範例               | 回應          |
| :----- | :----------- | :----------------- | :------------ |
| `PING` | 測試連線狀態 | `PING`             | `+PONG`       |
| `SET`  | 儲存鍵值對   | `SET user:1 alice` | `+OK`         |
| `GET`  | 取得指定鍵值 | `GET user:1`       | `$5\r\nalice` |

## 實作重點

### 1. TCP 監聽

伺服器使用：

```go
net.Listen("tcp", ":6380")
```

來建立 TCP listener，持續接受新的 client 連線。

### 2. 併發處理

每當有新連線建立時，會啟動一個 goroutine：

```go
go handleConnection(conn)
```

這讓伺服器可以同時處理多個 client。

### 3. 記憶體資料儲存

資料使用以下結構保存：

```go
var (
    db = make(map[string]string)
    dbLock sync.RWMutex
)
```

- `SET` 時加寫鎖
- `GET` 時加讀鎖

這是目前這個版本最重要的 thread-safety 保護機制。

### 4. 指令解析

目前採用簡單的 inline command 格式，也就是每行一個指令，例如：

```text
SET mykey hello
GET mykey
```

這和 Redis 正式使用的 RESP 協議不同，但很適合作為第一步練習。

## 目前限制

- 只支援單行文字指令，不支援完整 RESP protocol
- `SET` 的 value 目前不能包含空白
- 沒有資料持久化
- 沒有過期時間（TTL）
- 沒有刪除、列表、交易等進階功能
- 錯誤訊息格式尚未完全比照 Redis

## 後續可擴充方向

- 支援 RESP protocol
- 加入 `DEL`、`EXISTS`、`INCR` 等指令
- 實作 TTL / expiration
- 將 server 與 store 邏輯拆分成獨立 package
- 補上單元測試與整合測試
- 支援資料持久化（RDB / AOF 概念練習）

## 學習目標

這個專案很適合拿來練習：

- Go 網路程式設計
- goroutine 與 mutex 的使用時機
- in-memory database 的基本設計
- Redis server 的核心概念

如果你想把它繼續往真正可擴充的 mini Redis 發展，下一步很適合先做：

1. 模組化 `server` / `store`
2. 補上測試
3. 導入 RESP protocol
