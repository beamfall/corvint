package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func TestEmitQualifiedRetainsCanonicalUnknowns(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"}, Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "server-not-ready", Detail: "fixture"}}
	var stdout, stderr bytes.Buffer
	if err := emit(&stdout, &stderr, r, root); err == nil {
		t.Fatal("infrastructure exit became success")
	}
	want, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatal("stdout is not canonical")
	}
	files, err := filepath.Glob(filepath.Join(root, ".corvint", "test-evidence", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("retained %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(data, want) {
		t.Fatal("retained bytes differ")
	}
	if _, err = testvaliditydoc.Decode(data); err != nil {
		t.Fatal(err)
	}
}

// TestParseUnitConfig_RelativeDirResolvedAbsolute confirms the unit
// subcommand's default --dir "." (and any other relative --dir) is resolved
// to an absolute clean path before it reaches procgroup, which rejects a
// relative Spec.Dir.
func TestParseUnitConfig_RelativeDirResolvedAbsolute(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cfg, _, err := parseUnitConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cfg.Dir) {
		t.Fatalf("want absolute Dir, got %q", cfg.Dir)
	}
	if cfg.Dir != wd {
		t.Fatalf("want Dir=%q (cwd), got %q", wd, cfg.Dir)
	}
}

// LPCV-V0-055: --retain writes the stdout document's exact bytes into
// .corvint/test-evidence of the enclosing worktree, prunes only its own names
// to the newest 32, and refuses a symlinked location while still emitting.
func TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "web"), evidence} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for index := range 40 {
		name := filepath.Join(evidence, fmt.Sprintf("corvint-js-test-provider-%019d-%016x.json", index+1, index))
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(evidence, "operator.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, retain, err := parseUnitConfig([]string{"--dir", filepath.Join(root, "web"), "--retain"})
	if err != nil || !retain {
		t.Fatalf("retain=%v err=%v", retain, err)
	}
	receipt := jstestprovider.Receipt{Kind: "unit", Tests: []jstestprovider.TestOutcome{{Name: "adds", State: jstestprovider.StatePassed}}}
	var stdout, stderr bytes.Buffer
	if err := emit(&stdout, &stderr, receipt, cfg.Dir); err != nil || stderr.Len() != 0 {
		t.Fatalf("err=%v stderr=%s", err, stderr.String())
	}
	var plain bytes.Buffer
	if err := emit(&plain, &stderr, receipt, ""); err != nil || !bytes.Equal(plain.Bytes(), stdout.Bytes()) {
		t.Fatalf("err=%v: without --retain the stdout bytes changed", err)
	}
	retained, _ := filepath.Glob(filepath.Join(evidence, "corvint-js-test-provider-*.json"))
	sort.Strings(retained)
	newest := retained[len(retained)-1]
	data, err := os.ReadFile(newest)
	info, statErr := os.Stat(newest)
	if err != nil || statErr != nil || !bytes.Equal(data, stdout.Bytes()) || info.Mode().Perm() != 0o600 || len(retained) != 32 {
		t.Fatalf("retained=%d mode=%v equal=%v err=%v", len(retained), info, bytes.Equal(data, stdout.Bytes()), err)
	}
	if _, err := os.Stat(filepath.Join(evidence, "operator.json")); err != nil {
		t.Fatalf("pruned a foreign entry: %v", err)
	}

	linked, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(linked, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".corvint"), filepath.Join(linked, ".corvint")); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := emit(&stdout, &stderr, receipt, linked); !errors.Is(err, errRetention) || !bytes.Equal(stdout.Bytes(), plain.Bytes()) || !strings.Contains(stderr.String(), "retention failure") {
		t.Fatalf("err=%v stdout=%d stderr=%s", err, stdout.Len(), stderr.String())
	}
	if after, _ := filepath.Glob(filepath.Join(evidence, "corvint-js-test-provider-*.json")); len(after) != 32 {
		t.Fatalf("a refused retention wrote through the symlink: %d entries", len(after))
	}
}
