package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/mayowa/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	options := MuxOptions{
		Host:        "localhost",
		Port:        4000,
		Public:      "",
		Middlewares: nil,
		Log:         nil,
		Templates:   nil,
		SessionMgr:  nil,
	}

	srv := Init(options)

	assert := assert.New(t)
	assert.NotNil(srv)
	assert.Equal(options.Host, srv.Host)
	assert.Equal(options.Port, srv.Port)
	assert.Equal(options.Public, srv.Public)
	assert.Equal(options.Middlewares, srv.Middleware)
	assert.Equal(srv.log, appLog)
	assert.Equal(options.LogRequests, srv.logRequests)
	assert.Equal(options.Templates, srv.templates)
	assert.Equal(options.SessionMgr, srv.sessionMgr)
}

func runServerForTest(t *testing.T, options MuxOptions, reqUrl string) (*http.Response, error) {
	t.Helper()

	var err error

	srv := Init(options)
	if srv == nil {
		return nil, errors.New("server not initialized")
	}

	if err = srv.Route(); err != nil {
		return nil, err
	}

	err = nil
	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + reqUrl)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}

	return resp, nil
}

func TestServerRendering(t *testing.T) {

	tMgr, err := InitTemplates("./testData/templates", templates.TemplateOptions{
		Ext: ".tmpl",
	})
	require.NoError(t, err)

	test := []struct {
		name           string
		route          string
		url            string
		expectedStatus int
		expectedBody   string
		handler        HandlerFunc
	}{
		{
			name:           "render string",
			route:          "GET /hello",
			url:            "/hello",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
			handler: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello, World!")
			},
		},

		{
			name:           "render template",
			route:          "GET /test-template",
			url:            "/test-template",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
			handler: func(ctx Context) error {
				return ctx.Render(http.StatusOK, templates.RenderOption{
					Template: "hello",
				})
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			options := defaultOptions
			options.Templates = tMgr
			options.Routes = []Route{
				{Match: tt.route, Handler: tt.handler},
			}

			resp, err := runServerForTest(t, options, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, string(body))
		})
	}
}

var defaultOptions = MuxOptions{
	Host: "localhost",
	Port: 8923,
}

func TestServerMiddleware(t *testing.T) {

	tests := []struct {
		name           string
		route          string
		url            string
		expectedStatus int
		middleware     []Middleware
		handler        HandlerFunc
	}{
		{
			name:           "test reqId",
			route:          "GET /req-id",
			url:            "/req-id",
			expectedStatus: http.StatusOK,
			middleware: []Middleware{
				RequestIDMiddleware,
			},
			handler: func(ctx Context) error {
				temp := ctx.Request().Context().Value(requestIDKey)
				rId, ok := temp.(string)
				if !ok {
					return fmt.Errorf("requestId not string")
				}

				if rId == "" {
					return fmt.Errorf("request ID empty string")
				}

				headerRId := ctx.Response().Header().Get(RequestIDHeaderKey)
				if headerRId == "" {
					return fmt.Errorf("no request id in header")
				}

				if headerRId != rId {
					return fmt.Errorf("context value doesn't match header value")
				}
				return nil
			},
		},
		{
			name:           "test panic recovery",
			route:          "GET /panic",
			url:            "/panic",
			expectedStatus: http.StatusInternalServerError,
			middleware: []Middleware{
				RecoveryMiddleware,
			},
			handler: func(ctx Context) error {
				panic("catch me if you can")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := defaultOptions
			options.Middlewares = tt.middleware
			options.Routes = []Route{
				{Match: tt.route, Handler: tt.handler},
			}

			resp, err := runServerForTest(t, options, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestServerSessions(t *testing.T) {
	const sessionKey string = "test"
	const sessionValue string = "test"
	sessionManager := scs.New()

	tests := []struct {
		name           string
		route          string
		url            string
		expectedStatus int
		middleware     []Middleware
		handler        HandlerFunc
	}{
		{
			name:           "test write",
			route:          "GET /write",
			url:            "/write",
			expectedStatus: 200,
			handler: func(ctx Context) error {
				ctx.Session().Put(sessionKey, sessionValue)
				return nil
			},
		},
		{
			name:           "test read",
			route:          "GET /read",
			url:            "/read",
			expectedStatus: 200,
			handler: func(ctx Context) error {
				ctx.Session().Put(sessionKey, sessionValue)

				v := ctx.Session().Get(sessionKey)
				val, ok := v.(string)

				if !ok {
					return fmt.Errorf("value not string")
				}

				if val != sessionValue {
					return fmt.Errorf("value not equal to %s", sessionValue)
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			muxOptions := defaultOptions
			muxOptions.SessionMgr = sessionManager
			muxOptions.Routes = []Route{
				{Match: tt.route, Handler: tt.handler},
			}

			resp, err := runServerForTest(t, muxOptions, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			assert.NotEmpty(t, resp.Header.Get("Set-Cookie"))
		})
	}
}
