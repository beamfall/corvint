package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const convertFixtures = "../../internal/cem/convert/testdata"

// CEM-V1-102: `corvint cem convert` writes the converter's exact bytes to a new
// --output or in place, prints one corvint-cem-convert-report/1 report, and
// exits 0 converted, 1 refused and 2 for an argument, input or output failure.
func TestCEMV1102CEMConvertCommand(t *testing.T) {
	const bind = "a79ab231e86f5365d8a84732e90f4b2c0755ac4a"
	src, err := os.ReadFile(filepath.Join(convertFixtures, "reachable-a79ab231.source.json"))
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join(convertFixtures, "reachable-a79ab231.converted.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, bind+".cem.json")
	if err := os.WriteFile(input, src, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out.json")

	exit, report := runCEMStableEntry(t, "cem", "convert", "--map", input, "--output", output)
	provenance, _ := report["provenance"].(map[string]any)
	if exit != 0 || report["profile"] != "corvint-cem-convert-report/1" || report["status"] != "CONVERTED" || report["code"] != nil ||
		report["sourceBinding"] != "NOT_CHECKED" || provenance["bindCommit"] != bind || provenance["converter"] != "corvint-cem-convert/1" {
		t.Fatalf("convert = exit %d %v", exit, report)
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != string(golden) {
		t.Fatalf("output differs from golden: %v", err)
	}

	// An existing --output is never replaced.
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", input, "--output", output); exit != 2 || report["code"] != "publish-failed" {
		t.Fatalf("existing output = exit %d %v", exit, report)
	}
	// A converted map is refused, so conversion is applied once.
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", output, "--output", filepath.Join(dir, "again.json"), "--bind-commit", bind); exit != 1 || report["code"] != "convert-source-is-stable" || report["provenance"] != nil {
		t.Fatalf("reconvert = exit %d %v", exit, report)
	}
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", output, "--in-place"); exit != 1 || report["code"] != "convert-source-is-stable" {
		t.Fatalf("stable in place = exit %d %v", exit, report)
	}
	// Without a name-pinned commit and no --bind-commit the bind is unavailable.
	other := filepath.Join(dir, "change.cem.json")
	if err := os.WriteFile(other, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", other, "--output", filepath.Join(dir, "x.json")); exit != 1 || report["code"] != "convert-bind-commit-unavailable" {
		t.Fatalf("unnamed bind = exit %d %v", exit, report)
	}

	// --repository runs the binding check; an empty repository holds no object.
	repo := filepath.Join(dir, "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", input, "--in-place", "--repository", repo); exit != 0 || report["sourceBinding"] != "UNAVAILABLE" {
		t.Fatalf("in place = exit %d %v", exit, report)
	}
	if got, err := os.ReadFile(input); err != nil || string(got) != string(golden) {
		t.Fatalf("in-place output differs from golden: %v", err)
	}

	for _, args := range [][]string{
		{"cem", "convert", "--map", "relative.json", "--output", output},
		{"cem", "convert", "--map", input},
		{"cem", "convert", "--map", input, "--output", output, "--in-place"},
		{"cem", "convert", "--map", input, "--output", "--in-place"},
		{"cem", "convert", "--map", input, "--in-place", "--unknown", "x"},
		{"--root", dir, "cem", "convert", "--map", input, "--in-place"},
	} {
		if exit, report := runCEMStableEntry(t, args...); exit != 2 || report["code"] != "invalid-arguments" {
			t.Fatalf("%v = exit %d %v", args, exit, report)
		}
	}
	if exit, report := runCEMStableEntry(t, "cem", "convert", "--map", filepath.Join(dir, "missing.json"), "--output", filepath.Join(dir, "y.json")); exit != 2 || report["code"] != "map-unavailable" {
		t.Fatalf("missing map = exit %d %v", exit, report)
	}
}
