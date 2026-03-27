# 階段四文件

## 文件目的

記錄 Stage 4 的實作內容：AOF 持久化機制。以目前 `aof/aof.go`、`server/server.go`、`main.go` 的程式碼為準。

---

## 核心概念：AOF（Append-Only File）

每次執行寫入指令，就把這筆指令以 RESP 格式 **append** 到 `redis.aof` 檔案。
Server 重啟時，把檔案裡的指令從頭到尾重新跑一遍，資料就恢復了。

```
SET name taro
    ↓ 寫入 redis.aof
*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\ntaro\r\n

重啟時 replay：
讀取 redis.aof → 依序執行每筆指令 → store 恢復到關機前狀態
```

---

## 與 Stage 3 的差異

### 1. 新增 `aof/aof.go`

```go
type AOF struct {
    mu     sync.Mutex
    file   *os.File
    writer *bufio.Writer
}
```

| 方法                   | 說明                                                    |
| ---------------------- | ------------------------------------------------------- |
| `New(path)`            | 開啟或建立 AOF 檔案（`O_CREATE`, `O_RDWR`, `O_APPEND`） |
| `Write(args []string)` | 把指令轉成 RESP Array 格式 append 進檔案                |
| `Replay(handler)`      | 從頭讀取檔案，每筆指令解析後呼叫 handler                |
| `Close()`              | 關閉檔案                                                |

### 2. `server.go` 在寫入指令後呼叫 `aof.Write()`

| 指令      | 觸發條件               |
| --------- | ---------------------- |
| `SET`     | 永遠寫入（含 EX 參數） |
| `DEL`     | 有實際刪除才寫入       |
| `EXPIRE`  | key 存在才寫入         |
| `PERSIST` | key 有 TTL 才寫入      |

`GET`、`EXISTS`、`TTL`、`PING` 是唯讀指令，不寫入 AOF。

### 3. `main.go` 啟動時 replay AOF

```
啟動流程：
1. store.New()
2. aof.New("redis.aof")
3. aof.Replay() → 把每筆指令餵給 store 執行
4. server.New(..., aof) → 開始接受連線
```

---

## AOF 格式說明

寫入的格式就是標準 RESP Array，和 redis-cli 傳給 server 的格式完全一樣：

```
SET name taro      →   *3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\ntaro\r\n
SET k v EX 10      →   *5\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n$2\r\nEX\r\n$2\r\n10\r\n
DEL name           →   *2\r\n$3\r\nDEL\r\n$4\r\nname\r\n
EXPIRE k 30        →   *3\r\n$6\r\nEXPIRE\r\n$1\r\nk\r\n$2\r\n30\r\n
```

replay 時直接用 `resp.Parse()` 解析，不需要額外的解析邏輯。

---

## 測試方式

```bash
go run main.go
```

```bash
redis-cli -p 6380
> SET name taro
> SET session abc EX 60
```

Ctrl+C 停掉 server，再重啟：

```bash
go run main.go
```

```bash
redis-cli -p 6380
> GET name      # "taro" ← 資料恢復了
> GET session   # 60 秒內仍存在
```

---

## 目前限制

- AOF 檔案只會無限增長，不會自動壓縮（BGREWRITEAOF 留待後續）
- **TTL 重啟後會重置**：AOF 儲存的是原始指令，例如 `SET k v EX 60`。若跑了 30 秒後重啟，replay 會再執行一次 `SET k v EX 60`，TTL 從 60 重新計算，而不是剩下的 30 秒。真正的 Redis 解法是儲存**絕對過期時間戳**，而非相對秒數。
