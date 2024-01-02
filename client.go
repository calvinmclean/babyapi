package babyapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Response wraps an HTTP response from the API and allows easy access to the decoded response type (if JSON),
// the ContentType, string Body, and the original response
type Response[T any] struct {
	ContentType string
	Body        string
	Data        T
	Response    *http.Response
}

// Fprint writes the Response body to the provided Writer. If the ContentType is JSON, it will JSON encode
// the body. Setting pretty=true will print indented JSON.
func (sr *Response[T]) Fprint(out io.Writer, pretty bool) error {
	if sr == nil {
		_, err := fmt.Fprint(out, "null")
		return err
	}

	var err error
	switch sr.ContentType {
	case "application/json":
		encoder := json.NewEncoder(out)
		if pretty {
			encoder.SetIndent("", "\t")
		}
		err = encoder.Encode(sr.Data)
	default:
		_, err = fmt.Fprint(out, sr.Body)
	}
	return err
}

// RequestEditor is a function that can modify the HTTP request before sending
type RequestEditor = func(*http.Request) error

var DefaultRequestEditor RequestEditor = func(r *http.Request) error {
	return nil
}

// Client is used to interact with the provided Resource's API
type Client[T Resource] struct {
	addr          string
	base          string
	client        *http.Client
	requestEditor RequestEditor
	parentPaths   []string
}

// NewClient initializes a Client for interacting with the Resource API
func NewClient[T Resource](addr, base string) *Client[T] {
	return &Client[T]{addr, strings.TrimLeft(base, "/"), http.DefaultClient, DefaultRequestEditor, []string{}}
}

// NewSubClient creates a Client as a child of an existing Client. This is useful for accessing nested API resources
func NewSubClient[T, R Resource](parent *Client[T], path string) *Client[R] {
	newClient := NewClient[R](parent.addr, path)

	newClient.parentPaths = make([]string, len(parent.parentPaths))
	copy(newClient.parentPaths, parent.parentPaths)

	if parent.base != "" {
		newClient.parentPaths = append(newClient.parentPaths, parent.base)
	}
	return newClient
}

// SetHTTPClient allows overriding the Clients HTTP client with a custom one
func (c *Client[T]) SetHTTPClient(client *http.Client) {
	c.client = client
}

// SetRequestEditor sets a request editor function that is used to modify all requests before sending. This is useful
// for adding custom request headers or authorization
func (c *Client[T]) SetRequestEditor(requestEditor RequestEditor) {
	c.requestEditor = requestEditor
}

// Get will get a resource by ID
func (c *Client[T]) Get(ctx context.Context, id string, parentIDs ...string) (*Response[T], error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodGet, http.NoBody, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequest(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("error getting resource: %w", err)
	}

	return result, nil
}

// GetAll gets all resources from the API
func (c *Client[T]) GetAll(ctx context.Context, query url.Values, parentIDs ...string) (*Response[*ResourceList[T]], error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodGet, http.NoBody, "", parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.URL.RawQuery = query.Encode()

	result, err := MakeRequest[*ResourceList[T]](req, c.client, http.StatusOK, c.requestEditor)
	if err != nil {
		return nil, fmt.Errorf("error getting all resources: %w", err)
	}

	return result, nil
}

// Put makes a PUT request to create/modify a resource by ID
func (c *Client[T]) Put(ctx context.Context, resource T, parentIDs ...string) (*Response[T], error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return nil, fmt.Errorf("error encoding request body: %w", err)
	}

	return c.put(ctx, resource.GetID(), &body, parentIDs...)
}

// PutRaw makes a PUT request to create/modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PutRaw(ctx context.Context, id, body string, parentIDs ...string) (*Response[T], error) {
	return c.put(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) put(ctx context.Context, id string, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPut, body, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	result, err := c.MakeRequest(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("error putting resource: %w", err)
	}

	return result, nil
}

// Post makes a POST request to create a new resource
func (c *Client[T]) Post(ctx context.Context, resource T, parentIDs ...string) (*Response[T], error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return nil, fmt.Errorf("error encoding request body: %w", err)
	}

	return c.post(ctx, &body, parentIDs...)
}

