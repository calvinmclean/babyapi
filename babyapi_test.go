package babyapi_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
)

type Album struct {
	babyapi.DefaultResource
	Title string `json:"title"`
}

func (a *Album) Patch(newAlbum *Album) *babyapi.ErrResponse {
	if newAlbum.Title != "" {
		a.Title = newAlbum.Title
	}

	return nil
}

func waitForAPI(address string) {
	const maxLoops = 10
	for loops := 0; loops < maxLoops; loops++ {
		_, err := http.Get(address)
		if err == nil { // Connection timeout is always an error
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestBabyAPI(t *testing.T) {
	api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
	api.AddCustomRoute(chi.Route{
		Pattern: "/teapot",
		Handlers: map[string]http.Handler{
			http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}),
		},
	})

	api.AddCustomIDRoute(chi.Route{
		Pattern: "/teapot",
		Handlers: map[string]http.Handler{
			http.MethodGet: api.GetRequestedResourceAndDo(func(r *http.Request, album *Album) (render.Renderer, *babyapi.ErrResponse) {
				render.Status(r, http.StatusTeapot)
				return album, nil
			}),
		},
	})

	api.SetGetAllFilter(func(r *http.Request) babyapi.FilterFunc[*Album] {
		return func(a *Album) bool {
			title := r.URL.Query().Get("title")
			return title == "" || a.Title == title
		}
	})

	album1 := &Album{Title: "Album1"}

	go api.Serve("localhost:8080")
	serverURL := "http://localhost:8080"

	waitForAPI(serverURL)

	client := api.Client(serverURL)

	t.Run("PostAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			result, err := client.Post(context.Background(), album1)
			require.NoError(t, err)
			album1 = result.Data
			require.NotEqual(t, xid.NilID(), album1.GetID())
		})
	})

	t.Run("ActionRoute", func(t *testing.T) {
		address, err := client.URL("")
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodGet, address+"/teapot", http.NoBody)
		require.NoError(t, err)
		_, err = client.MakeRequest(req, http.StatusTeapot)
		require.NoError(t, err)
	})

	t.Run("ActionIDRoute", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			address, err := client.URL(album1.GetID())
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodGet, address+"/teapot", http.NoBody)
			require.NoError(t, err)
			_, err = client.MakeRequest(req, http.StatusTeapot)
			require.NoError(t, err)
		})
	})

	t.Run("GetAll", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			albums, err := client.GetAll(context.Background(), "")
			require.NoError(t, err)
			require.ElementsMatch(t, []*Album{album1}, albums.Data.Items)
		})

		t.Run("SuccessfulWithFilter", func(t *testing.T) {
			albums, err := client.GetAll(context.Background(), "title=Album1")
			require.NoError(t, err)
			require.ElementsMatch(t, []*Album{album1}, albums.Data.Items)
		})

		t.Run("SuccessfulWithFilterShowingNoResults", func(t *testing.T) {
			albums, err := client.GetAll(context.Background(), "title=Album2")
			require.NoError(t, err)
			require.Len(t, albums.Data.Items, 0)
		})
	})

	t.Run("GetAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			a, err := client.Get(context.Background(), album1.GetID())
			require.NoError(t, err)
			require.Equal(t, album1, a.Data)
		})

		t.Run("NotFound", func(t *testing.T) {
			a, err := client.Get(context.Background(), "2")
			require.Nil(t, a)
			require.Error(t, err)
			require.Equal(t, "error getting resource: unexpected response with text: Resource not found.", err.Error())
		})
	})

	t.Run("PatchAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			a, err := client.Patch(context.Background(), album1.GetID(), &Album{Title: "New Title"})
			require.NoError(t, err)
			require.Equal(t, "New Title", a.Data.Title)
			require.Equal(t, album1.GetID(), a.Data.GetID())

			a, err = client.Get(context.Background(), album1.GetID())
			require.NoError(t, err)
			require.Equal(t, "New Title", a.Data.Title)
			require.Equal(t, album1.GetID(), a.Data.GetID())
		})

		t.Run("NotFound", func(t *testing.T) {
			a, err := client.Patch(context.Background(), "2", &Album{Title: "2"})
			require.Nil(t, a)
			require.Error(t, err)
			require.Equal(t, "error patching resource: unexpected response with text: Resource not found.", err.Error())
		})
	})

	t.Run("PutAlbum", func(t *testing.T) {
		t.Run("SuccessfulUpdateExisting", func(t *testing.T) {
			newAlbum1 := *album1
			newAlbum1.Title = "NewAlbum1"
			a, err := client.Put(context.Background(), &newAlbum1)
			require.NoError(t, err)
			require.Equal(t, "NewAlbum1", a.Data.Title)
			require.Equal(t, album1.GetID(), a.Data.GetID())

			a, err = client.Get(context.Background(), album1.GetID())
			require.NoError(t, err)
			require.Equal(t, newAlbum1, *a.Data)
		})

		t.Run("SuccessfulCreateNewAlbum", func(t *testing.T) {
			a, err := client.Put(context.Background(), &Album{DefaultResource: babyapi.NewDefaultResource(), Title: "PutNew"})
			require.NoError(t, err)
			require.Equal(t, "PutNew", a.Data.Title)
		})
	})

	t.Run("DeleteAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			resp, err := client.Delete(context.Background(), album1.GetID())
			require.NoError(t, err)
			require.Equal(t, http.NoBody, resp.Response.Body)
		})

		t.Run("NotFound", func(t *testing.T) {
			_, err := client.Delete(context.Background(), album1.GetID())
			require.Error(t, err)
			require.Equal(t, "error deleting resource: unexpected response with text: Resource not found.", err.Error())
		})
	})

	t.Run("Done", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			timeout := time.After(2 * time.Second)
			select {
			case <-api.Done():
				t.Log("successfully stopped")
			case <-timeout:
				t.Error("timed out before graceful shutdown")
			}
			wg.Done()
		}()

		api.Stop()
		wg.Wait()
	})
}

