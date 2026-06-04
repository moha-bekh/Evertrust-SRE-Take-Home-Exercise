package observability

import (
	"io"
	"log/slog"
)

func NewLogger(out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{}))
}
