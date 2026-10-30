package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// stallingAuthority wraps a fixture authority so chosen post-run lease calls behave like a
// parent verifier that is slow, drifted or failing. Only calls whose deadline is under a minute
// away are post-run ones; the pre-run calls keep the 30-minute run budget and pass through.
type stallingAuthority struct {
	ExecutionAuthority
	mode map[string]string
}

func (authority *stallingAuthority) Acquire(ctx context.Context, request AuthorityRequest) (ExecutionLease, error) {
	lease, err := authority.ExecutionAuthority.Acquire(ctx, request)
	if err != nil {
		return nil, err
	}
	return &stallingLease{ExecutionLease: lease, receipt: lease.(ReceiptIdentityLease), mode: authority.mode}, nil
}

type stallingLease struct {
	ExecutionLease
	receipt ReceiptIdentityLease
	mode    map[string]string
}

// behave returns the injected result for operation, or ok=false to pass the call through.
// "stall" waits out the call's deadline and fails the way a killed verifier transport does;
// "stall-drift" waits it out and then reports drift; "fail" fails at once with an untyped error.
func (lease *stallingLease) behave(ctx context.Context, operation string) (error, bool) {
	deadline, bounded := ctx.Deadline()
	if !bounded || time.Until(deadline) > time.Minute {
		return nil, false
	}
	switch lease.mode[operation] {
	case "stall":
		<-ctx.Done()
		return fmt.Errorf("%w: parent verifier transport failed", ErrAuthorityUnavailable), true
	case "stall-drift":
		<-ctx.Done()
		return fmt.Errorf("%w: parent verifier reported drift", ErrAuthorityDrift), true
	case "fail":
		return errors.New("post-run failure"), true
	}
	return nil, false
}

func (lease *stallingLease) Revalidate(ctx context.Context) error {
	if err, injected := lease.behave(ctx, "revalidate"); injected {
		return err
	}
	return lease.ExecutionLease.Revalidate(ctx)
}

func (lease *stallingLease) ObserveReceiptIdentities(ctx context.Context) (ReceiptIdentityObservation, error) {
	if err, injected := lease.behave(ctx, "observe"); injected {
		return ReceiptIdentityObservation{}, err
	}
	return lease.receipt.ObserveReceiptIdentities(ctx)
}

func (lease *stallingLease) Release(ctx context.Context) error {
	if err, injected := lease.behave(ctx, "release"); injected {
		_ = lease.ExecutionLease.Release(context.Background())
		return err
	}
	return lease.ExecutionLease.Release(ctx)
}

func (lease *stallingLease) BindCanonical(ctx context.Context, request CanonicalBindingRequest) (CanonicalBinding, error) {
	return lease.receipt.BindCanonical(ctx, request)
}

// GLTP-V0-054 (V1-1428): post-run authority calls that only ran out of their own deadline are
// refused as ErrAuthorityUnavailable, never as ErrAuthorityDrift; any drift report or in-time
// failure among them keeps the refusal ErrAuthorityDrift. Either way the run is INCOMPLETE with an
// empty run root; its receipt is the infrastructure one unless the post-run identities went
// unobserved, which the receipt contract records as UNKNOWN currency and STALE.
func TestPostRunDeadlineIsUnavailableNotDrift(t *testing.T) {
	for _, test := range []struct {
		name  string
		bound time.Duration
		mode  map[string]string
		drift bool
	}{
		{"every post-run call overruns", time.Nanosecond, map[string]string{"revalidate": "stall", "observe": "stall", "release": "stall"}, false},
		{"release alone overruns", 20 * time.Millisecond, map[string]string{"release": "stall"}, false},
		{"drift reported after the deadline", time.Nanosecond, map[string]string{"revalidate": "stall-drift", "observe": "stall", "release": "stall"}, true},
		{"in-time failure joins an overrun", time.Second, map[string]string{"revalidate": "stall", "release": "fail"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t, false)
			fixture.config.Authority = &stallingAuthority{ExecutionAuthority: fixture.config.Authority, mode: test.mode}
			transcript, err := Execute(WithPostAuthorityBound(context.Background(), test.bound), fixture.config)
			if test.drift {
				if !errors.Is(err, ErrAuthorityDrift) || errors.Is(err, ErrAuthorityUnavailable) {
					t.Fatalf("refusal = %v, want ErrAuthorityDrift alone", err)
				}
			} else if !errors.Is(err, ErrAuthorityUnavailable) || errors.Is(err, ErrAuthorityDrift) ||
				!strings.HasSuffix(err.Error(), ": post-run authority deadline expired") {
				t.Fatalf("refusal = %v, want ErrAuthorityUnavailable post-run deadline", err)
			}
			if !transcript.Runner.Started || transcript.Execution != ExecutionIncomplete {
				t.Fatalf("transcript = %+v", transcript)
			}
			if test.mode["observe"] == "" {
				assertInfrastructureReceipt(t, transcript)
			} else {
				for _, want := range []string{`"status":"INCOMPLETE"`, `"failureClass":"STALE"`, `"sourceCurrency":"UNKNOWN"`, `"toolchainCurrency":"UNKNOWN"`} {
					if !bytes.Contains(transcript.Receipt.Run, []byte(want)) {
						t.Fatalf("missing %s in %s", want, transcript.Receipt.Run)
					}
				}
			}
			assertDirectoryEmpty(t, fixture.temporaryParent)
		})
	}
}

