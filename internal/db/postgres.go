package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/model"
)

type PostgresStore struct {
	dsn       string
	db        *sql.DB
	mu        sync.Mutex
	connected bool
	closeOnce sync.Once
}

func NewPostgresStore(dsn string) *PostgresStore {
	return &PostgresStore{dsn: dsn}
}

func (p *PostgresStore) resetConnection() {
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
		return customerrors.ErrNotConnected
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

func (p *PostgresStore) SetGauge(ctx context.Context, name string, value float64) error {
	return withPGRetry(func() error {
		if err := p.ensureConnected(ctx); err != nil {
			log.Printf("ensureConnected error: %v", err)
			return err
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		query := `
	        INSERT INTO metrics (name, mtype, value)
	        VALUES ($1, 'gauge', $2)
	        ON CONFLICT (name, mtype) DO UPDATE SET value = $2
	    `
		_, err := p.db.ExecContext(ctx, query, name, value)
		if err != nil {
			log.Printf("Query error: %v", err)
			if err == sql.ErrConnDone || err == sql.ErrTxDone || strings.Contains(strings.ToLower(err.Error()), "connection") {
				log.Println("Detected connection error, resetting...")
				p.resetConnection()
			}
		}
		return err
	})
}

func (p *PostgresStore) SetCounter(ctx context.Context, name string, value int64) error {
	return withPGRetry(func() error {
		if err := p.ensureConnected(ctx); err != nil {
			return err
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		query := `
	        INSERT INTO metrics (name, mtype, delta)
	        VALUES ($1, 'counter', $2)
	        ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $2
	    `
		_, err := p.db.ExecContext(ctx, query, name, value)
		return err
	})
}

func (p *PostgresStore) UpdateMetricsBatch(ctx context.Context, metrics []*model.Metrics) error {
	return withPGRetry(func() error {
		if err := p.ensureConnected(ctx); err != nil {
			return err
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		tx, err := p.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		for _, m := range metrics {
			switch m.MType {
			case model.GaugeType:
				_, err = tx.ExecContext(ctx, `
					INSERT INTO metrics (name, mtype, value)
					VALUES ($1, $2, $3) 
					ON CONFLICT (name, mtype) DO UPDATE SET value = $3
				`, m.ID, m.MType, m.Value)
			case model.CounterType:
				_, err = tx.ExecContext(ctx, `
					INSERT INTO metrics (name, mtype, delta)
					VALUES ($1, $2, $3) 
					ON CONFLICT (name, mtype) DO UPDATE SET delta = metrics.delta + $3
				`, m.ID, m.MType, m.Delta)
			default:
				return fmt.Errorf("unsupported metric type: %s", m.MType)
			}
			if err != nil {
				log.Printf("UpdateMetricsBatch SQL error for ID=%s: %v", m.ID, err)
				return fmt.Errorf("exec update for %s: %w", m.ID, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
	})
}

func (p *PostgresStore) GetGauge(ctx context.Context, name string) (float64, error) {
	if err := p.ensureConnected(ctx); err != nil {
		return 0, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var value float64
	query := "SELECT value FROM metrics WHERE name = $1 AND mtype = 'gauge'"
	err := p.db.QueryRowContext(ctx, query, name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, customerrors.ErrKeyNotFound
	}
	return value, err
}

func (p *PostgresStore) GetCounter(ctx context.Context, name string) (int64, error) {
	if err := p.ensureConnected(ctx); err != nil {
		return 0, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var delta int64
	query := "SELECT delta FROM metrics WHERE name = $1 AND mtype = 'counter'"
	err := p.db.QueryRowContext(ctx, query, name).Scan(&delta)
	if err == sql.ErrNoRows {
		return 0, customerrors.ErrKeyNotFound
	}
	return delta, err
}

func (p *PostgresStore) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if err := p.ensureConnected(ctx); err != nil {
		return nil, nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

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
		case model.GaugeType:
			if value.Valid {
				gauges[name] = value.Float64
			}
		case model.CounterType:
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

func (p *PostgresStore) Ping(ctx context.Context) error {
	if err := p.ensureConnected(ctx); err != nil {
		return customerrors.ErrNotConnected
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.connected {
		return customerrors.ErrNotConnected
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
