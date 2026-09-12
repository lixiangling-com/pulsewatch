package logging

import (
	"log/slog"
	"os"
)

func New(service, environment string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With(
		slog.String("service", service),
		slog.String("environment", environment),
	)
}
