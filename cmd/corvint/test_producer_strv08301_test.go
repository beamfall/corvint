//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestSTRV08301DryRunManifestNamesItsProducer_STRV08301: a `corvint test --dry-run --manifest`
// written by a binary stamped with a Git commit records that commit and state as the manifest's
// producer beside the planned HEAD (source.head), so script/heavy.sh --plan can refuse a manifest
// a stale binary wrote (V1-5011); a binary without a stamp records no producer.
func TestSTRV08301DryRunManifestNamesItsProducer_STRV08301(t *testing.T) {
	root, base, _ := sessionFixture(t, "pass")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/m", head))
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest); code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	m, _, err := sessiontest.ReadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Producer == nil || m.Producer.Commit != head || m.Producer.CommitState != "clean" || m.Source.Head != head {
		t.Fatalf("producer %+v, source.head %s, want commit %s", m.Producer, m.Source.Head, head)
	}
	// A binary built without a Git stamp states no producer rather than a guess.
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/m", ""))
	unstamped := filepath.Join(t.TempDir(), "manifest.json")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", unstamped); code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if m, _, err := sessiontest.ReadManifest(unstamped); err != nil || m.Producer != nil {
		t.Fatalf("unstamped producer %+v %v", m.Producer, err)
	}
}
