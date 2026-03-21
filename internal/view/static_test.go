package view

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticFileHandler(t *testing.T) {
	h := StaticFileHandler()
	r := httptest.NewRequest(http.MethodGet, "/statics/main.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "function")
}
