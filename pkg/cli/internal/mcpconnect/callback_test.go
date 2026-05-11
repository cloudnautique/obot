package mcpconnect

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorAwareObotCallbackReportsOAuthError(t *testing.T) {
	called := false
	handler := errorAwareObotCallback{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte("Success!!"))
	})}

	req := httptest.NewRequest(http.MethodGet, "/oauth/obot/callback?error=server_error&error_description=upstream+failed&state=abc", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Fatalf("wrapped callback handler was not called")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := w.Body.String(); !strings.Contains(got, "OAuth authorization failed: server_error: upstream failed") {
		t.Fatalf("body = %q, want OAuth error", got)
	}
}

func TestErrorAwareObotCallbackPreservesWrappedCallbackError(t *testing.T) {
	handler := errorAwareObotCallback{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid state", http.StatusBadRequest)
	})}

	req := httptest.NewRequest(http.MethodGet, "/oauth/obot/callback?error=server_error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := w.Body.String(); !strings.Contains(got, "invalid state") {
		t.Fatalf("body = %q, want wrapped callback error", got)
	}
}

func TestErrorAwareObotCallbackAllowsSuccessfulCallback(t *testing.T) {
	handler := errorAwareObotCallback{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success!!"))
	})}

	req := httptest.NewRequest(http.MethodGet, "/oauth/obot/callback?code=abc&state=abc", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "Success!!" {
		t.Fatalf("body = %q, want Success!!", got)
	}
}
