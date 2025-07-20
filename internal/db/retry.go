package db

import (
	"errors"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
)

var retryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

func withRetry(fn func() error, isRetriable func(error) bool) error {
	var err error
	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isRetriable(err) {
			return err
		}
		if attempt < len(retryDelays) {
			time.Sleep(retryDelays[attempt])
		}
	}
	return err
}

func withPGRetry(fn func() error) error {
	return withRetry(fn, isRetriable)
}

func isRetriable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure:
			return true
		}
	}
	return false
}
