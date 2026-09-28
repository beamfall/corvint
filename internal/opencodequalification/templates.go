package opencodequalification

import (
	_ "embed"
	"strings"
)

//go:embed probe.js.tmpl
var probeTemplate string

//go:embed observer.js.tmpl
var observerTemplate string

//go:embed inspector.tsx.tmpl
var inspectorTemplate string

func template(source string, values map[string]string) string {
	for k, v := range values {
		b, _ := jsonBytes(v)
		source = strings.ReplaceAll(source, k, string(b))
	}
	return source
}
