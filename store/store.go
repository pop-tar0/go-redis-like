package store

import "sync"

/**
 * Store 是一個簡單的線程安全的 key-value 儲存結構，使用 sync.RWMutex 來保護對內部 map 的讀寫操作。
 * 它提供了 Set、Get、Del 和 Exists 四個方法來操作資料。
 */
type Store struct {
	// mu 是一個讀寫鎖，用於保護對 data map 的訪問，確保線程安全。
	mu sync.RWMutex
	// data 是一個 map，用於存儲 key-value 對，其中 key 和 value 都是字符串。
	data map[string]string
}

/**
 * New 函式用於創建一個新的 Store 實例，初始化內部的 map。
 */
func New() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

/**
 * Set 設定指定 key 的值，使用寫鎖來確保線程安全。
 */
func (s *Store) Set(key, value string) {
	// 使用寫鎖來保護對 data map 的寫操作，確保在多線程環境下不會發生競爭條件。
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

/**
 * Get 取得指定 key 的值，若不存在則回傳 ("", false)
 */
func (s *Store) Get(key string) (string, bool) {
	// 使用讀鎖來保護對 data map 的讀操作，確保在多線程環境下不會發生競爭條件。
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	// 嘗試從 data map 中查找指定的 key，如果 key 存在則回傳 true，否則回傳 false。
	_, ok := s.data[key]
	return ok
}
