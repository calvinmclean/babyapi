package generate

import (
	"errors"
	"io"
	"os"
	"text/template"

	_ "embed"
)

//go:embed api_test.go.tmpl
var apiTestTmpl []byte

func GenerateTest(fname string, force bool) error {
	// Exit early if file exists and !force
	if !force {
		_, err := os.Stat(fname)
		if err == nil || os.IsExist(err) {
			return errors.New("file already exists")
		}
	}

	flag := os.O_RDWR | os.O_CREATE | os.O_TRUNC
	f, err := os.OpenFile(fname, flag, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return err
		}
	}

	defer func() {
		_ = f.Close()
	}()

	return generateTest(f)
}

func generateTest(wr io.Writer) error {
	tmpl, err := template.New("APITest").Parse(string(apiTestTmpl))
	if err != nil {
		return err
	}

	err = tmpl.Execute(wr, map[string]any{})
	if err != nil {
		return err
	}

	return nil
}
