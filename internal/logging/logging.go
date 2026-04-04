package logging

import (
	"io"
	"log/slog"
	"os"
)

func Configure() {
	slog.SetDefault(NewJSONLogger(os.Stdout, slog.LevelInfo))
}

func NewJSONLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}