type Song struct {
	babyapi.DefaultResource
	Title string `json:"title"`
}

type SongResponse struct {
	*Song
	AlbumTitle string `json:"album_title"`
	ArtistName string `json:"artist_name"`

	api *babyapi.API[*Song] `json:"-"`
}

func (sr *SongResponse) Render(w http.ResponseWriter, r *http.Request) error {
	album, err := babyapi.GetResourceFromContext[*Album](r.Context(), sr.api.ParentContextKey())
	if err != nil {
		return fmt.Errorf("error getting album: %w", err)
	}
	sr.AlbumTitle = album.Title

	artist, err := babyapi.GetResourceFromContext[*Artist](r.Context(), babyapi.ContextKey(sr.api.Parent().Parent().Name()))
	if err != nil {
		return fmt.Errorf("error getting artist: %w", err)
	}
	sr.ArtistName = artist.Name

	return nil
}

type MusicVideo struct {
	babyapi.DefaultResource
	Title string `json:"title"`
}

func (m *MusicVideo) Patch(newVideo *MusicVideo) *babyapi.ErrResponse {
	if newVideo.Title != "" {
		m.Title = newVideo.Title
	}

	return nil
}

type Artist struct {
	babyapi.DefaultResource
	Name string `json:"name"`
}

func TestNestedAPI(t *testing.T) {
	artistAPI := babyapi.NewAPI[*Artist]("Artists", "/artists", func() *Artist { return &Artist{} })
	albumAPI := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
	musicVideoAPI := babyapi.NewAPI[*MusicVideo]("MusicVideos", "/music_videos", func() *MusicVideo { return &MusicVideo{} })
	songAPI := babyapi.NewAPI[*Song]("Songs", "/songs", func() *Song { return &Song{} })

	songAPI.SetResponseWrapper(func(s *Song) render.Renderer {
		return &SongResponse{Song: s, api: songAPI}
	})

	artistAPI.AddNestedAPI(albumAPI).AddNestedAPI(musicVideoAPI)
	albumAPI.AddNestedAPI(songAPI)

	serverURL, stop := babytest.TestServe[*Artist](t, artistAPI)
	defer stop()

	artist1 := &Artist{Name: "Artist1"}
	album1 := &Album{Title: "Album1"}
	musicVideo1 := &MusicVideo{Title: "MusicVideo1"}
	song1 := &Song{Title: "Song1"}

	var song1Response *SongResponse

	artistClient := artistAPI.Client(serverURL)
	albumClient := babyapi.NewSubClient[*Artist, *Album](artistClient, "/albums")
	musicVideoClient := babyapi.NewSubClient[*Artist, *MusicVideo](artistClient, "/music_videos")
	songClient := babyapi.NewSubClient[*Album, *SongResponse](albumClient, "/songs")

	t.Run("PostArtist", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			result, err := artistClient.Post(context.Background(), artist1)
			require.NoError(t, err)
			artist1 = result.Data
		})
	})

	t.Run("PostAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			result, err := albumClient.Post(context.Background(), album1, artist1.GetID())
			require.NoError(t, err)
			album1 = result.Data
		})
	})

	t.Run("PostMusicVideo", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			result, err := musicVideoClient.Post(context.Background(), musicVideo1, artist1.GetID())
			require.NoError(t, err)
			musicVideo1 = result.Data
		})
	})

	t.Run("PostAlbumSong", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			result, err := songClient.Post(context.Background(), &SongResponse{Song: song1}, artist1.GetID(), album1.GetID())
			require.NoError(t, err)
			song1Response = result.Data
		})
		t.Run("ErrorParentArtistDNE", func(t *testing.T) {
			_, err := songClient.Post(context.Background(), &SongResponse{Song: &Song{Title: "Song2"}}, "2", album1.GetID())
			require.Error(t, err)
		})
		t.Run("ErrorParentAlbumDNE", func(t *testing.T) {
			_, err := songClient.Post(context.Background(), &SongResponse{Song: &Song{Title: "Song2"}}, artist1.GetID(), "2")
			require.Error(t, err)
		})
	})

	t.Run("GetAlbumSong", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			s, err := songClient.Get(context.Background(), song1Response.GetID(), artist1.GetID(), album1.GetID())
			require.NoError(t, err)
			require.Equal(t, song1Response, s.Data)
		})

		t.Run("SuccessfulParsedAsSongResponse", func(t *testing.T) {
			req, err := songClient.NewRequestWithParentIDs(context.Background(), http.MethodGet, http.NoBody, song1Response.GetID(), artist1.GetID(), album1.GetID())
			require.NoError(t, err)

			resp, err := songClient.MakeRequest(req, http.StatusOK)
			require.NoError(t, err)

			require.Equal(t, "Album1", resp.Data.AlbumTitle)
			require.Equal(t, "Artist1", resp.Data.ArtistName)
		})

		t.Run("NotFound", func(t *testing.T) {
			_, err := songClient.Get(context.Background(), "2", artist1.GetID(), album1.GetID())
			require.Error(t, err)
			require.Equal(t, "error getting resource: unexpected response with text: Resource not found.", err.Error())
		})

		t.Run("NotFoundBecauseParentDNE", func(t *testing.T) {
			_, err := songClient.Get(context.Background(), song1Response.GetID(), artist1.GetID(), "2")
			require.Error(t, err)
			require.Equal(t, "error getting resource: unexpected response with text: Resource not found.", err.Error())
		})
	})

	t.Run("GetAllAlbums", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			albums, err := albumClient.GetAll(context.Background(), "", artist1.GetID())
			require.NoError(t, err)
			require.ElementsMatch(t, []*Album{album1}, albums.Data.Items)
		})
	})

	t.Run("GetAllSongs", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			songs, err := songClient.GetAll(context.Background(), "", artist1.GetID(), album1.GetID())
			require.NoError(t, err)
			require.ElementsMatch(t, []*SongResponse{song1Response}, songs.Data.Items)
		})
	})

	t.Run("PatchSong", func(t *testing.T) {
		t.Run("MethodNotAllowed", func(t *testing.T) {
			_, err := songClient.Patch(context.Background(), song1Response.GetID(), &SongResponse{Song: &Song{Title: "NewTitle"}}, artist1.GetID(), album1.GetID())
			require.Error(t, err)
			require.Equal(t, "error patching resource: unexpected response with text: Method not allowed.", err.Error())
		})
	})
}

