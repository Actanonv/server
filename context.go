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
)

// RenderOpt is a short alias for templates.RenderOption
type RenderOpt struct {
	Layout       string
	Template     string
	RenderString bool
	Others       []string
	Data         any
	// NotDone prevents Render() from calling c.Response().WriteHeader(status)
	NotDone bool
}

var _ Context = &HandlerContext{}

type Context interface {
	Request() *http.Request
	Response() http.ResponseWriter
	Render(status int, ctx RenderOpt) error
	Redirect(url string) error
	HTMX() *htmx.HTMX
	Trigger() *htmx.Trigger
	String(code int, out string) error
	// Status sets the response status code
	Status(code int) error
	Log() *slog.Logger
	Session() *sessHelper
	Error(code int, msg any, args ...errorPageCtxArg) error
}

type HandlerContext struct {
	w         http.ResponseWriter
	r         *http.Request
	hx        *htmx.HTMX
	hxTrigger *htmx.Trigger
	srv       *Server
	errSet    bool
}

func NewContext(w http.ResponseWriter, r *http.Request) *HandlerContext {
	ctx := &HandlerContext{w: w, r: r}
	ctx.hx = htmx.New(w, r)
	ctx.srv = r.Context().Value(CtxKeyServer).(*Server)
	ctx.hxTrigger = htmx.NewTrigger()
	return ctx
}

func (c *HandlerContext) Request() *http.Request {
	return c.r
}

func (c *HandlerContext) Response() http.ResponseWriter {
	return c.w
}

var ErrRendererNotProvided = errors.New("templates renderer not provided")

type Renderer interface {
	Render(w io.Writer, ctx RenderOpt) error
}

func (c *HandlerContext) Render(status int, ctx RenderOpt) error {
	if c.srv != nil && c.srv.templateMgr == nil {
		return ErrRendererNotProvided
	}

	var rdr Renderer = c.srv.templateMgr

	out := new(bytes.Buffer)
	if err := rdr.Render(out, ctx); err != nil {
		return err
	}

	if !ctx.NotDone {
		c.writeContentType("text/html; charset=utf-8")
		c.Response().WriteHeader(status)
	}
	_, err := io.Copy(c.Response(), out)
	return err
}

func (c *HandlerContext) Redirect(url string) error {
	if c.HTMX().IsHxRequest() {
		c.HTMX().Redirect(url)
		return nil
	}

	http.Redirect(c.Response(), c.Request(), url, http.StatusSeeOther)
	return nil
}

func (c *HandlerContext) HTMX() *htmx.HTMX {
	return c.hx
}

func (c *HandlerContext) Trigger() *htmx.Trigger {
	return c.hxTrigger
}

func (c *HandlerContext) String(code int, out string) error {
	c.writeContentType("text/plain; charset=utf-8")
	c.Response().WriteHeader(code)

	_, err := c.Response().Write([]byte(out))
	return err
}

func (c *HandlerContext) Status(code int) error {
	c.Response().WriteHeader(code)
	return nil
}

func (c *HandlerContext) Log() *slog.Logger {
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
func (c *HandlerContext) Error(statusCode int, msg any, args ...errorPageCtxArg) error {
	if c.errSet {
		c.Log().Info("ctx.Error() ctx.isSet=true", "code", statusCode, "error", msg, "args", args)
		return nil
	}

	suffix := "page"
	// if c.HTMX().IsHxRequest() {
	// 	suffix = "hx"
	// }
	tplName := fmt.Sprintf("%d.%s", statusCode, suffix)
	c.errSet = true
	errCtx := errorPageCtx{Args: args}
	msgIsError := false
	switch m := msg.(type) {
	default:
		errCtx.Msg = fmt.Sprintf("%v", m)
	case error:
		errCtx.Msg = m.Error()
		msgIsError = true
	}

	if c.HTMX().IsHxRequest() {
		// deliberately ignores c.Trigger() so as to override it
		trigger := htmx.NewTrigger().AddEventObject("serverCtxError", map[string]any{
			"code": statusCode,
			"msg":  errCtx.Msg,
			"args": errCtx.Args,
		})
		c.HTMX().TriggerAfterSwapWithObject(trigger)
	} else {
		if err := c.Render(statusCode, RenderOpt{Template: tplName, Data: errCtx}); err != nil {
			c.Log().Error("failed to render error page", "code", statusCode, "suffix", suffix, "error", err)
			return fmt.Errorf("failed to render error page: %w", err)
		}
	}

	c.Response().WriteHeader(statusCode)
	if msgIsError {
		return msg.(error)
	}

	return nil
}

const HeaderContentType = "Content-Type"

func (c *HandlerContext) writeContentType(value string) {
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

func (h *sessHelper) Exists(key string) bool {
	return h.sess.Exists(h.r.Context(), key)
}

func (h *sessHelper) Mgr() *scs.SessionManager {
	return h.sess
}

func (c *HandlerContext) Session() *sessHelper {
	retv := c.Request().Context().Value(CtxKeySessionMgr)
	if retv == nil {
		return nil
	}

	sess, ok := retv.(*scs.SessionManager)
	if !ok || sess == nil {
		return nil
	}
	return &sessHelper{r: c.Request(), sess: sess}
}
