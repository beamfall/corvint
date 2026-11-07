// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// CAP-V0-9300 / AFP-V0-023: discovery integration must keep the live
// format scanner selected for every production source under internal/tasks.
func TestTasksCLIRecursiveReadSelection(t *testing.T) {
	var declared struct {
		Packages map[string][]string `json:"packages"`
	}
	raw, err := os.ReadFile("../../.corvint/test-read-scopes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		t.Fatal(err)
	}
	const unit = "internal/tasks/cli"
	scope, ok := declared.Packages[unit]
	if !ok {
		t.Fatal("missing Tasks CLI read scope")
	}
	// Use the selector's production predicate, including nested paths in the
	// two omitted directories, even if future source changes empty either one.
	for _, path := range []string{"internal/tasks/scale/nested/profile.go", "internal/tasks/testhooks/nested/profile.go"} {
		if !affected.InReadScope(unit, scope, path) {
			t.Errorf("%s does not select %s", path, unit)
		}
	}
	err = filepath.WalkDir("../../internal/tasks", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
			rel, err := filepath.Rel("../..", path)
			if err != nil {
				return err
			}
			if !affected.InReadScope(unit, scope, filepath.ToSlash(rel)) {
				t.Errorf("%s read by live format scanner does not select %s", rel, unit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var floor struct {
		Units []string `json:"units"`
	}
	raw, err = os.ReadFile("../../.corvint/unbounded-readers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &floor); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pkg := range floor.Units {
		found = found || pkg == unit
	}
	if !found {
		t.Fatal("Tasks CLI must retain its unbounded-reader floor")
	}
}
