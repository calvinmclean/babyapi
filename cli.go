package babyapi

import (
	"bytes"
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
	reqEditor := func(r *http.Request) error {
		for _, header := range input.headers {
			headerSplit := strings.SplitN(header, ":", 2)
			if len(headerSplit) != 2 {
				return fmt.Errorf("invalid header provided: %q", header)
			}

			header, val := strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1])

			r.Header.Add(header, val)
		}

		params, err := url.ParseQuery(input.query)
		if err != nil {
			return fmt.Errorf("error parsing query string: %w", err)
		}

		r.URL.RawQuery = params.Encode()

		return nil
	}

	var req *http.Request
	clientCmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("client for interacting with %s resources", name),
		// This will execute the request created by the subcommand and print the output
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			result, err := MakeRequest[any](req, c.client, 0, reqEditor)
			if err != nil {
				return fmt.Errorf("error executing request: %w", err)
			}

			return result.Fprint(cmd.OutOrStdout(), input.pretty)
		},
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

	// Each command's RunE will create the appropriate *http.Request
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "make a GET request to get a resource by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliGetRequest(parentIDs, args)
			return err
		},
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "make a GET request to list resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliGetAllRequest(parentIDs)
			return err
		},
	}
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "make a DELETE request to delete a resource by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliDeleteRequest(parentIDs, args)
			return err
		},
	}
	postCmd := &cobra.Command{
		Use:   "post",
		Short: "make a POST request to create a new resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliPostRequest(parentIDs, body)
			return err
		},
	}
	putCmd := &cobra.Command{
		Use:   "put",
		Short: "make a PUT request to create or modify a resource by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliPutRequest(parentIDs, body, args)
			return err
		},
	}
	patchCmd := &cobra.Command{
		Use:   "patch",
		Short: "make a PATCH request to modify a resource by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Address = input.address

			var err error
			req, err = c.cliPatchRequest(parentIDs, body, args)
			return err
		},
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

func (c *Client[T]) cliGetRequest(parentIDs, args []string) (*http.Request, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}

	req, err := c.GetRequest(context.Background(), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating GET request: %w", err)
	}

	return req, nil
}

func (c *Client[T]) cliDeleteRequest(parentIDs, args []string) (*http.Request, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	req, err := c.DeleteRequest(context.Background(), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error created DELETE request: %w", err)
	}

	return req, nil
}

func (c *Client[T]) cliGetAllRequest(parentIDs []string) (*http.Request, error) {
	req, err := c.GetAllRequest(context.Background(), "", parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating GET all request: %w", err)
	}

	return req, nil
}

func (c *Client[T]) cliPostRequest(parentIDs []string, body string) (*http.Request, error) {
	req, err := c.PostRequest(context.Background(), bytes.NewBufferString(body), parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating POST request: %w", err)
	}

	return req, nil
}

func (c *Client[T]) cliPutRequest(parentIDs []string, body string, args []string) (*http.Request, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	req, err := c.PutRequest(context.Background(), bytes.NewBufferString(body), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating PUT request: %w", err)
	}

	return req, nil
}

func (c *Client[T]) cliPatchRequest(parentIDs []string, body string, args []string) (*http.Request, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("at least one argument required")
	}
	req, err := c.PatchRequest(context.Background(), bytes.NewBufferString(body), args[0], parentIDs...)
	if err != nil {
		return nil, fmt.Errorf("error creating PATCH request: %w", err)
	}

	return req, nil
}
