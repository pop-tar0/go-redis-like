# 階段一文件

## 文件目的

這份文件用來描述專案在「階段一」已完成的功能與目前程式碼狀態。內容以目前 `main.go` 的實作為準，適合作為後續重構、拆模組與擴功能前的基準文件。

## 階段一目標

階段一的核心目標是先完成一個最小可運作的 Redis-like TCP Server，具備以下能力：

- 可以啟動 TCP server
- 可以接受多個 client 連線
- 可以解析簡單的文字指令
- 可以在記憶體中儲存與讀取 key-value
- 可以用基本的鎖機制保護共享資料

## 目前已完成功能

### 1. TCP Server 啟動

伺服器透過 Go 原生 `net.Listen("tcp", ":6380")` 啟動，監聽本機 `6380` port。

這代表目前可以透過：

```bash
go run main.go
```

啟動服務，並使用 `nc` 或其他 TCP client 進行連線測試。

### 2. Client 連線處理

Server 會持續呼叫 `Accept()` 等待 client 連入。每當有新連線建立，就會開一個新的 goroutine 呼叫 `handleConnection(conn)`。

這表示階段一已具備：

- 基本多連線處理能力
- I/O 與 connection handling 的 goroutine 化

### 3. 記憶體 Key-Value Store

目前資料儲存在記憶體中的：

```go
map[string]string
```

結構內，作為最簡化版本的 in-memory database。

特性如下：

- key 型別為字串
- value 型別為字串
- 資料只存在程式執行期間
- server 重啟後資料會消失

### 4. 併發安全

為了避免多個 goroutine 同時讀寫 map 造成 race condition，目前使用：

```go
sync.RWMutex
```

保護共享資料。

目前策略如下：

- `SET` 使用寫鎖 `Lock() / Unlock()`
- `GET` 使用讀鎖 `RLock() / RUnlock()`

這是階段一最重要的 thread-safety 基礎。

### 5. 指令解析

目前採用簡單的 inline command 格式，也就是 client 每送一行文字，就視為一個完整命令，例如：

```text
PING
SET name redis
GET name
```

伺服器會使用：

- `bufio.Scanner`
- `strings.Fields`
- `strings.ToUpper`

來完成逐行讀取、參數切分與大小寫無關的命令辨識。

## 階段一支援指令

### `PING`

用途：確認 server 是否可正常回應。

輸入：

```text
PING
```

輸出：

```text
+PONG
```

### `SET key value`

用途：將資料寫入記憶體 store。

輸入：

```text
SET user:1 alice
```

輸出：

```text
+OK
```

說明：

- 目前只會讀取第三個欄位作為 value
- value 不支援空白

### `GET key`

用途：查詢指定 key 的值。

輸入：

```text
GET user:1
```

若 key 存在，輸出類似：

```text
$5
alice
```

若 key 不存在，輸出：

```text
$-1
```

## 指令錯誤處理

階段一已具備基本錯誤處理：

- `SET` 參數不足時，回傳格式錯誤訊息
- `GET` 參數不足時，回傳格式錯誤訊息
- 未知命令時，回傳未知命令訊息

目前錯誤訊息為自訂中文字串，尚未完全比照 Redis 正式錯誤格式。

## 執行流程

目前程式執行流程如下：

1. 啟動 TCP listener，監聽 `:6380`
2. 進入無限迴圈，持續接受 client connection
3. 每個連線開一個 goroutine 處理
4. 在 goroutine 內用 `Scanner` 逐行讀取指令
5. 將輸入拆成 command 與參數
6. 依照 `PING`、`SET`、`GET` 分支執行對應邏輯
7. 將結果寫回 client

## 測試方式

### 啟動服務

```bash
go run main.go
```

### 使用 nc 測試

```bash
nc localhost 6380
```

範例操作：

```text
PING
SET name redis
GET name
GET not_found
```

預期結果：

```text
+PONG
+OK
$5
redis
$-1
```

## 階段一範圍界線

以下項目不在階段一範圍內，尚未實作：

- RESP protocol
- TTL / expiration
- 持久化
- `DEL`、`EXISTS`、`INCR` 等其他命令
- 模組化架構拆分
- 單元測試與整合測試
- 更完整的錯誤碼與 Redis 相容行為

## 已知限制

- `SET` 的 value 不能包含空白
- 目前只支援單行文字命令
- 所有資料都放在單一全域 map 中
- 所有邏輯集中在 `main.go`
- `server/` 與 `store/` 目錄目前尚未承載實際邏輯

## 階段一成果總結

階段一已經完成一個可實際連線、可儲存資料、可查詢資料的最小可行版本。雖然功能仍很精簡，但已經建立起後續擴充所需的幾個核心基礎：

- TCP server lifecycle
- goroutine connection handling
- in-memory store
- lock-based concurrency control
- command parsing 基本流程

這份階段一文件可以作為後續「階段二：模組拆分與協議升級」的起點。
