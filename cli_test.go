package babyapi

import (
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringSliceFlag(t *testing.T) {
	var headers stringSliceFlag
	flag.Var(&headers, "H", "add headers to request")
	os.Args = []string{"command", "-H", "arg1", "-H", "arg2"}
	flag.Parse()

	require.ElementsMatch(t, headers, []string{"arg1", "arg2"})

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
}
