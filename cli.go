package babyapi

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
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
	var port int
	var address string
	var pretty bool
	var headers stringSliceFlag
	flag.IntVar(&port, "port", 8080, "http port for server")
	flag.StringVar(&address, "address", "http://localhost:8080", "server address for client")
	flag.BoolVar(&pretty, "pretty", true, "pretty print JSON if enabled")
	flag.Var(&headers, "H", "add headers to request")

	flag.Parse()

	args := flag.Args()

	err := a.RunWithArgs(os.Stdout, args, port, address, pretty, headers)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func (a *API[T]) RunWithArgs(out io.Writer, args []string, port int, address string, pretty bool, headers []string) error {
	if len(args) < 1 {
		return fmt.Errorf("at least one argument required")
	}

	if args[0] == "serve" {
		a.Serve(fmt.Sprintf(":%d", port))
		return nil
	}

	return a.runClientCLI(out, args, address, pretty, headers)
}

func (a *API[T]) buildClientMap(selfClient *Client[*AnyResource], clientMap map[string]*Client[*AnyResource], reqEditor func(*http.Request) error) {
	for name, child := range a.subAPIs {
		childClient := NewSubClient[*AnyResource, *AnyResource](selfClient, child.Base())
		childClient.SetRequestEditor(reqEditor)

		clientMap[name] = childClient
		child.buildClientMap(childClient, clientMap, reqEditor)
	}
}

func (a *API[T]) runClientCLI(out io.Writer, args []string, address string, pretty bool, headers []string) error {
	if len(args) < 2 {
		return fmt.Errorf("at least two arguments required")
	}

	reqEditor := func(r *http.Request) error {
		for _, header := range headers {
			headerSplit := strings.SplitN(header, ":", 2)
			if len(headerSplit) != 2 {
				return fmt.Errorf("invalid header provided: %q", header)
			}

			header, val := strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1])

			r.Header.Add(header, val)
		}
		return nil
	}

	selfClient := a.AnyClient(address)
	selfClient.SetRequestEditor(reqEditor)

	clientMap := map[string]*Client[*AnyResource]{
		a.name: selfClient,
	}
	a.buildClientMap(selfClient, clientMap, reqEditor)

	targetAPI := args[1]
	client, ok := clientMap[targetAPI]
	if !ok {
		return fmt.Errorf("invalid API %q. valid options are: %v", targetAPI, maps.Keys[map[string]*Client[*AnyResource]](clientMap))
	}

	var cmd func([]string, *Client[*AnyResource]) (*Response[*AnyResource], error)
	switch args[0] {
	case "get":
		cmd = a.runGetCommand
	case "list":
		// list needs to be handled separately because ot returns *Response[*ResourceList[*AnyResource]]
		result, err := a.runListCommand(args[2:], client)
		if err != nil {
			return fmt.Errorf("error running client from CLI: %w", err)
		}
		return result.Fprint(out, pretty)
	case "post":
		cmd = a.runPostCommand
	case "put":
		cmd = a.runPutCommand
	case "patch":
		cmd = a.runPatchCommand
	case "delete":
		cmd = a.runDeleteCommand
	default:
		flag.Usage()
		return nil
	}

	result, err := cmd(args[2:], client)
	if err != nil {
		return fmt.Errorf("error running client from CLI: %w", err)
	}

	return result.Fprint(out, pretty)
}

func (a *API[T]) runGetCommand(args []string, client *Client[*AnyResource]) (*Response[*AnyResource], error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}

	result, err := client.Get(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Get: %w", err)
	}

	return result, nil
}

func (a *API[T]) runDeleteCommand(args []string, client *Client[*AnyResource]) (*Response[*AnyResource], error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	err := client.Delete(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Delete: %w", err)
	}

	return nil, nil
}

func (a *API[T]) runListCommand(args []string, client *Client[*AnyResource]) (*Response[*ResourceList[*AnyResource]], error) {
	items, err := client.GetAll(context.Background(), nil, args[0:]...)
	if err != nil {
		return nil, fmt.Errorf("error running GetAll: %w", err)
	}

	return items, nil
}

func (a *API[T]) runPostCommand(args []string, client *Client[*AnyResource]) (*Response[*AnyResource], error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := client.PostRaw(context.Background(), args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Post: %w", err)
	}

	return result, nil
}

func (a *API[T]) runPutCommand(args []string, client *Client[*AnyResource]) (*Response[*AnyResource], error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("at least two arguments required")
	}
	err := client.PutRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Put: %w", err)
	}

	return nil, nil
}

func (a *API[T]) runPatchCommand(args []string, client *Client[*AnyResource]) (*Response[*AnyResource], error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("at least two arguments required")
	}
	result, err := client.PatchRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return nil, fmt.Errorf("error running Patch: %w", err)
	}

	return result, nil
}
