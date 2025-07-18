package babyapi_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"

	"github.com/go-chi/render"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCP(t *testing.T) {
	artistAPI := babyapi.NewAPI("Artists", "/artists", func() *Artist { return &Artist{} }).
		EnableMCP(babyapi.MCPPermCRUD)
	albumAPI := babyapi.NewAPI("Albums", "/albums", func() *Album { return &Album{} }).
		EnableMCP(babyapi.MCPPermCRUD)
	musicVideoAPI := babyapi.NewAPI("MusicVideos", "/music_videos", func() *MusicVideo { return &MusicVideo{} }).
		EnableMCP(babyapi.MCPPermCRUD)
	songAPI := babyapi.NewAPI("Songs", "/songs", func() *Song { return &Song{} }).
		EnableMCP(babyapi.MCPPermCRUD)

	songAPI.SetResponseWrapper(func(s *Song) render.Renderer {
		return &SongResponse{Song: s, api: songAPI}
	})

	artistAPI.AddNestedAPI(albumAPI).AddNestedAPI(musicVideoAPI)
	albumAPI.AddNestedAPI(songAPI)

	serverURL, stop := babytest.TestServe(t, artistAPI)
	defer stop()

	mcpClient, err := client.NewStreamableHttpClient(serverURL + "/mcp")
	require.NoError(t, err)

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	_, err = mcpClient.Initialize(t.Context(), initReq)
	require.NoError(t, err)

	toolsResp, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)

	tools := map[string]mcp.Tool{}
	for _, tool := range toolsResp.Tools {
		tools[tool.Name] = tool
	}

	t.Run("AllToolsExist", func(t *testing.T) {
		expectedToolNames := []string{
			"create_Artists",
			"get_Artists",
			"list_Artists",
			"delete_Artists",

			"create_Albums",
			"get_Albums",
			"list_Albums",
			"delete_Albums",
			"update_Albums", // Patcher

			"create_MusicVideos",
			"get_MusicVideos",
			"list_MusicVideos",
			"delete_MusicVideos",
			"update_MusicVideos", // Patcher

			"create_Songs",
			"get_Songs",
			"list_Songs",
			"delete_Songs",
		}
		require.ElementsMatch(t, expectedToolNames, slices.Collect(maps.Keys(tools)))
	})

	// TODO: See comment in mcp.go
	// t.Run("ListAlbumsHasParentID", func(t *testing.T) {
	// 	listAlbums := tools["list_Albums"]
	// 	_, ok := listAlbums.InputSchema.Properties["Artists_id"]
	// 	require.True(t, ok)
	// })
	// t.Run("ListSongsHasParentID", func(t *testing.T) {
	// 	listSongs := tools["list_Songs"]
	// 	_, ok := listSongs.InputSchema.Properties["Albums_id"]
	// 	require.True(t, ok)
	// })
	// t.Run("ListMusicVideosHasParentID", func(t *testing.T) {
	// 	listMusicVideos := tools["list_MusicVideos"]
	// 	_, ok := listMusicVideos.InputSchema.Properties["Artists_id"]
	// 	require.True(t, ok)
	// })

	var artistID string
	t.Run("CreateArtist", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "create_Artists",
				Arguments: map[string]any{
					"name": "Baby API",
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)

		var idResp struct {
			ID string
		}
		err = json.Unmarshal([]byte(textContent.Text), &idResp)
		require.NoError(t, err)
		require.NotEmpty(t, idResp.ID)
		artistID = idResp.ID
	})

	t.Run("GetArtistByID", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "get_Artists",
				Arguments: map[string]any{
					"id": artistID,
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)

		var artist Artist
		err = json.Unmarshal([]byte(textContent.Text), &artist)
		require.NoError(t, err)
		require.Equal(t, artistID, artist.GetID())
		require.Equal(t, "Baby API", artist.Name)
	})

	t.Run("GetArtistByID_NotFound", func(t *testing.T) {
		_, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "get_Artists",
				Arguments: map[string]any{
					"id": "invalid",
				},
			},
		})
		require.Error(t, err)
		require.Equal(t, "resource not found", err.Error())
	})

	t.Run("ListArtists", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "list_Artists",
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)

		var artist []Artist
		err = json.Unmarshal([]byte(textContent.Text), &artist)
		require.NoError(t, err)
		require.Equal(t, 1, len(artist))
		require.Equal(t, artistID, artist[0].GetID())
		require.Equal(t, "Baby API", artist[0].Name)
	})

	var albumID string
	t.Run("CreateAlbum", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "create_Albums",
				Arguments: map[string]any{
					"title":     "Baby API Album",
					"artist_id": artistID,
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)

		var idResp struct {
			ID string
		}
		err = json.Unmarshal([]byte(textContent.Text), &idResp)
		require.NoError(t, err)
		require.NotEmpty(t, idResp.ID)
		albumID = idResp.ID
	})

	t.Run("GetAlbumByID", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "get_Albums",
				Arguments: map[string]any{
					"id": albumID,
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)

		var album Album
		err = json.Unmarshal([]byte(textContent.Text), &album)
		require.NoError(t, err)
		require.Equal(t, albumID, album.GetID())
		require.Equal(t, artistID, album.ArtistID)
		require.Equal(t, "Baby API Album", album.Title)
	})

	t.Run("UpdateAlbum", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "update_Albums",
				Arguments: map[string]any{
					"id":    albumID,
					"title": "Baby API Album 2",
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)
		require.Equal(t, "updated", textContent.Text)

		t.Run("GetAlbumByID", func(t *testing.T) {
			resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
				Request: mcp.Request{},
				Params: mcp.CallToolParams{
					Name: "get_Albums",
					Arguments: map[string]any{
						"id": albumID,
					},
				},
			})
			require.NoError(t, err)

			textContent, ok := resp.Content[0].(mcp.TextContent)
			require.True(t, ok)

			var album Album
			err = json.Unmarshal([]byte(textContent.Text), &album)
			require.NoError(t, err)
			require.Equal(t, albumID, album.GetID())
			require.Equal(t, artistID, album.ArtistID)
			require.Equal(t, "Baby API Album 2", album.Title)
		})
	})

	t.Run("DeleteAlbum", func(t *testing.T) {
		resp, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name: "delete_Albums",
				Arguments: map[string]any{
					"id": albumID,
				},
			},
		})
		require.NoError(t, err)

		textContent, ok := resp.Content[0].(mcp.TextContent)
		require.True(t, ok)
		require.Equal(t, "deleted", textContent.Text)

		t.Run("GetAlbumByID_NotFound", func(t *testing.T) {
			_, err := mcpClient.CallTool(t.Context(), mcp.CallToolRequest{
				Request: mcp.Request{},
				Params: mcp.CallToolParams{
					Name: "get_Albums",
					Arguments: map[string]any{
						"id": albumID,
					},
				},
			})
			require.Error(t, err)
			require.Equal(t, "resource not found", err.Error())
		})
	})
}

