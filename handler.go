package server

import (
	"net/http"
)

type HandlerFunc func(Context) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newContextImpl(w, r)
	err := h(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
