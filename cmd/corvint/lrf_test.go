package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func newLRFFixture(t *testing.T, spec string) lrfFixture {
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
	mapPath := "change.cem.json"
	if spec == wire.Spec02 {
		mapPath = ".corvint/change.cem.json"
		code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
		if code != 0 {
			t.Fatalf("prepare: %s", stderr)
		}
	} else {
		code, _, stderr := runCLI(t, "--root", root, "cem", "begin", "--patch", patchPath, "--output", mapPath, "--base", base)
		if code != 0 {
			t.Fatalf("begin: %s", stderr)
		}
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

func TestLRFAcceptedContextRows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		spec       string
		args       func(lrfFixture) []string
		wantSource string
		wantTarget bool
		wantOCM    bool
	}{
		{"cem-01-explicit", wire.Spec01, func(f lrfFixture) []string { return []string{"--patch", f.patchPath, "--target", f.target} }, "explicit-out-of-band", true, false},
		{"cem-01-default", wire.Spec01, func(f lrfFixture) []string {
			gitDir := cemGit(t, f.root, "rev-parse", "--absolute-git-dir")
			if err := os.MkdirAll(filepath.Join(gitDir, "corvint"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(gitDir, "corvint", "change.patch"), f.patch, 0o600); err != nil {
				t.Fatal(err)
			}
			return nil
		}, "default-out-of-band", false, false},
		{"cem-02", wire.Spec02, func(f lrfFixture) []string { return []string{"--expected-base", f.base, "--target", f.target} }, "canonical-derived", true, false},
		{"ocm-cem-02", wire.Spec02, func(f lrfFixture) []string {
			writeOCM(t, f, "change.ocm.json", true)
			return []string{"--ocm", "change.ocm.json", "--expected-base", f.base, "--target", f.target}
		}, "canonical-derived", true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newLRFFixture(t, test.spec)
			args := []string{"--root", fixture.root, "lrf", "--cem", fixture.mapPath}
			args = append(args, test.args(fixture)...)
			code, stdout, stderr := runCLI(t, args...)
			if code != 0 || stderr != "" {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			var result struct {
				Inputs  []any      `json:"inputs"`
				Results [][]string `json:"results"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			target := any(nil)
			if test.wantTarget {
				target = fixture.target
			}
			exclusion := any(nil)
			if test.spec == wire.Spec02 {
				exclusion = wire.ExcludedCEMPath
			}
			wantInputs := []any{
				test.spec, digestBytes(fixture.cemRaw), test.wantSource, fixture.base,
				target, digestBytes(fixture.patch), exclusion,
				nil, nil, nil, nil, nil, nil, nil,
			}
			if test.wantOCM {
				copy(wantInputs[7:], expectedOCMInputs(t, fixture.root, "change.ocm.json"))
			}
			if !reflect.DeepEqual(result.Inputs, wantInputs) {
				t.Fatalf("inputs=%#v\nwant=%#v", result.Inputs, wantInputs)
			}
			if test.wantOCM && !hasResultKind(result.Results, "ocm-hunk") {
				t.Fatalf("linked OCM projection missing: %#v", result.Results)
			}
		})
	}
}

// A cem/0.1 context refuses any OCM before reading or verifying it, so an
// invalid OCM cannot outrank the more specific context diagnosis.
func TestLRFOCMPlusCEM01IsUnsupportedBeforeOCMVerification(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec01)
	writeOCM(t, fixture, "change.ocm.json", true)
	if err := os.WriteFile(filepath.Join(fixture.root, "invalid.ocm.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ocm := range []string{"change.ocm.json", "invalid.ocm.json"} {
		code, stdout, stderr := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath,
			"--patch", fixture.patchPath, "--target", fixture.target, "--ocm", ocm)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "unsupported-lrf-context"`) {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q", ocm, code, stdout, stderr)
		}
	}
}

func TestLRFStructuralFailurePreservesVerifierCodeAndNoStdout(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec01)
	corrupt := bytes.Replace(fixture.cemRaw, []byte(`"patchSha256":`), []byte(`"surplus":true,"patchSha256":`), 1)
	if err := os.WriteFile(filepath.Join(fixture.root, fixture.mapPath), corrupt, 0o644); err != nil {
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

func TestLRFRejectsFabricatedOCMClaimSelector(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec02)
	writeOCM(t, fixture, "change.ocm.json", true)
	path := filepath.Join(fixture.root, "change.ocm.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	claim := document["claims"].([]any)[0].(map[string]any)
	claim["selector"] = "test:TestWidget/case:forged-selector"
	body := make(map[string]any, len(claim)-1)
	for key, value := range claim {
		if key != "id" {
			body[key] = value
		}
	}
	encodedBody, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	claim["id"] = "claim:sha256:" + digestBytes(encodedBody)
	document["obligations"].([]any)[0].(map[string]any)["claimIds"] = []any{claim["id"]}
	forged, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(forged, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath,
		"--ocm", "change.ocm.json", "--expected-base", fixture.base, "--target", fixture.target)
	if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "claim-not-reextractable"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestLRFRepositoryFixtureProducesCanonicalContext(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec02)
	args := []string{"--root", fixture.root, "lrf", "--cem", fixture.mapPath,
		"--expected-base", fixture.base, "--target", fixture.target}
	candidate := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, args...)...)
	candidate.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	candidateResult := execute(t, candidate)
	if candidateResult.exit != 0 || len(candidateResult.stderr) != 0 || len(candidateResult.stdout) == 0 ||
		!bytes.Contains(candidateResult.stdout, []byte(`"profile":"lrf/0"`)) ||
		!bytes.Contains(candidateResult.stdout, []byte(`"cem/0.2"`)) {
		t.Fatalf("candidate result=%#v", candidateResult)
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
