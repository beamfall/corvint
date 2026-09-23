package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bundleCLI runs `--root ROOT prove ARGS...` in process and returns stdout,
// the decoded stdout object when it is one, stderr and the exit code.
func bundleCLI(t *testing.T, root string, arguments ...string) (map[string]any, []byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "prove"}, arguments...), strings.NewReader(""), &stdout, &stderr)
	object, _ := decodeJSONObject(bytes.TrimSuffix(stdout.Bytes(), []byte{'\n'}))
	return object, stdout.Bytes(), stderr.String(), code
}

func exportBundle(t *testing.T, root string, arguments ...string) (map[string]any, string) {
	t.Helper()
	bundle, raw, stderr, code := bundleCLI(t, root, append([]string{"--export-bundle"}, arguments...)...)
	if code != 0 || bundle == nil {
		t.Fatalf("export exit %d: %s", code, stderr)
	}
	file := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle, file
}

func replayBundle(t *testing.T, root, file string) (map[string]any, string, int) {
	t.Helper()
	report, _, stderr, code := bundleCLI(t, root, "--replay-bundle", file)
	return report, stderr, code
}

func wantReplayRefusal(t *testing.T, root, file, want string) {
	t.Helper()
	report, stderr, code := replayBundle(t, root, file)
	var failure map[string]any
	if json.Unmarshal([]byte(stderr), &failure) != nil || code != 2 || report != nil || failure["code"] != want {
		t.Fatalf("wanted %s, got exit %d report=%v stderr=%s", want, code, report, stderr)
	}
}

func secondClone(t *testing.T, root string) string {
	t.Helper()
	second := filepath.Join(t.TempDir(), "second")
	affectedGit(t, filepath.Dir(second), "clone", "-q", root, second)
	affectedGit(t, second, "config", "user.email", "corvint@example.test")
	affectedGit(t, second, "config", "user.name", "Corvint Test")
	return second
}

// resealBundle applies edit to a bundle and recomputes bundle_sha256, so the
// digest cannot be what refuses it.
func resealBundle(t *testing.T, bundle map[string]any, edit func(map[string]any)) string {
	t.Helper()
	raw, _ := json.Marshal(bundle)
	copied, err := decodeJSONObject(raw)
	if err != nil {
		t.Fatal(err)
	}
	edit(copied)
	copied["bundle_sha256"] = bundleDigest(copied)
	encoded, _ := normalizedCanonical(t, copied)
	return writeRawBytes(t, append(encoded, '\n'))
}

func normalizedCanonical(t *testing.T, object map[string]any) ([]byte, map[string]any) {
	t.Helper()
	encoded, normalized, err := normalizedJSON(object)
	if err != nil {
		t.Fatal(err)
	}
	return encoded, normalized
}

func worktreeStatus(t *testing.T, root string) string {
	t.Helper()
	return affectedGit(t, root, "status", "--porcelain", "--ignored", "--untracked-files=all")
}

