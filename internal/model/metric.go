package model

type Metrics struct {
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`

	ID    string `json:"id"`
	MType string `json:"type"`
}
