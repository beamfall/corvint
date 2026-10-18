//go:build darwin || linux

package testvaliditydoc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// LPCV-V0-053: the newest completed provider document in the retained location
// is selected; a newer non-completed Go event is skipped, never projected.
func TestDiscoverSelectsNewestRetainedEvidence(t *testing.T) {
	root := discoveryRoot(t)
	base := time.Unix(1_800_000_000, 0)
	writeAt(t, evidencePath(root, "older.json"), `{"receipt":{"kind":"unit","tests":[{"name":"old","state":"failed"}]}}`, base)
	writeAt(t, evidencePath(root, "newer.json"), boundReceipt(t, root, "new"), base.Add(time.Minute))
	writeAt(t, evidencePath(root, "running.json"), `{"profile":"corvint-go-live-session-event/0","state":"running","identity":"abc","sequence":1,"scope":null,"detail":""}`, base.Add(2*time.Minute))
	document := discover(t, root)
	if document.Discovery.Evidence != EvidenceDirectory+"/newer.json" || document.Discovery.Skipped != 1 ||
		len(document.Tests) != 1 || document.Tests[0].Name != "new" {
		t.Fatalf("document=%+v discovery=%+v", document, document.Discovery)
	}
	if document.Tests[0].Projection.Freshness.State != testvalidity.FreshnessCurrent ||
		document.Run.Freshness.State != testvalidity.FreshnessCurrent {
		t.Fatalf("freshness test=%+v run=%+v", document.Tests[0].Projection.Freshness, document.Run.Freshness)
	}
}

// LPCV-V0-053: a symlinked evidence directory is refused and a symlinked entry
// is never read, so discovery cannot leave the worktree.
func TestDiscoverRefusesSymlinkedEvidence(t *testing.T) {
	outside := discoveryRoot(t)
	writeAt(t, filepath.Join(outside, "outside.json"), `{"receipt":{"kind":"unit","tests":[{"name":"outside","state":"passed"}]}}`, time.Unix(1_900_000_000, 0))
	t.Run("directory", func(t *testing.T) {
		root := discoveryRoot(t)
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(EvidenceDirectory))); err != nil {
			t.Fatal(err)
		}
		_, err := Discover(root)
		var refusal *DiscoveryError
		if !errors.As(err, &refusal) || refusal.Code != "invalid-test-evidence-location" {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("entry", func(t *testing.T) {
		root := discoveryRoot(t)
		writeAt(t, evidencePath(root, "inside.json"), `{"receipt":{"kind":"unit","tests":[{"name":"inside","state":"passed"}]}}`, time.Unix(1_800_000_000, 0))
		if err := os.Symlink(filepath.Join(outside, "outside.json"), evidencePath(root, "zz-newest.json")); err != nil {
			t.Fatal(err)
		}
		document := discover(t, root)
		if document.Discovery.Evidence != EvidenceDirectory+"/inside.json" || document.Discovery.Skipped != 1 || document.Tests[0].Name != "inside" {
			t.Fatalf("document=%+v discovery=%+v", document, document.Discovery)
		}
	})
}

// LPCV-V0-054: retained evidence whose bound digests no longer match the
// worktree is STALE, and evidence whose identity cannot be recomputed is
// UNKNOWN; neither states a CURRENT freshness axis. Each case pins its reason.
func TestDiscoverUnmatchedIdentityIsNeverCurrent(t *testing.T) {
	cases := map[string]struct {
		mutate func(t *testing.T, root string)
		body   func(t *testing.T, root string) string
		want   string
		reason string
	}{
		"changed test file": {
			body: func(t *testing.T, root string) string { return boundReceipt(t, root, "adds") },
			mutate: func(t *testing.T, root string) {
				writeAt(t, filepath.Join(root, "src", "add.test.js"), "changed\n", time.Now())
			},
			want:   testvalidity.FreshnessStale,
			reason: "retained-digest-mismatch",
		},
		"removed test file": {
			body:   func(t *testing.T, root string) string { return boundReceipt(t, root, "adds") },
			mutate: func(t *testing.T, root string) { _ = os.Remove(filepath.Join(root, "src", "add.test.js")) },
			want:   testvalidity.FreshnessStale,
			reason: "retained-digest-mismatch",
		},
		"bound path outside worktree": {
			body: func(t *testing.T, root string) string {
				return `{"receipt":{"kind":"unit","identity":{"testFileDigests":{"/etc/hosts":"00"}},"tests":[{"name":"adds","state":"passed"}]}}`
			},
			want:   testvalidity.FreshnessUnknown,
			reason: "retained-bound-path-outside-worktree",
		},
		"go session identity": {
			body: func(t *testing.T, root string) string {
				return `{"profile":"corvint-go-live-session-event/0","state":"passed","identity":"abc","sequence":4,"scope":["./..."],"detail":"","projection":{},"testProjections":[{"package":"example/a","name":"TestAdds","action":"pass","projection":{}}],"testProjectionsOmitted":0}`
			},
			want:   testvalidity.FreshnessUnknown,
			reason: "retained-session-identity-unverifiable",
		},
		"stale go session": {
			body: func(t *testing.T, root string) string {
				return `{"profile":"corvint-go-live-session-event/0","state":"stale","identity":"abc","sequence":4,"scope":["./..."],"detail":"","projection":{},"testProjections":[{"package":"example/a","name":"TestAdds","action":"pass","projection":{}}],"testProjectionsOmitted":0}`
			},
			want:   testvalidity.FreshnessStale,
			reason: "workspace-execution-identity-mismatch",
		},
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			root := discoveryRoot(t)
			writeAt(t, evidencePath(root, "run.json"), item.body(t, root), time.Unix(1_800_000_000, 0))
			if item.mutate != nil {
				item.mutate(t, root)
			}
			document := discover(t, root)
			if len(document.Tests) != 1 || document.Discovery.Freshness == nil || document.Discovery.Freshness.State != item.want ||
				document.Tests[0].Projection.Freshness.State != item.want || document.Run.Freshness.State != item.want ||
				document.Discovery.Freshness.Reason != item.reason || document.Run.Freshness.Reason != item.reason {
				t.Fatalf("discovery=%+v document=%+v", document.Discovery, document)
			}
		})
	}
}

