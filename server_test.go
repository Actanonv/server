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

func runServerForTest(t *testing.T, options MuxOptions, expectedStatus int, route, expectedBody string) {
	var err error
	assert := assert.New(t)
	require := require.New(t)

	srv := Init(options)
	require.NotNil(srv)

	err = srv.Route()
	require.NoError(err)

	go func() {
		err := srv.Run()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()

	time.Sleep(time.Millisecond * 10)

	resp, err := http.Get(fmt.Sprintf("http://localhost:4000/%s", route))
	assert.NoError(err)

	assert.Equal(expectedStatus, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}

	assert.Equal(expectedBody, string(respBody))

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err = srv.HTTPServer.Shutdown(ctx)
	assert.NoError(err)
}

func TestServerRendering(t *testing.T) {
	routes := []Route{
		{Match: "GET /hello",
			HandlerFn: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello, World!")
			},
		},

		{
			Match: "GET /test-template",
			HandlerFn: func(ctx Context) error {
				return ctx.Render(http.StatusOK, templates.RenderOption{
					Layout:   "",
					Template: "hello",
				})
			},
		},
	}

	tMgr, err := InitTemplates("./testData/templates", templates.TemplateOptions{
		Ext: ".tmpl",
	})

	assert := assert.New(t)
	assert.NoError(err)
	assert.NotNil(tMgr)

	tt := []struct {
		name           string
		route          string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "render hello:string",
			route:          "hello",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
		},

		{
			name:           "render template",
			route:          "test-template",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			options := MuxOptions{
				Host:        "localhost",
				Port:        4000,
				Public:      "",
				Middlewares: nil,
				Routes:      routes,
				Log:         nil,
				Templates:   templateMgr,
				SessionMgr:  nil,
			}

			runServerForTest(t, options, test.expectedStatus, test.route, test.expectedBody)
		})
	}
}

func TestServerMiddleware(t *testing.T) {

	routes := []Route{
		{Match: "GET /hello",
			HandlerFn: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello, World!")
			},
		},

		{Match: "GET /panic",
			HandlerFn: func(ctx Context) error {
				panic("testing recover")
			},
		},
	}

	tt := []struct {
		name           string
		route          string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "test reqId and trailing slash",
			route:          "hello/",
			expectedBody:   "Hello, World!",
			expectedStatus: http.StatusOK,
		},

		{
			name:           "test panic recovery",
			route:          "panic/",
			expectedBody:   "",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			options := MuxOptions{
				Host:   "localhost",
				Port:   4000,
				Public: "",
				Middlewares: []alice.Constructor{
					RemoveTrailingSlashMiddleware,
					RequestIDMiddleware,
					RecoveryMiddleware,
				},
				Routes:     routes,
				Log:        nil,
				SessionMgr: nil,
			}

			runServerForTest(t, options, test.expectedStatus, test.route, test.expectedBody)
		})
	}
}

func TestServerSessions(t *testing.T) {}
