package crypto

import "testing"

func TestHashSHA256_Deterministic(t *testing.T) {
	got1 := HashSHA256([]byte("data"), "k")
	got2 := HashSHA256([]byte("data"), "k")
	if got1 != got2 {
		t.Fatalf("expected deterministic hash, got %q vs %q", got1, got2)
	}
}

func TestHashSHA256_KeyMatters(t *testing.T) {
	a := HashSHA256([]byte("data"), "k1")
	b := HashSHA256([]byte("data"), "k2")
	if a == b {
		t.Fatalf("expected different hashes for different keys")
	}
}