func TestCLI(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedRegexp string
		expectedErr    bool
	}{
		{
			"MissingTargetAPIArg",
			[]string{},
			"at least one argument required",
			true,
		},
		{
			"InvalidTargetAPIArg",
			[]string{"bad", "bad"},
			`invalid API \"bad\". valid options are: (\[Albums Songs\]|\[Songs Albums\])`,
			true,
		},
		{
			"MissingArgs",
			[]string{"Albums"},
			"at least two arguments required",
			true,
		},
		{
			"GetAll",
			[]string{"list", "Albums"},
			`\[\{"id":"cljcqg5o402e9s28rbp0","title":"New Album"\}\]`,
			false,
		},
		{
			"Post",
			[]string{"post", "Albums", `{"title": "OtherNewAlbum"}`},
			`\{"id":"[0-9a-v]{20}","title":"OtherNewAlbum"\}`,
			false,
		},
		{
			"PostIncorrectParentArgs",
			[]string{"post", "Albums", `{"title": "OtherNewAlbum"}`, "ExtraID"},
			"error running client from CLI: error running Post: error creating request: error creating target URL: expected 0 parentIDs",
			true,
		},
		{
			"PostMissingArgs",
			[]string{"post", "Albums"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"PostError",
			[]string{"post", "Albums", `bad request`},
			"error running client from CLI: error running Post: error posting resource: unexpected response with text: Invalid request.",
			true,
		},
		{
			"Patch",
			[]string{"patch", "Albums", "cljcqg5o402e9s28rbp0", `{"title":"NewTitle"}`},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewTitle"\}`,
			false,
		},
		{
			"Put",
			[]string{"put", "Albums", "cljcqg5o402e9s28rbp0", `{"id":"cljcqg5o402e9s28rbp0","title":"NewAlbum"}`},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewAlbum"\}`,
			false,
		},
		{
			"PutError",
			[]string{"put", "Albums", "cljcqg5o402e9s28rbp0", `{"title":"NewAlbum"}`},
			"error running client from CLI: error running Put: error putting resource: unexpected response with text: Invalid request.",
			true,
		},
		{
			"GetByID",
			[]string{"get", "Albums", "cljcqg5o402e9s28rbp0"},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewAlbum"\}`,
			false,
		},
		{
			"GetByIDMissingArgs",
			[]string{"get", "Albums"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"GetAllSongs",
			[]string{"list", "Songs", "cljcqg5o402e9s28rbp0"},
			`\[{"id":"clknc0do4023onrn3bqg","title":"NewSong"}\]`,
			false,
		},
		{
			"GetSongByID",
			[]string{"get", "Songs", "clknc0do4023onrn3bqg", "cljcqg5o402e9s28rbp0"},
			`{"id":"clknc0do4023onrn3bqg","title":"NewSong"}`,
			false,
		},
		{
			"GetSongByIDMissingParentID",
			[]string{"get", "Songs", "clknc0do4023onrn3bqg"},
			"error running client from CLI: error running Get: error creating request: error creating target URL: expected 1 parentIDs",
			true,
		},
		{
			"PostSong",
			[]string{"post", "Songs", `{"title": "new song"}`, "cljcqg5o402e9s28rbp0"},
			`\{"id":"[0-9a-v]{20}","title":"new song"\}`,
			false,
		},
		{
			"Delete",
			[]string{"delete", "Albums", "cljcqg5o402e9s28rbp0"},
			``,
			false,
		},
		{
			"DeleteMissingArgs",
			[]string{"delete", "Albums"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"GetByIDNotFound",
			[]string{"get", "Albums", "cljcqg5o402e9s28rbp0"},
			"error running client from CLI: error running Get: error getting resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"DeleteNotFound",
			[]string{"delete", "Albums", "cljcqg5o402e9s28rbp0"},
			"error running client from CLI: error running Delete: error deleting resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"PatchNotFound",
			[]string{"patch", "Albums", "cljcqg5o402e9s28rbp0", ""},
			"error running client from CLI: error running Patch: error patching resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"PatchMissingArgs",
			[]string{"patch", "Albums"},
			"error running client from CLI: at least two arguments required",
			true,
		},
		{
			"PutMissingArgs",
			[]string{"put", "Albums"},
			"error running client from CLI: at least two arguments required",
			true,
		},
	}

	api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
	songAPI := babyapi.NewAPI[*Song]("Songs", "/songs", func() *Song { return &Song{} })
	api.AddNestedAPI(songAPI)
	go func() {
		err := api.RunWithArgs(os.Stdout, []string{"serve"}, "localhost:8080", "", false, nil, "")
		require.NoError(t, err)
	}()

	api.SetGetAllFilter(func(r *http.Request) babyapi.FilterFunc[*Album] {
		return func(a *Album) bool {
			title := r.URL.Query().Get("title")
			return title == "" || a.Title == title
		}
	})

	address := "http://localhost:8080"

	waitForAPI(address)

	// Create hard-coded album so we can use the ID
	album := &Album{DefaultResource: babyapi.NewDefaultResource(), Title: "New Album"}
	album.DefaultResource.ID.ID, _ = xid.FromString("cljcqg5o402e9s28rbp0")
	_, err := api.Client(address).Put(context.Background(), album)
	require.NoError(t, err)

	// Create hard-coded song so we can use the ID
	song := &Song{DefaultResource: babyapi.NewDefaultResource(), Title: "NewSong"}
	song.DefaultResource.ID.ID, _ = xid.FromString("clknc0do4023onrn3bqg")
	songClient := babyapi.NewSubClient[*Album, *Song](api.Client(address), "/songs")
	_, err = songClient.Put(context.Background(), song, album.GetID())
	require.NoError(t, err)

	t.Run("RunCLI", func(t *testing.T) {
		api.RunCLI()
	})

	t.Run("GetAllQueryParams", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			var out bytes.Buffer
			err := api.RunWithArgs(&out, []string{"list", "Albums"}, "", address, false, nil, "title=New Album")
			require.NoError(t, err)
			require.Equal(t, `{"items":[{"id":"cljcqg5o402e9s28rbp0","title":"New Album"}]}`, strings.TrimSpace(out.String()))
		})

		t.Run("NoMatch", func(t *testing.T) {
			var out bytes.Buffer
			err := api.RunWithArgs(&out, []string{"list", "Albums"}, "", address, false, nil, "title=badTitle")
			require.NoError(t, err)
			require.Equal(t, `{"items":[]}`, strings.TrimSpace(out.String()))
		})
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := api.RunWithArgs(&out, tt.args, "", address, false, nil, "")
			if !tt.expectedErr {
				require.NoError(t, err)
				require.Regexp(t, tt.expectedRegexp, strings.TrimSpace(out.String()))
				if tt.expectedRegexp == "" {
					require.Equal(t, tt.expectedRegexp, strings.TrimSpace(out.String()))
				}
			} else {
				require.Error(t, err)
				require.Regexp(t, tt.expectedRegexp, err.Error())
			}
		})
	}

	api.Stop()
}

