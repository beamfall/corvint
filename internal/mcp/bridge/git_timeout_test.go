package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

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
			_, callErr := registryWithOperations(registry, operations).Call(ctx, ToolImpact, []byte(`{"paths":["internal/widget/widget.go"]}`))
			if callErr == nil || callErr.Code != test.want {
				t.Fatalf("index refusal = %#v, want %s", callErr, test.want)
			}
		})
	}
}
