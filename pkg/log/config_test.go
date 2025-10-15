package log

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestConfig_SetDefault_SetsInfoWhenEmpty(t *testing.T) {
	c := &Config{}
	c.SetDefault()
	if c.Level != zerolog.InfoLevel.String() {
		t.Fatalf("Level = %q, want %q", c.Level, zerolog.InfoLevel.String())
	}
}

func TestConfig_SetDefault_PreservesExisting(t *testing.T) {
	c := &Config{Level: zerolog.DebugLevel.String()}
	c.SetDefault()
	if c.Level != zerolog.DebugLevel.String() {
		t.Fatalf("Level changed = %q, want %q", c.Level, zerolog.DebugLevel.String())
	}
}

func TestConfig_SetDefault_DoesNotTouchOtherFields(t *testing.T) {
	c := &Config{HumanFriendly: true, NoColoredOutput: true}
	c.SetDefault()
	if !c.HumanFriendly || !c.NoColoredOutput {
		t.Fatalf("flags were modified by SetDefault")
	}
}