// FPK-V0-041/042/044/046/047: an explicit export freezes a query result with
// its request, identities and engine, writes nothing, and a second plain
// clone reproduces it; the report is historical.
func TestProveBundleQueryReproducesOnASecondClone(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	before := worktreeStatus(t, root)
	bundle, file := exportBundle(t, root, "--task", "Original consumer", "--limit", "2")
	if after := worktreeStatus(t, root); after != before {
		t.Fatalf("export changed the worktree:\n%s\n%s", before, after)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	repository := bundle["repository"].(map[string]any)
	request := bundle["request"].(map[string]any)
	original := bundle["original"].(map[string]any)
	engine := bundle["engine"].(map[string]any)
	if bundle["historical"] != true || bundle["version"] != bundleVersion || repository["commit"] != head || request["mode"] != "query" {
		t.Fatalf("bundle identity: %v", bundle)
	}
	if strings.Join(stringSliceOf(request["arguments"]), " ") != "--task Original consumer --limit 2" || engine["prove_profile"] != proveProfile || engine["corvint_version"] != version {
		t.Fatalf("bundle request or engine: %v %v", request, engine)
	}
	receipt := original["receipt"].(map[string]any)
	if _, present := receipt["proof"].(map[string]any)["ledger"]; present || original["exit"] != json.Number("0") {
		t.Fatalf("original must be the ledger-free receipt: %v", original)
	}
	if len(repository["blobs"].([]any)) == 0 {
		t.Fatalf("a cited query bundle names its blobs: %v", repository)
	}
	report, stderr, code := replayBundle(t, secondClone(t, root), file)
	if code != 0 || report["outcome"] != "reproduced" || report["historical"] != true || report["profile"] != bundleReplayProfile {
		t.Fatalf("replay exit %d report=%v stderr=%s", code, report, stderr)
	}
	if strings.Join(stringSliceOf(report["excluded"]), ",") != "error.message,proof.ledger" || len(report["differences"].([]any)) != 0 {
		t.Fatalf("replay report: %v", report)
	}
}

func stringSliceOf(value any) []string {
	items, _ := value.([]any)
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.(string))
	}
	return values
}

// FPK-V0-042/044: a checkpoint bundle carries the caller's exact bytes, so a
// refused and an admitted checkpoint both reproduce after the file is gone.
func TestProveBundleCheckpointUsesFrozenBytes(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	second := secondClone(t, root)
	refused := checkpointInput(t, root, "AGENTS.md")
	refused["repository"].(map[string]any)["object_format"] = "sha256"
	for name, document := range map[string]map[string]any{"refused": refused, "admitted": checkpointInput(t, root, "pkg/code.go")} {
		input := writeCheckpoint(t, document)
		bundle, file := exportBundle(t, root, "--checkpoint", input)
		if err := os.Remove(input); err != nil {
			t.Fatal(err)
		}
		original := bundle["original"].(map[string]any)
		if (name == "refused") != (original["exit"] == json.Number("2")) {
			t.Fatalf("%s original: %v", name, original)
		}
		if bundle["inputs"].(map[string]any)["checkpoint_document"] == nil {
			t.Fatalf("%s bundle lacks the checkpoint bytes: %v", name, bundle["inputs"])
		}
		report, stderr, code := replayBundle(t, second, file)
		if code != 0 || report["outcome"] != "reproduced" {
			t.Fatalf("%s replay exit %d report=%v stderr=%s", name, code, report, stderr)
		}
	}
}

