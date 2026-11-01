package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

type lrfFixture struct {
	root, base, target, mapPath, patchPath string
	cemRaw, patch                          []byte
	hunkID                                 string
}

func newLRFFixture(t *testing.T) lrfFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "docs/rule.txt", "widget renderer authority\n")
	cemWrite(t, root, "docs/intent.md", "# Intent\n\n## Requirements\n\n- `LRF-CLI-001`: Widget renderer changes remain relevant.\n\n## Next\n")
	cemWrite(t, root, "src/app.txt", "alpha\n")
	cemWrite(t, root, "tests/widget_test.go", "package tests\n\nimport \"testing\"\n\nfunc TestWidget(t *testing.T) {\n\t_ = []struct{ name string }{{name: \"LRF-CLI-001 widget renderer\"}}\n}\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "src/app.txt", "alpha\nwidget renderer\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "target")
	target := cemGit(t, root, "rev-parse", "HEAD")
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	patchBytes, err := repository.CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	patchPath := "change.patch"
	if err := os.WriteFile(filepath.Join(root, patchPath), patchBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// CEM-V1-214: the fixture map is cem/1.0, produced by the stable prepare.
	mapPath := ".corvint/change.cem.json"
	if code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target); code != 0 {
		t.Fatalf("prepare: %s", stderr)
	}
	code, _, stderr := runCLI(t, "--root", root, "cem", "cite", "--map", mapPath, "--hunk", "1", "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification")
	if code != 0 {
		t.Fatalf("cite: %s", stderr)
	}
	cemRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mapPath)))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hunks []struct {
			ID string `json:"id"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(cemRaw, &document); err != nil || len(document.Hunks) != 1 {
		t.Fatalf("CEM map: %v %#v", err, document)
	}
	return lrfFixture{root: root, base: base, target: target, mapPath: mapPath, patchPath: patchPath, cemRaw: cemRaw, patch: patchBytes, hunkID: document.Hunks[0].ID}
}

// CEM-V1-210, LRF-V0-001: lrf/0 evaluates no map. A retired cem/0.x map
// refuses unsupported-spec with the recovery line, and a cem/1.0 map refuses
// unsupported-spec, both before any explicit, missing or default patch or any
// OCM is read (decision 0525 §1).
func TestCEMV1210LRFRefusesEveryMapBeforeAnyInput(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t)
	path := filepath.Join(fixture.root, filepath.FromSlash(fixture.mapPath))
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3", wire.StableSpec} {
		raw := bytes.Replace(fixture.cemRaw, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"`+spec+`"`), 1)
		if spec != wire.StableSpec && bytes.Equal(raw, fixture.cemRaw) {
			t.Fatal("fixture map names no cem/1.0 spec")
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		for name, extra := range map[string][]string{
			"no-patch":      nil,
			"missing-patch": {"--patch", "absent.patch"},
			"patch":         {"--patch", fixture.patchPath, "--target", fixture.target},
			"missing-ocm":   {"--ocm", "absent.ocm.json", "--expected-base", fixture.base, "--target", fixture.target},
		} {
			args := append([]string{"--root", fixture.root, "lrf", "--cem", fixture.mapPath}, extra...)
			code, stdout, stderr := runCLI(t, args...)
			want := "lrf/0 evaluates no cem/1.0 map"
			if spec != wire.StableSpec {
				want = wire.RetiredSpecFix
			}
			if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "unsupported-spec"`) || !strings.Contains(stderr, want) {
				t.Fatalf("%s %s: exit=%d stdout=%q stderr=%q", spec, name, code, stdout, stderr)
			}
		}
	}
}

func TestLRFStructuralFailurePreservesVerifierCodeAndNoStdout(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t)
	corrupt := bytes.Replace(fixture.cemRaw, []byte(`"patchSha256":`), []byte(`"surplus":true,"patchSha256":`), 1)
	if err := os.WriteFile(filepath.Join(fixture.root, filepath.FromSlash(fixture.mapPath)), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath, "--patch", fixture.patchPath)
	if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "unknown-field"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestLRFCLIArgumentErrorsKeepInvalidArguments(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runCLI(t, "lrf", "--bogus", "x")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-arguments"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func expectedOCMInputs(t *testing.T, root, path string) []any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	intent := document["intentScope"].(map[string]any)
	span := intent["span"].(map[string]any)
	return []any{
		document["spec"], digestBytes(raw), intent["path"], intent["blobOid"],
		span["start"], span["end"], intent["spanSha256"],
	}
}

func writeOCM(t *testing.T, fixture lrfFixture, path string, linked bool) {
	t.Helper()
	intent := []byte(cemGit(t, fixture.root, "show", fixture.target+":docs/intent.md"))
	intent = append(intent, '\n')
	start := bytes.Index(intent, []byte("## Requirements"))
	endOffset := bytes.Index(intent[start+1:], []byte("## Next"))
	end := start + 1 + endOffset
	intentOID := cemGit(t, fixture.root, "rev-parse", fixture.target+":docs/intent.md")
	claimRows := []any{}
	hunkIDs := []string{}
	claimIDs := []string{}
	if linked {
		testBlob := []byte(cemGit(t, fixture.root, "show", fixture.target+":tests/widget_test.go"))
		testBlob = append(testBlob, '\n')
		anchor := []byte("LRF-CLI-001 widget renderer")
		claimStart := bytes.Index(testBlob, anchor)
		claimEnd := claimStart + len(anchor)
		body := map[string]any{
			"extractor": "corvint-test-claim/1", "path": "tests/widget_test.go",
			"blobOid":  cemGit(t, fixture.root, "rev-parse", fixture.target+":tests/widget_test.go"),
			"selector": "test:TestWidget/case:lrf-cli-widget-renderer",
			"span":     map[string]any{"start": claimStart, "end": claimEnd}, "spanSha256": digestBytes(anchor),
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		claim := make(map[string]any, len(body)+1)
		for key, value := range body {
			claim[key] = value
		}
		claim["id"] = "claim:sha256:" + digestBytes(encoded)
		claimRows = []any{claim}
		hunkIDs = []string{fixture.hunkID}
		claimIDs = []string{claim["id"].(string)}
	}
	document := map[string]any{
		"spec": "ocm/0.1-experimental", "targetRevision": fixture.target,
		"intentScope": map[string]any{
			"path": "docs/intent.md", "blobOid": intentOID,
			"span": map[string]any{"start": start, "end": end}, "spanSha256": digestBytes(intent[start:end]),
		},
		"cem":    map[string]any{"mapSha256": digestBytes(fixture.cemRaw), "patchSha256": digestBytes(fixture.patch)},
		"claims": claimRows,
		"obligations": []any{map[string]any{
			"id": "LRF-CLI-001", "disposition": map[bool]string{true: "linked", false: "unknown"}[linked],
			"reason":  map[bool]string{true: "change-and-test-linked", false: "unassessed"}[linked],
			"hunkIds": hunkIDs, "claimIds": claimIDs,
		}},
	}
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(fixture.root, path), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasResultKind(results [][]string, kind string) bool {
	for _, row := range results {
		if len(row) > 0 && row[0] == kind {
			return true
		}
	}
	return false
}

// CEM-V1-955 (V1-3531): an lrf --cem map outside the repository root is refused
// with the rule, the resolved path, the root and the stable reader that accepts it.
func TestLRFOutsideRootRefusalNamesRulePathAndRoot(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t)
	outside := filepath.Join(t.TempDir(), "map.cem.json")
	if err := os.WriteFile(outside, fixture.cemRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "--root", fixture.root, "lrf", "--cem", outside)
	if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-arguments"`) ||
		!strings.Contains(stderr, "must remain inside repository root") || !strings.Contains(stderr, outside) ||
		!strings.Contains(stderr, root) || !strings.Contains(stderr, "corvint cem verify --map") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
