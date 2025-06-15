package db

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresStore(t *testing.T) {
	t.Run("Constructor doesn't connect immediately", func(t *testing.T) {
		store := NewPostgresStore("postgres://invalid:connection@localhost/nonexistentdb?sslmode=disable")
		assert.NotNil(t, store)
	})
}

func TestPostgresStore_SetGauge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
            INSERT INTO metrics (name, mtype, value)
            VALUES ($1, 'gauge', $2)
            ON CONFLICT (name, mtype) DO UPDATE SET value = $2
        `)).
			WithArgs("cpu", 42.5).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.SetGauge("cpu", 42.5)
		assert.NoError(t, err)
	})

	t.Run("Database error", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
            INSERT INTO metrics (name, mtype, value)
            VALUES ($1, 'gauge', $2)
            ON CONFLICT (name, mtype) DO UPDATE SET value = $2
        `)).
			WithArgs("cpu", 42.5).
			WillReturnError(sql.ErrConnDone)

		err := store.SetGauge("cpu", 42.5)
		assert.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_GetGauge(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"value"}).AddRow(42.5)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT value FROM metrics WHERE name = $1 AND mtype = 'gauge'")).
			WithArgs("cpu").
			WillReturnRows(rows)

		value, err := store.GetGauge("cpu")
		require.NoError(t, err)
		assert.Equal(t, 42.5, value)
	})

	t.Run("Not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT value FROM metrics WHERE name = $1 AND mtype = 'gauge'")).
			WithArgs("cpu").
			WillReturnError(sql.ErrNoRows)

		_, err := store.GetGauge("cpu")
		assert.ErrorIs(t, err, errors.ErrKeyNotFound)
	})

	t.Run("Database error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT value FROM metrics WHERE name = $1 AND mtype = 'gauge'")).
			WithArgs("cpu").
			WillReturnError(sql.ErrTxDone)

		_, err := store.GetGauge("cpu")
		assert.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_SetCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
            INSERT INTO metrics (name, mtype, delta)
            VALUES ($1, 'counter', $2)
            ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $2
        `)).
			WithArgs("requests", int64(10)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.SetCounter("requests", 10)
		assert.NoError(t, err)
	})

	t.Run("Increment", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
            INSERT INTO metrics (name, mtype, delta)
            VALUES ($1, 'counter', $2)
            ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $2
        `)).
			WithArgs("requests", int64(5)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectExec(regexp.QuoteMeta(`
            INSERT INTO metrics (name, mtype, delta)
            VALUES ($1, 'counter', $2)
            ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $2
        `)).
			WithArgs("requests", int64(3)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.SetCounter("requests", 5)
		require.NoError(t, err)

		err = store.SetCounter("requests", 3)
		require.NoError(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_GetCounter(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"delta"}).AddRow(15)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT delta FROM metrics WHERE name = $1 AND mtype = 'counter'")).
			WithArgs("requests").
			WillReturnRows(rows)

		value, err := store.GetCounter("requests")
		require.NoError(t, err)
		assert.Equal(t, int64(15), value)
	})

	t.Run("Not found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT delta FROM metrics WHERE name = $1 AND mtype = 'counter'")).
			WithArgs("requests").
			WillReturnError(sql.ErrNoRows)

		_, err := store.GetCounter("requests")
		assert.ErrorIs(t, err, errors.ErrKeyNotFound)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_GetAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"name", "mtype", "value", "delta"}).
			AddRow("cpu", "gauge", 42.5, nil).
			AddRow("requests", "counter", nil, 15)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT name, mtype, value, delta FROM metrics")).
			WillReturnRows(rows)

		gauges, counters, err := store.GetAll()
		require.NoError(t, err)

		assert.Equal(t, 42.5, gauges["cpu"])
		assert.Equal(t, int64(15), counters["requests"])
	})

	t.Run("Empty result", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"name", "mtype", "value", "delta"})
		mock.ExpectQuery(regexp.QuoteMeta("SELECT name, mtype, value, delta FROM metrics")).
			WillReturnRows(rows)

		gauges, counters, err := store.GetAll()
		require.NoError(t, err)

		assert.Empty(t, gauges)
		assert.Empty(t, counters)
	})

	t.Run("Database error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT name, mtype, value, delta FROM metrics")).
			WillReturnError(sql.ErrConnDone)

		_, _, err := store.GetAll()
		assert.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_Ping(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectPing()
		err := store.Ping(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mock.ExpectPing().WillReturnError(sql.ErrConnDone)
		err := store.Ping(context.Background())
		assert.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
	db.Close()
}

func TestPostgresStore_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	store := &PostgresStore{db: db, connected: true}

	mock.ExpectClose()
	err = store.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_ConnectionRecovery(t *testing.T) {
	db1, mock1, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	store := &PostgresStore{dsn: ""}
	store.db = db1
	store.connected = true

	mock1.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO metrics (name, mtype, value) VALUES ($1, 'gauge', $2) ON CONFLICT (name, mtype) DO UPDATE SET value = $2`,
	)).
		WithArgs("test", 1.0).
		WillReturnError(sql.ErrConnDone)

	err = store.SetGauge("test", 1.0)
	require.Error(t, err)

	store.mu.Lock()
	require.False(t, store.connected, "Connection should be reset after error")
	require.Nil(t, store.db, "DB connection should be nil after reset")
	store.mu.Unlock()

	db2, mock2, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db2.Close()

	store.mu.Lock()
	store.db = db2
	store.connected = true
	store.mu.Unlock()

	mock2.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO metrics (name, mtype, value) VALUES ($1, 'gauge', $2) ON CONFLICT (name, mtype) DO UPDATE SET value = $2`,
	)).
		WithArgs("test", 1.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.SetGauge("test", 1.0)
	require.NoError(t, err)

	store.mu.Lock()
	require.True(t, store.connected, "Connection should be reestablished")
	require.NotNil(t, store.db, "DB connection should be set")
	store.mu.Unlock()

	require.NoError(t, mock1.ExpectationsWereMet())
	require.NoError(t, mock2.ExpectationsWereMet())

	db1.Close()
}
