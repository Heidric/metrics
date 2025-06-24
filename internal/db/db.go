package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/model"
)

type MetricsStorage interface {
	SetGauge(name string, value float64) error
	GetGauge(name string) (float64, error)
	SetCounter(name string, value int64) error
	GetCounter(name string) (int64, error)
	GetAll() (map[string]float64, map[string]int64, error)
	UpdateMetricsBatch(metrics []*model.Metrics) error
	Ping(ctx context.Context) error
	Close() error
}

type Store struct {
	mu            sync.RWMutex
	gauges        map[string]float64
	counters      map[string]int64
	filePath      string
	storeInterval time.Duration
	syncMode      bool
	saveMutex     sync.Mutex
	ticker        *time.Ticker
	closeChan     chan struct{}
	closed        bool
}

func NewStore(filePath string, storeInterval time.Duration) *Store {
	s := &Store{
		gauges:        make(map[string]float64),
		counters:      make(map[string]int64),
		filePath:      filePath,
		storeInterval: storeInterval,
		syncMode:      storeInterval == 0,
		closeChan:     make(chan struct{}),
	}

	if err := s.LoadFromFile(); err != nil {
		fmt.Printf("Warning: failed to load data: %v\n", err)
	}

	if !s.syncMode && storeInterval > 0 {
		s.ticker = time.NewTicker(storeInterval)
		go s.periodicSave()
	}

	return s
}

func (s *Store) periodicSave() {
	for {
		select {
		case <-s.ticker.C:
			s.saveMutex.Lock()
			if err := s.saveToFile(); err != nil {
				fmt.Printf("Error saving to file: %v\n", err)
			}
			s.saveMutex.Unlock()
		case <-s.closeChan:
			if s.ticker != nil {
				s.ticker.Stop()
			}
			return
		}
	}
}

func (s *Store) SetGauge(name string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value

	if s.syncMode {
		s.saveMutex.Lock()
		defer s.saveMutex.Unlock()
		return s.saveToFile()
	}
	return nil
}

func (s *Store) GetGauge(name string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, exists := s.gauges[name]; exists {
		return value, nil
	}
	return 0, customerrors.ErrKeyNotFound
}

func (s *Store) SetCounter(name string, value int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.counters[name]
	if !exists {
		current = 0
	}
	s.counters[name] = current + value

	if s.syncMode {
		s.saveMutex.Lock()
		defer s.saveMutex.Unlock()
		return s.saveToFile()
	}
	return nil
}

func (s *Store) GetCounter(name string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if value, exists := s.counters[name]; exists {
		return value, nil
	}
	return 0, customerrors.ErrKeyNotFound
}

func (s *Store) GetAll() (map[string]float64, map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gaugesCopy := make(map[string]float64)
	for k, v := range s.gauges {
		gaugesCopy[k] = v
	}

	countersCopy := make(map[string]int64)
	for k, v := range s.counters {
		countersCopy[k] = v
	}

	return gaugesCopy, countersCopy, nil
}

func (s *Store) Close() error {
	s.closed = true
	close(s.closeChan)

	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	return s.saveToFile()
}

func (s *Store) SaveToFile() error {
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()
	return s.saveToFile()
}

func (s *Store) saveToFile() error {
	if s.filePath == "" {
		return nil
	}

	data := struct {
		Gauges   map[string]float64 `json:"gauges"`
		Counters map[string]int64   `json:"counters"`
	}{
		Gauges:   s.gauges,
		Counters: s.counters,
	}

	file, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	return nil
}

func (s *Store) LoadFromFile() error {
	s.saveMutex.Lock()
	defer s.saveMutex.Unlock()

	if s.filePath == "" {
		return nil
	}

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var data struct {
		Gauges   map[string]float64 `json:"gauges"`
		Counters map[string]int64   `json:"counters"`
	}

	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges = data.Gauges
	s.counters = data.Counters

	return nil
}

func (s *Store) UpdateMetricsBatch(metrics []*model.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case "gauge":
			if m.Value == nil {
				continue
			}
			s.gauges[m.ID] = *m.Value
		case "counter":
			if m.Delta == nil {
				continue
			}
			s.counters[m.ID] += *m.Delta
		default:
			return fmt.Errorf("unsupported metric type: %s", m.MType)
		}
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	return nil
}
