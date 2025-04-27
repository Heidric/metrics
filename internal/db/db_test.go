package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/Heidric/metrics.git/internal/errors"
)

func TestKeyValueStore(t *testing.T) {
	t.Run("Set and Get", func(t *testing.T) {
		store := NewKeyValueStore()
		defer store.Close()

		store.Set("key1", "value1")
		value, err := store.Get("key1")

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value != "value1" {
			t.Fatalf("Expected 'value1', got '%s'", value)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		store := NewKeyValueStore()
		defer store.Close()

		_, err := store.Get("nonexistent")
		if err != errors.ErrKeyNotFound {
			t.Fatalf("Expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := NewKeyValueStore()
		defer store.Close()

		store.Set("key1", "value1")
		store.Delete("key1")
		_, err := store.Get("key1")

		if err != errors.ErrKeyNotFound {
			t.Fatalf("Expected ErrKeyNotFound after delete, got %v", err)
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		store := NewKeyValueStore()
		defer store.Close()

		store.Set("key1", "value1")
		store.Set("key2", "value2")
		all := store.GetAll()

		if len(all) != 2 {
			t.Fatalf("Expected 2 items, got %d", len(all))
		}
		if all["key1"] != "value1" || all["key2"] != "value2" {
			t.Fatalf("GetAll returned unexpected values: %v", all)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		store := NewKeyValueStore()
		defer store.Close()

		go func() {
			for i := 0; i < 100; i++ {
				store.Set("key", fmt.Sprintf("value%d", i))
			}
		}()

		go func() {
			for i := 0; i < 100; i++ {
				store.Get("key")
			}
		}()

		time.Sleep(100 * time.Millisecond)
		value, err := store.Get("key")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value == "" {
			t.Fatal("Expected non-empty value")
		}
	})

	t.Run("Singleton", func(t *testing.T) {
		store1 := GetInstance()
		store2 := GetInstance()

		if store1 != store2 {
			t.Fatal("GetInstance should return the same instance")
		}
	})
}
