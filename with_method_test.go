package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMethod(t *testing.T) {
	srv, err := Init(Options{})
	require.NoError(t, err)

	srv.HandleFunc("/test", func(ctx Context) error {
		return ctx.String(http.StatusOK, "OK")
	}, WithMethods("POST"), WithName("testPost"))

	err = srv.Route()
	require.NoError(t, err)

	t.Run("GET request should fail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("POST request should succeed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, _ := io.ReadAll(w.Body)
		assert.Equal(t, "OK", string(body))
	})

	t.Run("RouteName for POST should work", func(t *testing.T) {
		assert.Equal(t, "/test", srv.RouteName("testPost"))
	})
}

func TestWithMultipleMethods(t *testing.T) {
	srv, err := Init(Options{})
	require.NoError(t, err)

	srv.HandleFunc("/multi", func(ctx Context) error {
		return ctx.String(http.StatusOK, "OK")
	}, WithMethods("GET", "POST"))

	err = srv.Route()
	require.NoError(t, err)

	t.Run("GET request should succeed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/multi", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("POST request should succeed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/multi", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DELETE request should fail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/multi", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestWithMultipleMethodsAndName(t *testing.T) {
	srv, err := Init(Options{})
	require.NoError(t, err)

	srv.HandleFunc("/multi-named", func(ctx Context) error {
		path := ctx.GetRoutePath("multi")
		return ctx.String(http.StatusOK, path)
	}, WithMethods("GET", "POST"), WithName("multi"))

	srv.Group("/grouped", "grouped", func(srv *Server) {
		srv.HandleFunc("/multi-named", func(ctx Context) error {
			path := ctx.GetRoutePath("grouped/multi")
			return ctx.String(http.StatusOK, path)
		}, WithMethods("GET", "POST"), WithName("multi"))
	})

	err = srv.Route()
	require.NoError(t, err)

	t.Run("GET request should return correct RoutePath", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/multi-named", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, _ := io.ReadAll(w.Body)
		assert.Equal(t, "/multi-named", string(body))
	})

	t.Run("POST request should return correct RoutePath", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/multi-named", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, _ := io.ReadAll(w.Body)
		assert.Equal(t, "/multi-named", string(body))
	})

	t.Run("Direct RouteName call", func(t *testing.T) {
		assert.Equal(t, "/multi-named", srv.RouteName("multi"))
	})

	t.Run("group:GET request should return correct RoutePath", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/grouped/multi-named", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, _ := io.ReadAll(w.Body)
		assert.Equal(t, "/grouped/multi-named", string(body))
	})

	t.Run("group:POST request should return correct RoutePath", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/grouped/multi-named", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, _ := io.ReadAll(w.Body)
		assert.Equal(t, "/grouped/multi-named", string(body))
	})

}
