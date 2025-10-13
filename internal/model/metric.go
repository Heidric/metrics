package model

// Metrics represents a single metric entry.
// For gauge, Value is set; for counter, Delta is set.
// ID holds the metric name, and MType is either "gauge" or "counter".
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}
