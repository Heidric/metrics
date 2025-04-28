package cfg

import (
	"github.com/Heidric/metrics.git/pkg/log"
)

type Config struct {
	Logger *log.Config
}

func NewConfig() (*Config, error) {
	c := &Config{
		Logger: &log.Config{},
	}

	c.Logger.SetDefault()

	return c, nil
}
