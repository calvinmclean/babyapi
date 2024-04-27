package babyapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func newResponse[T any](resp *http.Response, expectedStatusCode int) (*Response[T], error) {
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

	if resp.StatusCode != expectedStatusCode && expectedStatusCode != 0 {
		if result.Body == "" {
			return nil, fmt.Errorf("unexpected status and no body: %d", resp.StatusCode)
		}

		var httpErr *ErrResponse
		err := json.Unmarshal([]byte(result.Body), &httpErr)
		if err != nil {
			return nil, fmt.Errorf("error decoding error response %q: %w", result.Body, err)
		}
		httpErr.HTTPStatusCode = resp.StatusCode
		return nil, httpErr
	}

	if result.ContentType == "application/json" {
		err := json.Unmarshal([]byte(result.Body), &result.Data)
		if err != nil {
			return nil, fmt.Errorf("error decoding response body %q: %w", result.Body, err)
		}
	}

	return result, nil
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

type clientParent struct {
	name string
	path string
}

// Client is used to interact with the provided Resource's API
type Client[T Resource] struct {
	Address             string
	base                string
	name                string
	client              *http.Client
	requestEditor       RequestEditor
	parents             []clientParent
	customResponseCodes map[string]int
}

// NewClient initializes a Client for interacting with the Resource API
func NewClient[T Resource](addr, base string) *Client[T] {
	return &Client[T]{
		addr,
		strings.TrimLeft(base, "/"),
		"",
		http.DefaultClient,
		DefaultRequestEditor,
		[]clientParent{},
		defaultResponseCodes(),
	}
}

// NewSubClient creates a Client as a child of an existing Client. This is useful for accessing nested API resources
func NewSubClient[T, R Resource](parent *Client[T], path string) *Client[R] {
	newClient := NewClient[R](parent.Address, path)

	newClient.parents = make([]clientParent, len(parent.parents))
	copy(newClient.parents, parent.parents)

	if parent.base != "" {
		newClient.parents = append(newClient.parents, clientParent{path: parent.base, name: parent.name})
	}
	return newClient
}

// SetCustomResponseCode will override the default expected response codes for the specified HTTP verb
func (c *Client[T]) SetCustomResponseCode(verb string, code int) *Client[T] {
	c.customResponseCodes[verb] = code
	return c
}

// SetCustomResponseCodeMap sets the whole map for custom expected response codes
func (c *Client[T]) SetCustomResponseCodeMap(customResponseCodes map[string]int) *Client[T] {
	c.customResponseCodes = customResponseCodes
	return c
}

// SetHTTPClient allows overriding the Clients HTTP client with a custom one
func (c *Client[T]) SetHTTPClient(client *http.Client) *Client[T] {
	c.client = client
	return c
}

// SetRequestEditor sets a request editor function that is used to modify all requests before sending. This is useful
// for adding custom request headers or authorization
func (c *Client[T]) SetRequestEditor(requestEditor RequestEditor) *Client[T] {
	c.requestEditor = requestEditor
	return c
}

// Get will get a resource by ID
func (c *Client[T]) Get(ctx context.Context, id string, parentIDs ...string) (*Response[T], error) {
	req, err := c.GetRequest(ctx, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequest(req, c.customResponseCodes[http.MethodGet])
	if err != nil {
		return nil, fmt.Errorf("error getting resource: %w", err)
	}

	return result, nil
}

// GetRequest creates a request that can be used to get a resource
func (c *Client[T]) GetRequest(ctx context.Context, id string, parentIDs ...string) (*http.Request, error) {
	return c.NewRequestWithParentIDs(ctx, http.MethodGet, http.NoBody, id, parentIDs...)
}

// GetAll gets all resources from the API
func (c *Client[T]) GetAll(ctx context.Context, rawQuery string, parentIDs ...string) (*Response[*ResourceList[T]], error) {
	req, err := c.GetAllRequest(ctx, rawQuery, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := MakeRequest[*ResourceList[T]](req, c.client, c.customResponseCodes[MethodGetAll], c.requestEditor)
	if err != nil {
		return nil, fmt.Errorf("error getting all resources: %w", err)
	}

	return result, nil
}

// GetAllRequest creates a request that can be used to get all resources
func (c *Client[T]) GetAllRequest(ctx context.Context, rawQuery string, parentIDs ...string) (*http.Request, error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodGet, http.NoBody, "", parentIDs...)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = rawQuery

	return req, nil
}

// GetAllAny allows using GetAll when using a custom response wrapper
func (c *Client[T]) GetAllAny(ctx context.Context, rawQuery string, parentIDs ...string) (*Response[any], error) {
	req, err := c.GetAllRequest(ctx, rawQuery, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := MakeRequest[any](req, c.client, c.customResponseCodes[MethodGetAll], c.requestEditor)
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

// PutRequest creates a request that can be used to PUT a resource
func (c *Client[T]) PutRequest(ctx context.Context, body io.Reader, id string, parentIDs ...string) (*http.Request, error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPut, body, id, parentIDs...)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// PutRaw makes a PUT request to create/modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PutRaw(ctx context.Context, id, body string, parentIDs ...string) (*Response[T], error) {
	return c.put(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) put(ctx context.Context, id string, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.PutRequest(ctx, body, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequest(req, c.customResponseCodes[http.MethodPut])
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

// PostRequest creates a request that can be used to POST a resource
func (c *Client[T]) PostRequest(ctx context.Context, body io.Reader, parentIDs ...string) (*http.Request, error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPost, body, "", parentIDs...)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// PostRaw makes a POST request using the provided string as the body
func (c *Client[T]) PostRaw(ctx context.Context, body string, parentIDs ...string) (*Response[T], error) {
	return c.post(ctx, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) post(ctx context.Context, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.PostRequest(ctx, body, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	result, err := c.MakeRequest(req, c.customResponseCodes[http.MethodPost])
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

// PatchRequest creates a request that can be used to PATCH a resource
func (c *Client[T]) PatchRequest(ctx context.Context, body io.Reader, id string, parentIDs ...string) (*http.Request, error) {
	req, err := c.NewRequestWithParentIDs(ctx, http.MethodPatch, body, id, parentIDs...)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return req, nil
}

// PatchRaw makes a PATCH request to modify a resource by ID. It uses the provided string as the request body
func (c *Client[T]) PatchRaw(ctx context.Context, id, body string, parentIDs ...string) (*Response[T], error) {
	return c.patch(ctx, id, bytes.NewBufferString(body), parentIDs...)
}

func (c *Client[T]) patch(ctx context.Context, id string, body io.Reader, parentIDs ...string) (*Response[T], error) {
	req, err := c.PatchRequest(ctx, body, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.MakeRequest(req, c.customResponseCodes[http.MethodPatch])
	if err != nil {
		return nil, fmt.Errorf("error patching resource: %w", err)
	}

	return resp, nil
}

// Delete makes a DELETE request to delete a resource by ID
func (c *Client[T]) Delete(ctx context.Context, id string, parentIDs ...string) (*Response[T], error) {
	req, err := c.DeleteRequest(ctx, id, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.MakeRequest(req, c.customResponseCodes[http.MethodDelete])
	if err != nil {
		return nil, fmt.Errorf("error deleting resource: %w", err)
	}

	return resp, nil
}

// DeleteRequest creates a request that can be used to delete a resource
func (c *Client[T]) DeleteRequest(ctx context.Context, id string, parentIDs ...string) (*http.Request, error) {
	return c.NewRequestWithParentIDs(ctx, http.MethodDelete, http.NoBody, id, parentIDs...)
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
	if len(parentIDs) != len(c.parents) {
		return "", fmt.Errorf("expected %d parentIDs", len(c.parents))
	}

	path := c.Address
	for i, parent := range c.parents {
		path += fmt.Sprintf("/%s/%s", parent.path, parentIDs[i])
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

// MakeGenericRequest allows making a request without specifying the return type. It accepts a pointer receiver
// to pass to json.Unmarshal. This allows returning any type using the Client.
func (c *Client[T]) MakeGenericRequest(req *http.Request, target any) error {
	resp, err := makeRequest(req, c.client, c.requestEditor)
	if err != nil {
		return err
	}

	if resp.Body == nil {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error decoding error response: %w", err)
	}

	err = json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("error decoding response body %q: %w", string(body), err)
	}

	return nil
}

// MakeRequest generically sends an HTTP request after calling the request editor and checks the response code
// It returns a babyapi.Response which contains the http.Response after extracting the body to Body string and
// JSON decoding the resource type into Data if the response is JSON
func MakeRequest[T any](req *http.Request, client *http.Client, expectedStatusCode int, requestEditor RequestEditor) (*Response[T], error) {
	resp, err := makeRequest(req, client, requestEditor)
	if err != nil {
		return nil, err
	}

	return newResponse[T](resp, expectedStatusCode)
}

func makeRequest(req *http.Request, client *http.Client, requestEditor RequestEditor) (*http.Response, error) {
	if requestEditor != nil {
		err := requestEditor(req)
		if err != nil {
			return nil, fmt.Errorf("error returned from request editor: %w", err)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error doing request: %w", err)
	}

	return resp, nil
}

// makePathWithRoot will create a base API route if the parent is a root path. This is necessary because the parent
// root path could be defined as something other than / (slash)
func makePathWithRoot(base string, parent relatedAPI) string {
	if parent != nil && parent.isRoot() {
		return path.Join(parent.Base(), base)
	}

	return base
}
