package utils

import (
	"github.com/rs/zerolog"
	"os"
	"time"
)

func GetLogger() zerolog.Logger {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	return logger
}
