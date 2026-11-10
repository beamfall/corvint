// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// auditTest is the test the ownership rows of .corvint/test-selection.json
// name when nothing else exercises the owned paths (AZU-V0-002).
const auditTest = "internal/liveverify/affected/ownership_completeness_test.go"

// testFileSuffixes are the test file conventions of the languages this
// repository's plugins model, so the check needs no graph build.
var testFileSuffixes = []string{"_test.go", "_test.sh", "_test.py", "_spec.rb", ".test.ts", ".test.mts", ".test.js", ".test.mjs", ".test.cjs", ".spec.ts", ".spec.js", ".spec.mjs", ".spec.cjs"}

// AFP-V0-10329 (V1-5199): an owner row of this repository's declaration that
// names only the ownership audit test owns paths no repository test
// exercises: no tracked test file outside a fixture tree matches its
// pattern. A row that covers such a test file names the tests that exercise
// the owned paths instead: the planner selects them either way
// (TestOwnerSelectsExercisingTests_AFPV010329), and this check keeps the
// declaration honest about which tests those are.
func TestRepositoryOwnerRowsNameExercisingTests_AFPV010329(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.DeclaredSelectionPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Owners []affected.TestOwner `json:"owners"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00")
	// A nested Go module's tests are no unit of the graph (AFP-V0-2742): CI
	// runs the module's own suite outside selection, so an owner row cannot
	// name them and the audit test keeps the module's paths claimed.
	var nestedModules []string
	for _, path := range tracked {
		if path != "go.mod" && strings.HasSuffix(path, "/go.mod") {
			nestedModules = append(nestedModules, strings.TrimSuffix(path, "go.mod"))
		}
	}
	auditOnly := 0
	for _, owner := range declared.Owners {
		if !slices.Equal(owner.Tests, []string{auditTest}) {
			for _, test := range owner.Tests {
				if test == auditTest {
					t.Errorf("owner %q names the audit test beside exercising tests; name only the exercising tests", owner.Paths)
				}
			}
			continue
		}
		auditOnly++
		for _, path := range tracked {
			nested := slices.ContainsFunc(nestedModules, func(module string) bool { return strings.HasPrefix(path, module) })
			if affected.OwnerPatternMatches(owner.Paths, path) && repositoryTestFile(path) && !nested {
				t.Errorf("owner %q names only the ownership audit test, but %s is a test file the pattern matches: name the tests that exercise the owned paths (AFP-V0-10329)", owner.Paths, path)
			}
		}
	}
	if auditOnly == 0 {
		t.Fatal("no audit-only owner row: the check exercised nothing")
	}
}

// repositoryTestFile reports a tracked path with a test file suffix outside
// every testdata, fixture and fixtures tree.
func repositoryTestFile(path string) bool {
	directory := strings.Split(filepath.ToSlash(filepath.Dir(path)), "/")
	if slices.ContainsFunc(directory, func(segment string) bool {
		return segment == "testdata" || segment == "fixture" || segment == "fixtures"
	}) {
		return false
	}
	return slices.ContainsFunc(testFileSuffixes, func(suffix string) bool { return strings.HasSuffix(path, suffix) })
}
