package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func reproTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.email=fixture@example.test", "-c", "user.name=Fixture"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-q", "-m", "fixture")
	return root
}

func reproInvoke(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(""), &stdout, &stderr)
	var payload map[string]any
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("%v: stdout is not JSON: %v\n%s", args, err, stdout.String())
		}
	}
	return code, payload, stderr.String()
}

// TestRPBV0004RecordAndRunRoundTrip (V1-2840): record pins the repository and declared inputs,
// writes a corvint-repro/0 bundle, and run replays it with PASS per expectation plus the
// current and recording build identities.
func TestRPBV0004RecordAndRunRoundTrip(t *testing.T) {
	root := reproTestRepository(t)
	bundlePath := filepath.Join(t.TempDir(), "version.repro.json")
	code, record, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", bundlePath, "--input", "input.json", "--", "--version")
	if code != 0 || record["profile"] != "corvint-repro-record/0" || record["expectations"] != float64(2) {
		t.Fatalf("record: exit %d, %v, %s", code, record, stderr)
	}
	data, err := os.ReadFile(bundlePath)
	if err != nil || !strings.Contains(string(data), `"profile": "corvint-repro/0"`) || !strings.Contains(string(data), `"path": "input.json"`) {
		t.Fatalf("bundle: %v\n%s", err, data)
	}
	code, replay, stderr := reproInvoke(t, "--root", root, "repro", "run", bundlePath)
	if code != 0 || replay["profile"] != "corvint-repro-run/0" || replay["outcome"] != "PASS" {
		t.Fatalf("run: exit %d, %v, %s", code, replay, stderr)
	}
	built, recorded := replay["build"].(map[string]any), replay["recordedBy"].(map[string]any)
	if built["version"] != version || recorded["version"] != version || len(replay["results"].([]any)) != 2 {
		t.Fatalf("run lacks build identity or results: %v", replay)
	}
	// RPB-V0-002: an existing output is never overwritten.
	if code, _, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", bundlePath, "--", "--version"); code != 2 || !strings.Contains(stderr, "repro-output-exists") {
		t.Fatalf("existing output: exit %d, %s", code, stderr)
	}
}

// TestRPBV0006RunFailsAnUnmetExpectation (V1-2840): a failed expectation is FAIL with exit 1.
func TestRPBV0006RunFailsAnUnmetExpectation(t *testing.T) {
	root := reproTestRepository(t)
	bundlePath := filepath.Join(t.TempDir(), "b.json")
	if code, _, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", bundlePath, "--expect-exit", "0", "--expect-stdout-contains", "not-in-the-output", "--", "--version"); code != 0 {
		t.Fatalf("record: exit %d, %s", code, stderr)
	}
	code, replay, _ := reproInvoke(t, "--root", root, "repro", "run", bundlePath)
	results := replay["results"].([]any)
	if code != 1 || replay["outcome"] != "FAIL" || results[0].(map[string]any)["status"] != "PASS" || results[1].(map[string]any)["status"] != "FAIL" {
		t.Fatalf("run: exit %d, %v", code, replay)
	}
}

