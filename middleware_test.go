package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrailingSlashMiddleware(t *testing.T) {
	middleware := RemoveTrailingSlashMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	tt := []struct {
		name string
		w    http.ResponseWriter
		r    *http.Request
	}{
		{name: "request with slash",
			w: httptest.NewRecorder(),
			r: httptest.NewRequest(http.MethodGet, "http://dummy.com/target/", nil),
		},
		{name: "request without slash",
			w: httptest.NewRecorder(),
			r: httptest.NewRequest(http.MethodGet, "http://dummy.com/target", nil),
		},
	}
	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			middleware.ServeHTTP(test.w, test.r)
			assert.False(t, strings.HasSuffix(test.r.URL.Path, "/"))
		})
	}
}

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
