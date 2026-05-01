package observability

import (
	"io"
	"os"
	"time"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/rs/zerolog"
)

// Setup initializes the global zerolog logger and returns a request-scoped logger factory.
// LOG_PRETTY=false (default) outputs newline-delimited JSON — compatible with Loki/Grafana.
// LOG_PRETTY=true outputs human-readable colorized output for local development.
func Setup(cfg config.LogConfig) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var w io.Writer = os.Stdout
	if cfg.Pretty {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}
	}

	logger := zerolog.New(w).With().
		Timestamp().
		Str("service", "qr-dining").
		Logger()

	return logger
}
