package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type Logger struct {
	zerolog zerolog.Logger
}

func NewLogger(ctx context.Context, config *Config) (*Logger, error) {
	logger := &Logger{}
	config.SetDefault()
	level, err := zerolog.ParseLevel(strings.ToLower(config.Level))
	if err != nil {
		return nil, errors.Wrap(err, "parse level")
	}

	zerolog.SetGlobalLevel(level)

	output := buildLoggerOutput(config.HumanFriendly, config.NoColoredOutput)

	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	l := zerolog.New(output).With().Timestamp().Logger()

	logger.zerolog = l

	return logger, nil
}

func (l *Logger) Zerolog() *zerolog.Logger {
	return &l.zerolog
}

func buildLoggerOutput(isHumanFriendly, isNoColoredOutput bool) io.Writer {
	if !isHumanFriendly {
		return os.Stderr
	}

	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    isNoColoredOutput,
		TimeFormat: time.RFC3339,
	}

	output.FormatLevel = func(i interface{}) string {
		var v string

		if ii, ok := i.(string); ok {
			ii = strings.ToUpper(ii)
			switch ii {
			case zerolog.DebugLevel.String(), zerolog.ErrorLevel.String(), zerolog.FatalLevel.String(),
				zerolog.InfoLevel.String(), zerolog.WarnLevel.String(), zerolog.PanicLevel.String(),
				zerolog.TraceLevel.String():
				v = fmt.Sprintf("%-5s", ii)
			default:
				v = ii
			}
		}

		return fmt.Sprintf("| %s |", v)
	}

	return output
}
