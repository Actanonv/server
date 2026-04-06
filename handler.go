package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

type HandlerFunc func(Context) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(w, r)
	if ctx == nil {
		slog.Error("Failed to create context: server instance missing from request context")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	srv := ctx.srv

	defer func() {
		if rec := recover(); rec != nil {
			stack := debug.Stack()
			ctx.Log().Error("panic recovered", "panic", rec, "stack", string(stack))

			if srv != nil && srv.errorFunc != nil {
				// Wrap ErrorFunc in its own recovery block to prevent secondary panics
				defer func() {
					if rec2 := recover(); rec2 != nil {
						ctx.Log().Error("panic in ErrorFunc (during panic recovery)", "panic", rec2, "stack", string(debug.Stack()))
						if rw, ok := w.(*ResponseWriter); ok && !rw.Committed() {
							http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						} else if !ok {
							http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						}
					}
				}()
				// Pass a structured error if possible, or at least a cleaner one
				srv.errorFunc(ctx, fmt.Errorf("panic: %v", rec))
			} else {
				msg := http.StatusText(http.StatusInternalServerError)
				if srv != nil && srv.debug {
					msg = fmt.Sprintf("panic: %v\n%s", rec, string(stack))
				}
				http.Error(w, msg, http.StatusInternalServerError)
			}
		}
	}()

	err := h(ctx)
	if err != nil {
		// Only log if no ErrorFunc is provided to avoid redundancy
		if srv == nil || srv.errorFunc == nil {
			ctx.Log().Error("internal server error", "err", err, "code", http.StatusInternalServerError)
		}

		if srv != nil && srv.errorFunc != nil {
			// Wrap ErrorFunc in its own recovery block
			defer func() {
				if rec2 := recover(); rec2 != nil {
					ctx.Log().Error("panic in ErrorFunc (during error handling)", "panic", rec2, "stack", string(debug.Stack()))
					if rw, ok := w.(*ResponseWriter); ok && !rw.Committed() {
						http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					} else if !ok {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()
			srv.errorFunc(ctx, err)
		} else {
			msg := "Internal Server Error"
			if srv != nil && srv.debug {
				msg = err.Error()
			}
			http.Error(w, msg, http.StatusInternalServerError)
		}

		return
	}
}
