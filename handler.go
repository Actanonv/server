package server

import (
	"net/http"
)

type HandlerFunc func(Context) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(w, r)
	err := h(ctx)
	if err != nil {
		ctx.Log().Error(err.Error(), "code", http.StatusInternalServerError)

		_ = ctx.Error(http.StatusInternalServerError, err)
		return
	}
}
