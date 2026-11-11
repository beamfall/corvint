// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// owningSpecProblems lists each catalog command whose owningSpec names no live spec of the
// spec index (CAP-V0-9940): a code no index entry carries as its reqPrefix is missing, and a code
// whose every entry is retired (intent beginning "retired") or superseded is retired. It also
// returns how many commands named an owning spec, so a caller can tell an empty scan from a clean one.
func owningSpecProblems(catalog, index []byte) ([]string, int, error) {
	var document struct {
		Commands []struct {
			Name       string  `json:"name"`
			OwningSpec *string `json:"owningSpec"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(catalog, &document); err != nil {
		return nil, 0, fmt.Errorf("catalog: %w", err)
	}
	var specs []struct {
		Path         string  `json:"path"`
		ReqPrefix    string  `json:"reqPrefix"`
		Intent       string  `json:"intent"`
		SupersededBy *string `json:"supersededBy"`
	}
	if err := json.Unmarshal(index, &specs); err != nil {
		return nil, 0, fmt.Errorf("spec index: %w", err)
	}
	carried, live := map[string][]string{}, map[string]bool{}
	for _, spec := range specs {
		if spec.ReqPrefix == "" {
			continue
		}
		carried[spec.ReqPrefix] = append(carried[spec.ReqPrefix], spec.Path)
		if !strings.HasPrefix(spec.Intent, "retired") && spec.SupersededBy == nil {
			live[spec.ReqPrefix] = true
		}
	}
	var problems []string
	named := 0
	for _, command := range document.Commands {
		if command.OwningSpec == nil {
			continue
		}
		named++
		code := *command.OwningSpec
		switch {
		case len(carried[code]) == 0:
			problems = append(problems, fmt.Sprintf("%s: owningSpec %q is missing: no docs/specs/INDEX.json entry carries it as its reqPrefix", command.Name, code))
		case !live[code]:
			problems = append(problems, fmt.Sprintf("%s: owningSpec %q is retired: every spec carrying it (%s) is retired or superseded", command.Name, code, strings.Join(carried[code], ", ")))
		}
	}
	return problems, named, nil
}

// CAP-V0-9940 (V1-5294): every owningSpec in the shipped catalog names a live spec of the spec
// index. The fix for a failure is the parenthesized code ending the command's root help line,
// then make capability-catalog.
func TestCAPV09940OwningSpecIsLive(t *testing.T) {
	index, err := os.ReadFile(filepath.Join("..", "..", "docs", "specs", "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	problems, named, err := owningSpecProblems(snapshot, index)
	if err != nil {
		t.Fatal(err)
	}
	if named < 2 {
		t.Fatalf("the catalog names %d owning specs; the scan no longer sees the experimental commands", named)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// CAP-V0-9940: the check refuses a missing code and a code whose only specs are retired or
// superseded, and passes a code with one live spec beside a retired sibling of the same prefix.
func TestCAPV09940RetiredAndMissingCodesRefused(t *testing.T) {
	catalog := []byte(`{"commands": [
		{"name": "alive", "owningSpec": "LIV-V0"},
		{"name": "split", "owningSpec": "SPL-V0"},
		{"name": "gone", "owningSpec": "RET-V0"},
		{"name": "replaced", "owningSpec": "SUP-V0"},
		{"name": "invented", "owningSpec": "NOP-V0"},
		{"name": "core", "owningSpec": null}
	]}`)
	index := []byte(`[
		{"path": "docs/specs/live-v0.md", "reqPrefix": "LIV-V0", "intent": "proposed", "supersededBy": null},
		{"path": "docs/specs/split-a-v0.md", "reqPrefix": "SPL-V0", "intent": "retired 2026-11-09 (decision 0594)", "supersededBy": null},
		{"path": "docs/specs/split-b-v0.md", "reqPrefix": "SPL-V0", "intent": "accepted", "supersededBy": null},
		{"path": "docs/specs/retired-v0.md", "reqPrefix": "RET-V0", "intent": "retired 2026-11-09 (decision 0593)", "supersededBy": null},
		{"path": "docs/specs/superseded-v0.md", "reqPrefix": "SUP-V0", "intent": "accepted", "supersededBy": "docs/specs/live-v0.md"},
		{"path": "docs/specs/unnumbered.md", "reqPrefix": "", "intent": "accepted", "supersededBy": null}
	]`)
	problems, named, err := owningSpecProblems(catalog, index)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`gone: owningSpec "RET-V0" is retired: every spec carrying it (docs/specs/retired-v0.md) is retired or superseded`,
		`replaced: owningSpec "SUP-V0" is retired: every spec carrying it (docs/specs/superseded-v0.md) is retired or superseded`,
		`invented: owningSpec "NOP-V0" is missing: no docs/specs/INDEX.json entry carries it as its reqPrefix`,
	}
	if named != 5 || !slices.Equal(problems, want) {
		t.Fatalf("named %d, problems:\n%s\nwant 5 and:\n%s", named, strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
	if _, _, err := owningSpecProblems(catalog, []byte(`{}`)); err == nil {
		t.Fatal("an index that is not a list must refuse")
	}
}
