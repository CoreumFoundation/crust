package github

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/pkg/errors"
)

// GenerateWorkflow generates file from template.
// It doesn't use `template` package because it conflicts with github syntax `${{ ... }}`.
func GenerateWorkflow(dstFile, tmpl string, input any) error {
	inputVal := reflect.ValueOf(input)
	regExp := regexp.MustCompile(`{{ *\.([a-zA-Z0-9]+?) *}}`)

	return errors.WithStack(os.WriteFile(dstFile, []byte(regExp.ReplaceAllStringFunc(tmpl, func(match string) string {
		return fmt.Sprintf("%s", inputVal.FieldByName(regExp.FindStringSubmatch(match)[1]).Interface())
	})), 0o600))
}
