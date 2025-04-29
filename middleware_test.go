package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyHandler struct {
}

func (d dummyHandler) ServeHTTP(http.ResponseWriter, *http.Request) {

}

func TestTrailingSlashMiddleware(t *testing.T) {
	d := dummyHandler{}

	middleware := RemoveTrailingSlashMiddleware(d)
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
	d := dummyHandler{}

	middleware := RequestIDMiddleware(d)
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

type panickingHandler struct{}

func (p panickingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("testing recover")
}
func TestRecoveryMiddleware(t *testing.T) {
	p := panickingHandler{}

	middleware := RecoveryMiddleware(p)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "http://dummy.com/target", nil)
	middleware.ServeHTTP(w, r)

	// no error checks - a successful recover should leave no traces
	assert.Equal(t, w.Result().StatusCode, http.StatusInternalServerError)
}
