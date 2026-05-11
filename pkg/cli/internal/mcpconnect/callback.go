package mcpconnect

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	nmcp "github.com/obot-platform/nanobot/pkg/mcp"
)

type obotCallbackHandler interface {
	nmcp.CallbackHandler
	http.Handler
}

func newCallbackServer(gatewayURL *url.URL, obotCallback obotCallbackHandler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/oauth/obot/callback", errorAwareObotCallback{Handler: obotCallback})
	mcpOAuthCallback := func(w http.ResponseWriter, r *http.Request) {
		// Keep the browser in the flow so Obot receives its own auth cookies. The gateway
		// still performs the second-level upstream token exchange and then redirects back
		// to /oauth/obot/callback with the first-level Obot authorization code.
		u := *gatewayURL
		u.Path = strings.TrimRight(u.Path, "/") + "/oauth/mcp/callback"
		u.RawQuery = r.URL.RawQuery
		u.Fragment = ""
		fmt.Fprintf(os.Stderr, "Forwarding upstream MCP OAuth callback to Obot gateway: path=%s has_code=%t has_error=%t has_state=%t gateway_callback=%s\n",
			r.URL.Path, r.URL.Query().Get("code") != "", r.URL.Query().Get("error") != "", r.URL.Query().Get("state") != "", u.Path)
		http.Redirect(w, r, u.String(), http.StatusFound)
	}
	// Remote server configuration controls the loopback callback path used for
	// upstream MCP OAuth. Accept any local path so custom provider-specific paths
	// work without requiring a CLI flag or a local config update.
	mux.HandleFunc("/", mcpOAuthCallback)
	return &http.Server{Handler: mux}
}

type errorAwareObotCallback struct {
	http.Handler
}

func (e errorAwareObotCallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	oauthErr := r.URL.Query().Get("error")
	if oauthErr == "" {
		e.Handler.ServeHTTP(w, r)
		return
	}

	rec := &responseRecorder{header: http.Header{}, code: http.StatusOK}
	e.Handler.ServeHTTP(rec, r)
	if rec.code >= http.StatusBadRequest {
		copyHeader(w.Header(), rec.header)
		w.WriteHeader(rec.code)
		_, _ = w.Write(rec.body)
		return
	}

	description := r.URL.Query().Get("error_description")
	if description == "" {
		fmt.Fprintf(os.Stderr, "Obot gateway OAuth callback failed: error=%s\n", oauthErr)
		http.Error(w, fmt.Sprintf("OAuth authorization failed: %s", oauthErr), http.StatusBadRequest)
		return
	}
	fmt.Fprintf(os.Stderr, "Obot gateway OAuth callback failed: error=%s description=%s\n", oauthErr, description)
	http.Error(w, fmt.Sprintf("OAuth authorization failed: %s: %s", oauthErr, description), http.StatusBadRequest)
}

type responseRecorder struct {
	header http.Header
	body   []byte
	code   int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	r.body = append(r.body, body...)
	return len(body), nil
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		dst.Del(k)
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func shutdownCallbackServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
}

func gatewayBaseURL(connectURL string) (*url.URL, error) {
	u, err := url.Parse(connectURL)
	if err != nil {
		return nil, fmt.Errorf("invalid connect URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("connect URL must use http or https")
	}
	idx := strings.Index(u.Path, "/mcp-connect/")
	if idx < 0 && u.Path != "/mcp-connect" {
		return nil, fmt.Errorf("connect URL must point to an Obot /mcp-connect endpoint")
	}
	if idx >= 0 {
		u.Path = u.Path[:idx]
	} else {
		u.Path = ""
	}
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}
