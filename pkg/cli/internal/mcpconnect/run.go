package mcpconnect

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/adrg/xdg"
	nmcp "github.com/obot-platform/nanobot/pkg/mcp"
)

func Run(ctx context.Context, connectURL string) error {
	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	localCtx, cancelLocal := context.WithCancel(runCtx)
	defer cancelLocal()

	localStorageDir := filepath.Join(xdg.DataHome, "obot", "mcp-connect")
	if err := os.MkdirAll(localStorageDir, 0o700); err != nil {
		return fmt.Errorf("failed to create local token storage directory: %w", err)
	}
	logFile, err := redirectLogs(localStorageDir)
	if err != nil {
		return err
	}
	defer logFile.Close()

	gatewayURL, err := gatewayBaseURL(connectURL)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen on localhost callback port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("unexpected callback listener address type %T", listener.Addr())
	}
	// Bind only to loopback, but advertise localhost for OAuth compatibility
	// with providers that allow localhost redirect URIs but reject 127.0.0.1.
	callbackBaseURL := fmt.Sprintf("http://localhost:%d", addr.Port)
	obotCallbackURL := callbackBaseURL + "/oauth/obot/callback"
	fmt.Fprintf(os.Stderr, "Obot MCP connect local OAuth callback listening at %s\n", callbackBaseURL)
	fmt.Fprintf(os.Stderr, "Obot MCP connect callback for Obot gateway OAuth: %s\n", obotCallbackURL)

	authHandler := authHandler{}
	obotCallback := nmcp.NewCallbackServer(authHandler)
	proxy := newProxy(connectURL, obotCallbackURL, obotCallback, localStorageDir)
	context.AfterFunc(localCtx, proxy.closeUpstream)

	callbackServer := newCallbackServer(gatewayURL, obotCallback)
	go serveCallbackServer(callbackServer, listener)
	defer shutdownCallbackServer(callbackServer)

	stdio := nmcp.NewStdio("proxy", nil, os.Stdin, os.Stdout, func() {
		cancelLocal()
		proxy.closeLocal()
	})
	proxy.setLocal(stdio)
	proxy.startUpstream(localCtx)
	defer proxy.closeUpstream()
	if err := stdio.Start(localCtx, proxy.forwardFromLocal); err != nil {
		return err
	}
	stdio.Wait()
	return nil
}

func serveCallbackServer(server *http.Server, listener net.Listener) {
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "OAuth callback server failed: %v\n", err)
	}
}
