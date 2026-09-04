// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/cortezaproject/corteza/server/pkg/locale"
	"github.com/gorilla/csrf"
)

// aberaPlaintextHTTP keeps CSRF validation enabled for local HTTP deployments.
// It only adjusts the HTTPS-only origin check when session cookies are not secure.
func aberaPlaintextHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, csrf.PlaintextHTTPRequest(r))
	})
}

// aberaDefaultLocale applies the deployment default only to anonymous auth pages.
// An explicit lng query parameter remains authoritative.
func aberaDefaultLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lang := strings.TrimSpace(os.Getenv("CORTEZA_DEFAULT_LOCALE")); lang != "" &&
			r.URL.Query().Get("lng") == "" {
			r.Header.Set(locale.AcceptLanguageHeader, lang)
		}

		next.ServeHTTP(w, r)
	})
}
