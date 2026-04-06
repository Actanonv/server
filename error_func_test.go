package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFunc_TriggerByError(t *testing.T) {
	var errorFuncCalled bool
	var capturedErr error

	customErrorFunc := func(ctx Context, err error) {
		errorFuncCalled = true
		capturedErr = err
		ctx.String(http.StatusBadRequest, "Custom Error: "+err.Error())
	}

	options := Options{
		ErrorFunc: customErrorFunc,
	}
	srv, err := Init(options)
	require.NoError(t, err)

	expectedErr := errors.New("something went wrong")
	srv.HandleFunc("/error", func(ctx Context) error {
		return expectedErr
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/error")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, errorFuncCalled)
	assert.Equal(t, expectedErr, capturedErr)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "Custom Error: something went wrong", string(body))
}

func TestErrorFunc_TriggerByPanic(t *testing.T) {
	var errorFuncCalled bool
	var capturedErr error

	customErrorFunc := func(ctx Context, err error) {
		errorFuncCalled = true
		capturedErr = err
		ctx.Status(http.StatusTeapot)
	}

	options := Options{
		ErrorFunc: customErrorFunc,
	}
	srv, err := Init(options)
	require.NoError(t, err)

	srv.HandleFunc("/panic", func(ctx Context) error {
		panic("at the disco")
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/panic")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, errorFuncCalled)
	assert.Contains(t, capturedErr.Error(), "panic: at the disco")
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
}

func TestErrorFunc_DefaultBehavior(t *testing.T) {
	options := Options{} // No ErrorFunc, Debug: false
	srv, err := Init(options)
	require.NoError(t, err)

	expectedErr := errors.New("standard error")
	srv.HandleFunc("/error", func(ctx Context) error {
		return expectedErr
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/error")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Should not leak error message when Debug is false
	assert.Equal(t, "Internal Server Error\n", string(body))
}

func TestErrorFunc_DebugBehavior(t *testing.T) {
	options := Options{Debug: true} // Debug: true
	srv, err := Init(options)
	require.NoError(t, err)

	expectedErr := errors.New("sensitive error info")
	srv.HandleFunc("/error", func(ctx Context) error {
		return expectedErr
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/error")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Should show error message when Debug is true
	assert.Equal(t, "sensitive error info\n", string(body))
}

func TestErrorFunc_DefaultBehaviorPanic(t *testing.T) {
	options := Options{} // No ErrorFunc, Debug: false
	srv, err := Init(options)
	require.NoError(t, err)

	srv.HandleFunc("/panic", func(ctx Context) error {
		panic("oops")
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/panic")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Should not leak panic message when Debug is false
	assert.Equal(t, "Internal Server Error\n", string(body))
}

func TestErrorFunc_PanicInMiddleware(t *testing.T) {
	options := Options{
		Middleware: []Middleware{
			RecoveryMiddleware,
		},
	}
	srv, err := Init(options)
	require.NoError(t, err)

	srv.HandleFunc("/panic", func(ctx Context) error {
		panic("middleware catch me")
	})

	err = srv.Route()
	require.NoError(t, err)

	tSrv := httptest.NewServer(srv.HTTPServer.Handler)
	defer tSrv.Close()

	resp, err := tSrv.Client().Get(tSrv.URL + "/panic")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "Internal Server Error\n", string(body))
}
