package db

import (
	"errors"
	"testing"
	"time"
)

type retryTest struct {
	name          string
	exec          func() error
	expectedErr   error
	expectedTries int
}

func TestWithRetry(t *testing.T) {
	tests := []retryTest{
		{
			name:          "succeeds_after_some_failures",
			expectedErr:   nil,
			expectedTries: 3,
			exec: func() func() error {
				attempts := 0
				return func() error {
					attempts++
					if attempts < 3 {
						return errors.New("temporary error")
					}
					if attempts != 3 {
						t.Errorf("unexpected number of attempts: got %d, want 3", attempts)
					}
					return nil
				}
			}(),
		},
		{
			name:          "fails_after_all_retries",
			expectedErr:   errors.New("permanent failure"),
			expectedTries: 4,
			exec: func() func() error {
				attempts := 0
				return func() error {
					attempts++
					if attempts >= 4 {
						return errors.New("permanent failure")
					}
					return errors.New("temporary failure")
				}
			}(),
		},
		{
			name:          "delays_between_retries",
			expectedErr:   errors.New("not ready yet"),
			expectedTries: 2,
			exec: func() func() error {
				var calls []time.Time
				return func() error {
					calls = append(calls, time.Now())
					if len(calls) == 1 {
						return errors.New("not ready yet")
					}
					if len(calls) == 2 {
						delta := calls[1].Sub(calls[0])
						if delta < 100*time.Millisecond {
							t.Errorf("expected at least 100ms delay between retries, got %v", delta)
						}
						return errors.New("permanent failure")
					}
					t.Fatalf("expected 2 attempts, got %d", len(calls))
					return nil
				}
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempts := 0
			exec := func() error {
				attempts++
				return test.exec()
			}
			err := withRetry(exec, func(err error) bool { return err.Error() != "permanent failure" })
			if (err == nil) != (test.expectedErr == nil) {
				t.Errorf("unexpected error state: got %v, want %v", err, test.expectedErr)
			}
			if attempts != test.expectedTries {
				t.Errorf("unexpected number of attempts: got %d, want %d", attempts, test.expectedTries)
			}
		})
	}
}