type UnorderedList struct {
	Items []*ListItem
}

func (ul *UnorderedList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (ul *UnorderedList) HTML(r *http.Request) string {
	templates := map[string]string{
		"ul": `<ul>
{{- range .Items }}
{{ template "li" . }}
{{- end }}
</ul>`,
		"li": `<li>{{ .Content }}</li>`,
	}

	return babyapi.MustRenderHTMLMap(nil, templates, "ul", ul)
}

type ListItem struct {
	babyapi.DefaultResource
	Content string
}

func (d *ListItem) HTML(*http.Request) string {
	tmpl := template.Must(template.New("li").Parse(`<li>{{ .Content }}</li>`))
	return babyapi.MustRenderHTML(tmpl, d)
}

func TestHTML(t *testing.T) {
	api := babyapi.NewAPI[*ListItem]("Items", "/items", func() *ListItem { return &ListItem{} })

	api.SetGetAllResponseWrapper(func(d []*ListItem) render.Renderer {
		return &UnorderedList{d}
	})

	item1 := &ListItem{
		DefaultResource: babyapi.NewDefaultResource(),
		Content:         "Item1",
	}

	address, closer := babytest.TestServe[*ListItem](t, api)
	defer closer()

	client := api.Client(address)

	t.Run("CreateItem", func(t *testing.T) {
		err := api.Storage.Set(item1)
		require.NoError(t, err)
	})

	t.Run("GetItemHTML", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			url, err := client.URL(item1.GetID())
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")

			resp, err := client.MakeRequest(req, http.StatusOK)
			require.NoError(t, err)

			require.Equal(t, "<li>Item1</li>", resp.Body)
		})
	})

	t.Run("GetAllItemsHTML", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			url, err := client.URL("")
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")

			resp, err := client.MakeRequest(req, http.StatusOK)
			require.NoError(t, err)

			require.Equal(t, `<ul>
<li>Item1</li>
</ul>`, resp.Body)
		})
	})
}

