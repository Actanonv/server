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
		slog.Error("Failed to create context")
		return
	}

	defer func() {
		if rec := recover(); rec != nil {
			ctx.Log().Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))

			srv, ok := ctx.ContextGet(CtxKeyServer).(*Server)
			if ok && srv != nil && srv.errorFunc != nil {
				panicErr := fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
				srv.errorFunc(ctx, panicErr)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}
	}()

	err := h(ctx)
	if err != nil {
		ctx.Log().Error("internal server error", "err", err, "code", http.StatusInternalServerError)

		srv, ok := ctx.ContextGet(CtxKeyServer).(*Server)
		if ok && srv != nil && srv.errorFunc != nil {
			srv.errorFunc(ctx, err)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}
}