// PostRaw makes a POST request using the provided string as the body
func (c *Client[T]) PostRaw(ctx context.Context, body string, parentIDs ...string) (*Response[T], error) {
	return c.post(ctx, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) post(ctx context.Context, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPost, body, "", parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	result, err := c.MakeRequest(req, http.StatusCreated)
	if err != nil {
		return result, fmt.Errorf("error posting resource: %w", err)
	}

	return result, nil
}

// Patch makes a PATCH request to modify a resource by ID
func (c *Client[T]) Patch(ctx context.Context, id string, resource T, parentIDs ...string) (*Response[T], error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return nil, fmt.Errorf("error encoding request body: %w", err)
	}

	return c.patch(ctx, id, &body, parentIDs...)
}

// PatchRaw makes a PATCH request to modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PatchRaw(ctx context.Context, id, body string, parentIDs ...string) (*Response[T], error) {
	return c.patch(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) patch(ctx context.Context, id string, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPatch, body, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := c.MakeRequest(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("error patching resource: %w", err)
	}

	return resp, nil
}

// Delete makes a DELETE request to delete a resource by ID
func (c *Client[T]) Delete(ctx context.Context, id string, parentIDs ...string) error {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodDelete, http.NoBody, id, parentIDs...)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	_, err = c.MakeRequest(req, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("error deleting resource: %w", err)
	}

	return nil
}

// NewRequestWithParentIDs uses http.NewRequestWithContext to create a new request using the URL created from the provided ID and parent IDs
func (c *Client[T]) NewRequestWithParentIDs(ctx context.Context, method string, body io.Reader, id string, parentIDs ...string) (*http.Request, error) {
	address, err := c.URL(id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating target URL: %w", err)
	}

	return http.NewRequestWithContext(ctx, method, address, body)
}

// URL gets the URL based on provided ID and optional parent IDs
func (c *Client[T]) URL(id string, parentIDs ...string) (string, error) {
	if len(parentIDs) != len(c.parentPaths) {
		return "", fmt.Errorf("expected %d parentIDs", len(c.parentPaths))
	}

	path := c.addr
	for i, parentPath := range c.parentPaths {
		path += fmt.Sprintf("/%s/%s", parentPath, parentIDs[i])
	}

	path += fmt.Sprintf("/%s", c.base)

	if id != "" {
		path += fmt.Sprintf("/%s", id)
	}

	return path, nil
}

// MakeRequest generically sends an HTTP request after calling the request editor and checks the response code
// It returns a babyapi.Response which contains the http.Response after extracting the body to Body string and
// JSON decoding the resource type into Data if the response is JSON
func (c *Client[T]) MakeRequest(req *http.Request, expectedStatusCode int) (*Response[T], error) {
	return MakeRequest[T](req, c.client, expectedStatusCode, c.requestEditor)
}

// MakeRequest generically sends an HTTP request after calling the request editor and checks the response code
// It returns a babyapi.Response which contains the http.Response after extracting the body to Body string and
// JSON decoding the resource type into Data if the response is JSON
func MakeRequest[T any](req *http.Request, client *http.Client, expectedStatusCode int, requestEditor RequestEditor) (*Response[T], error) {
	err := requestEditor(req)
	if err != nil {
		return nil, fmt.Errorf("error returned from request editor: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing request: %w", err)
	}

	result := &Response[T]{
		ContentType: resp.Header.Get("Content-Type"),
		Response:    resp,
	}

	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error decoding error response: %w", err)
		}
		result.Body = string(body)
	}

	if resp.StatusCode != expectedStatusCode {
		if result.Body == "" {
			return nil, fmt.Errorf("unexpected status and no body: %d", resp.StatusCode)
		}

		var httpErr *ErrResponse
		err = json.Unmarshal([]byte(result.Body), &httpErr)
		if err != nil {
			return nil, fmt.Errorf("error decoding error response %q: %w", result.Body, err)
		}
		httpErr.HTTPStatusCode = resp.StatusCode
		return result, httpErr
	}

	if result.ContentType == "application/json" {
		err = json.Unmarshal([]byte(result.Body), &result.Data)
		if err != nil {
			return nil, fmt.Errorf("error decoding response body %q: %w", result.Body, err)
		}
	}

	return result, nil
}

// makePathWithRoot will create a base API route if the parent is a root path. This is necessary because the parent
// root path could be defined as something other than / (slash)
func makePathWithRoot(base string, parent relatedAPI) string {
	if parent != nil && parent.isRoot() {
		return path.Join(parent.Base(), base)
	}

	return base
}
