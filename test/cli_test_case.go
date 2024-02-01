package babytest

import (
	"testing"

	"github.com/calvinmclean/babyapi"
)

// CommandLineTest allows simulating CLI usage by passing a list of arguments
type CommandLineTest[T babyapi.Resource] struct {
	Args         []string
	ArgsFunc     func(getResponse PreviousResponseGetter) []string
	Headers      []string
	RawQuery     string
	RawQueryFunc func(getResponse PreviousResponseGetter) string
}

var _ Test[*babyapi.AnyResource] = CommandLineTest[*babyapi.AnyResource]{}

func (tt CommandLineTest[T]) Run(t *testing.T, client *babyapi.Client[T], getResponse PreviousResponseGetter) (*Response[T], error) {
	args := tt.Args
	if tt.ArgsFunc != nil {
		args = tt.ArgsFunc(getResponse)
	}

	rawQuery := tt.RawQuery
	if tt.RawQueryFunc != nil {
		rawQuery = tt.RawQueryFunc(getResponse)
	}

	out, err := client.RunFromCLI(args, tt.Headers, rawQuery)

	switch v := out.(type) {
	case *babyapi.Response[T]:
		return &Response[T]{Response: v}, err
	case *babyapi.Response[*babyapi.ResourceList[T]]:
		return &Response[T]{GetAllResponse: v}, err
	case nil:
		return nil, err
	default:
		t.Errorf("unexpected type for CLI response: %T", v)
		return nil, err
	}
}
