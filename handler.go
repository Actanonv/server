package server

import (
	"net/http"
)

type HandlerFunc func(Context) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newContextImpl(w, r)
	err := h(ctx)
	if err != nil {
		ctx.Log().Error(err.Error(), "code", http.StatusInternalServerError)
		ctx.Error(http.StatusInternalServerError, err.Error(), errorPageCtxArg{
			Key: "code", Value: http.StatusInternalServerError,
		})
		return
	}
}
