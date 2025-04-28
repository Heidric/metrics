package db

import (
	"sync"

	"github.com/Heidric/metrics.git/internal/errors"
)

var (
	instance *Store
	once     sync.Once
)

func GetInstance() *Store {
	once.Do(func() {
		instance = NewStore()
	})
	return instance
}

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
	commands chan command
}

func NewStore() *Store {
	s := &Store{
		commands: make(chan command),
	}
	go s.run()
	return s
}

func (s *Store) run() {
	data := make(map[string]string)
	for cmd := range s.commands {
		switch cmd.action {
		case Set:
			data[cmd.key] = cmd.value
			cmd.respond <- nil
		case Get:
			if value, exists := data[cmd.key]; exists {
				cmd.respond <- value
			} else {
				cmd.respond <- errors.ErrKeyNotFound
			}
		case Delete:
			delete(data, cmd.key)
			cmd.respond <- nil
		case GetAll:
			dataCopy := make(map[string]string)
			for k, v := range data {
				dataCopy[k] = v
			}
			cmd.respond <- dataCopy
		}
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

func (s *Store) Close() {
	close(s.commands)
}
