package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/csrf"
	"github.com/stretchr/testify/require"
)

func TestMarkPlaintextHTTPAllowsMatchingHTTPOrigin(t *testing.T) {
	var token string

	protected := aberaPlaintextHTTP(csrf.Protect(
		[]byte(strings.Repeat("x", 32)),
		csrf.Secure(false),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			token = csrf.Token(r)
		}

		w.WriteHeader(http.StatusNoContent)
	})))

	get := httptest.NewRequest(http.MethodGet, "http://localhost:8081/auth/login", nil)
	getRecorder := httptest.NewRecorder()
	protected.ServeHTTP(getRecorder, get)

	require.Equal(t, http.StatusNoContent, getRecorder.Code)
	require.NotEmpty(t, token)
	require.NotEmpty(t, getRecorder.Result().Cookies())

	post := httptest.NewRequest(http.MethodPost, "http://localhost:8081/auth/login", nil)
	post.Header.Set("Origin", "http://localhost:8081")
	post.Header.Set("X-CSRF-Token", token)
	post.AddCookie(getRecorder.Result().Cookies()[0])

	postRecorder := httptest.NewRecorder()
	protected.ServeHTTP(postRecorder, post)

	require.Equal(t, http.StatusNoContent, postRecorder.Code, fmt.Sprintf("CSRF response: %s", postRecorder.Body.String()))
}
