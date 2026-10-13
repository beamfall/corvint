package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/wire"
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

// CEM-PILOT-028 MCPV0-025: zero derived hunks cannot make an invalid map valid.
func TestReviewProjectionEmptyCanonicalDiffRetainsInvalidMap(t *testing.T) {
	t.Run("CEM-PILOT-028 MCPV0-025 empty derived denominator", func(t *testing.T) {
		root, base, target := makeRepo(t)
		if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		mapPath := filepath.Join(root, wire.ExcludedCEMPath)
		raw, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		doc["baseRevision"] = target
		raw, err = json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mapPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		options := ReadOptions{MapPath: wire.ExcludedCEMPath, ExpectedBase: target, Target: target, Format: "json"}
		result, err := openSession(t, root).Read(ctx(), "report", options)
		if err != nil {
			t.Fatal(err)
		}
		review := result["review"].(map[string]any)
		if result["ok"] != false || review["valid"] != false || len(review["hunks"].([]any)) != 0 || len(review["surplusMapHunks"].([]any)) != 2 {
			t.Fatalf("invalid empty diff upgraded or lost surplus: %v", result)
		}
		if review["patchSha256"] != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
			t.Fatal("empty patch digest lost")
		}
		// CEM-V1-210: the MCP preview is a consumer route; a retired 0.x map
		// refuses there with unsupported-spec and the fix line.
		if _, err := openSession(t, root).Read(ctx(), ActionReportPreview, options); cemcode.CodeOf(err) != cemcode.UnsupportedSpec ||
			!strings.Contains(err.Error(), "fix: "+wire.RetiredSpecFix) {
			t.Fatalf("0.x preview admitted: %v", err)
		}
		after, err := os.ReadFile(mapPath)
		if err != nil || string(after) != string(raw) {
			t.Fatal("review changed map")
		}
		// The exception belongs only to canonical derivation, not caller bytes.
		doc["spec"] = "cem/0.1"
		delete(doc, "excludedPath")
		legacy, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, wire.ExcludedCEMPath, string(legacy))
		writeFile(t, root, "empty.patch", "")
		_, err = openSession(t, root).Read(ctx(), "report", ReadOptions{MapPath: wire.ExcludedCEMPath, PatchPath: "empty.patch", PatchGiven: true, Format: "json"})
		if cemcode.CodeOf(err) != cemcode.BinaryPatch {
			t.Fatalf("legacy empty patch parser changed: %v", err)
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
		projection := map[string]any{"recordSetSha256": strings.Repeat("a", 64), "mapSha256": strings.Repeat("b", 64), "ocm": map[string]any{
			"reason": hostile, "valid": false, "state": hostile,
			"verification": map[string]any{"issues": []any{map[string]any{"code": hostile, "message": hostile}}},
		}, "hunks": []any{
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
