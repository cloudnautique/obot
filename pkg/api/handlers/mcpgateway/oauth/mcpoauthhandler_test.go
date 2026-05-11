package oauth

import "testing"

func TestErrorToQueryIncludesState(t *testing.T) {
	q := Error{
		Code:        ErrServerError,
		Description: "upstream failed",
		State:       "local-client-state",
	}.toQuery()

	if got := q.Get("error"); got != string(ErrServerError) {
		t.Fatalf("error = %q, want %q", got, ErrServerError)
	}
	if got := q.Get("error_description"); got != "upstream failed" {
		t.Fatalf("error_description = %q, want upstream failed", got)
	}
	if got := q.Get("state"); got != "local-client-state" {
		t.Fatalf("state = %q, want local-client-state", got)
	}
}

func TestCleanLocalhostCallbackPath(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
		want string
	}{
		{name: "default", want: "/oauth/callback"},
		{name: "custom absolute", path: "/custom/callback", want: "/custom/callback"},
		{name: "custom relative", path: "custom/callback", want: "/custom/callback"},
		{name: "trim whitespace", path: " /oauth/custom ", want: "/oauth/custom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanLocalhostCallbackPath(tt.path); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMCPOAuthRedirectURLUsesConfiguredLocalhostCallbackPath(t *testing.T) {
	factory := &MCPOAuthHandlerFactory{baseURL: "https://obot.example.com"}

	got := factory.mcpOAuthRedirectURL("http://localhost:1234/oauth/obot/callback", true, "/custom/callback")
	if want := "http://localhost:1234/custom/callback"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = factory.mcpOAuthRedirectURL("http://localhost:1234/oauth/obot/callback", true, "")
	if want := "http://localhost:1234/oauth/callback"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = factory.mcpOAuthRedirectURL("http://localhost:1234/oauth/obot/callback", false, "/custom/callback")
	if want := "https://obot.example.com/oauth/mcp/callback"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	got = factory.mcpOAuthRedirectURL("https://client.example.com/oauth/callback", true, "/custom/callback")
	if want := "https://obot.example.com/oauth/mcp/callback"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
