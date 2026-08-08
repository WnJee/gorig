package cache

import (
	"testing"
	"time"
)

func TestJSONCacheMissMatchesContract(t *testing.T) {
	cache, err := NewJSONCache[string]("contract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Flush()
	if _, err := cache.Get("missing"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
	if err := cache.Set("key", "value", 1100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := cache.Get("key"); err != ErrCacheMiss {
		t.Fatalf("expected expired ErrCacheMiss, got %v", err)
	}
}
