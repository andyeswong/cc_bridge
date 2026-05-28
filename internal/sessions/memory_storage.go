package sessions

import (
	"sync"
	"time"
)

const (
	DefaultTTL      = 2 * time.Hour
	DefaultMaxItems = 1000
)

type Item[T any] struct {
	Value     T
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
}

type MemoryStore[T any] struct {
	mu       sync.RWMutex
	items    map[string]Item[T]
	ttl      time.Duration
	maxItems int
}

func NewMemoryStore[T any](ttl time.Duration, maxItems int) *MemoryStore[T] {
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	if maxItems <= 0 {
		maxItems = DefaultMaxItems
	}

	return &MemoryStore[T]{
		items:    make(map[string]Item[T]),
		ttl:      ttl,
		maxItems: maxItems,
	}
}

func (s *MemoryStore[T]) Get(key string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var zero T

	item, ok := s.items[key]
	if !ok {
		return zero, false
	}

	if time.Now().After(item.ExpiresAt) {
		delete(s.items, key)
		return zero, false
	}

	item.UpdatedAt = time.Now()
	item.ExpiresAt = item.UpdatedAt.Add(s.ttl)
	s.items[key] = item

	return item.Value, true
}

func (s *MemoryStore[T]) Set(key string, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	s.items[key] = Item[T]{
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.evictIfNeededLocked()
}

func (s *MemoryStore[T]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, key)
}

func (s *MemoryStore[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items)
}

func (s *MemoryStore[T]) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, item := range s.items {
		if now.After(item.ExpiresAt) {
			delete(s.items, key)
			removed++
		}
	}

	return removed
}

func (s *MemoryStore[T]) evictIfNeededLocked() {
	for len(s.items) > s.maxItems {
		var oldestKey string
		var oldestTime time.Time
		first := true

		for key, item := range s.items {
			if first || item.UpdatedAt.Before(oldestTime) {
				oldestKey = key
				oldestTime = item.UpdatedAt
				first = false
			}
		}

		if oldestKey == "" {
			return
		}

		delete(s.items, oldestKey)
	}
}
