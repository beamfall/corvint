package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// CEM-PILOT-028..030: both renderings bind the same retained records, a JSON
// review writes nothing, and absent witnesses cannot become passing tests.
func TestReviewProjectionParityAndReadOnly(t *testing.T) {
	t.Run("CEM-PILOT-028 CEM-PILOT-030 CEM-PILOT-031 immutable projection parity and witness gaps", func(t *testing.T) {
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		citeAll(t, root)
		before, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		options := ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Format: "json"}
		result, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		if result["mutates"] != false || result["ok"] != true {
			t.Fatalf("JSON verdict: %v", result)
		}
		projection := result["review"].(map[string]any)
		digest := projection["recordSetSha256"]
		delete(projection, "recordSetSha256")
		if reviewDigest(projection) != digest {
			t.Fatal("record set digest does not bind projected records")
		}
		projection["recordSetSha256"] = digest
		if got := projection["ocm"].(map[string]any)["reason"]; got != "ocm-not-supplied" {
			t.Fatalf("missing OCM reason: %v", got)
		}
		rows := projection["hunks"].([]any)
		if len(rows) != 2 {
			t.Fatalf("hunk denominator: %d", len(rows))
		}
		for _, item := range rows {
			row := item.(map[string]any)
			if row["testExecution"].(map[string]any)["state"] != "NOT_RUN" {
				t.Fatal("unrun test upgraded")
			}
			if row["testObservation"].(map[string]any)["reason"] != "no-coverage-witness" {
				t.Fatal("missing witness omitted")
			}
		}
		after, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		if string(before) != string(after) {
			t.Fatal("JSON review changed map bytes")
		}
		gitDir := gitCmd(t, root, "rev-parse", "--git-dir")
		if _, err := os.Stat(filepath.Join(root, gitDir, defaultReportRelative)); !os.IsNotExist(err) {
			t.Fatalf("JSON wrote report: %v", err)
		}
		options.Format = "markdown"
		human, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		if human["recordSetSha256"] != digest {
			t.Fatal("human and JSON record digests differ")
		}
		text, err := os.ReadFile(human["report"].(string))
		if err != nil || !strings.Contains(string(text), digest.(string)) {
			t.Fatalf("human digest absent: %v", err)
		}
		options.Format, options.Output = "json", "forbidden.json"
		if _, err := openSession(t, root).Read(ctx(), "report", options); err == nil {
			t.Fatal("JSON output option accepted")
		}
	})
}

// CEM-PILOT-028: an invalid map cannot shrink the derived-patch denominator.
func TestReviewProjectionInvalidMapRetainsUnmappedHunks(t *testing.T) {
	t.Run("CEM-PILOT-028 invalid map retains derived hunk denominator", func(t *testing.T) {
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		document := readMapAt(t, root, wire.ExcludedCEMPath)
		document.Hunks = document.Hunks[:1]
		writeFile(t, root, wire.ExcludedCEMPath, string(encodeMap(document)))
		result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Format: "json"})
		if err != nil {
			t.Fatal(err)
		}
		projection := result["review"].(map[string]any)
		if result["ok"] != false || projection["valid"] != false {
			t.Fatal("invalid CEM upgraded")
		}
		rows := projection["hunks"].([]any)
		if len(rows) != 2 || rows[1].(map[string]any)["reason"] != "hunk-not-mapped" {
			t.Fatalf("invalid denominator: %v", rows)
		}
	})
}

// CEM-PILOT-029,031: only explicit verified hunk IDs join; unknown obligations
// stay scope-wide and a failed OCM contributes no apparent linked rows.
func TestReviewProjectionOCMJoinAndInvalidRefusal(t *testing.T) {
	t.Run("CEM-PILOT-029 explicit callback joins and invalid refusal", func(t *testing.T) {
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		citeAll(t, root)
		doc := readMapAt(t, root, wire.ExcludedCEMPath)
		raw, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		valid := true
		reader := func(_ context.Context, gotRoot, path string, bytes []byte, gotBase, gotTarget string) (map[string]any, error) {
			if gotRoot != root || path != "intent.ocm.json" || string(bytes) != string(raw) || gotBase != base || gotTarget != target {
				t.Fatal("OCM not bound to retained CEM and endpoints")
			}
			// Changing the input path after the retained read cannot alter these records.
			writeFile(t, root, wire.ExcludedCEMPath, "changed during callback")
			return map[string]any{"valid": valid, "state": "ready-for-review", "obligations": []any{
				map[string]any{"id": "FIX-V0-001", "disposition": "linked", "hunkIds": []any{doc.Hunks[0].ID}, "claimIds": []any{"claim-id"}},
				map[string]any{"id": "FIX-V0-002", "disposition": "unknown", "reason": "no-test-claim", "hunkIds": []any{}, "claimIds": []any{}},
			}, "claims": []any{}}, nil
		}
		options := ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, Format: "json", OCMPath: "intent.ocm.json", ReadOCM: reader}
		result, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		projection := result["review"].(map[string]any)
		rows := projection["hunks"].([]any)
		if len(rows[0].(map[string]any)["obligations"].([]any)) != 1 || len(rows[1].(map[string]any)["obligations"].([]any)) != 0 || len(projection["unmappedObligations"].([]any)) != 1 {
			t.Fatal("OCM join inferred or unknown lost")
		}
		writeFile(t, root, wire.ExcludedCEMPath, string(raw))
		valid = false
		result, err = openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		if result["ok"] != false || len(result["review"].(map[string]any)["hunks"].([]any)[0].(map[string]any)["obligations"].([]any)) != 0 {
			t.Fatal("invalid OCM admitted")
		}
	})
}

