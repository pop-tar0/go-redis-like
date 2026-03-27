# 階段三文件

## 文件目的

記錄 Stage 3 的實作內容：TTL / 過期機制。以目前 `store/store.go` 與 `server/server.go` 的程式碼為準。

---

## 與 Stage 2 的差異

### 1. `Store` struct 新增 `expiry` 欄位

Stage 2 的 `Store` 只有 `data`，Stage 3 加上 `expiry` 來記錄每個 key 的過期時間：

```go
type Store struct {
    mu     sync.RWMutex
    data   map[string]string
    expiry map[string]time.Time  // 新增
}
```

沒有設定過期時間的 key 不會出現在 `expiry` 中。

### 2. `Set` 加入 `ttl` 參數

```go
// Stage 2
func (s *Store) Set(key, value string)

// Stage 3
func (s *Store) Set(key, value string, ttl time.Duration)
```

- `ttl > 0`：寫入 expiry，設定過期時間
- `ttl == 0`：視為永久，並**清除**舊的 TTL（避免 SET 後還帶著之前的過期時間）

### 3. 兩種過期策略並用

**Lazy expiration（惰性刪除）**

每次 `Get` / `Exists` 時先呼叫 `isExpired()`，過期就當不存在：

```go
func (s *Store) Get(key string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.isExpired(key) {   // 過期 → 當作不存在
        return "", false
    }
    ...
}
```

優點：實作簡單。缺點：沒人讀的過期 key 不會被釋放。

**Active expiration（主動刪除）**

`New()` 啟動背景 goroutine，每秒掃描 `expiry` map 並刪除過期 key：

```go
func (s *Store) activeExpiry() {
    ticker := time.NewTicker(time.Second)
    for range ticker.C {
        s.mu.Lock()
        now := time.Now()
        for key, exp := range s.expiry {
            if now.After(exp) {
                delete(s.data, key)
                delete(s.expiry, key)
            }
        }
        s.mu.Unlock()
    }
}
```

兩種策略並用是 Redis 的真實做法：lazy 保證讀取正確，active 負責釋放記憶體。

### 4. `Del` 補上清理 `expiry`

Stage 2 的 `Del` 只刪 `data`，Stage 3 同時刪 `expiry`，避免殘留：

```go
delete(s.data, key)
delete(s.expiry, key)  // 新增
```

### 5. 新增方法與指令

| Store 方法 | 對應指令 | 說明 |
|---|---|---|
| `Expire(key, ttl)` | `EXPIRE key seconds` | 設定既有 key 的過期時間，key 不存在回傳 false |
| `TTL(key) int64` | `TTL key` | 回傳剩餘秒數，`-1` 永久，`-2` 不存在或已過期 |
| `Persist(key)` | `PERSIST key` | 移除過期時間使其永久，沒有 TTL 則回傳 false |

---

## 支援指令

### `SET key value EX seconds`

```
> SET session abc EX 10
OK
```

### `EXPIRE key seconds`

```
> EXPIRE session 30
(integer) 1    # key 存在，成功設定
(integer) 0    # key 不存在
```

### `TTL key`

```
> TTL session
(integer) 28   # 剩餘秒數
(integer) -1   # 永久（沒有 TTL）
(integer) -2   # key 不存在或已過期
```

### `PERSIST key`

```
> PERSIST session
(integer) 1    # 成功移除 TTL
(integer) 0    # key 沒有 TTL 或不存在
```

---

## 測試方式

```bash
go run main.go
```

```bash
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
