package main

import (
	"bytes"
	"context"
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

// TestCEMV1104ConvertParitySample runs the parity tool end to end, in its
// V1-1277 --migrated mode, over a deterministic sample (every 200th committed
// map in byte order) against a scratch clone and the portable consumer built
// from interop/cem01-go. Every sampled map must already be cem/1.0; a migrated
// map must equal the conversion of its provenance-named source blob (CEM-V1-230).
// A bind commit missing from the clone is a NOT_OBSERVED row, never a failure;
// any observed row must be idempotent, refuse reconversion and carry an
// explained class. The full-corpus runs are retained under
// docs/build-log/evidence/v1-1275 (pre-migration) and v1-1277 (migrated).
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
	if _, err := Run(Options{Repository: clone, Maps: filepath.Join(root, ".corvint", "changes"), Portable: portable, Out: out, Jobs: 2, Every: 200, Migrated: true}); err != nil {
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
		case "native-stable":
			if r.Committed != "stable-native" || r.ReconvertCode != "convert-source-is-stable" {
				t.Fatalf("%s native-stable: committed=%s reconvert=%s", r.Map, r.Committed, r.ReconvertCode)
			}
			continue
		}
		if r.Committed != "migrated" || !r.CommittedEqual {
			t.Fatalf("%s: committed=%s equal=%v", r.Map, r.Committed, r.CommittedEqual)
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

// TestCEMV1230MigratedSourceChecks pins the --migrated refusals: a map that is
// still cem/0.x, a source blob absent from the repository, committed bytes
// that differ from the conversion of the provenance-named source blob, and a
// stable map without provenance whose bytes are not its bind commit's sidecar.
func TestCEMV1230MigratedSourceChecks(t *testing.T) {
	testdata := filepath.Join("..", "..", "internal", "cem", "convert", "testdata")
	src, err := os.ReadFile(filepath.Join(testdata, "reachable-a79ab231.source.json"))
	if err != nil {
		t.Fatal(err)
	}
	converted, err := os.ReadFile(filepath.Join(testdata, "reachable-a79ab231.converted.json"))
	if err != nil {
		t.Fatal(err)
	}
	const bind = "a79ab231e86f5365d8a84732e90f4b2c0755ac4a"
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v: %s", err, out)
	}
	var row Row
	if _, reason := migratedSource(context.Background(), repo, bind, src, &row); reason != "not-migrated" {
		t.Fatalf("0.x map: reason %q", reason)
	}
	if _, reason := migratedSource(context.Background(), repo, bind, converted, &row); reason != "source-blob-unavailable" || row.Committed != "migrated" {
		t.Fatalf("absent blob: reason %q committed %q", reason, row.Committed)
	}
	if _, reason := migratedSource(context.Background(), repo, strings.Repeat("0", 40), converted, &row); reason != "provenance-mismatch" {
		t.Fatalf("other bind: reason %q", reason)
	}
	hash := exec.Command("git", "-C", repo, "hash-object", "-w", "--stdin")
	hash.Stdin = bytes.NewReader(src)
	if out, err := hash.CombinedOutput(); err != nil {
		t.Fatalf("hash-object: %v: %s", err, out)
	}
	got, reason := migratedSource(context.Background(), repo, bind, converted, &row)
	if reason != "" || !bytes.Equal(got, src) {
		t.Fatalf("present blob: reason %q equal %v", reason, bytes.Equal(got, src))
	}
	native, err := os.ReadFile(filepath.Join("..", "..", ".corvint", "changes", "860f59ea2858aa90e631a995b396af0191adc604.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	sidecar := func(data []byte) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(repo, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".corvint", "change.cem.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "bind"}} {
			if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, out)
			}
		}
		head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(head))
	}
	// A stable map without provenance whose bind commit tracks other bytes,
	// as a migrated map with its provenance removed would be.
	row = Row{}
	if _, reason := migratedSource(context.Background(), repo, sidecar(src), native, &row); reason != "native-bytes-not-bound" || row.Committed != "" {
		t.Fatalf("unbound native: reason %q committed %q", reason, row.Committed)
	}
	if _, reason := migratedSource(context.Background(), repo, sidecar(native), native, &row); reason != "" || row.Committed != "stable-native" {
		t.Fatalf("bound native: reason %q committed %q", reason, row.Committed)
	}
	if b := targets([]Row{{Map: bind, SubstitutedTarget: "t"}, {Map: "x"}}); string(b) != bind+" t\n" {
		t.Fatalf("targets = %q", b)
	}
}
