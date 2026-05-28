package sessions

import (
	"testing"
	"time"
)

func TestMemoryStore_SetGetDelete(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore[int](time.Second, 10)

	store.Set("a", 1)

	got, ok := store.Get("a")
	if !ok || got != 1 {
		t.Fatalf("get: ok=%v got=%v", ok, got)
	}

	store.Delete("a")

	_, ok = store.Get("a")
	if ok {
		t.Fatalf("expected deleted key to be missing")
	}
}

func TestMemoryStore_ExpirationCleanup(t *testing.T) {
	store := NewMemoryStore[string](20*time.Millisecond, 10)

	store.Set("a", "x")
	time.Sleep(35 * time.Millisecond)

	if _, ok := store.Get("a"); ok {
		t.Fatalf("expected expired key to be missing")
	}

	store.Set("b", "y")
	time.Sleep(35 * time.Millisecond)

	removed := store.CleanupExpired()
	if removed != 1 {
		t.Fatalf("cleanup: got %d want %d", removed, 1)
	}
}

func TestMemoryStore_GetExtendsTTL(t *testing.T) {
	store := NewMemoryStore[string](50*time.Millisecond, 10)

	store.Set("a", "x")
	time.Sleep(30 * time.Millisecond)

	if _, ok := store.Get("a"); !ok {
		t.Fatalf("expected key to exist before refresh")
	}

	time.Sleep(30 * time.Millisecond)

	if _, ok := store.Get("a"); !ok {
		t.Fatalf("expected key to exist after refresh")
	}

	time.Sleep(80 * time.Millisecond)

	if _, ok := store.Get("a"); ok {
		t.Fatalf("expected key to expire")
	}
}

func TestMemoryStore_MaxItemsEviction(t *testing.T) {
	store := NewMemoryStore[string](time.Minute, 2)

	store.Set("a", "a")
	time.Sleep(25 * time.Millisecond)
	store.Set("b", "b")
	time.Sleep(25 * time.Millisecond)

	if _, ok := store.Get("a"); !ok {
		t.Fatalf("expected a to exist")
	}

	time.Sleep(25 * time.Millisecond)
	store.Set("c", "c")

	if _, ok := store.Get("b"); ok {
		t.Fatalf("expected b to be evicted")
	}
	if _, ok := store.Get("a"); !ok {
		t.Fatalf("expected a to remain")
	}
	if _, ok := store.Get("c"); !ok {
		t.Fatalf("expected c to exist")
	}
}
