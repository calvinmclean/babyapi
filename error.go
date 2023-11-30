package babyapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/render"
)

var ErrNotFoundResponse = &ErrResponse{HTTPStatusCode: http.StatusNotFound, StatusText: "Resource not found."}
var ErrMethodNotAllowedResponse = &ErrResponse{HTTPStatusCode: http.StatusMethodNotAllowed, StatusText: "Method not allowed."}

// ErrResponse is an error that implements Renderer to be used in HTTP response
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Error() string {
	return fmt.Sprintf("unexpected response with text: %s", e.StatusText)
}

func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrRender(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 422,
		StatusText:     "Error rendering response.",
		ErrorText:      err.Error(),
	}
}

func ErrInvalidRequest(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		StatusText:     "Invalid request.",
		ErrorText:      err.Error(),
	}
}

func InternalServerError(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 500,
		StatusText:     "Server Error.",
		ErrorText:      err.Error(),
	}
}
