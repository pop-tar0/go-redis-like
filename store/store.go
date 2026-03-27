package store

import (
	"sync"
	"time"
)

/**
 * Store 是一個簡單的線程安全的 key-value 儲存結構，使用 sync.RWMutex 來保護對內部 map 的讀寫操作。
 * 它提供了 Set、Get、Del 和 Exists 四個方法來操作資料。
 */
type Store struct {
	// mu 是一個讀寫鎖，用於保護對 data map 的訪問，確保線程安全。
	mu sync.RWMutex

	// data 是一個 map，用於存儲 key-value 對，其中 key 和 value 都是字符串。
	data map[string]string

	// expiry 紀錄每個 key 的過期時間，沒有設定過期時間的 key 不會出現在此 map 中。
	expiry map[string]time.Time
}

/**
 * New 函式用於創建一個新的 Store 實例，初始化內部的 map。
 */
func New() *Store {
	s := &Store{
		data:   make(map[string]string),
		expiry: make(map[string]time.Time),
	}
	go s.activeExpiry()
	return s
}

/**
 * isExpired 檢查指定的 key 是否已經過期，如果 key 不存在或沒有設定過期時間則回傳 false。
 */
func (s *Store) isExpired(key string) bool {
	exp, ok := s.expiry[key]
	return ok && time.Now().After(exp)
}

/**
 * Set 設定指定 key 的值，使用寫鎖來確保線程安全。
 */
func (s *Store) Set(key, value string, ttl time.Duration) {
	// 使用寫鎖來保護對 data map 的寫操作，確保在多線程環境下不會發生競爭條件。
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	if ttl > 0 {
		s.expiry[key] = time.Now().Add(ttl)
	} else {
		delete(s.expiry, key)
	}
}

/**
 * Get 取得指定 key 的值，若不存在則回傳 ("", false)
 */
func (s *Store) Get(key string) (string, bool) {
	// 使用讀鎖來保護對 data map 的讀操作，確保在多線程環境下不會發生競爭條件。
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.isExpired(key) {
		return "", false
	}
	val, ok := s.data[key]
	return val, ok
}

/**
 * Del 刪除指定 key，回傳是否成功刪除（即 key 是否存在）。
 */
func (s *Store) Del(key string) bool {
	// 使用寫鎖來保護對 data map 的寫操作，確保在多線程環境下不會發生競爭條件。
	s.mu.Lock()
	defer s.mu.Unlock()
	// 嘗試從 data map 中刪除指定的 key，如果 key 存在則刪除並回傳 true，否則回傳 false。
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
		delete(s.expiry, key)
	}
	return ok
}

/**
 * Exists 回傳指定 key 是否存在
 */
func (s *Store) Exists(key string) bool {
	// 使用讀鎖來保護對 data map 的讀操作，確保在多線程環境下不會發生競爭條件。
	s.mu.RLock()
	defer s.mu.RUnlock()
	// 首先檢查 key 是否已經過期，如果過期則視為不存在，回傳 false。
	if s.isExpired(key) {
		return false
	}
	// 嘗試從 data map 中查找指定的 key，如果 key 存在則回傳 true，否則回傳 false。
	_, ok := s.data[key]
	return ok
}

/**
 * Expire 設定 key 的過期時間，回傳是否成功設定（即 key 是否存在）。
 */
func (s *Store) Expire(key string, ttl time.Duration) bool {
	// 使用寫鎖來保護對 data map 的寫操作，確保在多線程環境下不會發生競爭條件。
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return false
	}
	// 設定 key 的過期時間為當前時間加上 ttl，這樣在 activeExpiry 背景 goroutine 中就會定期檢查並刪除過期的 key。
	s.expiry[key] = time.Now().Add(ttl)
	return true
}

/**
 * TTL 回傳 key 的剩餘存活時間（以秒為單位），如果 key 不存在則回傳 -2，如果 key 存在但沒有設定過期時間則回傳 -1。
 */
func (s *Store) TTL(key string) int64 {
	// 使用讀鎖來保護對 data map 的讀操作，確保在多線程環境下不會發生競爭條件。
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data[key]; !ok {
		return -2
	}
	// 嘗試從 expiry map 中查找指定 key 的過期時間，如果 key 沒有設定過期時間則回傳 -1。
	exp, ok := s.expiry[key]
	if !ok {
		return -1
	}
	// 計算 key 的剩餘存活時間，如果已經過期則回傳 -2，否則回傳剩餘時間的秒數。
	remaining := time.Until(exp)
	if remaining <= 0 {
		return -2
	}
	return int64(remaining.Seconds())
}

/**
 * Persist 移除 key 的過期時間，使其永久存在，回傳是否有移除
 */
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.expiry[key]; !ok {
		return false
	}
	delete(s.expiry, key)
	return true
}

/**
 * activeExpiry 是一個背景 goroutine，定期檢查並刪除已經過期的 key。它使用 time.NewTicker 每秒觸發一次，遍歷 expiry map 中的 key，如果發現有過期的 key 就從 data map 和 expiry map 中刪除。
 */
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

// SnapshotEntry 代表一筆快照資料。
type SnapshotEntry struct {
	Key    string
	Value  string
	Expiry time.Time // 零值表示永久
}

/**
 * Snapshot 回傳目前所有未過期的 key-value 資料（含過期時間），供 AOF rewrite 使用。
 */
func (s *Store) Snapshot() []SnapshotEntry {
	// 使用讀鎖來保護對 data map 的讀操作，確保在多線程環境下不會發生競爭條件。
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	entries := make([]SnapshotEntry, 0, len(s.data))
	// 遍歷 data map 中的所有 key-value 對，檢查每個 key 是否已經過期，如果過期則跳過，否則將 key、value 和過期時間（如果有）封裝成 SnapshotEntry，加入 entries 切片中，最後回傳這個切片。
	for key, val := range s.data {
		exp, hasExp := s.expiry[key]
		if hasExp && now.After(exp) {
			continue // 跳過已過期的
		}

		// 將 key、value 和過期時間（如果有）封裝成 SnapshotEntry，加入 entries 切片中，最後回傳這個切片。
		entries = append(entries, SnapshotEntry{
			Key:    key,
			Value:  val,
			Expiry: exp,
		})
	}
	return entries
}
