package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Init("info", "json", &buf)

	Log().Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("expected JSON output with level:info, got: %s", output)
	}
	if !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("expected JSON output with message, got: %s", output)
	}
}

func TestInitLogger_ConsoleFormat(t *testing.T) {
	var buf bytes.Buffer
	Init("info", "console", &buf)

	Log().Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, "INF") {
		t.Errorf("expected console output with INF, got: %s", output)
	}
}

func TestInitLogger_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	Init("debug", "json", &buf)

	Log().Debug().Msg("debug message")

	output := buf.String()
	if !strings.Contains(output, `"level":"debug"`) {
		t.Errorf("expected debug level in output, got: %s", output)
	}
}