func TestMCPEndDateable(t *testing.T) {
	todoAPI := babyapi.NewAPI("TODO", "/todos", func() *babyapi.EndDateableTODO { return &babyapi.EndDateableTODO{} }).
		EnableMCP(babyapi.MCPPermCRUD)

	serverURL, stop := babytest.TestServe(t, todoAPI)
	defer stop()

	mcpClient, err := client.NewStreamableHttpClient(serverURL + "/mcp")
	require.NoError(t, err)

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	_, err = mcpClient.Initialize(t.Context(), initReq)
	require.NoError(t, err)

	toolsResp, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)

	for _, tool := range toolsResp.Tools {
		if tool.Name == "list_TODO" {
			_, ok := tool.InputSchema.Properties["include_end_dated"]
			require.True(t, ok)
		}
	}
}

func TestMCPPermissions(t *testing.T) {
	tests := []struct {
		name              string
		perm              babyapi.MCPPerm
		expectedToolNames []string
	}{
		{
			"None",
			babyapi.MCPPermNone,
			[]string{},
		},
		{
			"ReadOnly",
			babyapi.MCPPermRead,
			[]string{"get_Albums", "list_Albums"},
		},
		{
			"CreateOnly",
			babyapi.MCPPermCreate,
			[]string{"create_Albums"},
		},
		{
			"UpdateOnly",
			babyapi.MCPPermUpdate,
			[]string{"update_Albums"},
		},
		{
			"DeleteOnly",
			babyapi.MCPPermDelete,
			[]string{"delete_Albums"},
		},
		{
			"ReadCreate",
			babyapi.MCPPermRead | babyapi.MCPPermCreate,
			[]string{"get_Albums", "list_Albums", "create_Albums"},
		},
		{
			"CRUD",
			babyapi.MCPPermCRUD,
			[]string{"get_Albums", "list_Albums", "create_Albums", "update_Albums", "delete_Albums"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			albumAPI := babyapi.NewAPI("Albums", "/albums", func() *Album { return &Album{} }).
				EnableMCP(tt.perm)

			serverURL, stop := babytest.TestServe(t, albumAPI)
			defer stop()

			mcpClient, err := client.NewStreamableHttpClient(serverURL + "/mcp")
			require.NoError(t, err)

			var initReq mcp.InitializeRequest
			initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
			_, err = mcpClient.Initialize(t.Context(), initReq)
			require.NoError(t, err)

			toolsResp, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
			require.NoError(t, err)

			var toolNames []string
			for _, tool := range toolsResp.Tools {
				toolNames = append(toolNames, tool.Name)
			}

			require.ElementsMatch(t, tt.expectedToolNames, toolNames)
		})
	}
}
