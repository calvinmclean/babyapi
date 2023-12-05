package babyapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/calvinmclean/babyapi"
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

func TestBabyAPI(t *testing.T) {
	tests := []struct {
		name  string
		start func(*babyapi.API[*Album]) (string, func())
	}{
		{
			"UseTestServe",
			func(api *babyapi.API[*Album]) (string, func()) {
				return babyapi.TestServe[*Album](t, api)
			},
		},
		{
			"UseAPIStart",
			func(api *babyapi.API[*Album]) (string, func()) {
				go api.Serve(":8080")
				return "http://localhost:8080", func() {
					// Test `Done()`
					go func() {
						timeout := time.After(2 * time.Second)
						select {
						case <-api.Done():
						case <-timeout:
							t.Error("timed out before graceful shutdown")
						}
					}()

					api.Stop()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			serverURL, stop := tt.start(api)
			defer stop()

			client := api.Client(serverURL)

			t.Run("PostAlbum", func(t *testing.T) {
				t.Run("Successful", func(t *testing.T) {
					var err error
					album1, err = client.Post(context.Background(), album1)
					require.NoError(t, err)
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
					albums, err := client.GetAll(context.Background(), nil)
					require.NoError(t, err)
					require.ElementsMatch(t, []*Album{album1}, albums.Items)
				})

				t.Run("SuccessfulWithFilter", func(t *testing.T) {
					albums, err := client.GetAll(context.Background(), url.Values{
						"title": []string{"Album1"},
					})
					require.NoError(t, err)
					require.ElementsMatch(t, []*Album{album1}, albums.Items)
				})

				t.Run("SuccessfulWithFilterShowingNoResults", func(t *testing.T) {
					albums, err := client.GetAll(context.Background(), url.Values{
						"title": []string{"Album2"},
					})
					require.NoError(t, err)
					require.Len(t, albums.Items, 0)
				})
			})

			t.Run("GetAlbum", func(t *testing.T) {
				t.Run("Successful", func(t *testing.T) {
					a, err := client.Get(context.Background(), album1.GetID())
					require.NoError(t, err)
					require.Equal(t, album1, a)
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
					require.Equal(t, "New Title", a.Title)
					require.Equal(t, album1.GetID(), a.GetID())

					a, err = client.Get(context.Background(), album1.GetID())
					require.NoError(t, err)
					require.Equal(t, "New Title", a.Title)
					require.Equal(t, album1.GetID(), a.GetID())
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
					err := client.Put(context.Background(), &newAlbum1)
					require.NoError(t, err)

					a, err := client.Get(context.Background(), album1.GetID())
					require.NoError(t, err)
					require.Equal(t, newAlbum1, *a)
				})

				t.Run("SuccessfulCreateNewAlbum", func(t *testing.T) {
					err := client.Put(context.Background(), &Album{DefaultResource: babyapi.NewDefaultResource()})
					require.NoError(t, err)
				})
			})

			t.Run("DeleteAlbum", func(t *testing.T) {
				t.Run("Successful", func(t *testing.T) {
					err := client.Delete(context.Background(), album1.GetID())
					require.NoError(t, err)
				})

				t.Run("NotFound", func(t *testing.T) {
					err := client.Delete(context.Background(), album1.GetID())
					require.Error(t, err)
					require.Equal(t, "error deleting resource: unexpected response with text: Resource not found.", err.Error())
				})
			})
		})
	}
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

type Artist struct {
	babyapi.DefaultResource
	Name string `json:"name"`
}

func TestNestedAPI(t *testing.T) {
	artistAPI := babyapi.NewAPI[*Artist]("Artists", "/artists", func() *Artist { return &Artist{} })
	albumAPI := babyapi.NewAPI[*Album]("Albums", "/albums", func() *Album { return &Album{} })
	musicVideoAPI := babyapi.NewAPI[*MusicVideo]("MusicVideos", "/music_videos", func() *MusicVideo { return &MusicVideo{} })
	songAPI := babyapi.NewAPI[*Song]("Songs", "/songs", func() *Song { return &Song{} })

	songAPI.ResponseWrapper(func(s *Song) render.Renderer {
		return &SongResponse{Song: s, api: songAPI}
	})

	artistAPI.AddNestedAPI(albumAPI)
	artistAPI.AddNestedAPI(musicVideoAPI)
	albumAPI.AddNestedAPI(songAPI)

	serverURL, stop := babyapi.TestServe[*Artist](t, artistAPI)
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
			var err error
			artist1, err = artistClient.Post(context.Background(), artist1)
			require.NoError(t, err)
		})
	})

	t.Run("PostAlbum", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			var err error
			album1, err = albumClient.Post(context.Background(), album1, artist1.GetID())
			require.NoError(t, err)
		})
	})

	t.Run("PostMusicVideo", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			var err error
			musicVideo1, err = musicVideoClient.Post(context.Background(), musicVideo1, artist1.GetID())
			require.NoError(t, err)
		})
	})

	t.Run("PostAlbumSong", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			var err error
			song1Response, err = songClient.Post(context.Background(), &SongResponse{Song: song1}, artist1.GetID(), album1.GetID())
			require.NoError(t, err)
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
			require.Equal(t, song1Response, s)
		})

		t.Run("SuccessfulParsedAsSongResponse", func(t *testing.T) {
			req, err := songClient.NewRequestWithParentIDs(context.Background(), http.MethodGet, http.NoBody, song1Response.GetID(), artist1.GetID(), album1.GetID())
			require.NoError(t, err)

			resp, err := songClient.MakeRequest(req, http.StatusOK)
			require.NoError(t, err)

			var sr SongResponse
			err = json.NewDecoder(resp.Body).Decode(&sr)
			require.NoError(t, err)

			require.Equal(t, "Album1", sr.AlbumTitle)
			require.Equal(t, "Artist1", sr.ArtistName)
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
			albums, err := albumClient.GetAll(context.Background(), nil, artist1.GetID())
			require.NoError(t, err)
			require.ElementsMatch(t, []*Album{album1}, albums.Items)
		})
	})

	t.Run("GetAllSongs", func(t *testing.T) {
		t.Run("Successful", func(t *testing.T) {
			songs, err := songClient.GetAll(context.Background(), nil, artist1.GetID(), album1.GetID())
			require.NoError(t, err)
			require.ElementsMatch(t, []*SongResponse{song1Response}, songs.Items)
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
			`\[\{"id":"cljcqg5o402e9s28rbp0","title":"NewAlbum"\}\]`,
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
			`null`,
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
			`null`,
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
		err := api.RunWithArgs(os.Stdout, []string{"serve"}, 8080, "", false)
		require.NoError(t, err)
	}()
	defer api.Stop()

	address := "http://localhost:8080"

	// Create hard-coded album so we can use the ID
	album := &Album{DefaultResource: babyapi.NewDefaultResource(), Title: "NewAlbum"}
	album.DefaultResource.ID.ID, _ = xid.FromString("cljcqg5o402e9s28rbp0")
	err := api.Client(address).Put(context.Background(), album)
	require.NoError(t, err)

	// Create hard-coded song so we can use the ID
	song := &Song{DefaultResource: babyapi.NewDefaultResource(), Title: "NewSong"}
	song.DefaultResource.ID.ID, _ = xid.FromString("clknc0do4023onrn3bqg")
	songClient := babyapi.NewSubClient[*Album, *Song](api.Client(address), "/songs")
	err = songClient.Put(context.Background(), song, album.GetID())
	require.NoError(t, err)

	t.Run("RunCLI", func(t *testing.T) {
		api.RunCLI()
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := api.RunWithArgs(&out, tt.args, 0, address, false)
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
}

type UnorderedList struct {
	Items []*ListItem
}

func (ul *UnorderedList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (ul *UnorderedList) HTML() string {
	result := "<ul>\n"
	for _, li := range ul.Items {
		result += li.HTML() + "\n"
	}
	return result + "</ul>"
}

type ListItem struct {
	babyapi.DefaultResource
	Content string
}

func (d *ListItem) HTML() string {
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

	address, closer := babyapi.TestServe[*ListItem](t, api)
	defer closer()

	client := api.Client(address)

	t.Run("CreateItem", func(t *testing.T) {
		err := api.Storage().Set(item1)
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

			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "<li>Item1</li>", string(data))
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

			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, `<ul>
<li>Item1</li>
</ul>`, string(data))
		})
	})
}

func TestServerSentEvents(t *testing.T) {
	api := babyapi.NewAPI[*ListItem]("Items", "/items", func() *ListItem { return &ListItem{} })

	api.SetGetAllResponseWrapper(func(d []*ListItem) render.Renderer {
		return &UnorderedList{d}
	})

	events := api.AddServerSentEventHandler("/events")

	address, closer := babyapi.TestServe[*ListItem](t, api)
	defer closer()

	item1 := &ListItem{
		DefaultResource: babyapi.NewDefaultResource(),
		Content:         "Item1",
	}
	t.Run("CreateItem", func(t *testing.T) {
		err := api.Storage().Set(item1)
		require.NoError(t, err)
	})

	t.Run("GetServerSentEventsEndpoint", func(t *testing.T) {
		go func() {
			events <- &babyapi.ServerSentEvent{
				Event: "event",
				Data:  "hello",
			}
		}()

		response, err := http.Get(address + "/items/events")
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
