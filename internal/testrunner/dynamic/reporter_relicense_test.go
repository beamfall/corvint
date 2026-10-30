package dynamic

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// reporterHeaderRelicense records that decision 0586 changed only the SPDX first line of an
// embedded reporter after its fixtures or qualification were produced. reporterSha256 keeps
// naming the producing reporter; currentReporterSha256 names today's header-only successor.
type reporterHeaderRelicense struct {
	CurrentReporterSHA256 string `json:"currentReporterSha256"`
	Decision              string `json:"decision"`
	PriorSPDXExpression   string `json:"priorSpdxExpression"`
}

const currentReporterHeader = "// SPDX-License-Identifier: Apache-2.0\n"

// checkReporterProduced fails unless the embedded reporter is the producing one, or is its
// recorded header-only successor whose bytes, with the prior SPDX line restored, are exactly the
// producing reporter.
func checkReporterProduced(t *testing.T, name string, embedded []byte, produced string, relicense *reporterHeaderRelicense) {
	t.Helper()
	current := tr.Digest(embedded)
	if current == produced {
		return
	}
	if relicense == nil || relicense.Decision != "0586" || relicense.CurrentReporterSHA256 != current || relicense.PriorSPDXExpression == "" {
		t.Fatalf("embedded %s reporter differs from the one that produced the evidence", name)
	}
	body, ok := bytes.CutPrefix(embedded, []byte(currentReporterHeader))
	if !ok {
		t.Fatalf("embedded %s reporter does not start with the decision 0586 header", name)
	}
	prior := append([]byte("// SPDX-License-Identifier: "+relicense.PriorSPDXExpression+"\n"), body...)
	if tr.Digest(prior) != produced {
		t.Fatalf("embedded %s reporter differs from the producing reporter beyond its SPDX header", name)
	}
}

func TestCypressReporterMatchesQualifiedReporter(t *testing.T) {
	raw, err := os.ReadFile("testdata/runtime-qualification.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Cypress struct {
			ReporterSHA256          string                   `json:"reporterSha256"`
			ReporterHeaderRelicense *reporterHeaderRelicense `json:"reporterHeaderRelicense"`
		} `json:"cypress"`
	}
	if err = json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	reporter, err := reporters.ReadFile("reporters/cypress.cjs")
	if err != nil {
		t.Fatal(err)
	}
	checkReporterProduced(t, "Cypress", reporter, q.Cypress.ReporterSHA256, q.Cypress.ReporterHeaderRelicense)
}
