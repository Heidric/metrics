package model

import (
	"encoding/json"
	"testing"
)

func fptr(f float64) *float64 { return &f }
func iptr(i int64) *int64     { return &i }

func TestMarshal_Gauge_IncludesOnlyValue(t *testing.T) {
	m := Metrics{
		ID:    "alloc",
		MType: GaugeType,
		Value: fptr(123.45),
		Delta: nil,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal gauge: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if _, ok := obj["delta"]; ok {
		t.Fatalf("gauge JSON must not contain delta: %s", string(b))
	}
	if _, ok := obj["value"]; !ok {
		t.Fatalf("gauge JSON must contain value: %s", string(b))
	}
	if obj["type"] != GaugeType || obj["id"] != "alloc" {
		t.Fatalf("id/type mismatch: %v", obj)
	}
}

func TestMarshal_Counter_IncludesOnlyDelta(t *testing.T) {
	m := Metrics{
		ID:    "requests",
		MType: CounterType,
		Delta: iptr(7),
		Value: nil,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal counter: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if _, ok := obj["value"]; ok {
		t.Fatalf("counter JSON must not contain value: %s", string(b))
	}
	if _, ok := obj["delta"]; !ok {
		t.Fatalf("counter JSON must contain delta: %s", string(b))
	}
}

func TestMarshal_ZeroValuesArePreserved(t *testing.T) {
	zeroF := fptr(0.0)
	mGauge := Metrics{ID: "zeroGauge", MType: GaugeType, Value: zeroF}
	b, err := json.Marshal(mGauge)
	if err != nil {
		t.Fatalf("marshal zero gauge: %v", err)
	}
	if want := `"value":0`; !contains(string(b), want) {
		t.Fatalf("expected %q in %s", want, string(b))
	}

	zeroI := iptr(0)
	mCounter := Metrics{ID: "zeroCounter", MType: CounterType, Delta: zeroI}
	b2, err := json.Marshal(mCounter)
	if err != nil {
		t.Fatalf("marshal zero counter: %v", err)
	}
	if want := `"delta":0`; !contains(string(b2), want) {
		t.Fatalf("expected %q in %s", want, string(b2))
	}
}

func TestJSON_RoundTrip(t *testing.T) {
	orig := Metrics{
		ID:    "cpu",
		MType: GaugeType,
		Value: fptr(0.25),
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Metrics
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if orig.ID != back.ID || orig.MType != back.MType {
		t.Fatalf("id/type mismatch: %+v vs %+v", orig, back)
	}
	if back.Value == nil || *back.Value != 0.25 {
		t.Fatalf("value lost after roundtrip: %+v", back)
	}
	if back.Delta != nil {
		t.Fatalf("delta must remain nil for gauge: %+v", back)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