// LPCV-V0-053/LPCV-V0-049: with no retained evidence, or none that decodes,
// every run axis is UNSUPPORTED and no test is reported.
func TestDiscoverWithoutEvidenceIsUnsupported(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no location":     nil,
		"no usable entry": {"broken.json": `{"receipt":`, "notes.txt": `{"receipt":{"kind":"unit","tests":[]}}`},
	} {
		t.Run(name, func(t *testing.T) {
			root := discoveryRoot(t)
			for file, body := range files {
				writeAt(t, evidencePath(root, file), body, time.Unix(1_800_000_000, 0))
			}
			document := discover(t, root)
			run := document.Run
			for _, axis := range []testvalidity.Axis{run.Association, run.Hygiene, run.Freshness, run.Execution, run.Strength} {
				if axis.State != testvalidity.StateUnsupported {
					t.Fatalf("axis=%+v", axis)
				}
			}
			if document.Source != "none" || len(document.Tests) != 0 || document.Discovery.Evidence != "" || document.Discovery.Skipped != len(files)/2 {
				t.Fatalf("document=%+v discovery=%+v", document, document.Discovery)
			}
		})
	}
}

// externalReceipt is the canonical retained document an external Playwright
// run on the given Node version retains when the reporter drops every
// unqualified test identity (the issue #665 consumer shape).
func externalReceipt(t *testing.T, node string) string {
	t.Helper()
	r := jstestprovider.Receipt{
		Profile:        jstestprovider.ExternalProfile,
		Kind:           "e2e",
		Identity:       jstestprovider.Identity{RunnerName: "playwright", RunnerVersion: "1.63.0", NodeVersion: node},
		External:       &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"},
		Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "project-location-unknown"},
		Tests:          []jstestprovider.TestOutcome{{Name: "flow", State: jstestprovider.StatePassed}},
	}
	data, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// LPCV-V0-056: discovery states a closed, typed reason whenever it yields no
