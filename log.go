package server

import (
	"log/slog"
	"os"
)

var appLog *slog.Logger

func init() {
	appLog = slog.Default()
}

type ENVTypes string

const (
	ENVProduction = "production"
	ENVStaging    = "staging"
	ENVTest       = "test"
	ENVDev        = "dev"
)

func InitLog(env ENVTypes, logLevel slog.Level, setLogDefault bool) error {

	option := &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}

	if env == ENVProduction || env == ENVStaging {
		appLog = slog.New(slog.NewJSONHandler(os.Stdout, option))
	} else {
		appLog = slog.New(slog.NewTextHandler(os.Stderr, option))
	}

	if setLogDefault {
		slog.SetDefault(appLog)
	}

	return nil
}
