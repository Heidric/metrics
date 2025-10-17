package log

import (
	"context"
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/rs/zerolog"
)

func captureStderr(t *testing.T, fn func()) string {
	saved := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = saved }()

	fn()

	_ = w.Close()
	b, _ := io.ReadAll(r)
	_ = r.Close()
	return string(b)
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	cfg := &Config{Level: "not-a-level"}
	_, err := NewLogger(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error for invalid level")
	}
}

func TestBuildLoggerOutput_Types(t *testing.T) {
	w := buildLoggerOutput(false, false)
	if w != os.Stderr {
		t.Fatalf("non-human output should be os.Stderr")
	}
	w2 := buildLoggerOutput(true, true)
	if _, ok := w2.(zerolog.ConsoleWriter); !ok {
		t.Fatalf("human-friendly output should be ConsoleWriter")
	}
}

func TestNewLogger_SetsGlobalLevel(t *testing.T) {
	saved := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(saved)

	cfg := &Config{Level: "error"}
	_, err := NewLogger(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := zerolog.GlobalLevel(); got != zerolog.ErrorLevel {
		t.Fatalf("GlobalLevel=%v, want %v", got, zerolog.ErrorLevel)
	}
}

func TestNewLogger_JSONFormat_ContainsLevelAndMessage(t *testing.T) {
	cfg := &Config{
		Level:           "info",
		HumanFriendly:   false,
		NoColoredOutput: true,
	}
	out := captureStderr(t, func() {
		lg, err := NewLogger(context.Background(), cfg)
		if err != nil {
			t.Fatalf("NewLogger: %v", err)
		}
		lg.Zerolog().Info().Msg("ping")
	})
	if !regexp.MustCompile(`\"level\":\"info\"`).MatchString(out) {
		t.Fatalf("json out missing level: %s", out)
	}
	if !regexp.MustCompile(`\"message\":\"ping\"`).MatchString(out) {
		t.Fatalf("json out missing message: %s", out)
	}
}

func TestNewLogger_Console_NoColor_NoANSI(t *testing.T) {
	cfg := &Config{
		Level:           "info",
		HumanFriendly:   true,
		NoColoredOutput: true,
	}
	out := captureStderr(t, func() {
		lg, err := NewLogger(context.Background(), cfg)
		if err != nil {
			t.Fatalf("NewLogger: %v", err)
		}
		lg.Zerolog().Info().Msg("hello")
	})
	levelRe := regexp.MustCompile(`(?i)\binfo\b|\binf\b`)
	if !levelRe.MatchString(out) {
		t.Fatalf("console out missing level token: %q", out)
	}
	if !regexp.MustCompile(`hello`).MatchString(out) {
		t.Fatalf("console out missing message: %q", out)
	}
	ansi := regexp.MustCompile(`\\x1b\\[[0-9;]*m`)
	if ansi.MatchString(out) {
		t.Fatalf("console out must not contain ANSI codes when NoColoredOutput: %q", out)
	}
}
