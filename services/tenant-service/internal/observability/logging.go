package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger writes one JSON object per line using the ReserveHub log schema.
func NewLogger(output io.Writer, service, environment, levelName string) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(strings.TrimSpace(levelName))); err != nil {
		level = slog.LevelInfo
	}
	if environment == "" {
		environment = "local"
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 {
				switch attr.Key {
				case slog.TimeKey:
					attr.Key = "timestamp"
					attr.Value = slog.TimeValue(attr.Value.Time().UTC())
				case slog.MessageKey:
					attr.Key = "message"
				}
			}
			return attr
		},
	})
	return slog.New(handler).With("service", service, "environment", environment)
}

func DefaultLogger(service, environment, levelName string) *slog.Logger {
	return NewLogger(os.Stdout, service, environment, levelName)
}
