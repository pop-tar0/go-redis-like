package store

import "sync"

// Store 是一個執行緒安全的記憶體 key-value 資料庫
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// New 建立並回傳一個新的 Store 實例
func New() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Set 儲存一個 key-value 對
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get 取得指定 key 的值，若不存在則回傳 ("", false)
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Del 刪除指定 key，回傳是否實際刪除了某個值
func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
	}
	return ok
}

// Exists 回傳指定 key 是否存在
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[key]
	return ok
}
