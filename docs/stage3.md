# 階段三文件

## 文件目的

這份文件描述專案在「階段三」完成的所有變更，包含 TTL / 過期機制的實作。

## 階段三目標

在階段二的基礎上，讓 key 可以設定存活時間，時間到自動消失：

- `SET key value EX seconds` 支援
- `EXPIRE key seconds`：設定既有 key 的過期時間
- `TTL key`：查詢剩餘秒數
- `PERSIST key`：移除過期時間
- Lazy expiration：讀取時順便檢查是否過期
- Active expiration：背景 goroutine 每秒主動清除

## 與 Stage 2 的差異

### 1. `Store` 新增 expiry map

```go
type Store struct {
    mu     sync.RWMutex
    data   map[string]string
    expiry map[string]time.Time  // 新增
}
```

### 2. 兩種過期清除策略

**Lazy expiration（惰性刪除）**

每次 `Get` 或 `Exists` 時，先檢查 key 是否過期，若過期直接回傳不存在：

```go
func (s *Store) Get(key string) (string, bool) {
    // 過期 → 當作不存在
    if s.isExpired(key) {
        return "", false
    }
    ...
}
```

優點：實作簡單，不用主動掃描。  
缺點：已過期但沒人讀的 key 會一直佔記憶體。

**Active expiration（主動刪除）**

`New()` 啟動一個背景 goroutine，每秒掃描 `expiry` map，刪掉過期的 key：

```go
func (s *Store) activeExpiry() {
    ticker := time.NewTicker(time.Second)
    for range ticker.C {
        // 掃描並刪除過期 key
    }
}
```

優點：記憶體能被主動釋放。  
兩者並用是 Redis 的真實策略。

### 3. `Set` 加入 TTL 參數

```go
func (s *Store) Set(key, value string, ttl time.Duration)
```

`ttl > 0` 時設定過期時間，`ttl == 0` 視為永久（並清除舊的 TTL）。

### 4. 新增方法

| 方法               | 說明                                          |
| ------------------ | --------------------------------------------- |
| `Expire(key, ttl)` | 設定既有 key 的過期時間，key 不存在回傳 false |
| `TTL(key)`         | 回傳剩餘秒數，`-1` 永久，`-2` 不存在或已過期  |
| `Persist(key)`     | 移除過期時間，使 key 永久存在                 |

### 5. `Del` 同時清理 expiry

Stage 2 的 `Del` 只刪 `data`，Stage 3 補上 `delete(s.expiry, key)`，避免殘留資料。

## 階段三支援指令

### `SET key value EX seconds`

```
SET session abc EX 10
→ OK
```

### `EXPIRE key seconds`

```
EXPIRE session 30
→ (integer) 1   # 成功
→ (integer) 0   # key 不存在
```

### `TTL key`

```
TTL session
→ (integer) 28   # 剩餘秒數
→ (integer) -1   # 永久（沒有 TTL）
→ (integer) -2   # key 不存在或已過期
```

### `PERSIST key`

```
PERSIST session
→ (integer) 1   # 成功移除 TTL
→ (integer) 0   # key 沒有 TTL 或不存在
```

## 測試方式

```bash
# 啟動 server
go run main.go

# 開另一個 terminal
redis-cli -p 6380
```

```
127.0.0.1:6380> SET k v EX 5
OK
127.0.0.1:6380> TTL k
(integer) 4
127.0.0.1:6380> GET k
"v"
# 等 5 秒後
127.0.0.1:6380> GET k
(nil)
127.0.0.1:6380> TTL k
(integer) -2
```
