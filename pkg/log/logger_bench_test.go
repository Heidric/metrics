package log

import (
	"context"
	"testing"
)

func BenchmarkNewLogger(b *testing.B) {
	cfg := &Config{HumanFriendly: false, NoColoredOutput: true, Level: "info"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := NewLogger(context.Background(), cfg)
		if err != nil {
			b.Fatalf("NewLogger: %v", err)
		}
	}
}

func BenchmarkLogWrite_SteadyState(b *testing.B) {
	cfg := &Config{HumanFriendly: false, NoColoredOutput: true, Level: "info"}
	l, err := NewLogger(context.Background(), cfg)
	if err != nil {
		b.Fatalf("NewLogger: %v", err)
	}
	zl := l.Zerolog()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zl.Info().Str("component", "bench").Msg("ping")
	}
}