// TestRPBV0003UnpinnableInputsRefuse (V1-2840): record refuses a dirty tracked tree or a missing
// input, and run refuses, running nothing, when a pinned input is unavailable.
func TestRPBV0003UnpinnableInputsRefuse(t *testing.T) {
	root := reproTestRepository(t)
	scratch := t.TempDir()
	if code, _, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", filepath.Join(scratch, "missing.json"), "--input", "absent.json", "--", "--version"); code != 2 || !strings.Contains(stderr, "repro-inputs-unpinned") {
		t.Fatalf("missing input: exit %d, %s", code, stderr)
	}
	bundlePath := filepath.Join(scratch, "b.json")
	if code, _, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", bundlePath, "--input", "input.json", "--", "--version"); code != 0 {
		t.Fatalf("record: exit %d, %s", code, stderr)
	}
	refuses := func(name, want string) {
		t.Helper()
		code, payload, stderr := reproInvoke(t, "--root", root, "repro", "run", bundlePath)
		if code != 2 || payload != nil || !strings.Contains(stderr, "repro-inputs-unavailable") || !strings.Contains(stderr, want) {
			t.Fatalf("%s: exit %d, stdout %v, stderr %s", name, code, payload, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "input.json"), []byte("{\"changed\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refuses("changed input", "input.json changed since recording")
	if err := os.Remove(filepath.Join(root, "input.json")); err != nil {
		t.Fatal(err)
	}
	refuses("missing input", "input.json is missing")
	if err := os.WriteFile(filepath.Join(root, "input.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refuses("dirty tree", "uncommitted changes")
	if code, _, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", filepath.Join(scratch, "dirty.json"), "--", "--version"); code != 2 || !strings.Contains(stderr, "repro-inputs-unpinned") {
		t.Fatalf("dirty record: exit %d, %s", code, stderr)
	}
	command := exec.Command("git", "-C", root, "-c", "user.email=f@example.test", "-c", "user.name=F", "commit", "-qam", "next")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v %s", err, output)
	}
	refuses("moved HEAD", "not the pinned commit")
	data, _ := os.ReadFile(bundlePath)
	var bundle map[string]any
	_ = json.Unmarshal(data, &bundle)
	bundle["inputs"].(map[string]any)["repository"].(map[string]any)["commit"] = strings.Repeat("1", 40)
	forged, _ := json.Marshal(bundle)
	if err := os.WriteFile(bundlePath, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	refuses("absent commit", "is not in this repository")
}

// TestRPBV0002RunRefusesMalformedBundles (V1-2840): unreadable, malformed and invalid invocations
// are typed refusals.
func TestRPBV0002RunRefusesMalformedBundles(t *testing.T) {
	root := reproTestRepository(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"profile":"corvint-repro/0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		code string
	}{
		{[]string{"repro", "run", bad}, "repro-bundle-invalid"},
		{[]string{"repro", "run", bad + ".absent"}, "repro-bundle-unavailable"},
		{[]string{"repro", "run"}, "repro-invalid-arguments"},
		{[]string{"repro"}, "repro-invalid-arguments"},
		{[]string{"repro", "record", "--", "--version"}, "repro-invalid-arguments"},
		{[]string{"repro", "record", "--output", bad + ".new"}, "repro-invalid-arguments"},
		{[]string{"repro", "record", "--output", bad + ".new", "--", "repro", "run", bad}, "repro-invalid-arguments"},
		{[]string{"repro", "record", "--output", bad + ".new", "--expect-exit", "x", "--", "--version"}, "repro-invalid-arguments"},
	} {
		code, payload, stderr := reproInvoke(t, append([]string{"--root", root}, test.args...)...)
		if code != 2 || payload != nil || !strings.Contains(stderr, test.code) {
			t.Fatalf("%v: exit %d, stdout %v, stderr %s", test.args, code, payload, stderr)
		}
	}
}

// TestRPBV0047RelativeArgumentsResolveAgainstRoot (V1-2917, GitHub #1923): a relative path in the
// recorded command resolves against --root whatever the caller's working directory, so a record
// and a replay from different directories agree, and a recording whose own run fails its
// expectations says so (RPB-V0-048).
func TestRPBV0047RelativeArgumentsResolveAgainstRoot(t *testing.T) {
	root := reproTestRepository(t)
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	bundlePath := filepath.Join(t.TempDir(), "b.json")
	code, record, stderr := reproInvoke(t, "--root", root, "repro", "record", "--output", bundlePath, "--input", "README.md", "--", "migration-ratchet", "--profile", "README.md")
	if code != 0 {
		t.Fatalf("record: exit %d, %s", code, stderr)
	}
	t.Chdir(root)
	_, fromRoot, _ := reproInvoke(t, "--root", root, "repro", "record", "--output", filepath.Join(t.TempDir(), "r.json"), "--input", "README.md", "--", "migration-ratchet", "--profile", "README.md")
	observedStderr := func(result map[string]any) string {
		var bundle struct{ Observed struct{ Stderr string } }
		if err := json.Unmarshal(mustReadFile(t, result["bundle"].(string)), &bundle); err != nil {
			t.Fatal(err)
		}
		return bundle.Observed.Stderr
	}
	if record["exitCode"] != fromRoot["exitCode"] || observedStderr(record) != observedStderr(fromRoot) {
		t.Fatalf("record depends on the caller's directory: %v vs %v", record, fromRoot)
	}
	t.Chdir(elsewhere)
	// The same bundle replays identically from the root and from another directory.
	first, _, _ := reproInvoke(t, "--root", root, "repro", "run", bundlePath)
	t.Chdir(root)
	second, _, _ := reproInvoke(t, "--root", root, "repro", "run", bundlePath)
	if first != 0 || second != 0 {
		t.Fatalf("replay differs by working directory: exit %d from elsewhere, %d from root", first, second)
	}
	// RPB-V0-048: an expectation the observed run already fails is reported on record.
	_, warned, _ := reproInvoke(t, "--root", root, "repro", "record", "--output", filepath.Join(t.TempDir(), "w.json"), "--expect-stdout-contains", "not-in-the-output", "--", "--version")
	if warnings, _ := warned["warnings"].([]any); len(warnings) != 1 {
		t.Fatalf("record did not warn: %v", warned)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