func TestServerSentEvents(t *testing.T) {
	api := babyapi.NewAPI[*ListItem]("Items", "/items", func() *ListItem { return &ListItem{} })

	api.SetGetAllResponseWrapper(func(d []*ListItem) render.Renderer {
		return &UnorderedList{d}
	})

	events := api.AddServerSentEventHandler("/events")

	address, closer := babytest.TestServe[*ListItem](t, api)
	defer closer()

	item1 := &ListItem{
		DefaultResource: babyapi.NewDefaultResource(),
		Content:         "Item1",
	}
	t.Run("CreateItem", func(t *testing.T) {
		err := api.Storage.Set(item1)
		require.NoError(t, err)
	})

	t.Run("GetServerSentEventsEndpoint", func(t *testing.T) {
		quitTest := make(chan bool)
		go func() {
			for {
				select {
				case <-quitTest:
					return
				default:
					events <- &babyapi.ServerSentEvent{
						Event: "event",
						Data:  "hello",
					}
				}
			}
		}()
		response, err := http.Get(address + "/items/events")
		quitTest <- true
		require.NoError(t, err)
		defer response.Body.Close()

		require.Equal(t, http.StatusOK, response.StatusCode)

		expected := `event: event
data: hello
`
		body := make([]byte, len(expected))
		n, err := response.Body.Read(body)
		require.NoError(t, err)

		require.Equal(t, expected, string(body[:n]))
	})
}

func TestMustRenderHTML(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("{{ .UndefinedVariable }}"))
	require.Panics(t, func() {
		babyapi.MustRenderHTML(tmpl, "string is bad input")
	})
}

