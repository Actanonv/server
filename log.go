package server

import (
	"log/slog"
	"os"
)

var appLog *slog.Logger

func init() {
	appLog = slog.Default()
}

func InitLog(env string, logLevel slog.Level, setLogDefault bool) error {

	option := &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}

	if env == "production" {
		appLog = slog.New(slog.NewJSONHandler(os.Stdout, option))
	} else {
		appLog = slog.New(slog.NewTextHandler(os.Stderr, option))
	}

	if setLogDefault {
		slog.SetDefault(appLog)
	}

	return nil
}
