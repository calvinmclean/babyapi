package babyapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type MCPPerm uint8

const (
	MCPPermNone   MCPPerm = 0
	MCPPermCreate         = 1 << iota // 0001
	MCPPermRead                       // 0010
	MCPPermUpdate                     // 0100
	MCPPermDelete                     // 1000

	MCPPermCRUD = MCPPermCreate | MCPPermRead | MCPPermUpdate | MCPPermDelete
)

func (p MCPPerm) Has(flag MCPPerm) bool {
	return p&flag != 0
}

type MCPConfig struct {
	ServerOpts []server.ServerOption
	Tools      []server.ServerTool
	HTTPOpts   []server.StreamableHTTPOption
}

type mcpServer[T Resource] struct {
	storage Storage[T]
}

func (a *API[T]) mcpCRUDTools(perm MCPPerm) []server.ServerTool {
	mcpServer := mcpServer[T]{a.Storage}

	_, endDateable := any(a.instance()).(EndDateable)

	tools := []server.ServerTool{}

	if perm.Has(MCPPermRead) {
		listTool := mcp.NewTool(
			fmt.Sprintf("list_%s", a.name),
			mcp.WithDescription(fmt.Sprintf("list all %s", a.name)),
		)

		if endDateable {
			mcp.WithBoolean(
				"include_end_dated",
				mcp.Description(fmt.Sprintf("Include end-dated/deleted %s. Default is false.", a.name)),
			)(&listTool)
		}

		tools = append(tools,
			// TODO: how can I support url.Values{} for getAll? What about more complex filtering?
			server.ServerTool{
				Tool:    listTool,
				Handler: mcpServer.listAll,
			},
			server.ServerTool{
				Tool: mcp.NewTool(
					fmt.Sprintf("get_%s", a.name),
					mcp.WithDescription(fmt.Sprintf("get a %s by ID", a.name)),
					mcp.WithString("id", mcp.Required(), mcp.Description("Unique identifier for the item")),
				),
				Handler: mcpServer.get,
			},
		)
	}

	// TODO: enabling create and update will require reflection to know parameters
	if perm.Has(MCPPermCreate) {
		tools = append(tools, server.ServerTool{
			Tool: mcp.NewTool(
				fmt.Sprintf("create_%s", a.name),
			),
		})
	}

	if perm.Has(MCPPermUpdate) {
		tools = append(tools, server.ServerTool{
			Tool: mcp.NewTool(
				fmt.Sprintf("update_%s", a.name),
			),
		})
	}

	if perm.Has(MCPPermDelete) {
		tools = append(tools, server.ServerTool{
			Tool: mcp.NewTool(
				fmt.Sprintf("delete_%s", a.name),
				mcp.WithDescription(fmt.Sprintf("delete a %s by ID", a.name)),
				mcp.WithString("id", mcp.Required(), mcp.Description("Unique identifier for the item")),
			),
			Handler: mcpServer.delete,
		})
	}

	return tools
}

func (m mcpServer[T]) listAll(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	items, err := m.storage.GetAll(ctx, url.Values{})
	if err != nil {
		return nil, err
	}
	return newToolResultJSON(items)
}

func (m mcpServer[T]) get(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return nil, err
	}

	item, err := m.storage.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return newToolResultJSON(item)
}

func (m mcpServer[T]) delete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := request.RequireString("id")
	if err != nil {
		return nil, err
	}

	err = m.storage.Delete(ctx, id)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText("deleted"), nil
}

func newToolResultJSON(out any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// EnableMCP sets up an MCP server for CRUD operations
// TODO: this currently doesn't work well for multi-api or nested API servers. Do I want to make it only run at the root and use all children?
// or should each resource have its own MCP server?
func (a *API[T]) EnableMCP(name, path string, crudPerm MCPPerm, cfg MCPConfig) *API[T] {
	a.panicIfReadOnly()

	cfg.Tools = append(cfg.Tools, a.mcpCRUDTools(crudPerm)...)

	s := server.NewMCPServer(name, "", cfg.ServerOpts...)
	s.AddTools(cfg.Tools...)
	a.AddCustomRootRoute("", path, server.NewStreamableHTTPServer(s, cfg.HTTPOpts...))

	return a
}