// evidence that could pass, so callers can tell rediscovery will not help.
func TestDiscoverAbstentionReasons(t *testing.T) {
	for _, test := range []struct {
		name, body, reason, requires, node string
		directoryOnly                      bool
	}{
		{name: "missing location", reason: AbstentionNoRetainedEvidence, requires: RequiresRetainedProducerRun},
		{name: "no json entries", directoryOnly: true, reason: AbstentionNoRetainedEvidence, requires: RequiresRetainedProducerRun},
		{name: "only undecodable entries", body: `{"unknown":true}`, reason: AbstentionRetainedEvidenceUnusable, requires: RequiresRetainedProducerRun},
		{name: "unqualified node tuple", body: externalReceipt(t, "v24.11.0"), reason: AbstentionRuntimeTupleUnqualified, requires: RequiresQualifiedRuntimeTuple, node: "v24.11.0"},
		{name: "unobserved node tuple", body: externalReceipt(t, ""), reason: AbstentionUnknown, requires: AbstentionUnknown, node: ""},
		{name: "qualified candidate tuple", body: externalReceipt(t, "v22.23.2")},
		{name: "plain unit receipt", body: `{"receipt":{"kind":"unit","tests":[{"name":"adds","state":"passed"}]}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := discoveryRoot(t)
			if test.directoryOnly {
				writeAt(t, evidencePath(root, "notes.txt"), "not evidence", time.Unix(1_800_000_000, 0))
			}
			if test.body != "" {
				writeAt(t, evidencePath(root, "run.json"), test.body, time.Unix(1_800_000_000, 0))
			}
			document := discover(t, root)
			got := document.Discovery.Abstention
			if test.reason == "" {
				if got != nil {
					t.Fatalf("unexpected abstention %+v", got)
				}
				return
			}
			if got == nil || got.Reason != test.reason || got.Requires != test.requires {
				t.Fatalf("abstention=%+v source=%s", got, document.Source)
			}
			if (test.reason == AbstentionRuntimeTupleUnqualified || test.reason == AbstentionUnknown) && (got.Observed == nil || got.Observed.NodeVersion != test.node || got.Observed.RunnerVersion != "1.63.0" || document.Source != "corvint-js-test-provider") {
				t.Fatalf("observed tuple not retained: %+v source=%s", got.Observed, document.Source)
			}
			if test.requires == RequiresRetainedProducerRun && (document.Source != "none" || len(document.Tests) != 0) {
				t.Fatalf("no-evidence reason on a projected document: %+v", document)
			}
		})
	}
}

// LPCV-V0-056: an observed version that is not a short release token is
// withheld; the closed reason still states the tuple is unqualified.
func TestDiscoverAbstentionWithholdsUnboundedObservedVersion(t *testing.T) {
	for _, node := range []string{"v" + strings.Repeat("9", 70), "v24.11.0\nsecret"} {
		root := discoveryRoot(t)
		writeAt(t, evidencePath(root, "run.json"), externalReceipt(t, node), time.Unix(1_800_000_000, 0))
		got := discover(t, root).Discovery.Abstention
		if got == nil || got.Reason != AbstentionRuntimeTupleUnqualified || got.Observed != nil {
			t.Fatalf("observed version not withheld: %+v", got)
		}
	}
}

// LPCVD-V0-001: absent, empty and unusable evidence all name real opt-in
// producers; hints cannot claim the host is qualified or that a run occurred.
func TestDiscoverProducerGuidance(t *testing.T) {
	for _, body := range []string{"", "not JSON"} {
		root := discoveryRoot(t)
		if body != "" {
			writeAt(t, evidencePath(root, "bad.json"), body, time.Unix(1, 0))
		}
		document := discover(t, root)
		a := document.Discovery.Abstention
		if document.Source != "none" || a == nil || a.ExpectedEvidencePath != EvidenceDirectory || len(a.Producers) != 4 {
			t.Fatalf("missing guidance: %+v", a)
		}
		for i, prefix := range []string{"corvint-js-test-provider unit --retain", "corvint-js-test-provider e2e --retain", "corvint-go-test-provider session --retain --experimental --trusted-local --foreground --authority-bundle", "corvint-ruby-test-provider rspec --retain --dir"} {
			if !strings.HasPrefix(a.Producers[i].Command, prefix) || a.Producers[i].Qualification != "NOT_OBSERVED" {
				t.Fatalf("producer=%+v", a.Producers[i])
			}
		}
		for _, required := range []string{"--external-server", "--app-identity <application-identity>", "--server-ready-url <http-or-https-url>", "--config <playwright-config>", "--runner-version <playwright-version>", "--test-file <test-file>"} {
			if !strings.Contains(a.Producers[1].Command, required) {
				t.Fatalf("external producer missing %s: %+v", required, a.Producers[1])
			}
		}
		if _, err := os.Stat(filepath.Join(root, ".git")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("discovery unexpectedly created Git state: %v", err)
		}
		if body == "" {
			if _, err := os.Stat(evidencePath(root, "")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("discovery created evidence: %v", err)
			}
		}
	}
}

// LPCVD-V0-002: stale is established by existing bound evidence, never
// relabelled as absence. Missing repository revisions remain UNKNOWN.
func TestDiscoverStaleEvidenceGuidance(t *testing.T) {
	for _, kind := range []string{"javascript", "go-session"} {
		t.Run(kind, func(t *testing.T) {
			root := discoveryRoot(t)
			body := boundReceipt(t, root, "adds")
			if kind == "go-session" {
				body = `{"profile":"corvint-go-live-session-event/0","state":"stale","identity":"abc","sequence":4,"scope":[],"detail":"","projection":{},"testProjections":[],"testProjectionsOmitted":0}`
			}
			writeAt(t, evidencePath(root, "run.json"), body, time.Unix(1, 0))
			if kind == "javascript" {
				if got := discover(t, root); got.Run.Freshness.State != testvalidity.FreshnessCurrent || got.Discovery.Abstention != nil {
					t.Fatalf("fixture not current: %+v", got)
				}
				writeAt(t, filepath.Join(root, "src", "add.test.js"), "changed", time.Unix(2, 0))
			}
			got := discover(t, root)
			a := got.Discovery.Abstention
			if got.Source == "none" || got.Run.Freshness.State != testvalidity.FreshnessStale || a == nil || a.Reason != AbstentionStaleEvidence || a.Requires != RequiresRetainedProducerRun || a.ReceiptRevision != "UNKNOWN" || a.ExpectedEvidencePath != EvidenceDirectory || len(a.Producers) == 0 {
				t.Fatalf("stale evidence=%+v abstention=%+v", got, a)
			}
		})
	}
}

// LPCVD-V0-003: use only the newest decodable receipt's stable mtime age
// origin, skipping malformed newer entries and exposing unknown when absent.
func TestDiscoverReceiptAge(t *testing.T) {
	root := discoveryRoot(t)
	if got := discover(t, root).Discovery.ReceiptAge; got.Basis != "NOT_OBSERVED" || got.Since != "" {
		t.Fatalf("absent age=%+v", got)
	}
	base := time.Unix(1_700_000_000, 123000000).UTC()
	writeAt(t, evidencePath(root, "older.json"), `{"receipt":{"kind":"unit","tests":[]}}`, base.Add(-time.Hour))
	writeAt(t, evidencePath(root, "newer.json"), `{"receipt":{"kind":"unit","tests":[]}}`, base)
	writeAt(t, evidencePath(root, "broken.json"), `invalid`, base.Add(time.Hour))
	got := discover(t, root).Discovery
	if got.Evidence != EvidenceDirectory+"/newer.json" || got.ReceiptAge.Basis != "retained-file-mtime" || got.ReceiptAge.Since != base.Format(time.RFC3339Nano) || got.Skipped != 1 {
		t.Fatalf("age=%+v", got)
	}
	if again := discover(t, root).Discovery.ReceiptAge; again != got.ReceiptAge {
		t.Fatalf("age origin drifted: %+v %+v", got.ReceiptAge, again)
	}
}

// LPCVD-V0-002: echo only the test-repository revision retained at run start,
// never the application repository or an unbounded/untrusted revision string.
func TestDiscoverRetainedRevision(t *testing.T) {
	for _, revision := range []string{strings.Repeat("a", 40), strings.Repeat("b", 64), "", strings.Repeat("A", 40), "bad\nrevision"} {
		t.Run(revision, func(t *testing.T) {
			root := discoveryRoot(t)
			input, err := Decode([]byte(boundReceipt(t, root, "adds")))
			if err != nil {
				t.Fatal(err)
			}
			receipt := *input.js
			receipt.Profile = jstestprovider.AttestedExternalProfile
			receipt.Identity.RunnerVersion = "1.63.0"
			receipt.Identity.NodeVersion = "v24.11.0"
			receipt.TestRepositoryAtStart = &jstestprovider.ApplicationRepositoryIdentity{Revision: revision}
			receipt.TestRepositoryAtPublish = &jstestprovider.ApplicationRepositoryIdentity{Revision: strings.Repeat("c", 40)}
			receipt.ApplicationAttestation = &jstestprovider.ApplicationAttestationReceipt{Before: &jstestprovider.ApplicationAttestationObservation{Attestation: jstestprovider.ApplicationAttestation{Repository: jstestprovider.ApplicationRepositoryIdentity{Revision: strings.Repeat("d", 40)}}}}
			data, err := jstestprovider.EncodeQualified(receipt)
			if err != nil {
				t.Fatal(err)
			}
			writeAt(t, evidencePath(root, "run.json"), string(data), time.Unix(1, 0))
			before := discover(t, root)
			if before.Source == "none" || before.Discovery.Freshness.State != testvalidity.FreshnessUnknown || before.Discovery.Abstention == nil || before.Discovery.Abstention.Reason != AbstentionRuntimeTupleUnqualified {
				t.Fatalf("fixture not bound: %+v", before)
			}
			writeAt(t, filepath.Join(root, "src", "add.test.js"), "changed", time.Unix(2, 0))
			got := discover(t, root).Discovery.Abstention
			want := "UNKNOWN"
			if revision == strings.Repeat("a", 40) || revision == strings.Repeat("b", 64) {
				want = revision
			}
			if got == nil || got.Reason != AbstentionStaleEvidence || got.ReceiptRevision != want || got.Observed == nil || got.Observed.NodeVersion != "v24.11.0" || got.Observed.RunnerVersion != "1.63.0" {
				t.Fatalf("revision=%q abstention=%+v", revision, got)
			}
		})
	}
}

// LPCVD-V0-004: a receipt whose retained test-repository revision differs from
// the worktree HEAD reports stale-evidence even when every bound file is
// unchanged; an unresolved, unsafe or equal HEAD never does.
func headFixture(t *testing.T, retained string) string {
	t.Helper()
	root := discoveryRoot(t)
	input, err := Decode([]byte(boundReceipt(t, root, "adds")))
	if err != nil {
		t.Fatal(err)
	}
	receipt := *input.js
	receipt.Profile = jstestprovider.AttestedExternalProfile
	receipt.Identity.RunnerVersion = "1.63.0"
	receipt.Identity.NodeVersion = "v24.11.0"
	receipt.TestRepositoryAtStart = &jstestprovider.ApplicationRepositoryIdentity{Revision: retained}
	receipt.TestRepositoryAtPublish = &jstestprovider.ApplicationRepositoryIdentity{Revision: retained}
	data, err := jstestprovider.EncodeQualified(receipt)
	if err != nil {
		t.Fatal(err)
	}
	writeAt(t, evidencePath(root, "run.json"), string(data), time.Unix(1, 0))
	return root
}

func assertHeadClaim(t *testing.T, root string, stale bool, retained string) {
	t.Helper()
	got := discover(t, root).Discovery.Abstention
	if stale {
		if got == nil || got.Reason != AbstentionStaleEvidence || got.ReceiptRevision != retained {
			t.Fatalf("abstention=%+v", got)
		}
	} else if got != nil && got.Reason == AbstentionStaleEvidence {
		t.Fatalf("unexpected stale: %+v", got)
	}
}

func TestDiscoverRetainedRevisionDiffersFromHead(t *testing.T) {
	retained := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	cases := []struct {
		name  string
		files map[string]string
		stale bool
	}{
		{"detached other", map[string]string{".git/HEAD": other + "\n"}, true},
		{"detached equal", map[string]string{".git/HEAD": retained + "\n"}, false},
		{"loose ref other", map[string]string{".git/HEAD": "ref: refs/heads/main\n", ".git/refs/heads/main": other + "\n"}, true},
		{"packed ref other", map[string]string{".git/HEAD": "ref: refs/heads/main\n", ".git/packed-refs": "# pack-refs\n" + other + " refs/heads/main\n"}, true},
		{"packed ref equal", map[string]string{".git/HEAD": "ref: refs/heads/main\n", ".git/packed-refs": retained + " refs/heads/main\n"}, false},
		{"malformed loose ref does not fall back to packed", map[string]string{".git/HEAD": "ref: refs/heads/main\n", ".git/refs/heads/main": "garbage\n", ".git/packed-refs": other + " refs/heads/main\n"}, false},
		{"oversized HEAD", map[string]string{".git/HEAD": other + strings.Repeat(" ", 2<<20)}, false},
		{"unresolved ref", map[string]string{".git/HEAD": "ref: refs/heads/main\n"}, false},
		{"invalid ref name with dots", map[string]string{".git/HEAD": "ref: refs/heads/bad..ref\n", ".git/refs/heads/bad..ref": other + "\n"}, false},
		{"invalid ref name with lock suffix", map[string]string{".git/HEAD": "ref: refs/heads/x.lock\n", ".git/refs/heads/x.lock": other + "\n"}, false},
		{"escaping ref", map[string]string{".git/HEAD": "ref: refs/../../x\n"}, false},
		{"no git", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := headFixture(t, retained)
			for name, body := range tc.files {
				writeAt(t, filepath.Join(root, filepath.FromSlash(name)), body, time.Unix(1, 0))
			}
			assertHeadClaim(t, root, tc.stale, retained)
		})
	}
}

// LPCVD-V0-005 (V1-1156): a basic (unprofiled) retained JS receipt that
// carries its start revision supports the same HEAD staleness claim.
func TestDiscoverBasicReceiptRetainedRevision(t *testing.T) {
	retained := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	for _, tc := range []struct {
		name  string
		head  string
		stale bool
	}{{"other", other, true}, {"equal", retained, false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := discoveryRoot(t)
			document := boundReceipt(t, root, "adds")
			document = strings.Replace(document, `{"receipt":{"kind":"unit",`, `{"receipt":{"kind":"unit","testRepositoryAtStart":{"rootCommit":"`+other+`","revision":"`+retained+`","tree":"`+other+`","dirtyState":"clean"},`, 1)
			writeAt(t, evidencePath(root, "run.json"), document, time.Unix(1, 0))
			writeAt(t, filepath.Join(root, ".git", "HEAD"), tc.head+"\n", time.Unix(1, 0))
			assertHeadClaim(t, root, tc.stale, retained)
		})
	}
}

func TestDiscoverHeadReadRefusesSymlinksAndSpecialFiles(t *testing.T) {
	retained := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	t.Run("symlinked HEAD", func(t *testing.T) {
		root := headFixture(t, retained)
		writeAt(t, filepath.Join(root, "elsewhere"), other+"\n", time.Unix(1, 0))
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../elsewhere", filepath.Join(root, ".git", "HEAD")); err != nil {
			t.Fatal(err)
		}
		assertHeadClaim(t, root, false, retained)
	})
	t.Run("symlinked .git", func(t *testing.T) {
		root := headFixture(t, retained)
		writeAt(t, filepath.Join(root, "real", "HEAD"), other+"\n", time.Unix(1, 0))
		if err := os.Symlink("real", filepath.Join(root, ".git")); err != nil {
			t.Fatal(err)
		}
		assertHeadClaim(t, root, false, retained)
	})
	t.Run("FIFO HEAD does not block", func(t *testing.T) {
		root := headFixture(t, retained)
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(root, ".git", "HEAD"), 0o600); err != nil {
			t.Skip("fifo unavailable")
		}
		assertHeadClaim(t, root, false, retained)
	})
}
