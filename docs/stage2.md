# 階段二文件

## 文件目的

這份文件描述專案在「階段二」完成的所有變更，包含 RESP2 協議實作、程式碼模組化拆分，以及新增指令。以目前 `resp/`、`store/`、`server/`、`main.go` 的實作為準。

## 階段二目標

在階段一的基礎上，完成以下升級：

- 從自訂 inline 文字格式升級為標準 **RESP2 協議**
- 將所有邏輯從 `main.go` 拆分到獨立 package
- 讓 `redis-cli` 可以直接連線使用
- 錯誤回應改為標準 Redis 格式
- 新增 `DEL`、`EXISTS` 指令

## 與 Stage 1 的差異

### 1. 指令解析：inline 文字 → RESP2 Array

Stage 1 用 `bufio.Scanner` 讀一整行，再用 `strings.Fields` 切空格：

```
"SET foo bar\n"  →  ["SET", "foo", "bar"]
```

Stage 2 新增 `resp.Parse()`，能解析 RESP Array 格式：

```
*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n  →  ["SET", "foo", "bar"]
```

解析流程：

1. 讀第一個 byte，若是 `*` → 走 RESP Array 路徑
2. 讀取元素數量（`*3` → 3 個）
3. 依序讀 `count` 個 Bulk String，每個格式為 `$長度\r\n內容\r\n`
4. 回傳 `[]string`

inline fallback 仍保留（把第一個 byte 退回去讀整行），所以 `nc` 仍可使用。

### 2. 回應格式：純文字 → 標準 RESP

| 情境       | Stage 1        | Stage 2           |
| ---------- | -------------- | ----------------- |
| 成功       | `OK\n`         | `+OK\r\n`         |
| 錯誤       | `Error: ...\n` | `-ERR ...\r\n`    |
| 數字       | `1\n`          | `:1\r\n`          |
| 字串值     | `hello\n`      | `$5\r\nhello\r\n` |
| key 不存在 | `nil\n`        | `$-1\r\n`         |

### 3. 程式架構：單檔 → 模組化

Stage 1 全部邏輯在 `main.go`（全域變數 `db`、`dbLock`）。

Stage 2 拆成三個 package：

- `resp/` — 只負責 RESP 協議的解析與序列化
- `store/` — 只負責 thread-safe KV 儲存
- `server/` — 只負責 TCP 連線與指令路由

### 4. 新增指令

- `DEL key [key ...]` — 支援一次刪多個，回傳刪除數量
- `EXISTS key [key ...]` — 支援一次查多個，回傳存在數量

### 5. 測試工具升級

Stage 1：只能用 `nc`（純文字 inline 格式）
Stage 2：可用 `redis-cli`（標準 RESP 格式），nc 仍可用

`redis-cli` 的角色是**轉換層**，不是直接把你打的文字送給 server：

```
你輸入：        SET name taro
      ↓
redis-cli 轉換成 RESP：
      *3\r\n
      $3\r\nSET\r\n
      $4\r\nname\r\n
      $4\r\ntaro\r\n
      ↓
你的 server：   resp.Parse()  →  ["SET", "name", "taro"]
```

所以你在 `redis-cli` 打 `SET name taro`，你的 server 收到的永遠是 RESP Array 格式，不是原始文字。`redis-cli` 把「人類可讀的指令」和「機器傳輸的格式」這兩層完全分開。

---

## 什麼是 RESP2

RESP（Redis Serialization Protocol）是 Redis 客戶端與伺服器之間的通訊協議，版本 2 定義了五種資料型態：

| 型態          | 前綴 | 範例                               |
| ------------- | ---- | ---------------------------------- |
| Simple String | `+`  | `+OK\r\n`                          |
| Error         | `-`  | `-ERR unknown command\r\n`         |
| Integer       | `:`  | `:3\r\n`                           |
| Bulk String   | `$`  | `$5\r\nhello\r\n`                  |
| Array         | `*`  | `*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n` |

當 `redis-cli` 送出 `SET foo bar` 時，實際傳送的是：

```
*3\r\n
$3\r\n
SET\r\n
$3\r\n
foo\r\n
$3\r\n
bar\r\n
```

### Null Bulk String

當 GET 找不到 key 時，回傳：

```
$-1\r\n
```

這是 RESP2 表示「不存在」的標準方式，等同於 Redis 的 `(nil)`。

## 新增模組結構

```text
.
├── main.go          ← 只負責初始化與啟動
├── go.mod
├── resp/
│   └── resp.go      ← RESP2 解析器與序列化 helper
├── server/
│   └── server.go    ← TCP server + 指令路由
└── store/
    └── store.go     ← thread-safe KV store
```

## `resp` Package

### 解析器：`Parse(reader *bufio.Reader) ([]string, error)`

支援兩種輸入格式：

**1. RESP Array（redis-cli 使用）**

