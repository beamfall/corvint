package bridge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// MCPV0-025, MCPV0-038 (V1-1432 site 2, decision 0533): a CEM Git read that ran out of its own
// bound while the request context was live is `repository-probe-timeout`, not a missing
// repository. Caller cancellation stays `cancelled` and every other Git failure keeps
// `repository-unavailable`. The CEM read is injected through the cemReport operation seam.
func TestMCPV0038CEMGitTimeoutHasDistinctCode(t *testing.T) {
	registry, err := NewTaskReview(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	commit, revision := strings.Repeat("a", 40), strings.Repeat("b", 40)
	arguments := []byte(`{"map":"m.cem.json","expectedBase":"` + commit + `","target":"` + commit + `"}`)
	for _, test := range []struct {
		name   string
		err    error
		cancel bool
		want   string
	}{
		{"git timeout", cemcode.New(cemcode.GitTimeout, "git read timed out"), false, "repository-probe-timeout"},
		{"git diff timeout", cemcode.New(cemcode.GitDiffTimeout, "git diff timed out"), false, "repository-probe-timeout"},
		{"git read failure", cemcode.New(cemcode.GitReadFailed, "git read failed"), false, "repository-unavailable"},
		{"caller cancelled during git timeout", cemcode.New(cemcode.GitTimeout, "git read timed out"), true, "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			operations := registry.operations
			operations.probe = func(context.Context, string) (gokernel.Repository, error) {
				return gokernel.Repository{
					CommitRevision: commit, TreeRevision: revision, ObjectFormat: "sha1", ProfileID: "generic",
					WorktreeState: "clean", DirtyPathsSHA: strings.Repeat("c", 64),
				}, nil
			}
			operations.cemReport = func(context.Context, string, string, workflow.ReadOptions) (map[string]any, error) {
				if test.cancel {
					cancel()
				}
				return nil, test.err
			}
			_, callErr := registryWithOperations(registry, operations).Call(ctx, ToolCEMReport, arguments)
			if callErr == nil || callErr.Code != test.want {
				t.Fatalf("CEM refusal = %#v, want %s", callErr, test.want)
			}
		})
	}
}

// MCPV0-038 (V1-1432 site 2b, decision 0533): the context index's own Git deadline refusal,
// which carries no code and a "Git " message, is `repository-probe-timeout` while the caller's
// context is live, and `cancelled` once the caller ended the request.
func TestMCPV0038IndexDeadlineHasDistinctCode(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	deadline := &contextindex.Error{Message: "Git repository index exceeded its deadline", Cause: context.DeadlineExceeded}
	for _, test := range []struct {
		name string
		err  error
		end  func(context.Context, context.CancelFunc)
		want string
	}{
		{"index deadline", deadline, nil, "repository-probe-timeout"},
		{"plain git failure", &contextindex.Error{Message: "Git repository is unavailable", Cause: errors.New("exit 128")}, nil, "repository-unavailable"},
		{"caller cancelled", deadline, func(_ context.Context, cancel context.CancelFunc) { cancel() }, "cancelled"},
		{"caller deadline", deadline, func(ctx context.Context, _ context.CancelFunc) { <-ctx.Done() }, "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if test.name == "caller deadline" {
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			}
			defer cancel()
			operations := registry.operations
			operations.platform = "linux"
			build := func(context.Context, string) (*contextindex.Index, error) {
				if test.end != nil {
					test.end(ctx, cancel)
				}
				return nil, test.err
			}
			operations.build = build
			operations.buildQuery = func(ctx context.Context, root, _ string) (*contextindex.Index, error) { return build(ctx, root) }
			_, callErr := registryWithOperations(registry, operations).Call(ctx, ToolQuery, []byte(`{"task":"orient contributor roadmap ticket workflow"}`))
			if callErr == nil || callErr.Code != test.want {
				t.Fatalf("index refusal = %#v, want %s", callErr, test.want)
			}
		})
	}
}