// FPK-V0-045: each refusal reason is distinct and reported before any rerun.
func TestProveBundleReplayRefusalReasons(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	bundle, file := exportBundle(t, root, "--task", "Original consumer")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	edited := func(edit func(map[string]any)) string { return resealBundle(t, bundle, edit) }
	unsealed := strings.Replace(string(raw), `"exit":0`, `"exit":2`, 1)
	cases := []struct{ name, file, want string }{
		{"not JSON", writeRawBytes(t, []byte("not json")), "invalid-bundle"},
		{"not canonical", writeRawBytes(t, append([]byte(" "), raw...)), "invalid-bundle"},
		{"unknown member", edited(func(b map[string]any) { b["extra"] = true }), "invalid-bundle"},
		{"bundle flag in arguments", edited(func(b map[string]any) {
			b["request"].(map[string]any)["arguments"] = []any{"--export-bundle", "--task", "x"}
		}), "invalid-bundle"},
		{"arguments of another mode", edited(func(b map[string]any) {
			b["request"].(map[string]any)["arguments"] = []any{"pkg/code.go"}
		}), "invalid-bundle"},
		{"future version", edited(func(b map[string]any) { b["version"] = "corvint-failure-bundle/9" }), "unsupported-version"},
		{"edited without resealing", writeRawBytes(t, []byte(unsealed)), "tampered"},
		{"receipt edited under a resealed bundle", edited(func(b map[string]any) {
			b["original"].(map[string]any)["receipt"].(map[string]any)["state"] = "PROVEN"
		}), "tampered"},
		{"cited blob moved to another path", edited(func(b map[string]any) {
			b["repository"].(map[string]any)["blobs"].([]any)[0].(map[string]any)["path"] = "elsewhere.go"
		}), "tampered"},
		{"cited blobs emptied", edited(func(b map[string]any) { b["repository"].(map[string]any)["blobs"] = []any{} }), "tampered"},
		{"uncited blob added", edited(func(b map[string]any) {
			repository := b["repository"].(map[string]any)
			oid := repository["blobs"].([]any)[0].(map[string]any)["oid"]
			repository["blobs"] = append(repository["blobs"].([]any), map[string]any{"oid": oid, "path": "../../etc/passwd"})
		}), "tampered"},
		{"no arguments", edited(func(b map[string]any) { delete(b["request"].(map[string]any), "arguments") }), "missing-input"},
		{"no historical marker", edited(func(b map[string]any) { delete(b, "historical") }), "missing-input"},
		{"checkpoint without bytes", edited(func(b map[string]any) {
			b["request"].(map[string]any)["mode"] = "checkpoint"
		}), "missing-input"},
		{"other engine", edited(func(b map[string]any) { b["engine"].(map[string]any)["corvint_version"] = "0.0.1" }), "incompatible-engine"},
		{"absent commit", edited(func(b map[string]any) { b["repository"].(map[string]any)["commit"] = strings.Repeat("1", 40) }), "missing-git-object"},
		{"absent blob", edited(func(b map[string]any) {
			original := b["original"].(map[string]any)
			oid := b["repository"].(map[string]any)["blobs"].([]any)[0].(map[string]any)["oid"].(string)
			raw := strings.ReplaceAll(string(mustJSON(t, original["receipt"])), oid, strings.Repeat("2", 40))
			receipt, err := decodeJSONObject([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := normalizedCanonical(t, receipt)
			digest := sha256.Sum256(encoded)
			original["receipt"], original["receipt_sha256"] = receipt, hex.EncodeToString(digest[:])
			blobs := []any{}
			for _, blob := range bundleBlobs(receipt) {
				blobs = append(blobs, map[string]any{"oid": blob.OID, "path": blob.Path})
			}
			b["repository"].(map[string]any)["blobs"] = blobs
		}), "missing-git-object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantReplayRefusal(t, root, tc.file, tc.want) })
	}
	moved := secondClone(t, root)
	writeFixtureFiles(t, moved, map[string]string{"pkg/new.go": "package pkg\n"})
	affectedGit(t, moved, "add", ".")
	affectedGit(t, moved, "commit", "-qm", "move HEAD")
	wantReplayRefusal(t, moved, file, "drift")
	dirty := secondClone(t, root)
	writeFixtureFiles(t, dirty, map[string]string{"pkg/code.go": "package pkg\n\nfunc Edited() {}\n"})
	wantReplayRefusal(t, dirty, file, "mixed-worktree")
}

func writeRawBytes(t *testing.T, raw []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// FPK-V0-046: a result that no longer matches is diverged (exit 1) with the
// differing member paths; the report never carries the replayed receipt.
func TestProveBundleReportsDivergence(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	bundle, _ := exportBundle(t, root, "--task", "Original consumer")
	file := resealBundle(t, bundle, func(b map[string]any) {
		original := b["original"].(map[string]any)
		receipt := original["receipt"].(map[string]any)
		receipt["state"] = "FORGED"
		encoded, _ := normalizedCanonical(t, receipt)
		digest := sha256.Sum256(encoded)
		original["receipt_sha256"] = hex.EncodeToString(digest[:])
	})
	report, stderr, code := replayBundle(t, root, file)
	if code != 1 || report["outcome"] != "diverged" || strings.Join(stringSliceOf(report["differences"]), ",") != "receipt.state" {
		t.Fatalf("divergence exit %d report=%v stderr=%s", code, report, stderr)
	}
	if _, present := report["receipt"]; present {
		t.Fatalf("report carries a replayed receipt: %v", report)
	}
}

// FPK-V0-041/043: export refuses a dirty worktree, secret-shaped text, a
// non-UTF-8 checkpoint, an unsupported mode and ambiguous flags, and writes
// nothing to stdout when it refuses.
func TestProveBundleExportRefusals(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	badInput := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(badInput, []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"secret in the request", []string{"--export-bundle", "--task", "rotate sk-abcdefghijklmnopqrstuvwxyz0123"}, "bundle-secret-detected"},
		{"non-UTF-8 checkpoint", []string{"--export-bundle", "--checkpoint", badInput}, "unsupported-bundle-input"},
		{"repeated flag", []string{"--export-bundle", "--export-bundle", "--task", "x"}, "invalid-arguments"},
		{"impact mode", []string{"--export-bundle", "pkg/code.go"}, "invalid-arguments"},
		{"export with replay", []string{"--export-bundle", "--replay-bundle", "b.json"}, "invalid-arguments"},
		{"replay with other arguments", []string{"--replay-bundle", "b.json", "--task", "x"}, "invalid-arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stdout, stderr, code := bundleCLI(t, root, tc.args...)
			var failure map[string]any
			if json.Unmarshal([]byte(stderr), &failure) != nil || code != 2 || len(stdout) != 0 || failure["code"] != tc.want {
				t.Fatalf("wanted %s, got exit %d stdout=%s stderr=%s", tc.want, code, stdout, stderr)
			}
		})
	}
	writeFixtureFiles(t, root, map[string]string{"scratch.txt": "untracked\n"})
	_, stdout, stderr, code := bundleCLI(t, root, "--export-bundle", "--task", "Original consumer")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"mixed-worktree"`) {
		t.Fatalf("dirty export exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// FPK-V0-043: the size bound refuses a bundle past 8 MiB.
func TestProveBundleSizeBound(t *testing.T) {
	t.Parallel()
	_, err := sealProveBundle(proveBundle{Original: bundleResult{Receipt: map[string]any{"pad": strings.Repeat("x", maxProofDocumentBytes)}}})
	if bundleErrorCode(err) != "bundle-bound-exceeded" {
		t.Fatalf("oversized bundle: %v", err)
	}
}

// FPK-V0-048: prove-observe refuses a bundle, a replay report and a real
// receipt that carries the historical marker, and writes no ledger row.
func TestProveObserveRefusesHistoricalDocuments(t *testing.T) {
	t.Parallel()
	root := checkpointRepository(t)
	bundle, file := exportBundle(t, root, "--task", "Original consumer")
	report, _, code := replayBundle(t, root, file)
	if code != 0 {
		t.Fatalf("replay exit %d", code)
	}
	receipt := bundle["original"].(map[string]any)["receipt"].(map[string]any)
	if stderr, code := observeProof(t, root, string(mustJSON(t, receipt))); code != 0 {
		t.Fatalf("the unmarked receipt is a valid proof document: exit %d %s", code, stderr)
	}
	if err := os.Remove(filepath.Join(root, ".corvint", "self-observations.jsonl")); err != nil {
		t.Fatal(err)
	}
	marked := withMember(receipt, "historical", true)
	nullMarked := withMember(receipt, "historical", nil)
	for name, document := range map[string]map[string]any{"bundle": bundle, "report": report, "marked receipt": marked, "null-marked receipt": nullMarked} {
		if stderr, code := observeProof(t, root, string(mustJSON(t, document))); code != 2 || !strings.Contains(stderr, "invalid-proof-document") {
			t.Fatalf("%s: exit %d %s", name, code, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", "self-observations.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("a historical document reached the ledger: %v", err)
	}
}
