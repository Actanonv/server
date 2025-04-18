package server

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/mayowa/go-htmx"
	"github.com/mayowa/templates"
)

// RenderOpt is a short alias for templates.RenderOption
type RenderOpt = templates.RenderOption

var _ Context = &contextImpl{}

type Context interface {
	Request() *http.Request
	Response() http.ResponseWriter
	Render(status int, ctx RenderOpt) error
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

var ErrTemplatesNotInitialized = errors.New("templates not initialized")

func (c *contextImpl) Render(status int, ctx RenderOpt) error {
	if templateMgr == nil {
		return ErrTemplatesNotInitialized
	}

	out := new(bytes.Buffer)
	if err := templateMgr.Render(out, ctx); err != nil {
		return err
	}

	c.writeContentType("text/html; charset=utf-8")
	c.Response().WriteHeader(status)
	_, err := io.Copy(c.Response(), out)
	return err
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
