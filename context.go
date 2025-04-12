package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/mayowa/go-htmx"
	"github.com/mayowa/templates"
)

var _ Context = (&contextImpl{})

type Context interface {
	Request() *http.Request
	Response() http.ResponseWriter
	Render(ctx templates.RenderOption) error
	HTMX() *htmx.HTMX
	String(code int, out string) error
	Log() *slog.Logger
}

type contextImpl struct {
	w  http.ResponseWriter
	r  *http.Request
	hx *htmx.HTMX
}

func newContextImpl(w http.ResponseWriter, r *http.Request) *contextImpl {
	ctx := &contextImpl{w: w, r: r}
	ctx.hx = htmx.New(w, r)
	return ctx
}

func (c *contextImpl) Request() *http.Request {
	return c.r
}

func (c *contextImpl) Response() http.ResponseWriter {
	return c.w
}

var ErrTemplatesNotInitialzed = errors.New("templates not initialized")

func (c *contextImpl) Render(ctx templates.RenderOption) error {
	if templateMgr == nil {
		return ErrTemplatesNotInitialzed
	}
	return templateMgr.Render(c.Response(), ctx)
}

func (c *contextImpl) HTMX() *htmx.HTMX {
	return c.hx
}

func (c *contextImpl) String(code int, out string) error {
	c.writeContentType("text/plain; charset=utf-8")
	c.Response().WriteHeader(code)

	_, err := c.Response().Write([]byte(out))
	return err
}

func (c *contextImpl) Log() *slog.Logger {
	logger, ok := c.r.Context().Value("scopedLogger").(*slog.Logger)
	if !ok || logger == nil {
		return appLog
	}
	return logger
}

const HeaderContentType = "Content-Type"

func (c *contextImpl) writeContentType(value string) {
	header := c.Response().Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}