func TestAPIModifiers(t *testing.T) {
	middleware := 0
	idMiddlewareWithRequestResource := 0
	idMiddleware := 0
	beforeDelete := 0
	afterDelete := 0
	onCreateOrUpdate := 0
	afterCreateOrUpdate := 0

	api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} }).
		SetCustomResponseCode(http.MethodPut, http.StatusTeapot).
		AddMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				middleware++
				next.ServeHTTP(w, r)
			})
		}).
		SetOnCreateOrUpdate(func(r *http.Request, a *Album) *babyapi.ErrResponse {
			onCreateOrUpdate++
			return nil
		}).
		SetAfterCreateOrUpdate(func(r *http.Request, a *Album) *babyapi.ErrResponse {
			afterCreateOrUpdate++
			return nil
		}).
		SetBeforeDelete(func(r *http.Request) *babyapi.ErrResponse {
			beforeDelete++
			return nil
		}).
		SetAfterDelete(func(r *http.Request) *babyapi.ErrResponse {
			afterDelete++
			return nil
		})

	api.AddIDMiddleware(api.GetRequestedResourceAndDoMiddleware(
		func(r *http.Request, a *Album) (*http.Request, *babyapi.ErrResponse) {
			require.NotNil(t, a)
			idMiddlewareWithRequestResource++
			return r, nil
		},
	))

	api.AddIDMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idMiddleware++
			next.ServeHTTP(w, r)
		})
	})

	albumID := "cljcqg5o402e9s28rbp0"
	t.Run("CreateResource", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"Title": "NewAlbum", "ID": "%s"}`, albumID))
		r, err := http.NewRequest(http.MethodPut, "/albums/"+albumID, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", "application/json")

		w := babytest.TestRequest[*Album](t, api, r)
		require.Equal(t, http.StatusTeapot, w.Result().StatusCode)
	})

	t.Run("DeleteResource", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodDelete, "/albums/"+albumID, http.NoBody)
		require.NoError(t, err)

		w := babytest.TestRequest[*Album](t, api, r)
		require.Equal(t, http.StatusNoContent, w.Result().StatusCode)
	})

	t.Run("GetResourceNotFound", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "/albums/DoesNotExist", http.NoBody)
		require.NoError(t, err)

		w := babytest.TestRequest[*Album](t, api, r)
		require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})

	t.Run("AssertAllMiddlewaresUsed", func(t *testing.T) {
		// All requests hit this middleware
		require.Equal(t, 3, middleware)
		// All ID requests hit this except for the GET 404
		require.Equal(t, 2, idMiddleware)
		// Only hit for DELETE because PUT doesn't use it for creating new resources and GET had 404
		require.Equal(t, 1, idMiddlewareWithRequestResource)
		require.Equal(t, 1, beforeDelete)
		require.Equal(t, 1, afterDelete)
		require.Equal(t, 1, onCreateOrUpdate)
		require.Equal(t, 1, afterCreateOrUpdate)
	})
}

func TestAPIModifierErrors(t *testing.T) {
	t.Run("OnCreateOrUpdateErrors", func(t *testing.T) {
		api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
		albumID := "cljcqg5o402e9s28rbp0"

		api.SetOnCreateOrUpdate(func(_ *http.Request, _ *Album) *babyapi.ErrResponse {
			return babyapi.ErrRender(fmt.Errorf("test error"))
		})

		body := bytes.NewBufferString(fmt.Sprintf(`{"Title": "NewAlbum", "ID": "%s"}`, albumID))
		r, err := http.NewRequest(http.MethodPut, "/albums/"+albumID, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", "application/json")

		w := babytest.TestRequest[*Album](t, api, r)
		require.Equal(t, http.StatusUnprocessableEntity, w.Result().StatusCode)

		allAlbums, err := api.Storage.GetAll(nil)
		require.NoError(t, err)

		require.Equal(t, 0, len(allAlbums))
	})

	t.Run("AfterCreateOrUpdateErrors", func(t *testing.T) {
		api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
		albumID := "cljcqg5o402e9s28rbp0"

		api.SetAfterCreateOrUpdate(func(_ *http.Request, _ *Album) *babyapi.ErrResponse {
			return babyapi.ErrRender(fmt.Errorf("test error"))
		})

		body := bytes.NewBufferString(fmt.Sprintf(`{"Title": "NewAlbum", "ID": "%s"}`, albumID))
		r, err := http.NewRequest(http.MethodPut, "/albums/"+albumID, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", "application/json")
		w := babytest.TestRequest[*Album](t, api, r)
		require.Equal(t, http.StatusUnprocessableEntity, w.Result().StatusCode)

		allAlbums, err := api.Storage.GetAll(nil)
		require.NoError(t, err)

		require.Greater(t, len(allAlbums), 0)
	})

	t.Run("BeforeDeleteErrors", func(t *testing.T) {

	})

	t.Run("BeforeDeleteErrors", func(t *testing.T) {
		api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
		albumID := "cljcqg5o402e9s28rbp0"

		api.SetBeforeDelete(func(_ *http.Request) *babyapi.ErrResponse {
			return babyapi.ErrRender(fmt.Errorf("test error"))
		})

		t.Run("CreateInitialAlbum", func(t *testing.T) {
			body := bytes.NewBufferString(fmt.Sprintf(`{"Title": "NewAlbum", "ID": "%s"}`, albumID))
			r, err := http.NewRequest(http.MethodPut, "/albums/"+albumID, body)
			require.NoError(t, err)
			r.Header.Add("Content-Type", "application/json")
			babytest.TestRequest[*Album](t, api, r)

			allAlbums, err := api.Storage.GetAll(nil)
			require.NoError(t, err)

			require.Greater(t, len(allAlbums), 0)
		})

		t.Run("MakeDeleteRequest", func(t *testing.T) {
			r, err := http.NewRequest(http.MethodDelete, "/albums/"+albumID, http.NoBody)
			require.NoError(t, err)

			w := babytest.TestRequest[*Album](t, api, r)

			require.Equal(t, http.StatusUnprocessableEntity, w.Result().StatusCode)

			allAlbums, err := api.Storage.GetAll(nil)
			require.NoError(t, err)

			require.Equal(t, len(allAlbums), 1)
		})
	})

	t.Run("AfterDeleteErrors", func(t *testing.T) {
		api := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
		albumID := "cljcqg5o402e9s28rbp0"

		api.SetAfterDelete(func(_ *http.Request) *babyapi.ErrResponse {
			return babyapi.ErrRender(fmt.Errorf("test error"))
		})

		t.Run("CreateInitialAlbum", func(t *testing.T) {
			body := bytes.NewBufferString(fmt.Sprintf(`{"Title": "NewAlbum", "ID": "%s"}`, albumID))
			r, err := http.NewRequest(http.MethodPut, "/albums/"+albumID, body)
			require.NoError(t, err)
			r.Header.Add("Content-Type", "application/json")
			babytest.TestRequest[*Album](t, api, r)

			allAlbums, err := api.Storage.GetAll(nil)
			require.NoError(t, err)

			require.Greater(t, len(allAlbums), 0)
		})

		t.Run("MakeDeleteRequest", func(t *testing.T) {
			r, err := http.NewRequest(http.MethodDelete, "/albums/"+albumID, http.NoBody)
			require.NoError(t, err)

			w := babytest.TestRequest[*Album](t, api, r)

			require.Equal(t, http.StatusUnprocessableEntity, w.Result().StatusCode)

			allAlbums, err := api.Storage.GetAll(nil)
			require.NoError(t, err)
			afterCount := len(allAlbums)

			require.Less(t, afterCount, 1)
		})
	})
}

func TestRootAPIWithMiddlewareAndCustomHandlers(t *testing.T) {
	api := babyapi.NewRootAPI("root", "/")

	t.Run("CustomizationsForIDsPanics", func(t *testing.T) {
		require.Panics(t, func() {
			api.AddCustomIDRoute(chi.Route{})
		})
		require.Panics(t, func() {
			api.AddIDMiddleware(func(h http.Handler) http.Handler {
				return nil
			})
		})
	})

	api.Get = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}
	api.Delete = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	}
	api.Patch = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(203)
	}
	api.Post = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}
	api.Put = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(205)
	}

	api.AddCustomRoute(chi.Route{
		Handlers: map[string]http.Handler{
			http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(206)
			}),
		},
		Pattern: "/customRoute",
	})

	api.AddCustomRootRoute(chi.Route{
		Handlers: map[string]http.Handler{
			http.MethodGet: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(207)
			}),
		},
		Pattern: "/customRootRoute",
	})

	middlewareHits := 0
	api.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			middlewareHits++
			next.ServeHTTP(w, r)
		})
	})

	tests := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		{http.MethodGet, "/", 201},
		{http.MethodDelete, "/", 202},
		{http.MethodPatch, "/", 203},
		{http.MethodPost, "/", 204},
		{http.MethodPut, "/", 205},
		{http.MethodGet, "/customRoute", 206},
		{http.MethodGet, "/customRootRoute", 207},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			w := babytest.TestRequest[*babyapi.NilResource](t, api, r)

			require.Equal(t, tt.expectedStatus, w.Result().StatusCode)
		})
	}

	t.Run("MiddlewareIsHitForEachRequest", func(t *testing.T) {
		require.Equal(t, len(tests), middlewareHits)
	})
}

func TestRootAPIAsChildOfResourceAPI(t *testing.T) {
	musicVideoAPI := babyapi.NewAPI[*MusicVideo]("MusicVideos", "/music_videos", func() *MusicVideo { return &MusicVideo{} })
	songAPI := babyapi.NewAPI[*Song]("Songs", "/songs", func() *Song { return &Song{} })
	rootAPI := babyapi.
		NewRootAPI("root", "/").
		AddNestedAPI(musicVideoAPI).
		AddNestedAPI(songAPI)

	artistAPI := babyapi.NewAPI[*Artist]("Artists", "/artists", func() *Artist { return &Artist{} })
	artistAPI.AddNestedAPI(rootAPI)

	go func() {
		err := artistAPI.RunWithArgs(os.Stdout, []string{"serve"}, "localhost:8080", "", false, nil, "")
		require.NoError(t, err)
	}()

	address := "http://localhost:8080"

	waitForAPI(address)

	artist1 := &Artist{Name: "Artist1"}
	t.Run("CreateParentArtist", func(t *testing.T) {
		result, err := artistAPI.Client(address).Post(context.Background(), artist1)
		require.NoError(t, err)
		artist1 = result.Data
	})

	t.Run("TestGetAllSongsEmpty", func(t *testing.T) {
		var out bytes.Buffer
		err := artistAPI.RunWithArgs(&out, []string{"list", "Songs", artist1.GetID()}, "", address, false, nil, "")
		require.NoError(t, err)
		require.Regexp(t, `{"items":\[\]}`, strings.TrimSpace(out.String()))
	})

	t.Run("CreateSong", func(t *testing.T) {
		var out bytes.Buffer
		err := artistAPI.RunWithArgs(&out, []string{"post", "Songs", `{"title": "new song"}`, artist1.GetID()}, "", address, false, nil, "")
		require.NoError(t, err)
		require.Regexp(t, `\{"id":"[0-9a-v]{20}","title":"new song"\}`, strings.TrimSpace(out.String()))
	})

	artistAPI.Stop()
}

func TestRootAPICLI(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedRegexp string
		expectedErr    bool
	}{
		{
			"MissingTargetAPIArg",
			[]string{},
			"at least one argument required",
			true,
		},
		{
			"InvalidTargetAPIArg",
			[]string{"bad", "bad"},
			`invalid API \"bad\". valid options are: (\[MusicVideos Songs\]|\[Songs MusicVideos\])`,
			true,
		},
		{
			"MissingArgs",
			[]string{"MusicVideos"},
			"at least two arguments required",
			true,
		},
		{
			"GetAll",
			[]string{"list", "MusicVideos"},
			`\[\{"id":"cljcqg5o402e9s28rbp0","title":"New Video"\}\]`,
			false,
		},
		{
			"Post",
			[]string{"post", "MusicVideos", `{"title": "OtherNewMusicVideo"}`},
			`\{"id":"[0-9a-v]{20}","title":"OtherNewMusicVideo"\}`,
			false,
		},
		{
			"PostIncorrectParentArgs",
			[]string{"post", "MusicVideos", `{"title": "OtherNewMusicVideo"}`, "ExtraID"},
			"error running client from CLI: error running Post: error creating request: error creating target URL: expected 0 parentIDs",
			true,
		},
		{
			"PostMissingArgs",
			[]string{"post", "MusicVideos"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"PostError",
			[]string{"post", "MusicVideos", `bad request`},
			"error running client from CLI: error running Post: error posting resource: unexpected response with text: Invalid request.",
			true,
		},
		{
			"Patch",
			[]string{"patch", "MusicVideos", "cljcqg5o402e9s28rbp0", `{"title":"NewTitle"}`},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewTitle"\}`,
			false,
		},
		{
			"Put",
			[]string{"put", "MusicVideos", "cljcqg5o402e9s28rbp0", `{"id":"cljcqg5o402e9s28rbp0","title":"NewMusicVideo"}`},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewMusicVideo"\}`,
			false,
		},
		{
			"PutError",
			[]string{"put", "MusicVideos", "cljcqg5o402e9s28rbp0", `{"title":"NewMusicVideo"}`},
			"error running client from CLI: error running Put: error putting resource: unexpected response with text: Invalid request.",
			true,
		},
		{
			"GetByID",
			[]string{"get", "MusicVideos", "cljcqg5o402e9s28rbp0"},
			`\{"id":"cljcqg5o402e9s28rbp0","title":"NewMusicVideo"\}`,
			false,
		},
		{
			"GetByIDMissingArgs",
			[]string{"get", "MusicVideos"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"GetAllSongs",
			[]string{"list", "Songs"},
			`\[{"id":"clknc0do4023onrn3bqg","title":"NewSong"}\]`,
			false,
		},
		{
			"GetSongByID",
			[]string{"get", "Songs", "clknc0do4023onrn3bqg"},
			`{"id":"clknc0do4023onrn3bqg","title":"NewSong"}`,
			false,
		},
		{
			"PostSong",
			[]string{"post", "Songs", `{"title": "new song"}`},
			`\{"id":"[0-9a-v]{20}","title":"new song"\}`,
			false,
		},
		{
			"Delete",
			[]string{"delete", "MusicVideos", "cljcqg5o402e9s28rbp0"},
			``,
			false,
		},
		{
			"DeleteMissingArgs",
			[]string{"delete", "MusicVideos"},
			"error running client from CLI: at least one argument required",
			true,
		},
		{
			"GetByIDNotFound",
			[]string{"get", "MusicVideos", "cljcqg5o402e9s28rbp0"},
			"error running client from CLI: error running Get: error getting resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"DeleteNotFound",
			[]string{"delete", "MusicVideos", "cljcqg5o402e9s28rbp0"},
			"error running client from CLI: error running Delete: error deleting resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"PatchNotFound",
			[]string{"patch", "MusicVideos", "cljcqg5o402e9s28rbp0", ""},
			"error running client from CLI: error running Patch: error patching resource: unexpected response with text: Resource not found.",
			true,
		},
		{
			"PatchMissingArgs",
			[]string{"patch", "MusicVideos"},
			"error running client from CLI: at least two arguments required",
			true,
		},
		{
			"PutMissingArgs",
			[]string{"put", "MusicVideos"},
			"error running client from CLI: at least two arguments required",
			true,
		},
	}

	basePaths := []string{
		"/",
		"/api",
	}

	for _, base := range basePaths {
		t.Run("BasePath"+base, func(t *testing.T) {

			musicVideoAPI := babyapi.NewAPI[*MusicVideo]("MusicVideos", "/music_videos", func() *MusicVideo { return &MusicVideo{} })
			songAPI := babyapi.NewAPI[*Song]("Songs", "/songs", func() *Song { return &Song{} })
			rootAPI := babyapi.
				NewRootAPI("root", base).
				AddNestedAPI(musicVideoAPI).
				AddNestedAPI(songAPI)

			go func() {
				err := rootAPI.RunWithArgs(os.Stdout, []string{"serve"}, "localhost:8080", "", false, nil, "")
				require.NoError(t, err)
			}()

			songAPI.SetGetAllFilter(func(r *http.Request) babyapi.FilterFunc[*Song] {
				return func(s *Song) bool {
					title := r.URL.Query().Get("title")
					return title == "" || s.Title == title
				}
			})

			musicVideoAPI.SetGetAllFilter(func(r *http.Request) babyapi.FilterFunc[*MusicVideo] {
				return func(m *MusicVideo) bool {
					title := r.URL.Query().Get("title")
					return title == "" || m.Title == title
				}
			})

			address := "http://localhost:8080"

			waitForAPI(address)

			// Create hard-coded musicVideo so we can use the ID
			musicVideo := &MusicVideo{DefaultResource: babyapi.NewDefaultResource(), Title: "New Video"}
			musicVideo.DefaultResource.ID.ID, _ = xid.FromString("cljcqg5o402e9s28rbp0")
			_, err := musicVideoAPI.Client(address).Put(context.Background(), musicVideo)
			require.NoError(t, err)

			// Create hard-coded song so we can use the ID
			song := &Song{DefaultResource: babyapi.NewDefaultResource(), Title: "NewSong"}
			song.DefaultResource.ID.ID, _ = xid.FromString("clknc0do4023onrn3bqg")
			_, err = songAPI.Client(address).Put(context.Background(), song)
			require.NoError(t, err)

			t.Run("GetAllQueryParams", func(t *testing.T) {
				t.Run("Successful", func(t *testing.T) {
					var out bytes.Buffer
					err := rootAPI.RunWithArgs(&out, []string{"list", "MusicVideos"}, "", address, false, nil, "title=New Video")
					require.NoError(t, err)
					require.Equal(t, `{"items":[{"id":"cljcqg5o402e9s28rbp0","title":"New Video"}]}`, strings.TrimSpace(out.String()))
				})

				t.Run("NoMatch", func(t *testing.T) {
					var out bytes.Buffer
					err := rootAPI.RunWithArgs(&out, []string{"list", "MusicVideos"}, "", address, false, nil, "title=badTitle")
					require.NoError(t, err)
					require.Equal(t, `{"items":[]}`, strings.TrimSpace(out.String()))
				})
			})

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var out bytes.Buffer
					err := rootAPI.RunWithArgs(&out, tt.args, "", address, false, nil, "")
					if !tt.expectedErr {
						require.NoError(t, err)
						require.Regexp(t, tt.expectedRegexp, strings.TrimSpace(out.String()))
						if tt.expectedRegexp == "" {
							require.Equal(t, tt.expectedRegexp, strings.TrimSpace(out.String()))
						}
					} else {
						require.Error(t, err)
						require.Regexp(t, tt.expectedRegexp, err.Error())
					}
				})
			}

			rootAPI.Stop()
		})
	}
}
