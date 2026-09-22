//go:build darwin || linux

package testvaliditydoc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// discoveryRoot is a symlink-resolved worktree root, so bound absolute paths
// compare against the same spelling Discover confines to.
func discoveryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeAt(t *testing.T, path, body string, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func evidencePath(root, name string) string {
	return filepath.Join(root, filepath.FromSlash(EvidenceDirectory), name)
}

// boundReceipt is a JavaScript receipt binding one test file's current digest.
func boundReceipt(t *testing.T, root, testName string) string {
	t.Helper()
	source := filepath.Join(root, "src", "add.test.js")
	writeAt(t, source, "test('adds')\n", time.Unix(1_700_000_000, 0))
	sum := sha256.Sum256([]byte("test('adds')\n"))
	return `{"receipt":{"kind":"unit","identity":{"testFileDigests":{"` + source + `":"` + hex.EncodeToString(sum[:]) +
		`"}},"tests":[{"name":"` + testName + `","state":"passed"}]}}`
}

func discover(t *testing.T, root string) Document {
	t.Helper()
	document, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if document.Discovery == nil || document.Discovery.Location != EvidenceDirectory {
		t.Fatalf("discovery=%+v", document.Discovery)
	}
	return document
}

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
