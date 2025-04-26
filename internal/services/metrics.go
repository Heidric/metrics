package services

import (
	"fmt"
	"strconv"

	"github.com/Heidric/metrics.git/internal/errors"
)

type MetricsStorage interface {
	Set(key, value string)
	Get(key string) (string, error)
}

type MetricsService struct {
	storage MetricsStorage
}

func NewMetricsService(storage MetricsStorage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (ms *MetricsService) UpdateGauge(name, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return errors.ErrInvalidValue
	}
	ms.storage.Set(name, fmt.Sprint(intValue))

	return nil
}

func (ms *MetricsService) UpdateCounter(name, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return errors.ErrInvalidValue
	}

	counter := 0
	strCounter, err := ms.storage.Get(name)
	if err != nil {
		if err != errors.ErrKeyNotFound {
			return err
		}
	}
	if strCounter != "" {
		counter, err = strconv.Atoi(strCounter)
		if err != nil {
			return err
		}
	}

	ms.storage.Set(name, fmt.Sprint(counter+intValue))

	return nil
}
