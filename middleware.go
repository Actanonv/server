package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

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
	committed  bool
	intercept  bool
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	if rw.committed {
		return
	}

	if rw.intercept && (statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed) {
		rw.statusCode = statusCode
		return
	}

	rw.statusCode = statusCode
	rw.committed = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.committed {
		if rw.intercept && (rw.statusCode == http.StatusNotFound || rw.statusCode == http.StatusMethodNotAllowed) {
			return len(b), nil
		}
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *ResponseWriter) Committed() bool {
	return rw.committed
}

func (rw *ResponseWriter) Intercept(v bool) {
	rw.intercept = v
}

const RequestIDHeaderKey string = "X-Request-ID"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srvr := r.Context().Value("_server_")
		logr := appLog
		if srvr != nil && srvr.(*Server).log != nil {
			logr = srvr.(*Server).log
		}

		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, scopedLoggerKey, logr.With("reqID", requestID))
		*r = *r.WithContext(ctx)
		w.Header().Set(RequestIDHeaderKey, requestID)
		next.ServeHTTP(w, r)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				srvr := r.Context().Value(CtxKeyServer)
				var logr *slog.Logger = appLog
				var debugMode bool

				if s, ok := srvr.(*Server); ok && s != nil {
					if s.log != nil {
						logr = s.log
					}
					debugMode = s.debug
				}

				stack := debug.Stack()
				logr.Error("Recovered from panic (middleware)", "error", rec, "stack", string(stack))

				msg := http.StatusText(http.StatusInternalServerError)
				if debugMode {
					msg = fmt.Sprintf("panic: %v\n%s", rec, string(stack))
				}
				http.Error(w, msg, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
