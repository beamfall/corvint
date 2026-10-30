package testvaliditydoc

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// LPCV-V0-094: discovery binds the retained spec and helper blobs to the
// worktree: CURRENT only when every blob matches and the run's start and end
// identities agree; a changed blob is STALE with stale-evidence guidance.
//
// The file is unconstrained (V1-1841): the fixture is portable, but retained
// receipts are read only on darwin and linux (read_unix.go). Elsewhere
// read_other.go refuses the evidence location, so discovery must fail closed
// and never project the receipt as CURRENT.
func TestLPCVV0094DiscoverRubyFreshness(t *testing.T) {
	root := discoveryRoot(t)
	writeAt(t, filepath.Join(root, "spec", "user_spec.rb"), rubySpecBody, time.Unix(1, 0))
	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody, time.Unix(1, 0))
	document := rubyDocument(t, "passed")
	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-1.json"), string(encodeRuby(t, document)), time.Unix(10, 0))
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		var refusal *DiscoveryError
		if _, err := Discover(root); !errors.As(err, &refusal) || refusal.Code != "invalid-test-evidence-location" {
			t.Fatalf("unsupported receipt platform must refuse discovery, got %v", err)
		}
		return
	}
	current := discover(t, root)
	if current.Source != "corvint-ruby-test-provider" || current.Run.Freshness.State != testvalidity.FreshnessCurrent || current.Tests[0].Projection.Freshness.State != testvalidity.FreshnessCurrent || current.Discovery.Abstention != nil {
		t.Fatalf("current=%+v abstention=%+v", current.Run.Freshness, current.Discovery.Abstention)
	}

	changed := document
	after := *document.RepositoryAfter
	after.DirtyState, after.DirtyDigest = "dirty", "sha256:"+strings.Repeat("d", 64)
	changed.RepositoryAfter = &after
	writeAt(t, evidencePath(root, "corvint-ruby-test-provider-2.json"), string(encodeRuby(t, changed)), time.Unix(20, 0))
	if unknown := discover(t, root); unknown.Run.Freshness.State != testvalidity.FreshnessUnknown || unknown.Run.Freshness.Reason != "repository-changed-during-run" {
		t.Fatalf("unknown=%+v", unknown.Run.Freshness)
	}

	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody+"# edited\n", time.Unix(30, 0))
	stale := discover(t, root)
	if stale.Run.Freshness.State != testvalidity.FreshnessStale || stale.Run.Freshness.Reason != "retained-digest-mismatch" || stale.Discovery.Abstention == nil || stale.Discovery.Abstention.Reason != AbstentionStaleEvidence || stale.Discovery.Abstention.ReceiptRevision != strings.Repeat("b", 40) {
		t.Fatalf("stale=%+v abstention=%+v", stale.Run.Freshness, stale.Discovery.Abstention)
	}
	writeAt(t, filepath.Join(root, "spec", "spec_helper.rb"), rubyHelperBody, time.Unix(40, 0))
	writeAt(t, filepath.Join(root, "spec", "support", "added.rb"), "# added\n", time.Unix(40, 0))
	if added := discover(t, root); added.Run.Freshness.State != testvalidity.FreshnessStale || added.Run.Freshness.Reason != "retained-helper-set-changed" {
		t.Fatalf("added helper=%+v", added.Run.Freshness)
	}
	if err := os.Remove(filepath.Join(root, "spec", "user_spec.rb")); err != nil {
		t.Fatal(err)
	}
	if removed := discover(t, root); removed.Run.Freshness.State != testvalidity.FreshnessStale {
		t.Fatalf("removed=%+v", removed.Run.Freshness)
	}
}
