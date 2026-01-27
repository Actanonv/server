package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	Context() context.Context
	ContextGet(key any, defa ...any) any
	ContextSet(key any, val any) *http.Request
	Request() *http.Request
	Response() http.ResponseWriter
	Render(status int, ctx RenderOpt) error
	JSON(status int, data JSONResponse) error
	Redirect(url string) error
	HTMX() *htmx.HTMX
	Trigger() *htmx.Trigger
	String(code int, out string) error
	// Status sets the response status code
	Status(code int) error
	Log() *slog.Logger
	Session() *SessionHelper
	RequestID() string
	UrlParam(key string) string
	Param(key string) string
	GetRoutePath(name string, params ...string) string
	StillStreaming(state bool)
}

type HandlerContext struct {
	w                http.ResponseWriter
	r                *http.Request
	hx               *htmx.HTMX
	hxTrigger        *htmx.Trigger
	srv              *Server
	streamingNotDone bool
}

func NewContext(w http.ResponseWriter, r *http.Request) *HandlerContext {
	ctx := &HandlerContext{w: w, r: r}
	ctx.hx = htmx.New(w, r)
	ctx.srv = r.Context().Value(CtxKeyServer).(*Server)
	ctx.hxTrigger = htmx.NewTrigger()
	return ctx
}

func (c *HandlerContext) Context() context.Context {
	return c.r.Context()
}

func (c *HandlerContext) ContextGet(key any, defa ...any) any {
	var dv any = ""
	if len(defa) > 0 {
		dv = defa[0]
	}

	retv := c.Context().Value(key)
	if retv == nil {
		return dv
	}

	return retv
}

func (c *HandlerContext) ContextSet(key any, val any) *http.Request {
	c.r = c.Request().WithContext(context.WithValue(c.Request().Context(), key, val))
	return c.r
}

func (c *HandlerContext) GetRoutePath(name string, params ...string) string {
	srv := c.Context().Value(CtxKeyServer).(*Server)
	if srv == nil {
		return ""
	}

	return srv.RouteName(name, params...)
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

func (c *HandlerContext) StillStreaming(state bool) {
	c.streamingNotDone = state
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

	if ctx.NotDone {
		c.streamingNotDone = true
		return nil
	}

	if !c.streamingNotDone {
		c.writeContentType(ContentTypeHTML)
		c.Response().WriteHeader(status)
	}
	_, err := io.Copy(c.Response(), out)
	return err
}

func (c *HandlerContext) JSON(status int, data JSONResponse) error {
	c.writeContentType(ContentTypeJSON)
	c.Response().WriteHeader(status)

	if data.Error != nil && data.ErrorType == "" {
		if status >= 500 {
			data.ErrorType = ErrorTypeServer
		} else {
			data.ErrorType = ErrorTypeApplication
		}
	}

	enc := json.NewEncoder(c.Response())
	if err := enc.Encode(data); err != nil {
		return err
	}

	return nil
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
	c.writeContentType(ContentTypeText)
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

func (c *HandlerContext) RequestID() string {
	reqID, ok := c.r.Context().Value(requestIDKey).(string)
	if ok && reqID != "" {
		return reqID
	}

	c.Log().Debug("RequestID not found in context. check that the RequestID middleware is setup")
	return ""
}

func (c *HandlerContext) UrlParam(key string) string {
	return c.Request().PathValue(key)
}

func (c *HandlerContext) Param(key string) string {
	return c.Request().FormValue(key)
}

const HeaderContentType = "Content-Type"

func (c *HandlerContext) writeContentType(value string) {
	header := c.Response().Header()
	if header.Get(HeaderContentType) == "" {
		header.Set(HeaderContentType, value)
	}
}

type SessionHelper struct {
	r    *http.Request
	sess *scs.SessionManager
}

func (h *SessionHelper) Get(key string) any {
	return h.sess.Get(h.r.Context(), key)
}

func (h *SessionHelper) Put(key string, val interface{}) {
	h.sess.Put(h.r.Context(), key, val)
}

func (h *SessionHelper) Exists(key string) bool {
	return h.sess.Exists(h.r.Context(), key)
}

func (h *SessionHelper) Mgr() *scs.SessionManager {
	return h.sess
}

func (c *HandlerContext) Session() *SessionHelper {
	retv := c.Request().Context().Value(CtxKeySessionMgr)
	if retv == nil {
		return nil
	}

	sess, ok := retv.(*scs.SessionManager)
	if !ok || sess == nil {
		return nil
	}
	return &SessionHelper{r: c.Request(), sess: sess}
}
