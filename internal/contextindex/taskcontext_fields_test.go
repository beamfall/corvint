package contextindex

import (
	"testing"
)

func fieldFixture(t *testing.T) string {
	t.Helper()
	return impactRepositoryWithFiles(t, map[string]string{
		"go.mod": "module example.test/runner\n\ngo 1.27.0\n",
		"internal/testrunner/types.go": "package testrunner\n\n// Plan is one selection.\ntype Plan struct {\n" +
			"\tSessionID string\n\tExpectedTests []string\n\tSelect string\n}\n",
		"internal/testrunner/validate.go":        "package testrunner\n\nfunc Validate(plan Plan) bool { return len(plan.ExpectedTests) > 0 }\n",
		"internal/testrunner/dynamic/browser.go": "package dynamic\n\nfunc Browse() {}\n",
		"internal/selector/select.go":            "package selector\n\nfunc Select() {}\n",
	})
}
