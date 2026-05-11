package cli

import (
	"github.com/obot-platform/obot/pkg/cli/internal/mcpconnect"
	"github.com/spf13/cobra"
)

type MCPConnect struct{}

func (m *MCPConnect) Customize(cmd *cobra.Command) {
	cmd.Use = "mcp-connect URL"
	cmd.Short = "Run a local stdio MCP proxy to an Obot MCP server"
	cmd.Args = cobra.ExactArgs(1)
}

func (m *MCPConnect) Run(cmd *cobra.Command, args []string) error {
	return mcpconnect.Run(cmd.Context(), args[0])
}
