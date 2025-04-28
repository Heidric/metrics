package services

import (
	"fmt"
	"strconv"

	"github.com/Heidric/metrics.git/internal/errors"
)

type MetricsStorage interface {
	Set(key, value string)
	Get(key string) (string, error)
	GetAll() map[string]string
}

type MetricsService struct {
	storage MetricsStorage
}

func NewMetricsService(storage MetricsStorage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (ms *MetricsService) UpdateGauge(name, value string) error {
	_, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return errors.ErrInvalidValue
	}
	ms.storage.Set(name, value)
	return nil
}

func (ms *MetricsService) UpdateCounter(name, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return errors.ErrInvalidValue
	}

	current := 0
	if strValue, err := ms.storage.Get(name); err == nil {
		current, err = strconv.Atoi(strValue)
		if err != nil {
			return err
		}
	}

	ms.storage.Set(name, fmt.Sprintf("%d", current+intValue))
	return nil
}

func (ms *MetricsService) GetMetric(metricType, metricName string) (string, error) {
	return ms.storage.Get(metricName)
}

func (ms *MetricsService) ListMetrics() map[string]string {
	return ms.storage.GetAll()
}
