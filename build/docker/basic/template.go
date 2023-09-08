package basic

import (
	"bytes"
	_ "embed"
	"text/template"
)

var (
	//go:embed Dockerfile.tmpl
	tmpl       string
	dockerfile = template.Must(template.New("dockerfileBasic").Parse(tmpl))
)

// Data is the structure containing fields required by the template.
type Data struct {
	// From is the tag of the base image
	From string

	// Binary is the name of binary file to copy from build context
	Binary string

	// Run is an extra command to be run during image build. RUN directive will be used to execute it.
	Run string
}

// Execute executes dockerfile template and returns complete dockerfile.
func Execute(data Data) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := dockerfile.Execute(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
