package log

import "github.com/rs/zerolog"

// Config controls logger formatting and level.
// Fields can be populated from environment (see struct tags) and then used
// to construct a zerolog-based logger with the desired output format.
type Config struct {
	HumanFriendly   bool   `envconfig:"optional"` // when true, use a more readable (non-JSON) format
	NoColoredOutput bool   `envconfig:"optional"` // disable ANSI colors in human-friendly output
	Level           string `envconfig:"optional"` // log level name, e.g. "debug", "info", "warn", "error"
}

// SetDefault sets sane defaults on the configuration.
// If Level is empty, it defaults to zerolog.InfoLevel.
func (c *Config) SetDefault() {
	if c.Level == "" {
		c.Level = zerolog.InfoLevel.String()
	}
}
