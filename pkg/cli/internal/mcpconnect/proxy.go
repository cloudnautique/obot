package mcpconnect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	nmcp "github.com/obot-platform/nanobot/pkg/mcp"
	"github.com/obot-platform/obot/pkg/version"
)

type proxy struct {
	connectURL      string
	obotCallbackURL string
	callbackHandler nmcp.CallbackHandler
	localStorageDir string

	upstreamStartOnce sync.Once
	upstreamReady     chan struct{}

	lock           sync.RWMutex
	upstream       *nmcp.Client
	upstreamErr    error
	upstreamClosed bool
	localClosed    bool
	local          *nmcp.Stdio
}

func newProxy(connectURL, obotCallbackURL string, callbackHandler nmcp.CallbackHandler, localStorageDir string) *proxy {
	return &proxy{
		connectURL:      connectURL,
		obotCallbackURL: obotCallbackURL,
		callbackHandler: callbackHandler,
		localStorageDir: localStorageDir,
		upstreamReady:   make(chan struct{}),
	}
}

func (p *proxy) setLocal(local *nmcp.Stdio) {
	p.lock.Lock()
	p.local = local
	p.lock.Unlock()
}

func (p *proxy) forwardFromLocal(ctx context.Context, msg nmcp.Message) {
	// Response from the local MCP client to a request initiated by the upstream server.
	if msg.Method == "" && msg.ID != nil {
		upstream, err := p.waitUpstream(ctx)
		if err != nil {
			p.sendLocalError(ctx, msg, err)
			return
		}
		if err := upstream.Session.Send(ctx, &msg); err != nil {
			p.sendLocalError(ctx, msg, err)
		}
		return
	}

	// Complete the local MCP handshake immediately. Upstream Obot/Vercel OAuth may
	// require browser interaction, and many MCP clients will restart the stdio
	// process if initialize blocks while the user is authenticating.
	if msg.Method == "initialize" {
		var init nmcp.InitializeRequest
		_ = json.Unmarshal(msg.Params, &init)
		protocolVersion := init.ProtocolVersion
		if protocolVersion == "" {
			protocolVersion = "2025-06-18"
		}
		initResult := localInitializeResult(protocolVersion)
		if shouldWaitForUpstreamInitialize(init) {
			upstream, err := p.waitUpstream(ctx)
			if err != nil {
				p.sendLocalError(ctx, msg, err)
				return
			}
			initResult = upstream.Session.InitializeResult
			if initResult.ProtocolVersion == "" {
				initResult.ProtocolVersion = protocolVersion
			}
			if initResult.ServerInfo.Name == "" {
				initResult.ServerInfo.Name = "Obot MCP Gateway"
			}
			if initResult.ServerInfo.Version == "" {
				initResult.ServerInfo.Version = version.Get().String()
			}
		}
		result, err := json.Marshal(initResult)
		if err != nil {
			p.sendLocalError(ctx, msg, err)
			return
		}
		resp := nmcp.Message{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Result:  result,
		}
		if err := p.sendLocal(ctx, resp); err != nil {
			p.sendLocalError(ctx, msg, err)
		}
		return
	}

	// Drop duplicate local initialized notification because the upstream session has
	// already sent it as part of client creation.
	if msg.Method == "notifications/initialized" {
		return
	}

	upstream, err := p.waitUpstream(ctx)
	if err != nil {
		p.sendLocalError(ctx, msg, err)
		return
	}

	// Notifications do not expect a response.
	if msg.ID == nil {
		if err := upstream.Session.Send(ctx, &msg); err != nil {
			p.sendLocalError(ctx, msg, err)
		}
		return
	}

	var resp nmcp.Message
	if err := upstream.Session.Exchange(ctx, msg.Method, &msg, &resp); err != nil {
		p.sendLocalError(ctx, msg, err)
		return
	}

	resp = responseForLocalClient(msg, resp)
	if err := p.sendLocal(ctx, resp); err != nil {
		fmt.Fprintf(os.Stderr, "failed to send MCP response to local client: %v\n", err)
	}
}

