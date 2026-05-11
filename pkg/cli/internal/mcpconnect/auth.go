package mcpconnect

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/browser"
)

type authHandler struct{}

func (authHandler) HandleAuthURL(_ context.Context, _, authURL string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Opening browser to authenticate with Obot: %s\n", authURL)
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open browser automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "Paste this URL into your browser manually: %s\n", authURL)
	}
	return true, nil
}
