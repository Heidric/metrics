package db

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Heidric/metrics.git/internal/errors"
)

func TestStore(t *testing.T) {
	tempFile, err := os.CreateTemp("", "db_test")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	t.Run("Set and Get", func(t *testing.T) {
		store := NewStore("", 0)
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
		store := NewStore("", 0)
		defer store.Close()

		_, err := store.Get("nonexistent")
		if err != errors.ErrKeyNotFound {
			t.Fatalf("Expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := NewStore("", 0)
		defer store.Close()

		store.Set("key1", "value1")
		store.Delete("key1")
		_, err := store.Get("key1")

		if err != errors.ErrKeyNotFound {
			t.Fatalf("Expected ErrKeyNotFound after delete, got %v", err)
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		store := NewStore("", 0)
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
		store := NewStore("", 0)
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

	t.Run("File persistence", func(t *testing.T) {
		store1 := NewStore(tempFile.Name(), 0)
		store1.Set("persistent", "value")
		if err := store1.Close(); err != nil {
			t.Fatalf("Failed to close store: %v", err)
		}

		store2 := NewStore(tempFile.Name(), 0)
		defer store2.Close()

		value, err := store2.Get("persistent")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value != "value" {
			t.Fatalf("Expected 'value', got '%s'", value)
		}
	})

	t.Run("Close error handling", func(t *testing.T) {
		store := NewStore("/invalid/path/db.json", 0)
		store.Set("key", "value")
		err := store.Close()
		if err == nil {
			t.Fatal("Expected error when closing with invalid path")
		}
	})
}
