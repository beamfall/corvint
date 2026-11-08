// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// GLS-V0-020/034, LPCV-V0-955/963: exclusion requires qualification of the
// actual runner and executable, not just a matching coverage index.
func TestLocalQualificationRequiresMatchingEvidence(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	for _, p := range QualifiedSources {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "sources")
	rev := git(t, root, "rev-parse", "HEAD")
	oids, err := OIDs(context.Background(), root, rev)
	if err != nil {
		t.Fatal(err)
	}
	q := Qualified{Profile: "linecov-qualified/1", Family: "go", Runner: "runner", Sources: oids, Report: QualifiedReport{Digest: "sha256:fixture", Mutants: 1, Killed: 1, Controls: 1}}
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"}}}
	for _, tc := range []struct {
		name   string
		mutate func(*Qualified)
		runner string
		info   *debug.BuildInfo
		want   bool
	}{
		{"match", func(*Qualified) {}, "runner", info, true},
		{"miss", func(q *Qualified) { q.Report.Misses = 1 }, "runner", info, false},
		{"no controls", func(q *Qualified) { q.Report.Controls = 0 }, "runner", info, false},
		{"no kills", func(q *Qualified) { q.Report.Killed = 0 }, "runner", info, false},
		{"runner mismatch", func(*Qualified) {}, "other", info, false},
		{"missing provenance", func(*Qualified) {}, "runner", nil, false},
		{"dirty executable", func(*Qualified) {}, "runner", &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "true"}}}, false},
		{"wrong blob", func(q *Qualified) { q.Sources = map[string]string{} }, "runner", info, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git(t, root, "checkout", "-q", "--detach", rev)
			record := q
			tc.mutate(&record)
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			full := filepath.Join(root, ".github/linecov/qualification.json")
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, data, 0644); err != nil {
				t.Fatal(err)
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "record")
			base := git(t, root, "rev-parse", "HEAD")
			err = CheckQualification(context.Background(), root, base, tc.runner, tc.info)
			if (err == nil) != tc.want {
				t.Fatalf("admitted=%v want %v: %v", err == nil, tc.want, err)
			}
			if tc.want {
				// Source drift in the running executable invalidates an older passing record.
				if err := os.WriteFile(filepath.Join(root, QualifiedSources[0]), []byte("changed\n"), 0644); err != nil {
					t.Fatal(err)
				}
				git(t, root, "commit", "-qam", "changed selector")
				changed := *info
				changed.Settings = append([]debug.BuildSetting(nil), info.Settings...)
				changed.Settings[0].Value = git(t, root, "rev-parse", "HEAD")
				if err := CheckQualification(context.Background(), root, base, "runner", &changed); err == nil {
					t.Fatal("changed executable qualified")
				}
			}
		})
	}
	if err := CheckQualification(context.Background(), root, rev, "runner", info); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing record admitted: %v", err)
	}
}
