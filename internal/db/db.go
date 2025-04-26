package db

import (
	"sync"

	"github.com/Heidric/metrics.git/internal/errors"
)

var (
	instance *KeyValueStore
	once     sync.Once
)

func GetInstance() *KeyValueStore {
	once.Do(func() {
		instance = NewKeyValueStore()
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

type KeyValueStore struct {
	commands chan command
}

func NewKeyValueStore() *KeyValueStore {
	kvs := &KeyValueStore{
		commands: make(chan command),
	}
	go kvs.run()
	return kvs
}

func (kvs *KeyValueStore) run() {
	data := make(map[string]string)
	for cmd := range kvs.commands {
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

func (kvs *KeyValueStore) Set(key, value string) {
	response := make(chan interface{})
	kvs.commands <- command{
		action:  Set,
		key:     key,
		value:   value,
		respond: response,
	}
	<-response
}

func (kvs *KeyValueStore) Get(key string) (string, error) {
	response := make(chan interface{})
	kvs.commands <- command{
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

func (kvs *KeyValueStore) Delete(key string) {
	response := make(chan interface{})
	kvs.commands <- command{
		action:  Delete,
		key:     key,
		respond: response,
	}
	<-response
}

func (kvs *KeyValueStore) GetAll() map[string]string {
	response := make(chan interface{})
	kvs.commands <- command{
		action:  GetAll,
		respond: response,
	}
	return (<-response).(map[string]string)
}

func (kvs *KeyValueStore) Close() {
	close(kvs.commands)
}
