package mcpconnect

import nmcp "github.com/obot-platform/nanobot/pkg/mcp"

func messageForLocalClient(msg nmcp.Message) nmcp.Message {
	jsonrpc := msg.JSONRPC
	if jsonrpc == "" {
		jsonrpc = "2.0"
	}
	if msg.Result != nil || msg.Error != nil {
		return nmcp.Message{
			JSONRPC: jsonrpc,
			ID:      msg.ID,
			Result:  msg.Result,
			Error:   msg.Error,
		}
	}
	if msg.Method != "" {
		return nmcp.Message{
			JSONRPC: jsonrpc,
			ID:      msg.ID,
			Method:  msg.Method,
			Params:  msg.Params,
		}
	}
	return nmcp.Message{
		JSONRPC: jsonrpc,
		ID:      msg.ID,
		Result:  msg.Result,
		Error:   msg.Error,
	}
}

func responseForLocalClient(req, resp nmcp.Message) nmcp.Message {
	jsonrpc := resp.JSONRPC
	if jsonrpc == "" {
		jsonrpc = req.JSONRPC
	}
	if jsonrpc == "" {
		jsonrpc = "2.0"
	}
	return nmcp.Message{
		JSONRPC: jsonrpc,
		ID:      req.ID,
		Result:  resp.Result,
		Error:   resp.Error,
	}
}
