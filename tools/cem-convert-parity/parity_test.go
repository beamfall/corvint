package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func gitTop(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Skipf("git %v unavailable: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestCEMV1104ConvertParitySample runs the parity tool end to end over a
// deterministic sample (every 200th committed map in byte order) against a
// scratch clone and the portable consumer built from interop/cem01-go. A bind
// commit missing from the clone is a NOT_OBSERVED row, never a failure; any
// observed row must be idempotent, refuse reconversion and carry an explained
// class. The full-corpus run is retained under docs/build-log/evidence/v1-1275.
func TestCEMV1104ConvertParitySample(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the portable consumer and clones the repository")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	common := gitTop(t, "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(work, "clone")
	if out, err := exec.Command("git", "clone", "-q", "--local", "--no-checkout", common, clone).CombinedOutput(); err != nil {
		t.Skipf("scratch clone unavailable: %v: %s", err, out)
	}
	portable := filepath.Join(work, "cem01-go")
	build := exec.Command("go", "build", "-o", portable, ".")
	build.Dir = filepath.Join(root, "interop", "cem01-go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build portable consumer: %v: %s", err, out)
	}
	out := filepath.Join(work, "out")
	if _, err := Run(Options{Repository: clone, Maps: filepath.Join(root, ".corvint", "changes"), Portable: portable, Out: out, Jobs: 2, Every: 200}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "parity.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	if len(lines) == 0 {
		t.Fatal("no sampled rows")
	}
	for _, line := range lines {
		var r Row
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatal(err)
		}
		// The portable consumer's stable verification needs its qualified
		// process containment; where the host lacks it (every non-darwin CI
		// runner) the consumer refuses before it reads the map, so parity is
		// NOT_RUN there, not different (the interop fixture test skips the same way).
		if r.Portable.Code == "unsupported-process-containment" || r.DirectPortable.Code == "unsupported-process-containment" {
			t.Skip("NOT_RUN: portable consumer process containment is not qualified on " + runtime.GOOS)
		}
		switch r.Class {
		case "NOT_OBSERVED":
			t.Logf("%s NOT_OBSERVED: %s", r.Map, r.Reason)
			continue
		case "different":
			t.Fatalf("%s different: %s", r.Map, r.Reason)
		}
		if !r.Idempotent || r.ReconvertCode != "convert-source-is-stable" || r.SourceBinding != "EXACT" {
			t.Fatalf("%s: idempotent=%v reconvert=%s binding=%s", r.Map, r.Idempotent, r.ReconvertCode, r.SourceBinding)
		}
	}
}

// TestCEMV1104ConvertParityClassify pins the explanation classes, including the
// consumer partial-drift case a resource-limit abort produces.
func TestCEMV1104ConvertParityClassify(t *testing.T) {
	accept := Verdict{Outcome: "ACCEPT", Stage: "complete", ChangeIntegrity: "VERIFIED", Drift: []string{}, HunksEqual: true}
	limit := Verdict{Outcome: "UNSUPPORTED", Stage: "repository", Code: "unsupported-resource-limit", ChangeIntegrity: "NOT_CHECKED", Drift: []string{"a relocated"}, HunksEqual: true}
	limitMore := limit
	limitMore.Drift = []string{"a relocated", "b relocated"}
	base := Row{Idempotent: true, ReconvertCode: "convert-source-is-stable", NativeEqualsPortable: true}
	cases := []struct {
		name   string
		edit   func(*Row)
		class  string
		reason string
	}{
		{"identical", func(r *Row) { r.Source, r.Native, r.Portable = accept, accept, accept }, "identical", ""},
		{"not-idempotent", func(r *Row) { r.Idempotent = false }, "different", "not-idempotent"},
		{"reconvert", func(r *Row) { r.ReconvertCode = "" }, "different", "reconvert-not-refused"},
		{"limit", func(r *Row) { r.Source, r.Native, r.Portable = limit, limit, limit }, "pre-existing-refusal", "unsupported-resource-limit"},
		{"partial-drift", func(r *Row) {
			r.Source, r.Native, r.Portable, r.NativeEqualsPortable = limit, limit, limitMore, false
		}, "pre-existing-refusal", "unsupported-resource-limit consumer-partial-drift-differs"},
		{"refusal-stage-differs", func(r *Row) {
			n := limit
			n.Stage = "references"
			r.Source, r.Native, r.Portable = limit, n, n
		}, "different", "refusal-differs"},
		{"refusal-hunks-differ", func(r *Row) {
			n := limit
			n.HunksEqual = false
			r.Source, r.Native, r.Portable = limit, n, n
		}, "different", "refusal-differs"},
		{"refusal-stage-differs-consumers-differ", func(r *Row) {
			n := limit
			n.Stage = "references"
			r.Source, r.Native, r.Portable, r.NativeEqualsPortable = limit, n, limitMore, false
		}, "different", "native-portable-differ"},
		{"consumers-disagree", func(r *Row) {
			r.Source, r.Native, r.Portable, r.NativeEqualsPortable = accept, accept, limit, false
		}, "different", "native-portable-differ"},
		{"unavailable", func(r *Row) {
			v := Verdict{Outcome: "UNSUPPORTED", Code: "repository-object-unavailable"}
			r.Source, r.Native, r.Portable = v, v, v
		}, "unavailable-objects", "repository-object-unavailable"},
		{"references", func(r *Row) {
			n := accept
			n.Outcome, n.Stage = "REJECT", "references"
			r.Source, r.Native, r.Portable = accept, n, n
		}, "empty-member-explained", ""},
		{"verdict-differs", func(r *Row) {
			n := accept
			n.Outcome = "REJECT"
			r.Source, r.Native, r.Portable = accept, n, n
		}, "different", "verdict-differs"},
	}
	for _, c := range cases {
		r := base
		c.edit(&r)
		if class, reason := classify(r); class != c.class || reason != c.reason {
			t.Fatalf("%s: classify = %s/%q, want %s/%q", c.name, class, reason, c.class, c.reason)
		}
	}
}