偷看第一個 byte，若為 `*` 則走 RESP Array 解析路徑：

1. 讀取元素數量 `*n`
2. 對每個元素讀取 Bulk String（`$len\r\ndata\r\n`）
3. 回傳 `[]string`，第一個元素為指令名稱

**2. Inline Command（nc / telnet 使用）**

若第一個 byte 不是 `*`，退回去讀整行，再用 `strings.Fields` 切分。這讓你仍然可以用 `nc localhost 6380` 直接輸入純文字測試。

### 序列化 Helper

```go
WriteSimpleString(w, "OK")      // +OK\r\n
WriteError(w, "unknown command") // -ERR unknown command\r\n
WriteInteger(w, 3)               // :3\r\n
WriteBulkString(w, "hello")      // $5\r\nhello\r\n
WriteNullBulk(w)                 // $-1\r\n
```

## `store` Package

### `Store` struct

```go
type Store struct {
    mu   sync.RWMutex
    data map[string]string
}
```

### 方法

| 方法                             | 說明                         | 鎖類型 |
| -------------------------------- | ---------------------------- | ------ |
| `Set(key, value string)`         | 寫入 key-value               | 寫鎖   |
| `Get(key string) (string, bool)` | 讀取，不存在回傳 `"", false` | 讀鎖   |
| `Del(key string) bool`           | 刪除，回傳是否真的刪了       | 寫鎖   |
| `Exists(key string) bool`        | 檢查是否存在                 | 讀鎖   |

與階段一相比，`db` 和 `dbLock` 從全域變數封裝為 struct，可以測試、可以多實例。

## `server` Package

### `Server` struct

```go
type Server struct {
    addr  string
    store *store.Store
}
```

### 指令路由

每個連線由 `handleConn` 在獨立 goroutine 內處理：

```
resp.Parse(reader)
    ↓
strings.ToUpper(args[0])
    ↓
switch cmd { PING / SET / GET / DEL / EXISTS / default }
    ↓
resp.Write*(conn, ...)
```

### 錯誤處理

- 參數不足：`-ERR wrong number of arguments for 'cmd'`
- 未知指令：`-ERR unknown command 'cmd'`
- 連線中斷：靜默退出 goroutine

## 階段二支援指令

### `PING`

```
輸入：  PING
回應：  +PONG
```

### `SET key value`

```
輸入：  SET user:1 alice
回應：  +OK
```

### `GET key`

```
輸入（找到）：  GET user:1
回應：          $5\r\nalice

輸入（找不到）：GET not_exist
回應：          $-1
```

### `DEL key [key ...]`

支援一次刪除多個 key，回傳實際刪除的數量。

```
輸入：  DEL user:1 user:2
回應：  :2
```

### `EXISTS key [key ...]`

支援一次查詢多個 key，回傳存在的數量。

```
輸入：  EXISTS user:1 not_exist
回應：  :1
```

## 與 redis-cli 的相容性

```bash
redis-cli -p 6380
```

```
127.0.0.1:6380> PING
PONG
127.0.0.1:6380> SET name taro
OK
127.0.0.1:6380> GET name
"taro"
127.0.0.1:6380> DEL name
(integer) 1
127.0.0.1:6380> GET name
(nil)
127.0.0.1:6380> EXISTS name
(integer) 0
```

## 測試方式

### 啟動服務

```bash
go run main.go
```

### 使用 redis-cli 測試

```bash
redis-cli -p 6380
```

### 使用 nc 測試（inline 格式仍可用）

```bash
printf "PING\r\nSET k v\r\nGET k\r\n" | nc localhost 6380
```

預期回應：

```
+PONG
+OK
$1
v
```

### 使用 nc 測試（RESP 格式）

```bash
printf "*1\r\n\$4\r\nPING\r\n" | nc localhost 6380
```

預期回應：

```
+PONG
```

## 與階段一的差異

| 項目           | 階段一           | 階段二                             |
| -------------- | ---------------- | ---------------------------------- |
| 協議格式       | 純文字 inline    | RESP2（相容 inline）               |
| 程式架構       | 全部在 `main.go` | 拆分為 `resp` / `store` / `server` |
| 錯誤格式       | 中文自訂字串     | 標準 `-ERR` 格式                   |
| 支援指令       | PING / SET / GET | + DEL / EXISTS                     |
| redis-cli 相容 | ❌               | ✅                                 |
| value 含空白   | ❌ 不支援        | ✅ 支援（RESP Bulk String）        |

## 階段二範圍界線

以下項目不在階段二範圍內：

- TTL / 過期時間
- 持久化
- List、Hash 等複合資料型態
- Array 回應格式（`MGET`、`KEYS` 等）
- 單元測試

## 下一步

階段三預計實作 **TTL / 過期機制**，讓 key 可以設定存活時間。

📄 詳見 `docs/stage3.md`（待建立）｜`docs/roadmap.md`
