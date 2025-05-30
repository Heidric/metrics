package db

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Heidric/metrics.git/internal/errors"
)

type CommandType int

const (
	Set CommandType = iota
	Get
	Delete
	GetAll
)

type command struct {
	action  CommandType
	key     string
	value   string
	respond chan<- interface{}
}

type Store struct {
	commands      chan command
	data          map[string]string
	filePath      string
	storeInterval time.Duration
	saveMutex     sync.Mutex
	syncMode      bool
	closeChan     chan struct{}
	closed        bool
	closedMutex   sync.Mutex
}

func NewStore(filePath string, storeInterval time.Duration) *Store {
	s := &Store{
		commands:      make(chan command),
		data:          make(map[string]string),
		filePath:      filePath,
		storeInterval: storeInterval,
		syncMode:      storeInterval == 0,
		closeChan:     make(chan struct{}),
	}

	if err := s.LoadFromFile(); err != nil {
		fmt.Printf("Warning: failed to load data: %v\n", err)
	}

	go s.run()
	return s
}

func (s *Store) isClosed() bool {
	s.closedMutex.Lock()
	defer s.closedMutex.Unlock()
	return s.closed
}

func (s *Store) run() {
	for {
		select {
		case cmd, ok := <-s.commands:
			if !ok {
				return
			}
			s.processCommand(cmd)
		case <-s.closeChan:
			return
		}
	}
}

func (s *Store) processCommand(cmd command) {
	if s.isClosed() {
		if cmd.respond != nil {
			cmd.respond <- errors.ErrStoreClosed
		}
		return
	}

	switch cmd.action {
	case Set:
		s.data[cmd.key] = cmd.value
		cmd.respond <- nil
		if s.syncMode {
			s.saveMutex.Lock()
			if err := s.saveToFile(); err != nil {
				fmt.Printf("Error saving to file: %v\n", err)
			}
			s.saveMutex.Unlock()
		}
	case Get:
		if value, exists := s.data[cmd.key]; exists {
			cmd.respond <- value
		} else {
			cmd.respond <- errors.ErrKeyNotFound
		}
	case Delete:
		delete(s.data, cmd.key)
		cmd.respond <- nil
		if s.syncMode {
			s.saveMutex.Lock()
			if err := s.saveToFile(); err != nil {
				fmt.Printf("Error saving to file: %v\n", err)
			}
			s.saveMutex.Unlock()
		}
	case GetAll:
		dataCopy := make(map[string]string)
		for k, v := range s.data {
			dataCopy[k] = v
		}
		cmd.respond <- dataCopy
	}
}

func (s *Store) Set(key, value string) {
	response := make(chan interface{})
	s.commands <- command{
		action:  Set,
		key:     key,
		value:   value,
		respond: response,
	}
	<-response
}

func (s *Store) Get(key string) (string, error) {
	response := make(chan interface{})
	s.commands <- command{
		action:  Get,
		key:     key,
		respond: response,
	}
	result := <-response
	switch v := result.(type) {
	case string:
		return v, nil
	case error:
		return "", v
	default:
		return "", errors.ErrUnexpectedResponseType
	}
}

func (s *Store) Delete(key string) {
	response := make(chan interface{})
	s.commands <- command{
		action:  Delete,
		key:     key,
		respond: response,
	}
	<-response
}

func (s *Store) GetAll() map[string]string {
	response := make(chan interface{})
	s.commands <- command{
		action:  GetAll,
		respond: response,
	}
	return (<-response).(map[string]string)
}

func (s *Store) Close() error {
	s.closedMutex.Lock()
	s.closed = true
	s.closedMutex.Unlock()

	close(s.closeChan)
	close(s.commands)

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

	data := make(map[string]string)
	for k, v := range s.data {
		data[k] = v
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

	var data map[string]string
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode data: %w", err)
	}

	for key, value := range data {
		s.data[key] = value
	}

	return nil
}
