package main

import (
	"fmt"
	"net/http"

	"github.com/calvinmclean/babyapi"
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

func main() {
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

	artistAPI.Start(":8080")
}
