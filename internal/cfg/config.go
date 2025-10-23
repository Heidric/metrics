package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Heidric/metrics.git/pkg/log"
	"github.com/joho/godotenv"
)

// Config holds service configuration: listen address, signing/hash keys,
// storage backends, and feature flags.
type Config struct {
	Logger          *log.Config
	ServerAddress   string
	FileStoragePath string
	DatabaseDSN     string
	HashKey         string
	CryptoKeyPath   string

	PollInterval   time.Duration
	ReportInterval time.Duration
	StoreInterval  time.Duration

	Restore bool
}

// fileConfig captures low-priority options coming from a JSON file.
// Only fields relevant for JSON are included; missing fields are ignored.
type fileConfig struct {
	Address        string         `json:"address"`
	PollInterval   *time.Duration `json:"poll_interval,omitempty"`
	ReportInterval *time.Duration `json:"report_interval,omitempty"`
	StoreInterval  *time.Duration `json:"store_interval,omitempty"`
	StoreFile      string         `json:"store_file"`
	Restore        *bool          `json:"restore,omitempty"`
	DatabaseDSN    string         `json:"database_dsn"`
	HashKey        string         `json:"hash_key"`
	CryptoKey      string         `json:"crypto_key"`
}

// pickConfigPathFromArgs performs a light pre-scan of os.Args
// to support -c/-config (and their =value forms) before flag.Parse.
func pickConfigPathFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "--c" || a == "-config" || a == "--config":
			if i+1 < len(args) {
				return normalizePath(args[i+1])
			}
		case strings.HasPrefix(a, "-c=") || strings.HasPrefix(a, "--c="):
			return normalizePath(strings.SplitN(a, "=", 2)[1])
		case strings.HasPrefix(a, "-config=") || strings.HasPrefix(a, "--config="):
			return normalizePath(strings.SplitN(a, "=", 2)[1])
		}
	}
	return ""
}

// normalizePath resolves a relative file path to absolute where possible.
func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
	}
	return p
}

// loadFileConfig reads a JSON file into fileConfig with tolerant semantics.
// Unknown fields are ignored; durations accept the Go "1s" style.
func loadFileConfig(path string) (*fileConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file %q: %w", path, err)
	}

	var retErr error
	defer func() {
		if cerr := f.Close(); cerr != nil {
			cerr = fmt.Errorf("close config file %q: %w", path, cerr)
			if retErr != nil {
				retErr = errors.Join(retErr, cerr)
			} else {
				retErr = cerr
			}
		}
	}()

	var cfg fileConfig
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&cfg); err != nil {
		retErr = fmt.Errorf("decode config file %q: %w", path, err)
		return nil, retErr
	}

	return &cfg, retErr
}

// isAllDigits reports whether s contains only ASCII digits 0-9.
func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// parseDuration supports both Go duration strings ("7s") and plain integer seconds ("7").
// Empty or invalid values fall back to defaultValue.
func parseDuration(key string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	v := strings.TrimSpace(value)
	if v == "" {
		return defaultValue
	}
	// Fast path: plain integer seconds.
	if isAllDigits(v) {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return time.Duration(n) * time.Second
		}
	}
	// Fallback to time.ParseDuration for "1s", "500ms", etc.
	if dur, err := time.ParseDuration(v); err == nil {
		return dur
	}
	return defaultValue
}

// parseBool returns defaultValue if env is unset/empty or not a valid bool.
func parseBool(key string, defaultValue bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	v := strings.TrimSpace(value)
	if v == "" {
		return defaultValue
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return defaultValue
}

// getEnv returns defaultValue when the variable is unset or empty.
func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		v := strings.TrimSpace(value)
		if v != "" {
			return v
		}
		return defaultValue
	}
	return defaultValue
}

func NewConfig() (*Config, error) {
	godotenv.Load()

	config := &Config{
		Logger: &log.Config{},
	}

	cfgPath := pickConfigPathFromArgs(os.Args)
	if cfgPath == "" {
		if v, ok := os.LookupEnv("CONFIG"); ok {
			cfgPath = v
		}
	}

	var fc *fileConfig
	if cfgPath != "" {
		if loaded, err := loadFileConfig(cfgPath); err == nil {
			fc = loaded
		} else {
			return nil, err
		}
	}

	addrDefault := "localhost:8080"
	if fc != nil && fc.Address != "" {
		addrDefault = fc.Address
	}
	config.ServerAddress = getEnv("ADDRESS", addrDefault)

	pollDefault := 2 * time.Second
	if fc != nil && fc.PollInterval != nil {
		pollDefault = *fc.PollInterval
	}
	config.PollInterval = parseDuration("POLL_INTERVAL", pollDefault)

	reportDefault := 10 * time.Second
	if fc != nil && fc.ReportInterval != nil {
		reportDefault = *fc.ReportInterval
	}
	config.ReportInterval = parseDuration("REPORT_INTERVAL", reportDefault)

	storeIntDefault := 300 * time.Second
	if fc != nil && fc.StoreInterval != nil {
		storeIntDefault = *fc.StoreInterval
	}
	config.StoreInterval = parseDuration("STORE_INTERVAL", storeIntDefault)

	storeFileDefault := "/tmp/metrics-db.json"
	if fc != nil && fc.StoreFile != "" {
		storeFileDefault = fc.StoreFile
	}
	config.FileStoragePath = getEnv("STORE_FILE", storeFileDefault)

	restoreDefault := true
	if fc != nil && fc.Restore != nil {
		restoreDefault = *fc.Restore
	}
	config.Restore = parseBool("RESTORE", restoreDefault)

	dsnDefault := ""
	if fc != nil && fc.DatabaseDSN != "" {
		dsnDefault = fc.DatabaseDSN
	}
	config.DatabaseDSN = getEnv("DATABASE_DSN", dsnDefault)

	hashKeyDefault := ""
	if fc != nil && fc.HashKey != "" {
		hashKeyDefault = fc.HashKey
	}
	config.HashKey = getEnv("HASH_KEY", hashKeyDefault)

	cryptoDefault := ""
	if fc != nil && fc.CryptoKey != "" {
		cryptoDefault = fc.CryptoKey
	}
	config.CryptoKeyPath = getEnv("CRYPTO_KEY", cryptoDefault)

	config.Logger.SetDefault()
	return config, nil
}
