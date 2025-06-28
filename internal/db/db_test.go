package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	tempFile, err := os.CreateTemp("", "db_test")
	require.NoError(t, err)
	tempPath := tempFile.Name()

	_, err = tempFile.WriteString(`{"gauges":{"init":1.1},"counters":{"init":1}}`)
	require.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempPath)

	t.Run("Set and Get Gauge", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		err := store.SetGauge(ctx, "key1", 42.5)
		require.NoError(t, err)

		value, err := store.GetGauge(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, 42.5, value)
	})

	t.Run("Set and Get Counter", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		err := store.SetCounter(ctx, "counter1", 10)
		require.NoError(t, err)

		value, err := store.GetCounter(ctx, "counter1")
		require.NoError(t, err)
		assert.Equal(t, int64(10), value)
	})

	t.Run("Increment Counter", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		err := store.SetCounter(ctx, "counter1", 10)
		require.NoError(t, err)

		err = store.SetCounter(ctx, "counter1", 5)
		require.NoError(t, err)

		value, err := store.GetCounter(ctx, "counter1")
		require.NoError(t, err)
		assert.Equal(t, int64(15), value)
	})

	t.Run("Get non-existent Gauge", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		_, err := store.GetGauge(ctx, "nonexistent")
		assert.ErrorIs(t, err, customerrors.ErrKeyNotFound)
	})

	t.Run("Get non-existent Counter", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		_, err := store.GetCounter(ctx, "nonexistent")
		assert.ErrorIs(t, err, customerrors.ErrKeyNotFound)
	})

	t.Run("GetAll", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		err := store.SetGauge(ctx, "gauge1", 1.1)
		require.NoError(t, err)
		err = store.SetCounter(ctx, "counter1", 10)
		require.NoError(t, err)

		gauges, counters, err := store.GetAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(gauges))
		assert.Equal(t, 1.1, gauges["gauge1"])
		assert.Equal(t, 1, len(counters))
		assert.Equal(t, int64(10), counters["counter1"])
	})

	t.Run("Concurrent access", func(t *testing.T) {
		ctx := context.Background()
		store := NewStore("", 0)
		defer store.Close()

		for i := 0; i < 2; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					_ = store.SetCounter(ctx, "counter", 1)
				}
			}()
		}

		time.Sleep(500 * time.Millisecond)

		val, err := store.GetCounter(ctx, "counter")
		require.NoError(t, err)
		assert.Equal(t, int64(200), val)
	})

	t.Run("File persistence", func(t *testing.T) {
		ctx := context.Background()
		store1 := NewStore(tempPath, 0)
		err := store1.SetGauge(ctx, "gauge1", 123.45)
		require.NoError(t, err)
		err = store1.SetCounter(ctx, "counter1", 100)
		require.NoError(t, err)

		require.NoError(t, store1.Close())

		store2 := NewStore(tempPath, 0)
		defer store2.Close()

		gaugeVal, err := store2.GetGauge(ctx, "gauge1")
		require.NoError(t, err)
		assert.Equal(t, 123.45, gaugeVal)

		counterVal, err := store2.GetCounter(ctx, "counter1")
		require.NoError(t, err)
		assert.Equal(t, int64(100), counterVal)
	})

	t.Run("Periodic save", func(t *testing.T) {
		ctx := context.Background()
		tempFile, err := os.CreateTemp("", "periodic_test")
		require.NoError(t, err)
		tempPath := tempFile.Name()
		tempFile.Close()
		defer os.Remove(tempPath)

		store := NewStore(tempPath, 10*time.Millisecond)

		err = store.SetGauge(ctx, "periodic_test", 99.99)
		require.NoError(t, err)

		var val float64
		for i := 0; i < 10; i++ {
			time.Sleep(50 * time.Millisecond)
			store2 := NewStore(tempPath, 0)
			val, err = store2.GetGauge(ctx, "periodic_test")
			store2.Close()
			if err == nil {
				break
			}
		}
		require.NoError(t, err, "Should find saved value")
		assert.Equal(t, 99.99, val)

		require.NoError(t, store.Close())
	})
}
