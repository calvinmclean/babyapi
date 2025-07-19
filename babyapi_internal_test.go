package babyapi

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func TestMCPOptions(t *testing.T) {
	todoAPI := NewAPI("TODO", "/todos", func() *EndDateableTODO { return &EndDateableTODO{} }).
		EnableMCP(MCPPermCRUD).
		AddMCPTools(server.ServerTool{}).
		AddMCPHTTPOptions(server.WithEndpointPath("/mcp")).
		AddMCPServerOptions(server.WithInstructions("")).
		SetMCPPath("")

	require.Equal(t, 1, len(todoAPI.mcpConfig.Tools))
	require.Equal(t, 1, len(todoAPI.mcpConfig.HTTPOpts))
	require.Equal(t, 1, len(todoAPI.mcpConfig.ServerOpts))
}
