package services

import (
	"fmt"
	"strconv"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/model"
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

func (ms *MetricsService) UpdateMetricJSON(metric *model.Metrics) error {
	switch metric.MType {
	case "gauge":
		if metric.Value == nil {
			return errors.ErrInvalidValue
		}
		ms.storage.Set(metric.ID, strconv.FormatFloat(*metric.Value, 'f', -1, 64))
		return nil
	case "counter":
		if metric.Delta == nil {
			return errors.ErrInvalidValue
		}
		current := int64(0)
		if strValue, err := ms.storage.Get(metric.ID); err == nil {
			if val, err := strconv.ParseInt(strValue, 10, 64); err == nil {
				current = val
			}
		}
		newValue := current + *metric.Delta
		ms.storage.Set(metric.ID, fmt.Sprintf("%d", newValue))
		metric.Delta = &newValue
		return nil
	default:
		return errors.ErrInvalidType
	}
}

func (ms *MetricsService) GetMetricJSON(metric *model.Metrics) error {
	strValue, err := ms.storage.Get(metric.ID)
	if err != nil {
		return err
	}

	switch metric.MType {
	case "gauge":
		val, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			return errors.ErrInvalidValue
		}
		metric.Value = &val
		return nil
	case "counter":
		val, err := strconv.ParseInt(strValue, 10, 64)
		if err != nil {
			return errors.ErrInvalidValue
		}
		metric.Delta = &val
		return nil
	default:
		return errors.ErrInvalidType
	}
}
