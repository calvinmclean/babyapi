package babyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/invopop/jsonschema"
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

type mcpConfig struct {
	Enabled     bool
	Path        string
	Permissions MCPPerm

	ServerOpts []server.ServerOption
	Tools      []server.ServerTool
	HTTPOpts   []server.StreamableHTTPOption
}

type mcpServer[T Resource] struct {
	storage  Storage[T]
	instance func() T
	parent   RelatedAPI
}

// mcpCRUDTools is the default CRUD tools based on the API's permissions
func (a *API[T]) mcpCRUDTools() []server.ServerTool {
	// RootAPIs don't have any CRUD resources, so the default CRUD tools are not relevant
	if a.isRoot() {
		return nil
	}
	mcpServer := mcpServer[T]{a.Storage, a.instance, a.Parent()}

	_, endDateable := any(a.instance()).(EndDateable)

	tools := []server.ServerTool{}

	if a.mcpConfig.Permissions.Has(MCPPermRead) {
		searchTool := mcp.NewTool(
			fmt.Sprintf("search_%s", a.name),
			mcp.WithDescription(fmt.Sprintf("search all %s", a.name)),
		)

		if parentParam := mcpParentIDInput[T](a.Parent()); parentParam != "" {
			mcp.WithString(
				parentParam,
				mcp.Required(),
				mcp.Description("This is the ID for the parent object needed to list instances of this object."),
			)(&searchTool)
		}

		if endDateable {
			mcp.WithBoolean(
				"include_end_dated",
				mcp.Description(fmt.Sprintf("Include end-dated/deleted %s. Default is false.", a.name)),
			)(&searchTool)
		}

		tools = append(tools,
			// TODO: how can I support url.Values{} for search? What about more complex filtering?
			server.ServerTool{
				Tool:    searchTool,
				Handler: mcpServer.search,
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

	schema := (&jsonschema.Reflector{
		ExpandedStruct:             true,
		DoNotReference:             true,
		RequiredFromJSONSchemaTags: true,
	}).Reflect(a.instance())
	schema.Version = ""
	schemaJSON, _ := json.Marshal(schema)

	if a.mcpConfig.Permissions.Has(MCPPermCreate) {
		tools = append(tools, server.ServerTool{
			Tool: mcp.NewToolWithRawSchema(
				fmt.Sprintf("create_%s", a.name),
				fmt.Sprintf("Create a new %s. Do not include an ID since it will be ignored", a.name),
				schemaJSON,
			),
			Handler: mcpServer.create,
		})
	}

	_, patchable := any(a.instance()).(Patcher[T])
	if a.mcpConfig.Permissions.Has(MCPPermUpdate) && patchable {
		tools = append(tools, server.ServerTool{
			Tool: mcp.NewToolWithRawSchema(
				fmt.Sprintf("update_%s", a.name),
				fmt.Sprintf("Update a %s by ID. This is similar to a PATCH request and will only change specified fields", a.name),
				schemaJSON,
			),
			Handler: mcpServer.updatePatch,
		})
	}

	if a.mcpConfig.Permissions.Has(MCPPermDelete) {
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

func mcpParentIDInput[T Resource](parent RelatedAPI) string {
	if parent == nil {
		return ""
	}
	return fmt.Sprintf("%s_id", parent.Name())
}

func (m mcpServer[T]) search(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	values := url.Values{}

	_, endDateable := any(m.instance()).(EndDateable)
	if endDateable {
		endDated := request.GetBool("include_end_dated", false)
		values.Set("end_dated", fmt.Sprint(endDated))
	}

	parentIDKey := mcpParentIDInput[T](m.parent)
	var parentID string
	if parentIDKey != "" {
		var err error
		parentID, err = request.RequireString(parentIDKey)
		if err != nil {
			return nil, err
		}
	}

	items, err := m.storage.Search(ctx, parentID, values)
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

func (m mcpServer[T]) create(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	newItem := m.instance()
	err := request.BindArguments(newItem)
	if err != nil {
		return nil, err
	}

	// Run Bind which should do any initialization like setting an ID
	err = newItem.Bind(&http.Request{Method: http.MethodPost})
	if err != nil {
		return nil, err
	}

	err = m.storage.Set(ctx, newItem)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf(`{"id":"%s"}`, newItem.GetID())), nil
}

func (m mcpServer[T]) updatePatch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	newItem := m.instance()
	err := request.BindArguments(newItem)
	if err != nil {
		return nil, err
	}

	item, err := m.storage.Get(ctx, newItem.GetID())
	if err != nil {
		return nil, err
	}

	patchable, ok := any(item).(Patcher[T])
	if !ok {
		return nil, errors.New("cannot use update with non-patchable type")
	}

	patchErr := patchable.Patch(newItem)
	if patchErr != nil {
		return nil, patchErr.Err
	}

	err = m.storage.Set(ctx, patchable.(T))
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText("updated"), nil
}

func newToolResultJSON(out any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// MCPHandler creates an http.Handler for the MCP server with configured tools
func (a *API[T]) MCPHandler() http.Handler {
	s := server.NewMCPServer(a.name, "", a.mcpConfig.ServerOpts...)
	s.AddTools(a.mcpConfig.Tools...)

	return server.NewStreamableHTTPServer(s, a.mcpConfig.HTTPOpts...)
}

// SetMCPPath sets the base path for the MCP handler. Default is "/mcp" at the root level
func (a *API[T]) SetMCPPath(path string) *API[T] {
	a.panicIfReadOnly()

	a.mcpConfig.Path = path

	return a
}

// AddMCPHTTPOptions adds StreamableHTTPOptions for the MCP HTTP server
func (a *API[T]) AddMCPHTTPOptions(opts ...server.StreamableHTTPOption) *API[T] {
	a.panicIfReadOnly()

	a.mcpConfig.HTTPOpts = append(a.mcpConfig.HTTPOpts, opts...)

	return a
}

// AddMCPServerOptions adds ServerOptions to use when initializing the MCP server
func (a *API[T]) AddMCPServerOptions(opts ...server.ServerOption) *API[T] {
	a.panicIfReadOnly()

	a.mcpConfig.ServerOpts = append(a.mcpConfig.ServerOpts, opts...)

	return a
}

// AddMCPTools appends custom tools to the MCP server
func (a *API[T]) AddMCPTools(tools ...server.ServerTool) *API[T] {
	a.panicIfReadOnly()

	a.mcpConfig.Tools = append(a.mcpConfig.Tools, tools...)

	return a
}

// EnableMCP sets MCP to Enabled with the provided permissions for initializing default CRUD tools for the API.
// Update only works if the API Resource implements Patcher (can be PATCHed).
// Update and Create use jsonschema (https://github.com/invopop/jsonschema) to define the tool input.
func (a *API[T]) EnableMCP(defaultCRUDPerm MCPPerm) *API[T] {
	a.panicIfReadOnly()

	a.mcpConfig.Enabled = true
	a.mcpConfig.Permissions = defaultCRUDPerm

	if a.mcpConfig.Path == "" {
		a.mcpConfig.Path = "/mcp"
	}

	return a
}

// mcpTools initializes the default CRUD tools for this API based on the provided permission,
// appends user-added tools, and child API tools using aggregateChildTools()
func (a *API[T]) mcpTools() []server.ServerTool {
	if !a.mcpConfig.Enabled {
		return nil
	}

	a.mcpConfig.Tools = append(a.mcpConfig.Tools, a.mcpCRUDTools()...)
	a.aggregateChildTools()
	return a.mcpConfig.Tools
}

// aggregateChildTools appends all child API's tools to this API's tools. It uses mcpTools() to initialize
// the child's tools
func (a *API[T]) aggregateChildTools() {
	if !a.mcpConfig.Enabled {
		return
	}
	for _, childAPI := range a.subAPIs {
		a.mcpConfig.Tools = append(a.mcpConfig.Tools, childAPI.mcpTools()...)
	}
}
