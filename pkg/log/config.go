package log

import "github.com/rs/zerolog"

type Config struct {
	HumanFriendly   bool   `envconfig:"optional"`
	NoColoredOutput bool   `envconfig:"optional"`
	Level           string `envconfig:"optional"`
}

func (c *Config) SetDefault() {
	if c.Level == "" {
		c.Level = zerolog.InfoLevel.String()
	}
}
