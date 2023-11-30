package babyapi

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func (a *API[T]) RunCLI() {
	var port int
	var address string
	var pretty bool
	flag.IntVar(&port, "port", 8080, "http port for server")
	flag.StringVar(&address, "address", "http://localhost:8080", "server address for client")
	flag.BoolVar(&pretty, "pretty", true, "pretty print JSON if enabled")

	flag.Parse()

	args := flag.Args()

	err := a.RunWithArgs(os.Stdout, args, port, address, pretty)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func (a *API[T]) RunWithArgs(out io.Writer, args []string, port int, address string, pretty bool) error {
	if len(args) < 1 {
		return fmt.Errorf("at least one argument required")
	}

	if args[0] == "serve" {
		a.Serve(fmt.Sprintf(":%d", port))
		return nil
	}

	return a.runClientCLI(out, args, address, pretty)
}

func (a *API[T]) runClientCLI(out io.Writer, args []string, address string, pretty bool) error {
	if len(args) < 1 {
		return fmt.Errorf("at least one argument required")
	}

	var cmd func([]string, string) (any, error)
	switch args[0] {
	case "get":
		cmd = a.runGetCommand
	case "list":
		cmd = a.runListCommand
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

	result, err := cmd(args[1:], address)
	if err != nil {
		return fmt.Errorf("error running client from CLI: %w", err)
	}

	encoder := json.NewEncoder(out)
	if pretty {
		encoder.SetIndent("", "\t")
	}
	return encoder.Encode(result)
}

func (a *API[T]) runGetCommand(args []string, address string) (any, error) {
	if len(args) < 1 {
		return *new(T), fmt.Errorf("at least one argument required")
	}
	result, err := a.Client(address).Get(context.Background(), args[0], args[1:]...)
	if err != nil {
		return *new(T), fmt.Errorf("error running Get: %w", err)
	}

	return result, nil
}

func (a *API[T]) runDeleteCommand(args []string, address string) (any, error) {
	if len(args) < 1 {
		return *new(T), fmt.Errorf("at least one argument required")
	}
	err := a.Client(address).Delete(context.Background(), args[0], args[1:]...)
	if err != nil {
		return *new(T), fmt.Errorf("error running Delete: %w", err)
	}

	return nil, nil
}

func (a *API[T]) runListCommand(args []string, address string) (any, error) {
	items, err := a.Client(address).GetAll(context.Background(), nil, args...)
	if err != nil {
		return nil, fmt.Errorf("error running GetAll: %w", err)
	}

	return items.Items, nil
}

func (a *API[T]) runPostCommand(args []string, address string) (any, error) {
	if len(args) < 1 {
		return *new(T), fmt.Errorf("at least one argument required")
	}
	result, err := a.Client(address).PostRaw(context.Background(), args[0], args[1:]...)
	if err != nil {
		return *new(T), fmt.Errorf("error running Post: %w", err)
	}

	return result, nil
}

func (a *API[T]) runPutCommand(args []string, address string) (any, error) {
	if len(args) < 2 {
		return *new(T), fmt.Errorf("at least two arguments required")
	}
	err := a.Client(address).PutRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return *new(T), fmt.Errorf("error running Put: %w", err)
	}

	return nil, nil
}

func (a *API[T]) runPatchCommand(args []string, address string) (any, error) {
	if len(args) < 2 {
		return *new(T), fmt.Errorf("at least two arguments required")
	}
	result, err := a.Client(address).PatchRaw(context.Background(), args[0], args[1], args[2:]...)
	if err != nil {
		return *new(T), fmt.Errorf("error running Patch: %w", err)
	}

	return result, nil
}
