package server

import (
	"context"
	"net/http"

	"log/slog"

	"github.com/google/uuid"
)

// Define custom types for context keys
type contextKey string

const (
	requestIDKey    contextKey = "requestID"
	scopedLoggerKey contextKey = "scopedLogger"
)

// ResponseWriter a response writer that captures the status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

const RequestIDHeaderKey string = "X-Request-ID"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, scopedLoggerKey, appLog.With("reqID", requestID))
		*r = *r.WithContext(ctx)
		w.Header().Set(RequestIDHeaderKey, requestID)
		next.ServeHTTP(w, r)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("Recovered from panic", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
