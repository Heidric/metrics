package log

import "github.com/rs/zerolog"

type Config struct {
	// HumanFriendly enable writes log in human-friendly format to Out
	HumanFriendly bool `envconfig:"optional"`
	// NoColoredOutput forces logger to output things without
	// shell colorcodes.
	NoColoredOutput bool `envconfig:"optional"`
	// Level is a logger's loglevel. Possible values: "DEBUG",
	// "INFO", "WARN", "ERROR", "FATAL", "TRACE". Setting this variable
	// to any other value will force INFO level.
	// Case-insensitive value.
	Level string `envconfig:"optional"`
}

func (c *Config) SetDefault() {
	if c.Level == "" {
		c.Level = zerolog.InfoLevel.String()
	}
}