func (p *proxy) startUpstream(ctx context.Context) {
	p.upstreamStartOnce.Do(func() {
		go func() {
			tokenStorage := nmcp.NewLocalTokenStorage(p.localStorageDir)
			client, err := p.newUpstreamClient(ctx, tokenStorage)
			if err != nil && shouldReauthenticate(err) {
				if client != nil {
					client.Close(true)
					client = nil
				}
				fmt.Fprintf(os.Stderr, "Cached Obot MCP credentials were rejected; reauthenticating.\n")
				if deleteErr := tokenStorage.DeleteTokenConfig(ctx, p.connectURL); deleteErr != nil {
					err = errors.Join(err, fmt.Errorf("failed to clear cached Obot MCP credentials: %w", deleteErr))
				} else {
					client, err = p.newUpstreamClient(ctx, tokenStorage)
				}
			}

			closeClient := p.setUpstream(client, err)
			if closeClient {
				client.Close(true)
			}
			close(p.upstreamReady)
		}()
	})
}

func localInitializeResult(protocolVersion string) nmcp.InitializeResult {
	return nmcp.InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: nmcp.ServerCapabilities{
			Prompts:   &nmcp.PromptsServerCapability{},
			Resources: &nmcp.ResourcesServerCapability{},
			Tools:     &nmcp.ToolsServerCapability{},
		},
		ServerInfo: nmcp.ServerInfo{
			Name:    "Obot MCP Proxy",
			Version: version.Get().String(),
		},
	}
}

func shouldWaitForUpstreamInitialize(init nmcp.InitializeRequest) bool {
	clientName := strings.ToLower(init.ClientInfo.Name)
	return strings.Contains(clientName, "inspector")
}

func (p *proxy) newUpstreamClient(ctx context.Context, tokenStorage nmcp.TokenStorage) (*nmcp.Client, error) {
	return nmcp.NewClient(ctx, "Obot MCP Gateway", nmcp.Server{
		BaseURL: p.connectURL,
	}, nmcp.ClientOption{
		ClientName:    "Obot CLI",
		ClientVersion: version.Get().String(),
		OnMessage:     p.forwardFromUpstream,
		OnNotify:      p.forwardFromUpstream,
		HTTPClientOptions: nmcp.HTTPClientOptions{
			OAuthClientName:  "Obot CLI",
			OAuthRedirectURL: p.obotCallbackURL,
			CallbackHandler:  p.callbackHandler,
			TokenStorage:     tokenStorage,
		},
	})
}

func shouldReauthenticate(err error) bool {
	if err == nil {
		return false
	}
	var authErr nmcp.AuthRequiredErr
	if errors.As(err, &authErr) {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "401 unauthorized") ||
		strings.Contains(errText, "authentication required")
}

func (p *proxy) setUpstream(client *nmcp.Client, err error) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.upstreamClosed {
		if err == nil {
			p.upstreamErr = context.Canceled
		}
		return client != nil
	}
	p.upstream = client
	p.upstreamErr = err
	return false
}

func (p *proxy) waitUpstream(ctx context.Context) (*nmcp.Client, error) {
	p.startUpstream(ctx)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.upstreamReady:
	}

	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.upstreamErr != nil {
		return nil, fmt.Errorf("failed to connect to upstream MCP server: %w", p.upstreamErr)
	}
	if p.upstream == nil || p.upstream.Session == nil {
		return nil, fmt.Errorf("upstream MCP server is not connected")
	}
	return p.upstream, nil
}

func (p *proxy) closeLocal() {
	p.lock.Lock()
	p.localClosed = true
	p.local = nil
	p.lock.Unlock()
	p.closeUpstream()
}

func (p *proxy) closeUpstream() {
	p.lock.Lock()
	if p.upstreamClosed {
		p.lock.Unlock()
		return
	}
	p.upstreamClosed = true
	upstream := p.upstream
	p.upstream = nil
	p.lock.Unlock()
	if upstream != nil {
		upstream.Close(true)
	}
}

func (p *proxy) forwardFromUpstream(ctx context.Context, msg nmcp.Message) error {
	return p.sendLocal(ctx, msg)
}

func (p *proxy) sendLocal(ctx context.Context, msg nmcp.Message) error {
	p.lock.RLock()
	local := p.local
	closed := p.localClosed
	p.lock.RUnlock()
	if local == nil || closed {
		return nil
	}
	return local.Send(ctx, messageForLocalClient(msg))
}

func (p *proxy) sendLocalError(ctx context.Context, req nmcp.Message, err error) {
	if req.ID == nil {
		fmt.Fprintf(os.Stderr, "failed to proxy MCP message: %v\n", err)
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	resp := nmcp.Message{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Error:   nmcp.ErrRPCInternal.WithError(err),
	}
	if sendErr := p.sendLocal(ctx, resp); sendErr != nil {
		fmt.Fprintf(os.Stderr, "failed to send MCP error to local client: %v\n", sendErr)
	}
}
