package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/justinas/alice"
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
		Routes:      nil,
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
	assert.Equal(options.Middlewares, srv.Middlewares)
	assert.Equal(options.Routes, srv.Routes)
	assert.Equal(srv.log, appLog)
	assert.Equal(options.LogRequests, srv.logRequests)
	assert.Equal(options.Templates, srv.templates)
	assert.Equal(options.SessionMgr, srv.sessionMgr)
}

func runServerForTest(t *testing.T, options MuxOptions, reqUrl string) (int, string, error) {
	t.Helper()

	var err error

	srv := Init(options)
	if srv == nil {
		return 0, "", errors.New("server not initialized")
	}

	if err = srv.Route(); err != nil {
		return 0, "", err
	}

	err = nil
	go func() {
		err = srv.Run()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return
		}
	}()

	time.Sleep(time.Millisecond * 10)
	if err != nil {
		return 0, "", fmt.Errorf("run server: %w", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		err = srv.HTTPServer.Shutdown(ctx)
		if err != nil {
			t.Log(err)
		}
	}()

	resp, err := http.Get(fmt.Sprintf("http://%s:%d%s", options.Host, options.Port, reqUrl))
	if err != nil {
		return 0, "", fmt.Errorf("get request: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return resp.StatusCode, string(body), nil
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
			name:           "render hello:string",
			route:          "GET /hello",
			url:            "/hello",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
			handler: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello, World!")
			},
		},
		//
		//{
		//	name:           "render template",
		//	route:          "test-template",
		//	expectedBody:   "Hello, World!",
		//	expectedStatus: http.StatusOK,
		//},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			options := defaultOptions
			options.Templates = tMgr
			options.Routes = []Route{
				{Match: tt.route, HandlerFn: tt.handler},
			}

			statusCode, body, err := runServerForTest(t, options, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, statusCode)
			assert.Equal(t, tt.expectedBody, body)
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
		middleware     []alice.Constructor
		handler        HandlerFunc
	}{
		{
			name:           "test reqId and trailing slash",
			route:          "GET /hello/",
			url:            "/hello",
			expectedStatus: http.StatusOK,
			middleware: []alice.Constructor{
				//RemoveTrailingSlashMiddleware,
				RequestIDMiddleware,
				RecoveryMiddleware,
			},
			handler: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello, World!")
			},
		},

		//{
		//	name:           "test reqId and trailing slash",
		//	route:          "hello/",
		//	expectedBody:   "Hello, World!",
		//	expectedStatus: http.StatusOK,
		//	middleware: []alice.Constructor{
		//		RemoveTrailingSlashMiddleware,
		//	},
		//},
		//
		//{
		//	name:           "test panic recovery",
		//	route:          "panic/",
		//	expectedBody:   "",
		//	expectedStatus: http.StatusInternalServerError,
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := defaultOptions
			options.Middlewares = tt.middleware
			options.Routes = []Route{
				{Match: tt.route, HandlerFn: tt.handler},
			}

			statusCode, _, err := runServerForTest(t, options, tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, statusCode)
		})
	}
}

func TestServerSessions(t *testing.T) {}
