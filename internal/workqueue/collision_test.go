package workqueue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestIndexCollisionSourceResolvesGoModuleImports(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":                 "module example.com/project\n\ngo 1.27.0\n",
		"a/a.go":                 "package a\n",
		"b/b.go":                 "package b\n\nimport _ \"example.com/project/a\"\n",
		"external/external.go":   "package external\n\nimport _ \"example.net/dependency\"\n",
		"nested/pkg/value.go":    "package pkg\n",
		"tools/go.mod":           "module example.com/project/nested\n\ngo 1.27.0\n",
		"tools/pkg/value.go":     "package pkg\n",
		"tools/user/importer.go": "package user\n\nimport _ \"example.com/project/nested/pkg\"\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.test", "commit", "-qm", "fixture"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	source := IndexCollisionSource(index)
	for _, test := range []struct {
		name string
		path string
		want []string
	}{
		{name: "root module importer", path: "a/a.go", want: []string{"a/a.go", "b/b.go"}},
		{name: "external import", path: "external/external.go", want: []string{"external/external.go"}},
		{name: "nested module longest prefix", path: "tools/pkg/value.go", want: []string{"tools/pkg/value.go", "tools/user/importer.go"}},
		{name: "enclosing module shadow excluded", path: "nested/pkg/value.go", want: []string{"nested/pkg/value.go"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, complete := source.Closure([]string{test.path})
			if !complete || !equalStrings(got, test.want) {
				t.Fatalf("Closure(%q) = %v, %v; want %v, true; imports=%v", test.path, got, complete, test.want, index.Imports)
			}
		})
	}
}

func TestIndexCollisionSourceExpansion(t *testing.T) {
	index := &contextindex.Index{
		CommitRevision: "commit",
		Tracked: map[string]struct{}{
			"a.go": {}, "b.go": {}, "dir/x.go": {}, "dir/y.md": {}, "plain.bin": {},
		},
		Imports: map[string]map[string]struct{}{
			"a.go": {"b.go": {}},
		},
	}
	source := IndexCollisionSource(index)
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{"exact and imported", []string{"a.go"}, []string{"a.go", "b.go"}},
		{"importer reverse", []string{"b.go"}, []string{"a.go", "b.go"}},
		{name: "WQO-V0-041 prefix", paths: []string{"dir/"}, want: []string{"dir/", "dir/x.go", "dir/y.md"}},
		{"unindexed", []string{"plain.bin"}, []string{"plain.bin"}},
		{"untracked", []string{"missing.go"}, []string{"missing.go"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, complete := source.Closure(test.paths)
			if !complete || !equalStrings(got, test.want) {
				t.Fatalf("Closure() = %v, %v; want %v, true", got, complete, test.want)
			}
		})
	}
}

func TestIndexCollisionSourceUnparsedContributesOnlyItself(t *testing.T) {
	index := &contextindex.Index{
		CommitRevision: "commit",
		Tracked:        map[string]struct{}{"a.go": {}, "b.go": {}},
		Imports:        map[string]map[string]struct{}{"a.go": {"b.go": {}}},
		Unparsed:       []contextindex.Unparsed{{Path: "a.go", Facts: "imports"}},
	}
	got, complete := IndexCollisionSource(index).Closure([]string{"a.go"})
	if !complete || !equalStrings(got, []string{"a.go"}) {
		t.Fatalf("Closure() = %v, %v", got, complete)
	}
}

func TestIndexCollisionSourceDirectNeighboursOnly(t *testing.T) {
	t.Run("WQO-V0-042 direct neighbours are not a transitive independence proof", func(t *testing.T) {
		index := &contextindex.Index{
			CommitRevision: "commit",
			Tracked: map[string]struct{}{
				"a/a.go": {}, "b/b.go": {}, "c/c.go": {}, "d/d.go": {},
			},
			Imports: map[string]map[string]struct{}{
				"b/b.go": {"a/a.go": {}},
				"c/c.go": {"b/b.go": {}},
				"d/d.go": {"c/c.go": {}},
			},
		}
		source := IndexCollisionSource(index)
		for _, test := range []struct {
			path string
			want []string
		}{
			{"a/a.go", []string{"a/a.go", "b/b.go"}},
			{"d/d.go", []string{"c/c.go", "d/d.go"}},
		} {
			got, complete := source.Closure([]string{test.path})
			if !complete || !equalStrings(got, test.want) {
				t.Fatalf("Closure(%q) = %v, %v; want %v, true", test.path, got, complete, test.want)
			}
		}
		first, second := testTicket("one", 1), testTicket("two", 2)
		first.TouchPaths, second.TouchPaths = []string{"a/a.go"}, []string{"d/d.go"}
		snapshot := testSnapshot(first, second)
		snapshot.RepositorySource.Commit = "commit"
		closure := DeriveCollisions(snapshot, source)
		if !closure.Complete || len(closure.Groups) != 0 {
			t.Fatalf("direct closure = %#v; want complete with no groups", closure)
		}
	})
}

