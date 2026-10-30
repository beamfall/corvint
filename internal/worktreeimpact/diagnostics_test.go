package worktreeimpact

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/diagnostic"
)

// TestWorkingTreeImpactRefusalDiagnostics freezes the DRC-V0 refusal of each of the seven
// unsupported-working-tree-impact-* sites field by field (decision 0201 (b)); the cleaned-path
// suffix site is unreachable through Compile because the raw-operand check runs first, so it is
// driven through cleanPath.
func TestWorkingTreeImpactRefusalDiagnostics(t *testing.T) {
	const module, revision = "example.test/module", "0123abcd"
	captured := func(sources []string, dirty []string) *contextindex.Index {
		index := &contextindex.Index{Module: module, Revision: revision, Sources: map[string]contextindex.Source{}, DirtyPaths: dirty}
		for _, source := range sources {
			index.Sources[source] = contextindex.Source{}
		}
		return index
	}
	compile := func(index *contextindex.Index, paths ...string) error {
		_, err := Compile(context.Background(), index, paths, 10)
		return err
	}
	_, cleanErr := cleanPath("pkg/notes.txt")
	pathFixes := []string{"worktree-impact.remove-path", "git.add-intent-to-add", "impact.use-tracked-path-profile"}
	for _, test := range []struct {
		name string
		err  error
		code string
		want diagnostic.Refusal
	}{
		{"missing index", compile(nil, "pkg/a.go"), repositoryCode, diagnostic.Refusal{
			Subject: diagnostic.Subject{Kind: "repository-state", Value: "captured-revision-index"}, Terminal: "absent-evidence"}},
		{"unqualified module", compile(&contextindex.Index{Module: "module"}, "pkg/a.go"), repositoryCode, diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "repository-state", Value: "go-module-path"},
			Evidence:       []diagnostic.Evidence{{Name: "module", Value: "module"}},
			SupportedFixes: []string{"go-mod.qualify-module-path"}}},
		{"tracked path", compile(captured([]string{"pkg/a.go"}, nil), "pkg/a.go"), pathCode, diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "value", Value: "pkg/a.go"},
			Evidence:       []diagnostic.Evidence{{Name: "revision", Value: revision}},
			SupportedFixes: []string{"worktree-impact.remove-path", "impact.use-tracked-path-profile"}}},
		{"unobserved path", compile(captured(nil, nil), "pkg/b.go"), pathCode, diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "value", Value: "pkg/b.go"},
			Evidence:       []diagnostic.Evidence{{Name: "revision", Value: revision}},
			SupportedFixes: []string{"worktree-impact.remove-path", "worktree-impact.make-path-untracked"}}},
		{"raw suffix", compile(nil, "pkg/./notes.txt"), pathCode, diagnostic.Refusal{
			Subject: diagnostic.Subject{Kind: "value", Value: "pkg/./notes.txt"}, SupportedFixes: pathFixes}},
		{"cleaned suffix", cleanErr, pathCode, diagnostic.Refusal{
			Subject: diagnostic.Subject{Kind: "value", Value: "pkg/notes.txt"}, SupportedFixes: pathFixes}},
		{"root path", compile(captured(nil, []string{"main.go"}), "main.go"), pathCode, diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "value", Value: "main.go"},
			SupportedFixes: []string{"worktree-impact.remove-path", "worktree-impact.nest-path"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireCode(t, test.err, test.code)
			var carried *diagnostic.Error
			if !errors.As(test.err, &carried) {
				t.Fatalf("refusal carries no diagnostic: %v", test.err)
			}
			if !reflect.DeepEqual(carried.Refusal, test.want) {
				t.Fatalf("refusal = %+v, want %+v", carried.Refusal, test.want)
			}
			if err := carried.Refusal.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