// GLTP-V0-054 (V1-1432 site 3, decision 0533): the deferred release on an early exit before the
// post-run sequence is refused as ErrAuthorityUnavailable when its only failure is its own expired
// deadline; a drift report or an in-time failure keeps ErrAuthorityDrift. The early exit is an
// invalid source identity in the acquired binding, so the runner never starts.
func TestEarlyExitReleaseDeadlineIsUnavailableNotDrift(t *testing.T) {
	for _, test := range []struct {
		mode  string
		drift bool
	}{
		{"stall", false},
		{"stall-drift", true},
		{"fail", true},
	} {
		t.Run(test.mode, func(t *testing.T) {
			fixture := newFixture(t, false)
			fixture.authority.binding.SourceIdentity = "not-a-source-identity"
			fixture.config.Authority = &stallingAuthority{ExecutionAuthority: fixture.config.Authority, mode: map[string]string{"release": test.mode}}
			transcript, err := Execute(WithPostAuthorityBound(context.Background(), 20*time.Millisecond), fixture.config)
			if transcript.Runner.Started || transcript.AuthorityError == nil || transcript.Execution != ExecutionIncomplete {
				t.Fatalf("early exit transcript = %+v (err %v)", transcript, err)
			}
			if test.drift {
				if !errors.Is(transcript.AuthorityError, ErrAuthorityDrift) || !errors.Is(err, ErrAuthorityDrift) {
					t.Fatalf("release refusal = %v, want ErrAuthorityDrift", transcript.AuthorityError)
				}
			} else if !errors.Is(transcript.AuthorityError, ErrAuthorityUnavailable) || errors.Is(err, ErrAuthorityDrift) ||
				!strings.HasSuffix(transcript.AuthorityError.Error(), ": release execution authority deadline expired") {
				t.Fatalf("release refusal = %v (joined %v), want ErrAuthorityUnavailable release deadline", transcript.AuthorityError, err)
			}
			assertDirectoryEmpty(t, fixture.temporaryParent)
		})
	}
}

// preLaunchDriftAuthority reports drift from the pre-launch revalidation and otherwise behaves as
// the stalling lease it wraps.
type preLaunchDriftAuthority struct{ *stallingAuthority }

type preLaunchDriftLease struct{ *stallingLease }

func (authority preLaunchDriftAuthority) Acquire(ctx context.Context, request AuthorityRequest) (ExecutionLease, error) {
	lease, err := authority.stallingAuthority.Acquire(ctx, request)
	if err != nil {
		return nil, err
	}
	return preLaunchDriftLease{lease.(*stallingLease)}, nil
}

func (preLaunchDriftLease) Revalidate(context.Context) error {
	return fmt.Errorf("%w: parent verifier reported drift", ErrAuthorityDrift)
}

// GLTP-V0-054 (decision 0533): an early exit that already observed drift keeps drift when its
// release then only overruns its deadline, so the joined error carries no ErrAuthorityUnavailable
// and the pre-launch diagnostic stays PARENT_AUTHORITY_DRIFT.
func TestEarlyExitAfterDriftKeepsDriftWhenReleaseOverruns(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.config.Authority = preLaunchDriftAuthority{&stallingAuthority{ExecutionAuthority: fixture.config.Authority, mode: map[string]string{"release": "stall"}}}
	transcript, err := Execute(WithPostAuthorityBound(context.Background(), 20*time.Millisecond), fixture.config)
	if transcript.Runner.Started || transcript.Execution != ExecutionIncomplete {
		t.Fatalf("early exit transcript = %+v (err %v)", transcript, err)
	}
	if !errors.Is(err, ErrAuthorityDrift) || errors.Is(err, ErrAuthorityUnavailable) ||
		!errors.Is(transcript.AuthorityError, ErrAuthorityDrift) {
		t.Fatalf("refusal = %v (release %v), want drift only", err, transcript.AuthorityError)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}
