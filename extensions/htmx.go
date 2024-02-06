package extensions

import (
	"net/http"

	"github.com/calvinmclean/babyapi"
)

// HTMX is a shortcut to apply HTMX compatibility. Currently this just sets a 200 response on Delete
type HTMX[T babyapi.Resource] struct{}

func (HTMX[T]) Apply(api *babyapi.API[T]) error {
	// HTMX requires a 200 response code to do a swap after delete
	api.SetCustomResponseCode(http.MethodDelete, http.StatusOK)
	return nil
}
