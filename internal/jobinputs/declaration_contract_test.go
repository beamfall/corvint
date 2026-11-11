// SPDX-License-Identifier: Apache-2.0
package jobinputs

import (
	"reflect"
	"strings"
	"testing"
)

// TestCMPV1003CIJobInputsFileContract (CMP-V1-003 item 3, CMP-V1-004; V1-5434): the documented
// file .corvint/ci-job-inputs.json (corvint-ci-job-inputs/0) keeps every member the 1.x
// compatibility promise freezes. A declaration naming each member parses unchanged, so a renamed
// or removed member is refused as unknown; an unknown member or another profile is refused.
func TestCMPV1003CIJobInputsFileContract(t *testing.T) {
	const declaration = `{"profile":"corvint-ci-job-inputs/0","jobs":[{` +
		`"workflow":".github/workflows/ci.yml","job":"test",` +
		`"paths":["go.mod","internal/"],` +
		`"goPackages":{"packages":["./cmd/..."],"platforms":["linux/amd64"]},` +
		`"goTests":["./internal/x"],` +
		`"exportedTree":true}]}`
	got, err := Parse([]byte(declaration))
	if err != nil {
		t.Fatal(err)
	}
	want := Declaration{Profile: DeclarationProfile, Jobs: []Job{{
		Workflow:     ".github/workflows/ci.yml",
		Job:          "test",
		Paths:        []string{"go.mod", "internal/"},
		GoPackages:   &GoPackages{Packages: []string{"./cmd/..."}, Platforms: []string{"linux/amd64"}},
		GoTests:      []string{"./internal/x"},
		ExportedTree: true,
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed %+v, want %+v", got, want)
	}
	for label, body := range map[string]string{
		"unknown member":     strings.Replace(declaration, `"jobs":`, `"added":1,"jobs":`, 1),
		"unknown job member": strings.Replace(declaration, `"job":"test",`, `"job":"test","added":1,`, 1),
		"other profile":      strings.Replace(declaration, DeclarationProfile, "corvint-ci-job-inputs/1", 1),
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: accepted", label)
		}
	}
}
