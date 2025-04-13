package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
		appLog = slog.New(NewCustomHandler(os.Stderr, option))
	}

	if setLogDefault {
		slog.SetDefault(appLog)
	}

	return nil
}

// CustomHandler is a custom slog handler that displays time (YYYY/MM/DD HH:MM:SS)
// without a key, followed by Level, Message and any other provided attributes
type CustomHandler struct {
	slog.Handler
	opts *slog.HandlerOptions
	w    *os.File
}

// NewCustomHandler creates a new CustomHandler that writes to w
func NewCustomHandler(w *os.File, opts *slog.HandlerOptions) *CustomHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	return &CustomHandler{
		Handler: slog.NewTextHandler(w, opts),
		opts:    opts,
		w:       w,
	}
}

// Handle implements slog.Handler.Handle
func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	timeStr := r.Time.Format("2006/01/02 15:04:05")

	// Create a new record with the formatted time and same attributes
	attrs := make([]slog.Attr, 0)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	// Build the log line with custom format
	level := r.Level.String()
	message := r.Message

	// Start with formatted time
	line := timeStr + " " + level + " " + message

	// Create a buffer for the attributes
	var buf strings.Builder
	for _, attr := range attrs {
		buf.WriteString(" ")
		buf.WriteString(attr.Key)
		buf.WriteString("=")
		buf.WriteString(attr.Value.String())
	}

	line += buf.String()

	// Write to the output
	_, err := fmt.Fprintln(h.w, line)
	return err
}

// WithAttrs implements slog.Handler.WithAttrs
func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomHandler{
		Handler: h.Handler.WithAttrs(attrs),
		opts:    h.opts,
	}
}

// WithGroup implements slog.Handler.WithGroup
func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{
		Handler: h.Handler.WithGroup(name),
		opts:    h.opts,
	}
}
