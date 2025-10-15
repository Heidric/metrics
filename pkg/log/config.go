package log

import "github.com/rs/zerolog"

type Config struct {
	Level           string `envconfig:"optional"`
	HumanFriendly   bool   `envconfig:"optional"`
	NoColoredOutput bool   `envconfig:"optional"`
}

func (c *Config) SetDefault() {
	if c.Level == "" {
		c.Level = zerolog.InfoLevel.String()
	}
}
