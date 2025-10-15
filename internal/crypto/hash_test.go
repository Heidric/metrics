package crypto

import (
	"strings"
	"testing"
)

func TestHashSHA256_Vector1(t *testing.T) {
	key := strings.Repeat("\x0b", 20)
	msg := []byte("Hi There")
	exp := "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"
	got := HashSHA256(msg, key)
	if got != exp {
		t.Fatalf("HMAC-SHA256 mismatch:\n got: %s\nexp: %s", got, exp)
	}
}

func TestHashSHA256_Vector2(t *testing.T) {
	key := "Jefe"
	msg := []byte("what do ya want for nothing?")
	exp := "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
	got := HashSHA256(msg, key)
	if got != exp {
		t.Fatalf("HMAC-SHA256 mismatch:\n got: %s\nexp: %s", got, exp)
	}
}

func TestHashSHA256_EmptyKeyAndData(t *testing.T) {
	key := ""
	msg := []byte("")
	exp := "b613679a0814d9ec772f95d778c35fc5ff1697c493715653c6c712144292c5ad"
	got := HashSHA256(msg, key)
	if got != exp {
		t.Fatalf("HMAC-SHA256(\"\") mismatch:\n got: %s\nexp: %s", got, exp)
	}
}

func TestHashSHA256_NonASCII(t *testing.T) {
	key := "ключ"
	msg := []byte("данные")
	got := HashSHA256(msg, key)
	if len(got) != 64 {
		t.Fatalf("hex length = %d, want 64", len(got))
	}
	for _, c := range got {
		if c >= 'A' && c <= 'F' {
			t.Fatalf("hex not lowercase: %s", got)
		}
	}
}
