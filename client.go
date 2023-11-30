package babyapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RequestEditor is a function that can modify the HTTP request before sending
type RequestEditor = func(*http.Request) error

var defaultRequestEditor RequestEditor = func(r *http.Request) error {
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
	return &Client[T]{addr, strings.TrimPrefix(base, "/"), http.DefaultClient, defaultRequestEditor, []string{}}
}

// NewSubClient creates a Client as a child of an existing Client. This is useful for accessing nested API resources
func NewSubClient[T, R Resource](parent *Client[T], path string) *Client[R] {
	newClient := NewClient[R](parent.addr, path)
	newClient.parentPaths = append(parent.parentPaths, parent.base)
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
func (c *Client[T]) Get(ctx context.Context, id string, parentIDs ...string) (T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL(id, parentIDs...), http.NoBody)
	if err != nil {
		return *new(T), fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequestWithResponse(req, http.StatusOK)
	if err != nil {
		return *new(T), fmt.Errorf("error getting resource: %w", err)
	}

	return result, nil
}

// GetAll gets all resources from the API
func (c *Client[T]) GetAll(ctx context.Context, query url.Values, parentIDs ...string) (*ResourceList[T], error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL("", parentIDs...), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.URL.RawQuery = query.Encode()

	resp, err := c.MakeRequest(req, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("error getting all resources: %w", err)
	}

	var result *ResourceList[T]
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return result, nil
}

// Put makes a PUT request to create/modify a resource by ID
func (c *Client[T]) Put(ctx context.Context, resource T, parentIDs ...string) error {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return fmt.Errorf("error encoding request body: %w", err)
	}

	return c.put(ctx, resource.GetID(), &body, parentIDs...)
}

// PutRaw makes a PUT request to create/modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PutRaw(ctx context.Context, id, body string, parentIDs ...string) error {
	return c.put(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) put(ctx context.Context, id string, body io.Reader, parentIDs ...string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.URL(id, parentIDs...), body)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	_, err = c.MakeRequest(req, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("error putting resource: %w", err)
	}

	return nil
}

// Post makes a POST request to create a new resource
func (c *Client[T]) Post(ctx context.Context, resource T, parentIDs ...string) (T, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return *new(T), fmt.Errorf("error encoding request body: %w", err)
	}

	return c.post(ctx, &body, parentIDs...)
}

// PostRaw makes a POST request using the provided string as the body
func (c *Client[T]) PostRaw(ctx context.Context, body string, parentIDs ...string) (T, error) {
	return c.post(ctx, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) post(ctx context.Context, body io.Reader, parentIDs ...string) (T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL("", parentIDs...), body)
	if err != nil {
		return *new(T), fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequestWithResponse(req, http.StatusCreated)
	if err != nil {
		return *new(T), fmt.Errorf("error posting resource: %w", err)
	}

	return result, nil
}

// Patch makes a PATCH request to modify a resource by ID
func (c *Client[T]) Patch(ctx context.Context, id string, resource T, parentIDs ...string) (T, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(resource)
	if err != nil {
		return *new(T), fmt.Errorf("error encoding request body: %w", err)
	}

	return c.patch(ctx, id, &body, parentIDs...)
}

// PatchRaw makes a PATCH request to modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PatchRaw(ctx context.Context, id, body string, parentIDs ...string) (T, error) {
	return c.patch(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) patch(ctx context.Context, id string, body io.Reader, parentIDs ...string) (T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.URL(id, parentIDs...), body)
	if err != nil {
		return *new(T), fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.MakeRequest(req, http.StatusOK)
	if err != nil {
		return *new(T), fmt.Errorf("error patching resource: %w", err)
	}

	var result T
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return *new(T), fmt.Errorf("error decoding response: %w", err)
	}

	return result, nil
}

// Delete makes a DELETE request to delete a resource by ID
func (c *Client[T]) Delete(ctx context.Context, id string, parentIDs ...string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.URL(id, parentIDs...), http.NoBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	_, err = c.MakeRequest(req, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("error deleting resource: %w", err)
	}

	return nil
}

// URL gets the URL based on provided ID and optional parent IDs
func (c *Client[T]) URL(id string, parentIDs ...string) string {
	if len(parentIDs) != len(c.parentPaths) {
		panic("incorrect number of parent IDs provided")
	}

	path := c.addr
	for i, parentPath := range c.parentPaths {
		path += fmt.Sprintf("/%s/%s", parentPath, parentIDs[i])
	}

	path += fmt.Sprintf("/%s", c.base)

	if id != "" {
		path += fmt.Sprintf("/%s", id)
	}

	return path
}

// MakeRequest generically sends an HTTP request after calling the request editor and checks the response code
func (c *Client[T]) MakeRequest(req *http.Request, expectedStatusCode int) (*http.Response, error) {
	req.Header.Add("Content-Type", "application/json")

	err := c.requestEditor(req)
	if err != nil {
		return nil, fmt.Errorf("error returned from request editor: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing request: %w", err)
	}

	if resp.StatusCode != expectedStatusCode {
		if resp.Body == nil {
			return nil, fmt.Errorf("unexpected status and no body: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error decoding error response: %w", err)
		}

		var httpErr *ErrResponse
		err = json.Unmarshal(body, &httpErr)
		if err != nil {
			return nil, fmt.Errorf("error decoding error response %q: %w", string(body), err)
		}
		return nil, httpErr
	}

	return resp, nil
}

// MakeRequestWithResponse calls MakeRequest and decodes the response body into the Resource type
func (c *Client[T]) MakeRequestWithResponse(req *http.Request, expectedStatusCode int) (T, error) {
	resp, err := c.MakeRequest(req, expectedStatusCode)
	if err != nil {
		return *new(T), err
	}

	var result T
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return *new(T), fmt.Errorf("error decoding response: %w", err)
	}

	return result, nil
}
