package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// stableReport0xKeys are the 0.x report fields a stable report must never
// synthesize (CEM-V1-034). A CLI report over a parseable map carries counts
// (CCP-V0-010), so each caller names it in the exact key set; the MCP preview
// still does not.
var stableReport0xKeys = []string{"verification", "worklist", "policyIssues", "state", "patchSource", "excludedPath", "warnings", "selfModifiedAuthority"}

func envelopeKeys(envelope map[string]any) []string {
	keys := make([]string, 0, len(envelope))
	for key := range envelope {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// requireStableReportEnvelope checks one stable report envelope against
// verify-stable: the same exit, the complete unmodified result, exactly the
// named keys and no 0.x field.
func requireStableReportEnvelope(t *testing.T, name string, exit, wantExit int, envelope, want map[string]any, keys []string) {
	t.Helper()
	if exit != wantExit {
		t.Errorf("%s exit = %d, verify-stable exit = %d", name, exit, wantExit)
	}
	got, _ := envelope["stableVerification"].(map[string]any)
	if len(got) != 23 || !reflect.DeepEqual(got, want) {
		t.Errorf("%s stableVerification differs from verify-stable:\n got %v\nwant %v", name, got, want)
	}
	if envelope["ok"] != (wantExit == 0) || envelope["tool"] != "cem-report" {
		t.Errorf("%s envelope head = ok %v tool %v", name, envelope["ok"], envelope["tool"])
	}
	for _, key := range stableReport0xKeys {
		if _, present := envelope[key]; present {
			t.Errorf("%s synthesized the 0.x field %q for a stable map", name, key)
		}
	}
	sort.Strings(keys)
	if got := envelopeKeys(envelope); !reflect.DeepEqual(got, keys) {
		t.Errorf("%s envelope keys = %v, want %v", name, got, keys)
	}
}

// stableResultDigest recomputes the review's result digest from the result's
// JSON alone: key-sorted, compact, without HTML escaping.
func stableResultDigest(t *testing.T, result map[string]any) string {
	t.Helper()
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

// CEM-V1-034: `cem report --format json` over an exact cem/1.0 map embeds the
// complete verifier result unchanged beside a stable review projection, exits
// with the verifier's code, and never folds into 0.x verification.
func TestCoreStableReportJSONEmbedsVerifierResult(t *testing.T) {
	f := materializeStableRead(t)
	for _, artifacts := range []string{f.artifacts, ""} {
		wantExit, want := runVerifyStable(t, f, artifacts)
		exit, envelope, refusal := runCoreRead(t, f.repo, append(coreStableArguments("report", f, artifacts), "--format", "json")...)
		if refusal != nil {
			t.Fatalf("report refused before the verifier: %v", refusal)
		}
		requireStableReportEnvelope(t, "report json", exit, wantExit, envelope, want, []string{"ok", "mutates", "tool", "review", "stableVerification", "counts"})
		if envelope["mutates"] != false {
			t.Errorf("JSON review mutates = %v", envelope["mutates"])
		}
		review, _ := envelope["review"].(map[string]any)
		if review["profile"] != "cem-stable-review/0" || review["outcome"] != want["outcome"] ||
			review["stableResultSha256"] != stableResultDigest(t, want) || review["mapSha256"] != want["mapSha256"] {
			t.Errorf("review head = %v %v %v %v", review["profile"], review["outcome"], review["stableResultSha256"], review["mapSha256"])
		}
		rows, _ := review["hunks"].([]any)
		declared, _ := want["hunks"].([]any)
		if len(rows) != len(declared) {
			t.Fatalf("review rows = %d, declared hunks = %d", len(rows), len(declared))
		}
		for index, item := range rows {
			row, _ := item.(map[string]any)
			hunk, _ := declared[index].(map[string]any)
			execution, _ := row["testExecution"].(map[string]any)
			if row["id"] != hunk["id"] || row["path"] != hunk["path"] || row["disposition"] != hunk["disposition"] ||
				execution["state"] != "NOT_RUN" {
				t.Errorf("row %d = %v, hunk %v", index, row, hunk)
			}
		}
		if review["recordSetSha256"] == nil {
			t.Error("review has no record-set digest")
		}
		if artifacts == "" || runtime.Version() != f.acceptGo {
			continue
		}
		evidence, _ := rows[0].(map[string]any)["evidence"].([]any)
		basis, _ := evidence[0].(map[string]any)
		if exit != 0 || review["outcome"] != "ACCEPT" || basis["driftState"] != "stable" {
			t.Errorf("accepted report = exit %d outcome %v first evidence %v", exit, review["outcome"], basis)
		}
	}
	if runtime.Version() != f.acceptGo {
		t.Logf("NOT_RUN: report ACCEPT requires %s, running %s; equality with verify-stable still checked", f.acceptGo, runtime.Version())
	}
}

// CEM-V1-035: the Markdown report publishes for every verifier outcome, its
// bytes equal the read-only preview, and both carry the complete result.
func TestCoreStableReportPublishesMarkdown(t *testing.T) {
	f := materializeStableRead(t)
	reportPath := filepath.Join(f.repo, ".git", "corvint", "cem-review.md")
	for _, artifacts := range []string{f.artifacts, ""} {
		wantExit, want := runVerifyStable(t, f, artifacts)
		exit, envelope, refusal := runCoreRead(t, f.repo, coreStableArguments("report", f, artifacts)...)
		if refusal != nil {
			t.Fatalf("report refused before the verifier: %v", refusal)
		}
		requireStableReportEnvelope(t, "report", exit, wantExit, envelope, want, []string{"ok", "mutates", "tool", "report", "recordSetSha256", "stableVerification", "counts"})
		if envelope["mutates"] != true || envelope["report"] != reportPath {
			t.Errorf("report mutates %v path %v, want true %s", envelope["mutates"], envelope["report"], reportPath)
		}
		written, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		markdown := string(written)
		for _, fragment := range []string{"- Spec: `cem/1.0`\n", "- Outcome: `" + want["outcome"].(string) + "`\n", "- Stable result SHA-256: `" + stableResultDigest(t, want) + "`\n", "\n## Hunk review projection\n"} {
			if !strings.Contains(markdown, fragment) {
				t.Errorf("published report lacks %q:\n%s", fragment, markdown)
			}
		}
		if code, _ := want["code"].(string); code != "" && !strings.Contains(markdown, "- Code: `"+code+"`\n") {
			t.Errorf("published report does not name the verifier code %s", code)
		}
		if strings.Contains(markdown, "## Dispositions") || strings.Contains(markdown, "ready-for-ci") {
			t.Error("stable report rendered a 0.x section")
		}
		session, err := workflow.Open(f.repo)
		if err != nil {
			t.Fatal(err)
		}
		options := workflow.ReadOptions{MapPath: "change.cem.json", ExpectedBase: f.expectedBase, Target: f.target, ArtifactRoot: artifacts, ArtifactsGiven: artifacts != ""}
		preview, err := session.Read(context.Background(), workflow.ActionReportPreview, options)
		if err != nil {
			t.Fatal(err)
		}
		if preview["markdown"] != markdown || preview["recordSetSha256"] != envelope["recordSetSha256"] || preview["mutates"] != false {
			t.Error("preview differs from the published report")
		}
		encoded, _ := json.Marshal(preview)
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		requireStableReportEnvelope(t, "preview", wantExit, wantExit, decoded, want, []string{"ok", "mutates", "tool", "markdown", "recordSetSha256", "stableVerification"})
		_, jsonEnvelope, _ := runCoreRead(t, f.repo, append(coreStableArguments("report", f, artifacts), "--format", "json")...)
		review, _ := jsonEnvelope["review"].(map[string]any)
		if review["recordSetSha256"] != envelope["recordSetSha256"] {
			t.Error("published and JSON reviews bind different record sets")
		}
	}
}

// dropStructuralHunks rewrites the fixture map without its structural
// mechanical hunks, so every Go release reaches repository admission instead
// of the verifier's structural runtime refusal.
func dropStructuralHunks(t *testing.T, repo string) {
	t.Helper()
	path := filepath.Join(repo, "change.cem.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(data)
	if err != nil {
		t.Fatal(err)
	}
	kept := document.Change.Hunks[:0]
	for _, hunk := range document.Change.Hunks {
		if hunk.Disposition != "mechanical" || !wire.StructuralReasons[hunk.Reason] {
			kept = append(kept, hunk)
		}
	}
	document.Change.Hunks = kept
	if data, err = wire.EncodeStable(document); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// CEM-V1-035: Markdown publication does not depend on the 0.x repository
// admission. A repository the verifier refuses at stage repository (object
// alternates) still publishes its UNSUPPORTED review to the default and to a
// safe external output. When the worktree's Git directories cannot be located,
// no output path can be checked, so publication fails closed with nothing
// written while JSON still carries the verifier's result.
func TestCoreStableReportPublishesRepositoryRefusals(t *testing.T) {
	f := materializeStableRead(t)
	dropStructuralHunks(t, f.repo)
	alternates := filepath.Join(f.repo, ".git", "objects", "info", "alternates")
	if err := os.MkdirAll(filepath.Dir(alternates), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alternates, []byte(filepath.Join(t.TempDir(), "objects")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantExit, want := runVerifyStable(t, f, f.artifacts)
	if want["outcome"] != "UNSUPPORTED" || want["stage"] != "repository" || want["code"] != "unsupported-object-alternates" {
		t.Fatalf("fixture does not reach the alternates refusal: %v %v %v", want["outcome"], want["stage"], want["code"])
	}
	external, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	external = filepath.Join(external, "review.md")
	for _, c := range []struct{ output, path string }{
		{"", filepath.Join(f.repo, ".git", "corvint", "cem-review.md")},
		{external, external},
	} {
		arguments := coreStableArguments("report", f, f.artifacts)
		if c.output != "" {
			arguments = append(arguments, "--output", c.output)
		}
		exit, envelope, refusal := runCoreRead(t, f.repo, arguments...)
		if refusal != nil {
			t.Fatalf("--output %q: report refused instead of publishing: %v", c.output, refusal)
		}
		requireStableReportEnvelope(t, "report", exit, wantExit, envelope, want, []string{"ok", "mutates", "tool", "report", "recordSetSha256", "stableVerification", "counts"})
		written, err := os.ReadFile(c.path)
		if err != nil || envelope["report"] != c.path {
			t.Fatalf("--output %q: report %v not published at %s: %v", c.output, envelope["report"], c.path, err)
		}
		if !strings.Contains(string(written), "- Outcome: `UNSUPPORTED`\n") || !strings.Contains(string(written), "- Code: `unsupported-object-alternates`\n") {
			t.Errorf("published report does not carry the repository refusal:\n%s", written)
		}
	}

	g := materializeStableRead(t)
	dropStructuralHunks(t, g.repo)
	if err := os.Rename(filepath.Join(g.repo, ".git"), filepath.Join(g.repo, "moved.git")); err != nil {
		t.Fatal(err)
	}
	wantExit, want = runVerifyStable(t, g, g.artifacts)
	if want["stage"] != "repository" {
		t.Fatalf("fixture without a Git directory reached stage %v", want["stage"])
	}
	exit, envelope, _ := runCoreRead(t, g.repo, append(coreStableArguments("report", g, g.artifacts), "--format", "json")...)
	requireStableReportEnvelope(t, "json", exit, wantExit, envelope, want, []string{"ok", "mutates", "tool", "review", "stableVerification", "counts"})
	exit, envelope, refusal := runCoreRead(t, g.repo, append(coreStableArguments("report", g, g.artifacts), "--output", external+".closed")...)
	if exit != 2 || envelope != nil || refusal["code"] != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("unlocatable Git directory = exit %d envelope %v refusal %v, want exit 2 %s", exit, envelope, refusal, cemcode.RepositoryObjectUnavailable)
	}
	if _, err := os.Stat(external + ".closed"); !os.IsNotExist(err) {
		t.Fatalf("a fail-closed report published: %v", err)
	}
}

// CEM-V1-036: a stable report refuses an empty --ocm, --patch, an empty ceiling
// and an output that names the map, with exit 2 and nothing published. A
// supplied flag is refused even when its value is empty; an --ocm naming an
// absent file is map-unavailable (V1-1305); the `--flag=` spelling is not a CEM
// argument and is refused by the argument parser. The refusals
// before the verifier are marked as stable refusals; a retired or unknown spec is an
// unmarked unsupported-spec refusal (CEM-V1-215).
func TestCoreStableReportRefusals(t *testing.T) {
	f := materializeStableRead(t)
	reportPath := filepath.Join(f.repo, ".git", "corvint", "cem-review.md")
	report := func(extra ...string) []string {
		return append(coreStableArguments("report", f, f.artifacts), extra...)
	}
	for _, c := range []struct {
		name      string
		arguments []string
		code      string
		message   string
	}{
		{"absent ocm", report("--ocm", "map.ocm.json"), cemcode.MapUnavailable, ""},
		{"ocm outside root", report("--ocm", "../map.ocm.json"), cemcode.InvalidArguments, "path-traversal"},
		{"empty ocm", report("--ocm", ""), cemcode.InvalidArguments, "cem/1.0 report does not accept --ocm"},
		{"ocm=", report("--ocm="), cemcode.InvalidArguments, "unrecognized arguments: --ocm="},
		{"json empty ocm", report("--format", "json", "--ocm", ""), cemcode.InvalidArguments, "cem/1.0 report does not accept --ocm"},
		{"patch", report("--patch", "x.patch"), cemcode.InvalidArguments, "cem/1.0 report does not accept --patch"},
		{"empty patch", report("--patch", ""), cemcode.InvalidArguments, "cem/1.0 report does not accept --patch"},
		{"patch=", report("--patch="), cemcode.InvalidArguments, "unrecognized arguments: --patch="},
		{"empty ceiling", report("--max-unknown", ""), cemcode.InvalidArguments, "--max-unknown must be a non-negative integer"},
		{"output names map", report("--output", "change.cem.json"), cemcode.InvalidArguments, "report and map paths must differ"},
	} {
		t.Run(c.name, func(t *testing.T) {
			exit, envelope, refusal := runCoreRead(t, f.repo, c.arguments...)
			if exit != 2 || envelope != nil || refusal["code"] != c.code {
				t.Fatalf("exit %d envelope %v refusal %v, want exit 2 code %s", exit, envelope, refusal, c.code)
			}
			if message, _ := refusal["error"].(string); !strings.Contains(message, c.message) {
				t.Fatalf("refusal %q does not name %q", message, c.message)
			}
			if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
				t.Fatalf("a refused report published %s: %v", reportPath, err)
			}
		})
	}
	session, err := workflow.Open(f.repo)
	if err != nil {
		t.Fatal(err)
	}
	limit := 0
	_, err = session.Read(context.Background(), workflow.ActionReportPreview, workflow.ReadOptions{
		MapPath: "change.cem.json", ExpectedBase: f.expectedBase, Target: f.target,
		Limits: workflow.PolicyLimits{MaxUnknown: &limit},
	})
	if cemcode.CodeOf(err) != cemcode.InvalidArguments || !workflow.IsStableRefusal(err) {
		t.Fatalf("preview ceiling refusal = %v (stable %v)", err, workflow.IsStableRefusal(err))
	}
	for name, document := range map[string]string{"other.json": `{"spec":"cem/9"}`, "legacy.json": `{"spec":"cem/0.3","hunks":[]}`} {
		if err := os.WriteFile(filepath.Join(f.repo, name), []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := session.Read(context.Background(), workflow.ActionReportPreview, workflow.ReadOptions{
			MapPath: name, ExpectedBase: f.expectedBase, Target: f.target, ArtifactRoot: f.artifacts, ArtifactsGiven: true,
		})
		if err == nil || workflow.IsStableRefusal(err) {
			t.Fatalf("0.x preview %s = %v, want an unmarked refusal", name, err)
		}
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
			t.Fatalf("0.x preview %s = %v, want unsupported-spec", name, err)
		}
	}
}

// TestV11305StableReportOCMWithoutVerifier (V1-1305): a stable report whose
// entry point installs no stable OCM verifier keeps the obligations
// NOT_PRODUCED with reason ocm-verifier-unavailable, joins no obligation and
// is not ok; the same report without --ocm keeps its projection unchanged.
func TestV11305StableReportOCMWithoutVerifier(t *testing.T) {
	f := materializeStableRead(t)
	if err := os.WriteFile(filepath.Join(f.repo, "map.ocm.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	arguments := append(coreStableArguments("report", f, f.artifacts), "--format", "json")
	exit, envelope, refusal := runCoreRead(t, f.repo, append(arguments, "--ocm", "map.ocm.json")...)
	if exit != 1 || refusal != nil || envelope["ok"] != false {
		t.Fatalf("exit %d ok %v refusal %v", exit, envelope["ok"], refusal)
	}
	review := envelope["review"].(map[string]any)
	ocm := review["ocm"].(map[string]any)
	if ocm["reason"] != "ocm-verifier-unavailable" || review["ocmValid"] != false || len(review["unmappedObligations"].([]any)) != 0 {
		t.Fatalf("review without verifier: %v", review)
	}
	exit, plain, _ := runCoreRead(t, f.repo, arguments...)
	plainReview := plain["review"].(map[string]any)
	if exit != 0 || plainReview["ocmValid"] != nil || plainReview["recordSetSha256"] == review["recordSetSha256"] {
		t.Fatalf("plain report: exit %d review %v", exit, plainReview)
	}
}
