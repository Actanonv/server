package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/mayowa/go-htmx"
	"github.com/mayowa/templates"
)

// RenderOpt is a short alias for templates.RenderOption
type RenderOpt struct {
	Layout       string
	Template     string
	RenderString bool
	Others       []string
	Data         any
	NotDone      bool
}

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
	Error(code int, msg any, args ...errorPageCtxArg)
}

type contextImpl struct {
	w   http.ResponseWriter
	r   *http.Request
	hx  *htmx.HTMX
	srv *Server
}

func newContextImpl(w http.ResponseWriter, r *http.Request) *contextImpl {
	ctx := &contextImpl{w: w, r: r}
	ctx.hx = htmx.New(w, r)
	ctx.srv = r.Context().Value("_server_").(*Server)
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
	if c.srv != nil && c.srv.templateMgr == nil {
		return ErrTemplatesNotInitialized
	}

	out := new(bytes.Buffer)
	tplCtx := templates.RenderOption{
		Layout:       ctx.Layout,
		Template:     ctx.Template,
		RenderString: ctx.RenderString,
		Others:       ctx.Others,
		Data:         ctx.Data,
	}
	if err := c.srv.templateMgr.Render(out, tplCtx); err != nil {
		return err
	}

	if !ctx.NotDone {
		c.writeContentType("text/html; charset=utf-8")
		c.Response().WriteHeader(status)
	}
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
	if ok && logger != nil {
		return logger
	}

	if c.srv != nil && c.srv.log != nil {
		return c.srv.log
	}

	return appLog
}

type errorPageCtx struct {
	Msg  string
	Args []errorPageCtxArg
}
type errorPageCtxArg struct {
	Key   string
	Value any
}

// Error renders an error page with the specified status code and message, supporting HTMX-specific handling when applicable.
// The status code is used to determine the template name, e.g. 404.page or 404.hx
func (c *contextImpl) Error(statusCode int, msg any, args ...errorPageCtxArg) {
	suffix := "page"
	if c.HTMX().IsHxRequest() {
		suffix = "hx"
	}
	tplName := fmt.Sprintf("%d.%s", statusCode, suffix)

	errCtx := errorPageCtx{Msg: fmt.Sprintf("%v", msg), Args: args}
	if err := c.Render(statusCode, RenderOpt{Template: tplName, Data: errCtx}); err != nil {
		c.Log().Error("failed to render error page", "code", statusCode, "suffix", suffix, "error", err)
	}

	if c.HTMX().IsHxRequest() {
		trigger := htmx.NewTrigger().AddEventObject("serverCtxError", map[string]any{
			"code": statusCode,
			"msg":  errCtx.Msg,
			"args": errCtx.Args,
		})
		c.HTMX().TriggerAfterSwapWithObject(trigger)
	}

	c.Response().WriteHeader(statusCode)
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
