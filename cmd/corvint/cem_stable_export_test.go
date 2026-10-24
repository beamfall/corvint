package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/receiptbundle"
)

// CEM-V1-060, CEM-V1-062: `cem export` of the sealed cem/1.0 positive writes a
// stable bundle whose envelope carries verify-stable's complete result; moved
// to another directory, the bundle's own map and artifacts reproduce that
// result and exit through `cem verify-stable`.
func TestCEMExportStableBundleRelocates(t *testing.T) {
	f := materializeCEMStableRoot(t)
	if v := runtime.Version(); v != "go1.27.1" && v != "go1.27.2" && v != "go1.27.3" {
		t.Skipf("NOT_RUN: %s is not an admitted structural tuple", v)
	}
	sealed, err := os.ReadFile(filepath.Join(cemStablePacket, "inherited/fixtures/maps/sha1-positive-sealed.json"))
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, f.repo, "change.cem.json", string(sealed))
	verifyStable := func(mapPath, artifacts string) (int, map[string]any) {
		return runCEMStableEntry(t, "cem", "verify-stable", "--repository", f.repo, "--map", mapPath,
			"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", artifacts)
	}
	wantExit, want := verifyStable(filepath.Join(f.repo, "change.cem.json"), f.artifacts)
	if wantExit != 0 || want["outcome"] != "ACCEPT" {
		t.Fatalf("sealed positive: exit %d %v", wantExit, want)
	}

	output := filepath.Join(filepath.Dir(f.repo), "bundle")
	exit, envelope := runCEMStableEntry(t, "--root", f.repo, "cem", "export", "--map", "change.cem.json",
		"--expected-base", f.expectedBase, "--target", f.target, "--output", output, "--artifacts", f.artifacts)
	if exit != 0 || envelope["ok"] != true || envelope["profile"] != receiptbundle.StableProfile ||
		!reflect.DeepEqual(envelope["stableVerification"], want) {
		t.Fatalf("cem export = exit %d %v", exit, envelope)
	}

	moved := filepath.Join(filepath.Dir(f.repo), "elsewhere", "bundle")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(output, moved); err != nil {
		t.Fatal(err)
	}
	exit, got := verifyStable(filepath.Join(moved, receiptbundle.StableMapName), filepath.Join(moved, receiptbundle.StableArtifactDir))
	if exit != wantExit || !reflect.DeepEqual(got, want) {
		t.Fatalf("relocated bundle = exit %d %v, want exit %d %v", exit, got, wantExit, want)
	}
	if bundled, err := os.ReadFile(filepath.Join(moved, receiptbundle.StableMapName)); err != nil || string(bundled) != string(sealed) {
		t.Fatal("the bundle rewrote the map")
	}
}
