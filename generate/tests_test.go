package generate

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	placeholder = "placeholder to test overwriting the file\n"
)

func TestGenerateTest(t *testing.T) {
	t.Run("RunWithoutFileToCompare", func(t *testing.T) {
		var out bytes.Buffer

		err := generateTest(&out)
		require.NoError(t, err)

		expected, err := os.ReadFile("testdata/api_test.go.expected")
		require.NoError(t, err)

		require.Equal(t, string(expected), out.String())
	})

	t.Run("FileNotOverwritten", func(t *testing.T) {
		err := GenerateTest("testdata/api_test.go", false)
		require.Error(t, err)
		require.Equal(t, "file already exists", err.Error())

		unchanged, err := os.ReadFile("testdata/api_test.go")
		require.NoError(t, err)

		require.Equal(t, placeholder, string(unchanged))
	})

	t.Run("OverwriteFile", func(t *testing.T) {
		err := GenerateTest("testdata/api_test.go", true)
		require.NoError(t, err)

		defer func() {
			err = os.WriteFile("testdata/api_test.go", []byte(placeholder), 0o644)
			require.NoError(t, err)
		}()

		expected, err := os.ReadFile("testdata/api_test.go.expected")
		require.NoError(t, err)

		actual, err := os.ReadFile("testdata/api_test.go")
		require.NoError(t, err)

		require.Equal(t, string(expected), string(actual))
	})
}
