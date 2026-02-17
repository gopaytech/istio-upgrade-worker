package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// Init initializes the global logger with specified level and format.
// format: "json" (default) or "console"
// level: "debug", "info", "warn", "error" (default: "info")
func Init(level, format string, w ...io.Writer) {
	var output io.Writer = os.Stdout
	if len(w) > 0 {
		output = w[0]
	}

	if strings.ToLower(format) == "console" {
		output = zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}
	}

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	log = zerolog.New(output).Level(lvl).With().Timestamp().Logger()
}

// Log returns the global logger instance.
func Log() *zerolog.Logger {
	return &log
}