// CEM-PILOT-030: hostile repository strings remain inert in Markdown and JSON.
func TestReviewProjectionHostileText(t *testing.T) {
	t.Run("CEM-PILOT-030 hostile strings remain inert in both renderings", func(t *testing.T) {
		hostile := "</script>\n# forged\n[x](https://example.invalid) `"
		projection := map[string]any{"recordSetSha256": strings.Repeat("a", 64), "mapSha256": strings.Repeat("b", 64), "ocm": map[string]any{"reason": hostile}, "hunks": []any{
			map[string]any{"ordinal": 1, "path": hostile, "id": "hunk:sha256:abc", "disposition": "unknown", "reason": hostile, "obligations": []any{}},
		}, "unmappedObligations": []any{}}
		text := renderReviewProjection(projection)
		if strings.Contains(text, "\n# forged") || !strings.Contains(text, mdreport.CodeSpan(hostile)) {
			t.Fatalf("hostile Markdown:\n%s", text)
		}
		raw, err := json.Marshal(projection)
		if err != nil || strings.Contains(string(raw), "</script>") || strings.Contains(string(raw), "\n# forged") {
			t.Fatal("hostile JSON did not escape content")
		}
	})
}

// CEM-PILOT-029: the native OCM adapter verifies real producer bytes, retains
// unknowns, and ignores a replaced CEM path. Linked adapter qualification is
// retained separately on the final integrated requirement-bearing OCM.
func TestReviewProjectionNativeOCMAdapterUnknownOnly(t *testing.T) {
	t.Run("CEM-PILOT-029 native unknown obligations and retained CEM bytes", func(t *testing.T) {
		root := t.TempDir()
		gitCmd(t, root, "init", "-q", "-b", "main")
		writeFile(t, root, "intent.md", "# Intent\n\n## Requirements\n\n- `REV-TEST-001`: show exact review evidence.\n- `REV-TEST-002`: retain absent execution.\n\n## Non-goals\n")
		writeFile(t, root, "widget.go", "package widget\nfunc Frob() int { return 1 }\n")
		writeFile(t, root, "widget_test.go", "package widget\nimport \"testing\"\nfunc TestWidget(t *testing.T) {\n t.Run(\"REV-TEST-001\", func(t *testing.T) {})\n}\n")
		gitCmd(t, root, "add", ".")
		gitCmd(t, root, "commit", "-qm", "base")
		base := gitCmd(t, root, "rev-parse", "HEAD")
		writeFile(t, root, "widget.go", "package widget\nfunc Frob() int { return 2 }\n")
		gitCmd(t, root, "add", ".")
		gitCmd(t, root, "commit", "-qm", "target")
		target := gitCmd(t, root, "rev-parse", "HEAD")
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		if _, err := openSession(t, root).Cite(ctx(), CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "intent.md", Lines: "5:5", Relation: "specification"}); err != nil {
			t.Fatal(err)
		}
		path := ".corvint/change.ocm.json"
		if _, err := lrfrepo.PrepareOCM(ctx(), root, lrfrepo.PrepareOptions{MapPath: path, CEMPath: wire.ExcludedCEMPath, IntentPath: "intent.md", ExpectedBase: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
		// At this point CEMRaw is the only admitted CEM input to the adapter.
		writeFile(t, root, wire.ExcludedCEMPath, "replaced input path")
		projection, err := lrfrepo.ReadOCMReview(ctx(), root, path, raw, base, target)
		if err != nil {
			t.Fatal(err)
		}
		if projection["valid"] != true || len(projection["obligations"].([]any)) != 2 || len(projection["claims"].([]any)) != 0 {
			t.Fatalf("native OCM projection: %v", projection)
		}
		if projection["testExecution"].(map[string]any)["state"] != "NOT_RUN" {
			t.Fatal("native adapter invented test execution")
		}
		for _, item := range projection["obligations"].([]any) {
			if item.(map[string]any)["disposition"] != "unknown" {
				t.Fatal("native unknown became linked")
			}
		}
		invalid, err := lrfrepo.ReadOCMReview(ctx(), root, path, raw, target, target)
		if err != nil {
			t.Fatal(err)
		}
		if invalid["valid"] != false || len(invalid["obligations"].([]any)) != 0 {
			t.Fatalf("wrong-base adapter admitted joins: %v", invalid)
		}
	})
}
