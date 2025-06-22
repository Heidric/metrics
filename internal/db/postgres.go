package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	dsn       string
	db        *sql.DB
	mu        sync.Mutex
	connected bool
	closeOnce sync.Once
}

func NewPostgresStore(dsn string) *PostgresStore {
	return &PostgresStore{
		dsn: dsn,
	}
}

func (p *PostgresStore) resetConnection() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("Resetting connection")

	if p.db != nil {
		log.Println("Closing database connection")
		p.db.Close()
		p.db = nil
	}
	p.connected = false

	log.Println("Connection reset complete")
}

func (p *PostgresStore) handleConnectionError(err error) {
	log.Printf("Handling connection error: %v", err)

	p.resetConnection()
}

func (p *PostgresStore) ensureConnected(ctx context.Context) error {
	log.Println("Ensuring connection")

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connected {
		log.Println("Already connected, skipping")
		return nil
	}

	if p.dsn == "" {
		return errors.ErrNotConnected
	}

	log.Println("Opening new database connection")
	db, err := sql.Open("pgx", p.dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	log.Println("Pinging database")
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("Creating table")
	if err := p.createTable(db); err != nil {
		db.Close()
		return fmt.Errorf("failed to create table: %w", err)
	}

	log.Println("Connection established successfully")
	p.db = db
	p.connected = true
	return nil
}

func (p *PostgresStore) createTable(db *sql.DB) error {
	query := `
        CREATE TABLE IF NOT EXISTS metrics (
            id SERIAL PRIMARY KEY,
            name VARCHAR(255) NOT NULL,
            mtype VARCHAR(10) NOT NULL,
            delta BIGINT,
            value DOUBLE PRECISION,
            UNIQUE (name, mtype)
        )
    `
	_, err := db.Exec(query)
	return err
}

func (p *PostgresStore) SetGauge(name string, value float64) error {
	ctx := context.Background()

	if err := p.ensureConnected(ctx); err != nil {
		log.Printf("ensureConnected error: %v", err)
		return err
	}

	query := `
        INSERT INTO metrics (name, mtype, value)
        VALUES ($1, 'gauge', $2)
        ON CONFLICT (name, mtype) DO UPDATE SET value = $2
    `

	_, err := p.db.ExecContext(ctx, query, name, value)

	if err != nil {
		log.Printf("Query error: %v", err)

		if err == sql.ErrConnDone ||
			err == sql.ErrTxDone ||
			strings.Contains(strings.ToLower(err.Error()), "connection") {
			log.Println("Detected connection error, resetting...")
			p.resetConnection()
		}
	}
	return err
}

func (p *PostgresStore) GetGauge(name string) (float64, error) {
	ctx := context.Background()
	if err := p.ensureConnected(ctx); err != nil {
		return 0, err
	}

	var value float64
	query := "SELECT value FROM metrics WHERE name = $1 AND mtype = 'gauge'"
	err := p.db.QueryRowContext(ctx, query, name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, errors.ErrKeyNotFound
	}
	return value, err
}

func (p *PostgresStore) SetCounter(name string, value int64) error {
	ctx := context.Background()
	if err := p.ensureConnected(ctx); err != nil {
		return err
	}

	query := `
        INSERT INTO metrics (name, mtype, delta)
        VALUES ($1, 'counter', $2)
        ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $2
    `
	_, err := p.db.ExecContext(ctx, query, name, value)
	return err
}

func (p *PostgresStore) GetCounter(name string) (int64, error) {
	ctx := context.Background()
	if err := p.ensureConnected(ctx); err != nil {
		return 0, err
	}

	var delta int64
	query := "SELECT delta FROM metrics WHERE name = $1 AND mtype = 'counter'"
	err := p.db.QueryRowContext(ctx, query, name).Scan(&delta)
	if err == sql.ErrNoRows {
		return 0, errors.ErrKeyNotFound
	}
	return delta, err
}

func (p *PostgresStore) GetAll() (map[string]float64, map[string]int64, error) {
	ctx := context.Background()
	if err := p.ensureConnected(ctx); err != nil {
		return nil, nil, err
	}

	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	rows, err := p.db.QueryContext(ctx, "SELECT name, mtype, value, delta FROM metrics")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			name  string
			mtype string
			value sql.NullFloat64
			delta sql.NullInt64
		)
		if err := rows.Scan(&name, &mtype, &value, &delta); err != nil {
			return nil, nil, err
		}

		switch mtype {
		case "gauge":
			if value.Valid {
				gauges[name] = value.Float64
			}
		case "counter":
			if delta.Valid {
				counters[name] = delta.Int64
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return gauges, counters, nil
}

func (p *PostgresStore) UpdateMetricsBatch(metrics []*model.Metrics) error {
	ctx := context.Background()
	if err := p.ensureConnected(ctx); err != nil {
		return err
	}

	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, m := range metrics {
		switch m.MType {
		case "gauge":
			_, err = tx.Exec(`
				INSERT INTO metrics (name, mtype, value)
				VALUES ($1, $2, $3) 
				ON CONFLICT (name, mtype) DO UPDATE SET value = $3
			`, m.ID, m.MType, m.Value)
		case "counter":
			_, err = tx.Exec(`
				INSERT INTO metrics (name, mtype, delta)
				VALUES ($1, $2, $3) 
				ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $3
			`, m.ID, m.MType, m.Delta)
		default:
			return fmt.Errorf("unsupported metric type: %s", m.MType)
		}
		if err != nil {
			return fmt.Errorf("exec update for %s: %w", m.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (p *PostgresStore) Ping(ctx context.Context) error {
	if err := p.ensureConnected(ctx); err != nil {
		return errors.ErrNotConnected
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.connected {
		return errors.ErrNotConnected
	}
	return p.db.PingContext(ctx)
}

func (p *PostgresStore) Close() error {
	if !p.connected {
		return nil
	}

	var err error
	p.closeOnce.Do(func() {
		err = p.db.Close()
		p.connected = false
	})
	return err
}
