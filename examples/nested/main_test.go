package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/calvinmclean/babyapi"
	babytest "github.com/calvinmclean/babyapi/test"
	"github.com/stretchr/testify/require"
)

func TestIndividualTest(t *testing.T) {
	api := createAPI()

	artist := &Artist{
		DefaultResource: babyapi.NewDefaultResource(),
		Name:            "Artist",
	}

	t.Run("SearchArtistForAlbum_Empty", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/artists/%s/albums", artist.GetID()), http.NoBody)
		w := babytest.TestWithParentRoute[*Album, *Artist](t, api.Albums, artist, "Artist", "/artists", r)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, `{"items":[]}`, strings.TrimSpace(w.Body.String()))
	})
}
