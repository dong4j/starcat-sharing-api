// Package store 提供内存数据存储功能
package store

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"os"
	"sync"

	"github.com/dong4j/starcat-sharing-api/internal/model"
)

// MemoryStore 基于内存 + JSON 文件的并发安全存储
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]model.ShareData
	file string
}

// NewMemoryStore 创建新的内存存储实例
func NewMemoryStore(filename string) (*MemoryStore, error) {
	s := &MemoryStore{
		data: make(map[string]model.ShareData),
		file: filename,
	}
	// 尝试从文件加载已有数据
	if b, err := os.ReadFile(filename); err == nil {
		json.Unmarshal(b, &s.data)
	}
	return s, nil
}

// Save 将数据持久化到 JSON 文件
func (s *MemoryStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0644)
}

// SaveAsync 异步持久化（避免阻塞请求）
func (s *MemoryStore) SaveAsync() {
	go func() {
		if err := s.Save(); err != nil {
			panic("failed to save store: " + err.Error())
		}
	}()
}

// Set 添加或更新分享数据
func (s *MemoryStore) Set(data model.ShareData) {
	s.mu.Lock()
	s.data[data.ID] = data
	s.mu.Unlock()
}

// Get 根据 ID 获取分享数据
func (s *MemoryStore) Get(id string) (model.ShareData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.data[id]
	return data, ok
}

// generateID 生成唯一的分享 ID
func generateID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// NewID 生成一个新的唯一 ID
func NewID(length int) string {
	return generateID(length)
}
