package server

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
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
	Redirect(status int, url string) error
	HTMX() *htmx.HTMX
	String(code int, out string) error
	Log() *slog.Logger
	Session() *sessHelper
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

func (c *contextImpl) Redirect(statusCode int, url string) error {
	http.Redirect(c.Response(), c.Request(), url, statusCode)
	return nil
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
	logger, ok := c.r.Context().Value(scopedLoggerKey).(*slog.Logger)
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

type sessHelper struct {
	r    *http.Request
	sess *scs.SessionManager
}

func (h *sessHelper) Get(key string) any {
	return h.sess.Get(h.r.Context(), key)
}

func (h *sessHelper) Put(key string, val interface{}) {
	h.sess.Put(h.r.Context(), key, val)
}

func (h *sessHelper) Mgr() *scs.SessionManager {
	return h.sess
}

func (c *contextImpl) Session() *sessHelper {
	retv := c.Request().Context().Value("_sessMgr_")
	if retv == nil {
		return nil
	}

	sess, ok := retv.(*scs.SessionManager)
	if !ok || sess == nil {
		return nil
	}
	return &sessHelper{r: c.Request(), sess: sess}
}
