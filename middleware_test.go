package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDMiddleware(t *testing.T) {
	middleware := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "http://dummy.com/target", nil)
	middleware.ServeHTTP(w, r)

	temp := r.Context().Value(requestIDKey)
	rid, ok := temp.(string)
	require.True(t, ok)

	require.NotEmpty(t, rid)

	require.NotEmpty(t, w.Header().Get("X-Request-ID"))
	require.Equal(t, rid, w.Header().Get("X-Request-ID"))
}

func TestRecoveryMiddleware(t *testing.T) {
	middleware := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("testing recover")
	}),
	)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "http://dummy.com/target", nil)
	middleware.ServeHTTP(w, r)

	// no error checks - a successful recover should leave no traces
	assert.Equal(t, w.Result().StatusCode, http.StatusInternalServerError)
}
