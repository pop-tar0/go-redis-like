# Handcrafted Redis Go

使用 Go 從零開始實作的簡化版 Redis-like TCP Key-Value Server，透過實作理解 Redis 最核心的幾個概念。

## 執行方式

```bash
go run main.go
```

```bash
redis-cli -p 6380
```

## 專案結構

```text
.
├── main.go
├── go.mod
├── resp/
│   └── resp.go      ← RESP2 解析器與序列化
├── server/
│   └── server.go    ← TCP server + 指令路由
├── store/
│   └── store.go     ← thread-safe KV store
└── docs/
    ├── roadmap.md
    ├── stage1.md
    ├── stage2.md
    └── stage3.md
```

## 支援指令

| 指令                       | 說明               | 範例                  |
| -------------------------- | ------------------ | --------------------- |
| `PING`                     | 測試連線           | `PING`                |
| `SET key value`            | 寫入               | `SET name taro`       |
| `SET key value EX seconds` | 寫入並設定 TTL     | `SET name taro EX 10` |
| `GET key`                  | 讀取               | `GET name`            |
| `DEL key [key ...]`        | 刪除，回傳刪除數量 | `DEL name`            |
| `EXISTS key [key ...]`     | 查存在數量         | `EXISTS name`         |
| `EXPIRE key seconds`       | 設定過期時間       | `EXPIRE name 30`      |
| `TTL key`                  | 查剩餘秒數         | `TTL name`            |
| `PERSIST key`              | 移除過期時間       | `PERSIST name`        |

## Roadmap

| Stage   | 主題                        | 狀態      |
| ------- | --------------------------- | --------- |
| Stage 1 | TCP Server + Inline 指令    | ✅ 已完成 |
| Stage 2 | RESP2 協議 + 模組化         | ✅ 已完成 |
| Stage 3 | TTL / 過期機制              | ✅ 已完成 |
| Stage 4 | 持久化（AOF）               | ✅ 已完成 |
| Stage 5 | 更多資料型態（List / Hash） | 🔲 待實作 |
| Stage 6 | 單元測試與整合測試          | 🔲 待實作 |

### Stage 1｜TCP Server + Inline 指令

- `net.Listen` 建立 TCP listener
- goroutine 處理每個 client 連線
- `sync.RWMutex` 保護共享 map
- 支援 `PING`、`SET`、`GET`

📄 詳見 `docs/stage1.md`

### Stage 2｜RESP2 協議 + 模組化

- `resp` package：解析 RESP2 Array 格式、支援 inline fallback
- `store` / `server` package 拆分
- 錯誤回應改為標準 `-ERR` 格式
- 新增 `DEL`、`EXISTS`

📄 詳見 `docs/stage2.md`

### Stage 3｜TTL / 過期機制

- `store` 新增 `expiry map[string]time.Time`
- `SET key value EX seconds` 語法
- `EXPIRE`、`TTL`、`PERSIST` 指令
- Lazy expiration + Active expiration 雙策略

📄 詳見 `docs/stage3.md`

### Stage 4｜持久化（AOF）

- Append-Only File：每次寫入指令 append 到 `.aof`
- 啟動時 replay AOF 恢復資料
- `BGREWRITEAOF` 壓縮 AOF

### Stage 5｜更多資料型態（List / Hash）

- `LPUSH`、`RPUSH`、`LPOP`、`RPOP`、`LRANGE`
- `HSET`、`HGET`、`HDEL`、`HGETALL`
- store 升級為 `map[string]interface{}`

### Stage 6｜單元測試與整合測試

- `resp` / `store` / `server` 各模組 unit test
- `net.Pipe()` 模擬 TCP 整合測試
- `go test ./... -race` 全部通過
