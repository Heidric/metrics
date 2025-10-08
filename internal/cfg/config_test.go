package cfg

import (
	"os"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	os.Setenv("X", "10")
	if got := parseDuration("X", 5*time.Second); got != 10*time.Second {
		t.Fatalf("got %v", got)
	}
	os.Setenv("X", "250ms")
	if got := parseDuration("X", 5*time.Second); got != 250*time.Millisecond {
		t.Fatalf("got %v", got)
	}
	os.Unsetenv("X")
	if got := parseDuration("X", 5*time.Second); got != 5*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestParseBool(t *testing.T) {
	os.Setenv("B", "true")
	if !parseBool("B", false) {
		t.Fatal("expected true")
	}
	os.Setenv("B", "false")
	if parseBool("B", true) {
		t.Fatal("expected false")
	}
	os.Unsetenv("B")
	if !parseBool("B", true) {
		t.Fatal("expected default true")
	}
}
