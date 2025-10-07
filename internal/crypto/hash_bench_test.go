package crypto

import (
	"crypto/rand"
	"testing"
)

func BenchmarkHashSHA256_1KB(b *testing.B)  { benchHash(b, 1<<10) }
func BenchmarkHashSHA256_32KB(b *testing.B) { benchHash(b, 32<<10) }
func BenchmarkHashSHA256_1MB(b *testing.B)  { benchHash(b, 1<<20) }

func benchHash(b *testing.B, size int) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		b.Fatalf("rand: %v", err)
	}
	key := "bench-key"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = HashSHA256(data, key)
	}
}
