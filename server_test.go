package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	options := Options{
		Host:        "localhost",
		Port:        4000,
		Public:      "",
		Middlewares: nil,
		Routes:      nil,
		Log:         nil,
		LogRequests: true,
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

func TestServer(t *testing.T) {
	routes := []Route{
		{Match: "GET /hello",
			HandlerFn: func(ctx Context) error {
				return ctx.String(http.StatusOK, "Hello World!")
			},
		},
	}

	// add test that uses a template
	options := Options{
		Host:        "localhost",
		Port:        4000,
		Public:      "",
		Middlewares: nil,
		Routes:      routes,
		Log:         nil,
		LogRequests: true,
		Templates:   nil,
		SessionMgr:  nil,
	}

	srv := Init(options)
	require.NotNil(t, srv)

	err := srv.Route()
	require.NoError(t, err)

	go func() {
		err := srv.Run()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()

	time.Sleep(time.Millisecond * 10)

	resp, err := http.Get("http://localhost:4000/hello")
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}

	assert.Equal(t, "Hello World!", string(respBody))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = srv.HTTPServer.Shutdown(ctx)
	assert.NoError(t, err)
}