func TestDeriveCollisionsGroupsReadyTicketsAndPreservesAdapterFacts(t *testing.T) {
	t.Run("WQO-V0-043", func(t *testing.T) {
		first := testTicket("one", 1)
		second := testTicket("two", 2)
		notReady := testTicket("three", 3)
		first.TouchPaths = []string{"a.go"}
		second.TouchPaths = []string{"a.go"}
		notReady.TouchPaths = []string{"a.go"}
		notReady.Lifecycle = "BLOCKED"
		adapterID := "collision:corvint:worklist:adapter"
		first.CollisionGroupIDs = []string{adapterID}
		second.CollisionGroupIDs = []string{adapterID}
		snapshot := testSnapshot(first, second, notReady)
		snapshot.RepositorySource.Commit = "commit"
		snapshot.Leases = []LeaseSummary{{CollisionGroupIDs: []string{adapterID}}}
		index := &contextindex.Index{
			CommitRevision: "commit",
			Tracked:        map[string]struct{}{"a.go": {}},
			Imports:        map[string]map[string]struct{}{},
		}
		result := DeriveCollisions(snapshot, IndexCollisionSource(index))
		if !result.Complete || len(result.Groups) != 2 {
			t.Fatalf("result = %#v", result)
		}
		digest := sha256.Sum256([]byte("a.go"))
		wantDerived := "collision:corvint:worklist:corvint-" + hex.EncodeToString(digest[:])[:32]
		if result.Groups[0].ID != wantDerived && result.Groups[1].ID != wantDerived {
			t.Fatalf("derived ID missing from %v", result.Groups)
		}
		if len(result.TicketGroupIDs[notReady.TicketID]) != 0 {
			t.Fatal("non-READY ticket gained a derived group")
		}
		if !equalStrings(snapshot.Leases[0].CollisionGroupIDs, []string{adapterID}) {
			t.Fatal("lease collision groups were mutated")
		}
	})
}

func TestDeriveCollisionsRequiresSharedReadyPath(t *testing.T) {
	first := testTicket("one", 1)
	second := testTicket("two", 2)
	first.TouchPaths = []string{"a.go"}
	second.TouchPaths = []string{"b.go"}
	snapshot := testSnapshot(first, second)
	snapshot.RepositorySource.Commit = "commit"
	index := &contextindex.Index{
		CommitRevision: "commit",
		Tracked:        map[string]struct{}{"a.go": {}, "b.go": {}},
		Imports:        map[string]map[string]struct{}{},
	}
	result := DeriveCollisions(snapshot, IndexCollisionSource(index))
	if len(result.Groups) != 0 {
		t.Fatalf("groups = %v, want none", result.Groups)
	}
}

func TestDeriveCollisionsIncomplete(t *testing.T) {
	snapshot := testSnapshot(testTicket("one", 1))
	snapshot.RepositorySource.Commit = "wanted"
	index := &contextindex.Index{CommitRevision: "other"}
	result := DeriveCollisions(snapshot, IndexCollisionSource(index))
	if result.Complete || result.State != StateUnknown || !equalStrings(result.Unknowns, []string{UnknownCollisionClosureIncomplete}) {
		t.Fatalf("commit mismatch result = %#v", result)
	}

	paths := make([]string, maxCollisionPaths+1)
	for index := range paths {
		paths[index] = fmt.Sprintf("p/%04d", index)
	}
	result = DeriveCollisions(snapshot, staticCollisionSource{paths: paths})
	if result.Complete || len(result.Groups) != 0 {
		t.Fatalf("overflow result = %#v", result)
	}
}

type staticCollisionSource struct {
	paths []string
}

func (source staticCollisionSource) Closure([]string) ([]string, bool) {
	return append([]string(nil), source.paths...), true
}
