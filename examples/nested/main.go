package main

import (
	"fmt"
	"net/http"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/extensions"
	"github.com/go-chi/render"
)

type Artist struct {
	babyapi.DefaultResource
	Name string `json:"name"`
}

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

type API struct {
	Artists     *babyapi.API[*Artist]
	Albums      *babyapi.API[*Album]
	MusicVideos *babyapi.API[*MusicVideo]
	Songs       *babyapi.API[*Song]
}

func createAPI() API {
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

	artistAPI.AddNestedAPI(albumAPI)
	artistAPI.AddNestedAPI(musicVideoAPI)
	albumAPI.AddNestedAPI(songAPI)

	artistAPI.ApplyExtension(extensions.HATEOAS[*Artist]{})
	albumAPI.ApplyExtension(extensions.HATEOAS[*Album]{})
	musicVideoAPI.ApplyExtension(extensions.HATEOAS[*MusicVideo]{})
	songAPI.ApplyExtension(extensions.HATEOAS[*Song]{})

	return API{artistAPI, albumAPI, musicVideoAPI, songAPI}
}

func main() {
	api := createAPI()
	api.Artists.RunCLI()
}
