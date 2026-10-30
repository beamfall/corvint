package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestReadOCMBindsOCMRawToVerifiedBytes guards against internal/dogfoodocm
// reopening a map by path a second time after ReadOCM already verified it: a
// caller that needs the exact bytes ReadOCM verified (rather than a fresh,
// racy re-read of the same path, which an attacker could swap between reads)
// must be able to read them off the result.
func TestReadOCMBindsOCMRawToVerifiedBytes(t *testing.T) {
	root, options, _, claims := makeOCMAnchorFixture(t)
	for _, claim := range claims {
		if strings.Contains(claim.selector, "/case:") {
			options.Claims = []string{claim.selector}
		}
	}
	if _, err := LinkOCM(context.Background(), root, options); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, options.MapPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked.State != "ready-for-review" {
		t.Fatalf("fixture state = %q, verification = %v", checked.State, checked.Verification)
	}
	if !bytes.Equal(checked.OCMRaw, raw) {
		t.Fatalf("OCMRaw = %q, want the exact verified map bytes %q", checked.OCMRaw, raw)
	}
}

// TestOCMBindingNamesGitBudgetExpiryNotMissingObject pins OCM-V0-012: when the
// Git verification budget elapses or is exhausted, or the caller cancels, while
// resolving the OCM target, the refusal names that bound, so load is not reported as a missing
// object; a target that is genuinely absent still reports
// repository-object-unavailable.
func TestOCMBindingNamesGitBudgetExpiryNotMissingObject(t *testing.T) {
	root, _, target := makeOCMPrepareRepository(t)
	cemRaw := []byte("{}")
	missing := strings.Repeat("0", len(target))
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name, target, want string
		budget             *gitrun.Budget
		ctx                context.Context
	}{
		{"elapsed", target, "git-timeout", gitrun.NewBudget(gitrun.DefaultOperations, -time.Second), context.Background()},
		{"exhausted", target, "git-budget-exceeded", gitrun.NewBudget(0, time.Minute), context.Background()},
		{"cancelled", target, "git-cancelled", gitrun.NewDefaultBudget(), cancelled},
		{"missing", missing, "repository-object-unavailable", gitrun.NewDefaultBudget(), context.Background()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repository, err := gitauth.Open(root, tc.budget)
			if err != nil {
				t.Fatal(err)
			}
			document := &ocmDocument{target: tc.target, cemMap: sha256Hex(cemRaw)}
			err = verifyOCMBinding(tc.ctx, repository, &wire.Map{}, cemRaw, nil, document, Options{})
			if got := CodeOf(err); got != tc.want {
				t.Fatalf("code = %q (%v), want %q", got, err, tc.want)
			}
		})
	}
}
