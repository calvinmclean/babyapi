package babyapi

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/exp/maps"
)

// stringSliceFlag is a custom flag type to handle multiple occurrences of the same string flag
type stringSliceFlag []string

// String is the string representation of the flag's value
func (ssf *stringSliceFlag) String() string {
	return fmt.Sprintf("%v", *ssf)
}

// Set appends the value to the slice
func (ssf *stringSliceFlag) Set(value string) error {
	*ssf = append(*ssf, value)
	return nil
}

func (a *API[T]) RunCLI() {
	var bindAddress string
	var address string
	var pretty bool
	var headers stringSliceFlag
	var query string
	flag.StringVar(&bindAddress, "bindAddress", "", "Address and port to bind to for example :8080 for port only, localhost:8080 or 172.0.0.1:8080")
	flag.StringVar(&address, "address", "http://localhost:8080", "server address for client")
	flag.BoolVar(&pretty, "pretty", true, "pretty print JSON if enabled")
	flag.Var(&headers, "H", "add headers to request")
	flag.StringVar(&query, "q", "", "add query parameters to request")

	flag.Parse()

	args := flag.Args()

	err := a.RunWithArgs(os.Stdout, args, bindAddress, address, pretty, headers, query)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func (a *API[T]) RunWithArgs(out io.Writer, args []string, bindAddress string, address string, pretty bool, headers []string, query string) error {
	if len(args) < 1 {
		return fmt.Errorf("at least one argument required")
	}

	if args[0] == "serve" {
		a.Serve(bindAddress)
		return nil
	}

	return a.runClientCLI(out, args, address, pretty, headers, query)
}

func (a *API[T]) runClientCLI(out io.Writer, args []string, address string, pretty bool, headers []string, query string) error {
	if len(args) < 2 {
		return fmt.Errorf("at least two arguments required")
	}

	clientMap := a.CreateClientMap(a.AnyClient(address))

	targetAPI := args[1]
	client, ok := clientMap[targetAPI]
	if !ok {
		return fmt.Errorf("invalid API %q. valid options are: %v", targetAPI, maps.Keys[map[string]*Client[*AnyResource]](clientMap))
	}

	result, err := client.RunFromCLI(args, headers, query)
	if err != nil {
		return fmt.Errorf("error running client from CLI: %w", err)
	}

	return result.Fprint(out, pretty)
}

// CreateClientMap returns a map of API names to the corresponding Client for that child API. This makes it easy to use
// child APIs dynamically. The initial parent/base client must be provided so child APIs can use NewSubClient
func (a *API[T]) CreateClientMap(parent *Client[*AnyResource]) map[string]*Client[*AnyResource] {
	clientMap := map[string]*Client[*AnyResource]{}
	if !a.rootAPI {
		clientMap[a.name] = parent
	}

	for _, child := range a.subAPIs {
		base := makePathWithRoot(child.Base(), a)
		var childClient *Client[*AnyResource]

		if a.rootAPI && a.parent == nil {
			// If the current API is a root API and has no parent, then this client has no need for parent IDs
			childClient = NewClient[*AnyResource](parent.Address, base)
		} else {
			childClient = NewSubClient[*AnyResource, *AnyResource](parent, base)
		}

		childMap := child.CreateClientMap(childClient)
		for n, c := range childMap {
			clientMap[n] = c
		}
	}

	return clientMap
}

// PrintableResponse allows CLI method to generically return a type that can be written to out
type PrintableResponse interface {
	Fprint(out io.Writer, pretty bool) error
}

// RunFromCLI executes the client with arguments from the CLI
func (c *Client[T]) RunFromCLI(args []string, headers []string, query string) (PrintableResponse, error) {
	reqEditor := func(r *http.Request) error {
		for _, header := range headers {
			headerSplit := strings.SplitN(header, ":", 2)
			if len(headerSplit) != 2 {
				return fmt.Errorf("invalid header provided: %q", header)
			}

			header, val := strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1])

			r.Header.Add(header, val)
		}

		params, err := url.ParseQuery(query)
		if err != nil {
			return fmt.Errorf("error parsing query string: %w", err)
		}

		r.URL.RawQuery = params.Encode()

		return nil
	}

	c.SetRequestEditor(reqEditor)

	switch args[0] {
	case "get":
		return c.runGetCommand(args[2:])
	case "list":
		return c.runListCommand(args[2:])
	case "post":
		return c.runPostCommand(args[2:])
	case "put":
		return c.runPutCommand(args[2:])
	case "patch":
		return c.runPatchCommand(args[2:])
	case "delete":
		return c.runDeleteCommand(args[2:])
	default:
		return nil, fmt.Errorf("missing http verb argument")
	}
}

func (c *Client[T]) runGetCommand(args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}

	result, err := c.Get(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Get: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runDeleteCommand(args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := c.Delete(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Delete: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runListCommand(args []string) (PrintableResponse, error) {
	items, err := c.GetAll(context.Background(), "", args[0:]...)
	if err != nil {
		return nil, fmt.Errorf("error running GetAll: %w", err)
	}

	return items, nil
}

func (c *Client[T]) runPostCommand(args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := c.PostRaw(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Post: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runPutCommand(args []string) (PrintableResponse, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("at least two arguments required")
	}
	result, err := c.PutRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Put: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runPatchCommand(args []string) (PrintableResponse, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("at least two arguments required")
	}
	result, err := c.PatchRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Patch: %w", err)
	}

	return result, nil
}
