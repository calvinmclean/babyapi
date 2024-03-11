package babyapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	bindAddress string
	address     string
	pretty      bool
	headers     []string
	query       string
)

// RunCLI is an alternative entrypoint to running the API beyond just Serve. It allows running a server or client based on the provided
// CLI arguments. Use this in your main() function
func (a *API[T]) RunCLI() {
	err := a.Command().Execute()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func (a *API[T]) Command() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   a.name,
		Short: "automatic CLI for babyapi server",
		RunE:  a.serveCmd,
	}
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "run the API server",
		RunE:  a.serveCmd,
	}
	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "HTTP client for interacting with API Resources",
	}

	// TODO: Change to address since now flags are separate from client
	serveCmd.Flags().StringVar(&bindAddress, "bindAddress", "", "Address and port to bind to for example :8080 for port only, localhost:8080 or 172.0.0.1:8080")

	clientCmd.PersistentFlags().StringVar(&address, "address", "http://localhost:8080", "server address for client")
	clientCmd.PersistentFlags().BoolVar(&pretty, "pretty", true, "pretty print JSON if enabled")
	clientCmd.PersistentFlags().StringSliceVar(&headers, "headers", []string{}, "add headers to request")
	clientCmd.PersistentFlags().StringVarP(&query, "query", "q", "", "add query parameters to request")

	for name, client := range a.CreateClientMap(a.AnyClient(address)) {
		clientCmd.AddCommand(client.Command(name))
	}

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(clientCmd)

	return rootCmd
}

func (a *API[T]) serveCmd(_ *cobra.Command, _ []string) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		a.Stop()
	}()

	return a.Serve(bindAddress)
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

		childClient.SetCustomResponseCodeMap(child.getCustomResponseCodeMap())

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

func (c *Client[T]) Command(name string) *cobra.Command {
	clientCmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("client for interacting with %s resources", name),
	}

	runE := func(cmd *cobra.Command, args []string) error {
		c.Address = address

		result, err := c.RunFromCLI(append([]string{cmd.Name()}, args...), headers, query)
		if err != nil {
			return fmt.Errorf("error running client from CLI: %w", err)
		}

		return result.Fprint(cmd.OutOrStdout(), pretty)
	}

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "make a GET request to get a resource by ID",
		RunE:  runE,
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "make a GET request to list resources",
		RunE:  runE,
	}
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "make a DELETE request to delete a resource by ID",
		RunE:  runE,
	}
	postCmd := &cobra.Command{
		Use:   "post",
		Short: "make a POST request to create a new resource",
		RunE:  runE,
	}
	putCmd := &cobra.Command{
		Use:   "put",
		Short: "make a PUT request to create or modify a resource by ID",
		RunE:  runE,
	}
	patchCmd := &cobra.Command{
		Use:   "patch",
		Short: "make a PATCH request to modify a resource by ID",
		RunE:  runE,
	}

	clientCmd.AddCommand(getCmd)
	clientCmd.AddCommand(listCmd)
	clientCmd.AddCommand(deleteCmd)
	clientCmd.AddCommand(postCmd)
	clientCmd.AddCommand(putCmd)
	clientCmd.AddCommand(patchCmd)

	return clientCmd
}

func (c *Client[T]) RunFromCLI(args, requestHeaders []string, rawQuery string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}

	reqEditor := func(r *http.Request) error {
		for _, header := range requestHeaders {
			headerSplit := strings.SplitN(header, ":", 2)
			if len(headerSplit) != 2 {
				return fmt.Errorf("invalid header provided: %q", header)
			}

			header, val := strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1])

			r.Header.Add(header, val)
		}

		params, err := url.ParseQuery(rawQuery)
		if err != nil {
			return fmt.Errorf("error parsing query string: %w", err)
		}

		r.URL.RawQuery = params.Encode()

		return nil
	}

	c.SetRequestEditor(reqEditor)

	switch args[0] {
	case "get":
		return c.runGetCommand(args[1:])
	case "list":
		return c.runListCommand(args[1:])
	case "post":
		return c.runPostCommand(args[1:])
	case "put":
		return c.runPutCommand(args[1:])
	case "patch":
		return c.runPatchCommand(args[1:])
	case "delete":
		return c.runDeleteCommand(args[1:])
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
