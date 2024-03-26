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

func init() {
	cobra.EnableCaseInsensitive = true
}

// RunCLI is an alternative entrypoint to running the API beyond just Serve. It allows running a server or client based on the provided
// CLI arguments. Use this in your main() function
func (a *API[T]) RunCLI() {
	err := a.Command().Execute()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

type cliArgs struct {
	address string
	pretty  bool
	headers []string
	query   string
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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if a.cliArgs.address == "" {
				a.cliArgs.address = "http://localhost:8080"
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&a.cliArgs.address, "address", "", "bind address for server or target host address for client")

	clientCmd.PersistentFlags().BoolVar(&a.cliArgs.pretty, "pretty", true, "pretty print JSON if enabled")
	clientCmd.PersistentFlags().StringSliceVar(&a.cliArgs.headers, "headers", []string{}, "add headers to request")
	clientCmd.PersistentFlags().StringVarP(&a.cliArgs.query, "query", "q", "", "add query parameters to request")

	for name, client := range a.CreateClientMap(a.AnyClient(a.cliArgs.address)) {
		clientCmd.AddCommand(client.Command(name, &a.cliArgs))
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

	return a.Serve(a.cliArgs.address)
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
		childClient.name = child.Name()

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

func (c *Client[T]) Command(name string, input *cliArgs) *cobra.Command {
	clientCmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("client for interacting with %s resources", name),
	}

	// currently this is not working correctly because the child will override the length of shared parent IDs, so client.URL fails
	// this is because all subcommands use the same cliArgs struct
	parentIDs := make([]string, len(c.parents))
	for i, parent := range c.parents {
		flagName := fmt.Sprintf("%s-id", strings.ToLower(parent.name))

		clientCmd.PersistentFlags().StringVar(
			&(parentIDs[i]),
			flagName,
			"",
			fmt.Sprintf("ID for %q parent", parent.name),
		)

		_ = clientCmd.MarkPersistentFlagRequired(flagName)
	}

	var body string

	runE := func(cmd *cobra.Command, args []string) error {
		c.Address = input.address

		result, err := c.RunFromCLI(append([]string{cmd.Name()}, args...), parentIDs, input.headers, input.query, body)
		if err != nil {
			return fmt.Errorf("error running client from CLI: %w", err)
		}

		cmd.SetContext(NewContextWithCLIResult(cmd.Context(), result))

		return result.Fprint(cmd.OutOrStdout(), input.pretty)
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

	postCmd.Flags().StringVarP(&body, "data", "d", "", "data for request body")
	putCmd.Flags().StringVarP(&body, "data", "d", "", "data for request body")
	patchCmd.Flags().StringVarP(&body, "data", "d", "", "data for request body")

	_ = postCmd.MarkFlagRequired("data")
	_ = putCmd.MarkFlagRequired("data")
	_ = patchCmd.MarkFlagRequired("data")

	clientCmd.AddCommand(getCmd)
	clientCmd.AddCommand(listCmd)
	clientCmd.AddCommand(deleteCmd)
	clientCmd.AddCommand(postCmd)
	clientCmd.AddCommand(putCmd)
	clientCmd.AddCommand(patchCmd)

	return clientCmd
}

func (c *Client[T]) RunFromCLI(args, parentIDs, requestHeaders []string, rawQuery, body string) (PrintableResponse, error) {
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
		return c.runGetCommand(parentIDs, args[1:])
	case "list":
		return c.runListCommand(parentIDs)
	case "post":
		return c.runPostCommand(parentIDs, body)
	case "put":
		return c.runPutCommand(parentIDs, body, args[1:])
	case "patch":
		return c.runPatchCommand(parentIDs, body, args[1:])
	case "delete":
		return c.runDeleteCommand(parentIDs, args[1:])
	default:
		return nil, fmt.Errorf("missing http verb argument")
	}
}

func (c *Client[T]) runGetCommand(parentIDs, args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}

	result, err := c.Get(context.Background(), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running Get: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runDeleteCommand(parentIDs, args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := c.Delete(context.Background(), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running Delete: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runListCommand(parentIDs []string) (PrintableResponse, error) {
	items, err := c.GetAll(context.Background(), "", parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running GetAll: %w", err)
	}

	return items, nil
}

func (c *Client[T]) runPostCommand(parentIDs []string, body string) (PrintableResponse, error) {
	result, err := c.PostRaw(context.Background(), body, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running Post: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runPutCommand(parentIDs []string, body string, args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := c.PutRaw(context.Background(), args[0], body, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running Put: %w", err)
	}

	return result, nil
}

func (c *Client[T]) runPatchCommand(parentIDs []string, body string, args []string) (PrintableResponse, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	result, err := c.PatchRaw(context.Background(), args[0], body, parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error running Patch: %w", err)
	}

	return result, nil
}
