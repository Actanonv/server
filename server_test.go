package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	options := Options{
		Host:       "localhost",
		Port:       4000,
		Public:     "",
		Middleware: nil,
		Log:        nil,
		Templates:  nil,
		SessionMgr: nil,
	}

	srv, _ := Init(options)

	assert := assert.New(t)
	assert.NotNil(srv)
	assert.Equal(options.Host, srv.Host)
	assert.Equal(options.Port, srv.Port)
	assert.Equal(options.Public, srv.Public)
	assert.Equal(options.Middleware, srv.Middleware)
	assert.Equal(srv.log, appLog)
	assert.Equal(options.LogRequests, srv.logRequests)
	assert.Nil(srv.templateMgr)
	assert.Equal(options.SessionMgr, srv.sessionMgr)
}

func runServerForTest(t *testing.T, options Options, reqUrl string) (*http.Response, error) {
	t.Helper()

	var err error

	srv, err := Init(options)
	require.NoError(t, err, "server init failed")

	if err = srv.Route(); err != nil {
		return nil, err
	}

	return runTestServer(t, srv, reqUrl)
}

func runTestServer(t *testing.T, srv *Server, reqURl string) (*http.Response, error) {
	t.Helper()
	if srv == nil {
		return nil, errors.New("server not initialized")
	}

	var err error

	err = nil
	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + reqURl)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}

	return resp, nil
}

func TestServerMux_Handle(t *testing.T) {
	options := Options{}
	srv, err := Init(options)
	require.NoError(t, err, "server init failed")

	srv.HandleFunc("/hello", func(ctx Context) error {
		ctx.String(http.StatusOK, "Hello, World!")
		return nil
	})

	if err := srv.Route(); err != nil {
		require.Error(t, err)
		return
	}

	resp, err := runTestServer(t, srv, "/hello")
	if err != nil {
		require.NoError(t, err)
		return
	}

	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, resp.Body)
	if err != nil {
		require.NoError(t, err)
		return
	}
	assert.Equal(t, "Hello, World!", buff.String())

}

func TestServerMux_Group(t *testing.T) {
	options := Options{}
	srv, err := Init(options)
	require.NoError(t, err, "server init failed")

	srv.Group("/greet", "", func(srv *Server) {
		srv.HandleFunc("/hello", func(ctx Context) error {
			return ctx.String(http.StatusOK, "Hello, World!")
		})
	})

	if err := srv.Route(); err != nil {
		require.Error(t, err)
		return
	}

	resp, err := runTestServer(t, srv, "/greet/hello")
	if err != nil {
		require.NoError(t, err)
		return
	}

	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, resp.Body)
	if err != nil {
		require.NoError(t, err)
		return
	}
	assert.Equal(t, "Hello, World!", buff.String())

}

func TestServerMux_ChainedGroup(t *testing.T) {
	options := Options{}
	srv, err := Init(options)
	require.NoError(t, err, "server init failed")

	srv.Group("/greet", "", func(srv *Server) {
		srv.Middleware = []Middleware{
			func(next http.Handler) http.Handler {
				return HandlerFunc(func(ctx Context) error {
					r := ctx.Request()
					r = r.WithContext(context.WithValue(r.Context(), "age", 22))

					next.ServeHTTP(ctx.Response(), r)
					return nil
				})
			},
		}

		srv.HandleFunc("/hello", func(ctx Context) error {
			age := ctx.Request().Context().Value("age")
			return ctx.String(http.StatusOK, fmt.Sprint("Hello, ", age, " year old World!"))
		})
	})

	if err := srv.Route(); err != nil {
		require.Error(t, err)
		return
	}

	resp, err := runTestServer(t, srv, "/greet/hello")
	if err != nil {
		require.NoError(t, err)
		return
	}

	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, resp.Body)
	if err != nil {
		require.NoError(t, err)
		return
	}
	assert.Equal(t, "Hello, 22 year old World!", buff.String())

}

func TestServerRendering(t *testing.T) {

	tplOptions := TemplateOptions{
		Root: "./testData/templates",
		Ext:  ".tmpl",
	}

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
				return ctx.Render(http.StatusOK, RenderOpt{
					Template: "hello",
				})
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			options := defaultOptions
			options.Templates = &tplOptions
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

var defaultOptions = Options{
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
			options.Middleware = tt.middleware
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

func TestServer_RouteName(t *testing.T) {
	srv, err := Init(Options{})
	require.NoError(t, err, "server init failed")

	srv.HandleFunc("/users/{id}/profile", func(ctx Context) error {
		return nil
	}, WithName("userProfile"))
	srv.HandleFunc("/users/{id}/{profile}", func(ctx Context) error {
		return nil
	}, WithName("userSwitch"))
	srv.Group("/catalogs", "catalog", func(srv *Server) {
		srv.HandleFunc("/", func(ctx Context) error { return nil }, WithName("list"))
		srv.HandleFunc("/items/{itemId}", func(ctx Context) error { return nil }, WithName("item"))
	})

	err = srv.Route()
	require.NoError(t, err, "routing failed")

	t.Run("test single parameter", func(t *testing.T) {
		rtn := srv.RouteName("userProfile", "id", "42")
		assert.Equal(t, "/users/42/profile", rtn)
	})

	t.Run("test unbalanced parameters", func(t *testing.T) {
		rtn := srv.RouteName("userProfile", "id")
		assert.Equal(t, "/users//profile", rtn)
	})

	t.Run("test multiple parameters", func(t *testing.T) {
		rtn := srv.RouteName("userSwitch", "id", "42", "profile", "settings")
		assert.Equal(t, "/users/42/settings", rtn)
	})

	t.Run("test group routes", func(t *testing.T) {
		rtn := srv.RouteName("catalog/list", "itemId", "1001")
		assert.Equal(t, "/catalogs", rtn)

		rtn = srv.RouteName("catalog/item", "itemId", "1001")
		assert.Equal(t, "/catalogs/items/1001", rtn)

	})
}
