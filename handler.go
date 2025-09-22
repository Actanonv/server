package server

import (
	"net/http"
)

type HandlerFunc func(Context) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newContextImpl(w, r)
	err := h(ctx)
	if err != nil {
		ctx.Log().Error("error", err)
		ctx.Response().WriteHeader(http.StatusInternalServerError)
		return
	}
}

func ErrorPage(ctx Context, err error) {
	if err := ctx.Render(http.StatusInternalServerError, RenderOpt{Template: "500.page"}); err != nil {

		ctx.Log().Error("error", err)
	}
}
