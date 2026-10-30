package provider

import (
	"context"
	"sync"
	"testing"
	"time"
)

// deadlineRecordingAuthority records, per lease operation, the time left on each call's context,
// so a test can see which bound the post-run calls ran under.
type deadlineRecordingAuthority struct {
	ExecutionAuthority
	mu        sync.Mutex
	remaining map[string][]time.Duration
}

func (authority *deadlineRecordingAuthority) Acquire(ctx context.Context, request AuthorityRequest) (ExecutionLease, error) {
	lease, err := authority.ExecutionAuthority.Acquire(ctx, request)
	if err != nil {
		return nil, err
	}
	return &deadlineRecordingLease{ExecutionLease: lease, receipt: lease.(ReceiptIdentityLease), authority: authority}, nil
}

func (authority *deadlineRecordingAuthority) record(ctx context.Context, operation string) {
	left := time.Duration(-1)
	if deadline, ok := ctx.Deadline(); ok {
		left = time.Until(deadline)
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.remaining[operation] = append(authority.remaining[operation], left)
}

type deadlineRecordingLease struct {
	ExecutionLease
	receipt   ReceiptIdentityLease
	authority *deadlineRecordingAuthority
}

func (lease *deadlineRecordingLease) Revalidate(ctx context.Context) error {
	lease.authority.record(ctx, "revalidate")
	return lease.ExecutionLease.Revalidate(ctx)
}

func (lease *deadlineRecordingLease) Release(ctx context.Context) error {
	lease.authority.record(ctx, "release")
	return lease.ExecutionLease.Release(ctx)
}

func (lease *deadlineRecordingLease) ObserveReceiptIdentities(ctx context.Context) (ReceiptIdentityObservation, error) {
	lease.authority.record(ctx, "observe")
	return lease.receipt.ObserveReceiptIdentities(ctx)
}

func (lease *deadlineRecordingLease) BindCanonical(ctx context.Context, request CanonicalBindingRequest) (CanonicalBinding, error) {
	return lease.receipt.BindCanonical(ctx, request)
}

// GLTP-V0-053: post-run revalidation, post-run identity observation and lease release run under
// the fixed 15-second bound unless the caller's context carries WithPostAuthorityBound, and each
// runs under its own deadline either way. The pre-run calls keep the 30-minute run budget.
func TestPostAuthorityBoundDefaultsToFifteenSecondsAndSeamReplacesIt(t *testing.T) {
	if postAuthorityTimeout != 15*time.Second || postAuthorityBound(context.Background()) != postAuthorityTimeout {
		t.Fatalf("production post-run bound = %s / %s, want 15s", postAuthorityTimeout, postAuthorityBound(context.Background()))
	}
	for _, bound := range []time.Duration{0, -time.Second} {
		if got := postAuthorityBound(WithPostAuthorityBound(context.Background(), bound)); got != postAuthorityTimeout {
			t.Fatalf("non-positive seam %s replaced the bound with %s", bound, got)
		}
	}
	const lifted = 7 * time.Minute
	for _, test := range []struct {
		name     string
		ctx      context.Context
		min, max time.Duration
	}{
		{"production", context.Background(), 0, postAuthorityTimeout},
		{"lifted", WithPostAuthorityBound(context.Background(), lifted), postAuthorityTimeout, lifted},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t, false)
			recorder := &deadlineRecordingAuthority{ExecutionAuthority: fixture.config.Authority, remaining: map[string][]time.Duration{}}
			fixture.config.Authority = recorder
			if _, err := Execute(test.ctx, fixture.config); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			for _, operation := range []string{"revalidate", "observe", "release"} {
				calls := recorder.remaining[operation]
				if len(calls) == 0 {
					t.Fatalf("%s was not called", operation)
				}
				// The last call of each operation is the post-run one; earlier calls run under the
				// 30-minute run budget, above either post-run bound.
				if last := calls[len(calls)-1]; last <= test.min || last > test.max {
					t.Fatalf("post-run %s ran with %s left, want (%s, %s]", operation, last, test.min, test.max)
				}
			}
		})
	}
}
