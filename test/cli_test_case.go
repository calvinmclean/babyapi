package babytest

import (
	"bytes"
	"testing"

	"github.com/calvinmclean/babyapi"
	"github.com/spf13/cobra"
)

// CommandLineTest allows simulating CLI usage by passing a list of arguments
type CommandLineTest[T babyapi.Resource] struct {
	// Command is the api.Command function that is used to create a new command. Always use the parent API
	Command  func() *cobra.Command
	Args     []string
	ArgsFunc func(getResponse PreviousResponseGetter) []string
}

var _ Test[*babyapi.AnyResource] = CommandLineTest[*babyapi.AnyResource]{}

func (tt CommandLineTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*Response[T], error) {
	args := tt.Args
	if tt.ArgsFunc != nil {
		args = tt.ArgsFunc(getResponse)
	}

	cmd := tt.Command()

	var cliOut bytes.Buffer
	cmd.SetOut(&cliOut)
	cmd.SetArgs(append([]string{"client", "--address", client.Address}, args...))

	var out any
	cmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		out = babyapi.GetCLIResultFromContext(cmd.Context())
	}

	err := cmd.Execute()

	switch v := out.(type) {
	case *babyapi.Response[T]:
		return &Response[T]{Response: v, CLIOut: cliOut.String()}, err
	case *babyapi.Response[*babyapi.ResourceList[T]]:
		return &Response[T]{GetAllResponse: v, CLIOut: cliOut.String()}, err
	case nil:
		return &Response[T]{CLIOut: cliOut.String()}, err
	default:
		t.Errorf("unexpected type for CLI response: %T", v)
		return nil, err
	}
}
