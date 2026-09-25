package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func newOCMReadFixture(t *testing.T) lrfFixture {
	t.Helper()
	fixture := newLRFFixture(t, wire.Spec02)
	writeOCM(t, fixture, "change.ocm.json", false)
	return fixture
}

func ocmReadArguments(fixture lrfFixture, action string, extra ...string) []string {
	arguments := []string{
		"--root", fixture.root, "ocm", action,
		"--map", "change.ocm.json", "--cem", fixture.mapPath,
		"--expected-base", fixture.base, "--target", fixture.target,
	}
	return append(arguments, extra...)
}

func runOCMProcess(t *testing.T, arguments []string) processResult {
	t.Helper()
	return execute(t, candidateCommand(arguments...))
}

func TestDogfoodOCMArgumentErrorsAreInvalidArguments(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"dogfood-ocm"},
		{"dogfood-ocm", "unknown"},
		{"dogfood-ocm", "status", "--expected-base", "base"},
	} {
		code, stdout, stderr := runCLI(t, arguments...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-arguments"`) {
			t.Errorf("arguments=%q code=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
		}
	}
}

func TestDogfoodOCMLinkedWorktreePreservesStandaloneVerdict(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	cemGit(t, fixture.root, "add", fixture.mapPath)
	cemGit(t, fixture.root, "commit", "-qm", "CEM sidecar")
	fixture.target = cemGit(t, fixture.root, "rev-parse", "HEAD")
	linked := filepath.Join(t.TempDir(), "linked")
	cemGit(t, fixture.root, "worktree", "add", "--detach", linked, fixture.target)
	fixture.root = linked
	info, err := os.Stat(filepath.Join(linked, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("linked worktree gitfile: %v, %v", info, err)
	}
	mapPath := ".corvint/change.ocm.001.json"
	writeOCM(t, fixture, mapPath, false)
	cemWrite(t, linked, ".corvint/change.ocm-intents", "docs/intent.md\n")
	raw, err := os.ReadFile(filepath.Join(linked, mapPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, failureKind := range []string{"", "noncanonical-map", "repository-object-unavailable"} {
		switch failureKind {
		case "noncanonical-map":
			cemWrite(t, linked, mapPath, " "+string(raw))
		case "repository-object-unavailable":
			cemWrite(t, linked, mapPath, string(raw))
			cemWrite(t, linked, ".git", "gitdir: missing-admin\n")
		}
		standalone := candidateCommand("ocm", "status", "--map", mapPath, "--cem", fixture.mapPath,
			"--expected-base", fixture.base, "--target", fixture.target)
		standalone.Dir = linked
		checked := execute(t, standalone)
		aggregate := candidateCommand("dogfood-ocm", "status", "--expected-base", fixture.base, "--target", fixture.target)
		aggregate.Dir = linked
		result := execute(t, aggregate)
		if failureKind == "" {
			if checked.exit != 0 || result.exit != 0 {
				t.Fatalf("standalone=%+v aggregate=%+v", checked, result)
			}
			continue
		}
		var verdict struct {
			Verification struct {
				Issues []struct{ Code, Message string }
			}
		}
		var failure struct{ Code, Error string }
		if err := json.Unmarshal(checked.stdout, &verdict); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(result.stderr, &failure); err != nil {
			t.Fatal(err)
		}
		if checked.exit != 1 || result.exit != 2 || len(verdict.Verification.Issues) != 1 {
			t.Fatalf("standalone=%+v aggregate=%+v", checked, result)
		}
		issue := verdict.Verification.Issues[0]
		message := issue.Message
		if failureKind == "repository-object-unavailable" {
			message = "linked worktree Git directory does not resolve"
		}
		if failure.Code != failureKind || failure.Code != issue.Code || !strings.Contains(failure.Error, message) || !strings.Contains(failure.Error, mapPath) {
			t.Fatalf("aggregate failure=%+v, standalone issue=%+v", failure, issue)
		}
	}
}

func newOCMLinkValidationFixture(t *testing.T, anchor string) lrfFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "docs/rule.txt", "task mutation authority\n")
	cemWrite(t, root, "docs/intent.md", "# Intent\n\n## Requirements\n\n- `TM-V0-008`: Rejected task mutations leave no effects.\n\n## Next\n")
	cemWrite(t, root, "src/app.txt", "before\n")
	cemWrite(t, root, "tests/ocm_link_test.go", fmt.Sprintf("package fixture\n\nimport \"testing\"\n\nfunc TestOCMLinkFixture(t *testing.T) {\n\tt.Run(%q, func(t *testing.T) {})\n}\n", anchor))
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "src/app.txt", "after\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "target")
	target := cemGit(t, root, "rev-parse", "HEAD")
	code, _, stderr := runCLI(t, "--root", root, "cem", "prepare", "--base", base, "--target", target)
	if code != 0 {
		t.Fatalf("prepare CEM: %s", stderr)
	}
	code, _, stderr = runCLI(t, "--root", root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification")
	if code != 0 {
		t.Fatalf("cite CEM: %s", stderr)
	}
	cemRaw, err := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runCLI(t, "--root", root, "ocm", "prepare", "--map", "change.ocm.json", "--cem", ".corvint/change.cem.json", "--intent", "docs/intent.md", "--expected-base", base, "--target", target)
	if code != 0 {
		t.Fatalf("prepare OCM: %s", stderr)
	}
	return lrfFixture{root: root, base: base, target: target, mapPath: ".corvint/change.cem.json", cemRaw: cemRaw}
}

// ocmCaseClaimSelector is the fixture's subtest claim selector for an anchor:
// the lower-cased letter-led words of the anchor joined by "-", as the Go
// extractor derives it. Claim ids sort by digest, so the subtest's ordinal
// position is not fixed and the selector is passed instead.
func ocmCaseClaimSelector(anchor string) string {
	var words []string
	for _, word := range regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`).FindAllString(anchor, -1) {
		words = append(words, strings.ToLower(word))
	}
	return "test:TestOCMLinkFixture/case:" + strings.Join(words, "-")
}

func ocmLinkValidationArguments(fixture lrfFixture, anchor, output string) []string {
	return []string{
		"--root", fixture.root, "ocm", "link", "--map", "change.ocm.json", "--cem", fixture.mapPath,
		"--obligation", "TM-V0-008", "--hunk", "1", "--test-path", "tests/ocm_link_test.go",
		"--claim", ocmCaseClaimSelector(anchor), "--expected-base", fixture.base,
		"--target", fixture.target, "--output", output,
	}
}

func TestOCMLinkRejectsInvalidReaderVerdict(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"digest", "target"} {
		for _, destination := range []string{"change.ocm.json", "existing.ocm.json", "absent/nested/change.ocm.json"} {
			t.Run("OCM-V0-006 "+mutation+" "+destination, func(t *testing.T) {
				fixture := newOCMLinkValidationFixture(t, "TM-V0-008 exact anchor")
				mutateOCMLinkBinding(t, fixture, mutation)
				cemWrite(t, fixture.root, "existing.ocm.json", "existing destination\n")
				if err := os.Chmod(filepath.Join(fixture.root, "existing.ocm.json"), 0600); err != nil {
					t.Fatal(err)
				}
				assertOCMLinkBindingRefusal(t, fixture, ocmLinkValidationArguments(fixture, "TM-V0-008 exact anchor", destination))
			})
		}
	}
	t.Run("GPK-V0-008 binding refusal precedes invalid claim and absent output", func(t *testing.T) {
		fixture := newOCMLinkValidationFixture(t, "TM-V0-008 exact anchor")
		mutateOCMLinkBinding(t, fixture, "digest")
		arguments := ocmLinkValidationArguments(fixture, "TM-V0-008 exact anchor", "absent/nested/change.ocm.json")
		for i, arg := range arguments {
			if arg == "--obligation" || arg == "--claim" || arg == "--test-path" {
				arguments[i+1] = "missing"
			}
		}
		assertOCMLinkBindingRefusal(t, fixture, arguments)
	})
	t.Run("OCM-V0-007 noncanonical refusal remains unchanged", func(t *testing.T) {
		fixture := newOCMLinkValidationFixture(t, "TM-V0-008 exact anchor")
		path := filepath.Join(fixture.root, "change.ocm.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append([]byte(" "), raw...), 0600); err != nil {
			t.Fatal(err)
		}
		before := repositoryBytesDigest(t, fixture.root)
		candidate := execute(t, candidateCommand(ocmLinkValidationArguments(fixture, "TM-V0-008 exact anchor", "change.ocm.json")...))
		want := "{\"code\": \"noncanonical-map\", \"error\": \"OCM bytes are not canonical\", \"ok\": false}\n"
		if candidate.exit != 2 || len(candidate.stdout) != 0 || string(candidate.stderr) != want {
			t.Fatalf("noncanonical refusal changed: %#v", candidate)
		}
		if repositoryBytesDigest(t, fixture.root) != before {
			t.Fatal("noncanonical refusal changed repository or Git entries, bytes or modes")
		}
	})
}

func mutateOCMLinkBinding(t *testing.T, fixture lrfFixture, mutation string) {
	t.Helper()
	path := filepath.Join(fixture.root, "change.ocm.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if mutation == "digest" {
		document["cem"].(map[string]any)["mapSha256"] = strings.Repeat("0", 64)
	} else {
		document["targetRevision"] = fixture.base
	}
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertOCMLinkBindingRefusal(t *testing.T, fixture lrfFixture, arguments []string) {
	t.Helper()
	before := repositoryBytesDigest(t, fixture.root)
	reader := execute(t, candidateCommand(ocmReadArguments(fixture, "verify")...))
	var verdict struct {
		Verification struct {
			Issues []struct {
				Code string `json:"code"`
			} `json:"issues"`
		} `json:"verification"`
	}
	if err := json.Unmarshal(reader.stdout, &verdict); err != nil || reader.exit != 1 || len(verdict.Verification.Issues) == 0 {
		t.Fatalf("invalid binding reader verdict: %#v, %v", reader, err)
	}
	candidate := execute(t, candidateCommand(arguments...))
	if repositoryBytesDigest(t, fixture.root) != before {
		t.Error("candidate refusal changed repository or Git entries, bytes or modes")
	}
	if candidate.exit != 2 || len(candidate.stdout) != 0 || refusalCode(t, candidate.stderr) != verdict.Verification.Issues[0].Code {
		t.Errorf("reader issue=%s candidate=%#v", verdict.Verification.Issues[0].Code, candidate)
	}
	t.Logf("first reader issue=%s; native exit=%d; repository/Git bytes and modes unchanged", verdict.Verification.Issues[0].Code, candidate.exit)
}

func TestOCMLinkRejectsClaimWithoutExactObligationIDBeforePublication(t *testing.T) {
	t.Parallel()
	t.Run("OCM-V0-005 exact ID candidate rejection", func(t *testing.T) {
		candidateFixture := newOCMLinkValidationFixture(t, "_TM-V0-008_")
		candidateMap, err := os.ReadFile(filepath.Join(candidateFixture.root, "change.ocm.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(candidateMap) == 0 {
			t.Fatal("empty fixture map")
		}
		candidateBefore := repositoryBytesDigest(t, candidateFixture.root)
		output := "change.ocm.json"
		candidate := execute(t, candidateCommand(ocmLinkValidationArguments(candidateFixture, "_TM-V0-008_", output)...))
		if candidate.exit != 2 || len(candidate.stdout) != 0 || !bytes.Contains(candidate.stderr, []byte(`"code": "claim-obligation-mismatch"`)) {
			t.Errorf("candidate=%#v", candidate)
		}
		if after := repositoryBytesDigest(t, candidateFixture.root); after != candidateBefore {
			t.Error("rejected candidate changed repository or Git bytes")
		}
		if candidate.exit == 0 {
			verified := execute(t, candidateCommand(ocmReadArguments(candidateFixture, "verify", "--map", output)...))
			if verified.exit != 1 || !bytes.Contains(verified.stdout, []byte(`"code":"claim-obligation-mismatch"`)) || len(verified.stderr) != 0 {
				t.Fatalf("pre-repair candidate verify=%#v", verified)
			}
		}
		candidateMapSHA, candidateMapMode := "absent", "absent"
		if data, readErr := os.ReadFile(filepath.Join(candidateFixture.root, filepath.FromSlash(output))); readErr == nil {
			candidateMapSHA = digestBytes(data)
			info, statErr := os.Stat(filepath.Join(candidateFixture.root, filepath.FromSlash(output)))
			if statErr != nil {
				t.Fatal(statErr)
			}
			candidateMapMode = info.Mode().String()
		}
		t.Logf("candidate exit=%d stdout-sha256=%s stderr-sha256=%s map-sha256=%s map-mode=%s before=%x after=%x", candidate.exit, digestBytes(candidate.stdout), digestBytes(candidate.stderr), candidateMapSHA, candidateMapMode, candidateBefore, repositoryBytesDigest(t, candidateFixture.root))
	})

	t.Run("GPK-V0-008 rejected mutator leaves no effects", func(t *testing.T) {
		fixture := newOCMLinkValidationFixture(t, "_TM-V0-008_")
		mapPath := filepath.Join(fixture.root, "change.ocm.json")
		beforeMap, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeInfo, err := os.Stat(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		before := repositoryBytesDigest(t, fixture.root)
		result := execute(t, candidateCommand(ocmLinkValidationArguments(fixture, "_TM-V0-008_", "absent/parents/change.ocm.json")...))
		afterMap, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		afterInfo, err := os.Stat(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		if result.exit != 2 || len(result.stdout) != 0 || !bytes.Contains(result.stderr, []byte(`"code": "claim-obligation-mismatch"`)) {
			t.Fatalf("result=%#v", result)
		}
		if after := repositoryBytesDigest(t, fixture.root); after != before {
			t.Fatal("rejected link changed file or directory bytes or modes")
		}
		if !bytes.Equal(afterMap, beforeMap) || afterInfo.Mode() != beforeInfo.Mode() {
			t.Fatal("rejected link changed original map bytes or mode")
		}
		if _, err := os.Stat(filepath.Join(fixture.root, "absent")); !os.IsNotExist(err) {
			t.Fatalf("rejected link created absent output parents: %v", err)
		}
		t.Logf("candidate exit=%d stdout-sha256=%s stderr-sha256=%s before=%x after=%x map-sha256=%s map-mode=%s", result.exit, digestBytes(result.stdout), digestBytes(result.stderr), before, repositoryBytesDigest(t, fixture.root), digestBytes(afterMap), afterInfo.Mode())
	})

	t.Run("delimiter-valid link still publishes", func(t *testing.T) {
		fixture := newOCMLinkValidationFixture(t, "TM-V0-008")
		output := "valid/change.ocm.json"
		if err := os.Mkdir(filepath.Join(fixture.root, "valid"), 0o755); err != nil {
			t.Fatal(err)
		}
		result := execute(t, candidateCommand(ocmLinkValidationArguments(fixture, "TM-V0-008", output)...))
		if result.exit != 0 || len(result.stdout) == 0 || len(result.stderr) != 0 {
			t.Fatalf("result=%#v", result)
		}
		verified := execute(t, candidateCommand(ocmReadArguments(fixture, "verify", "--map", output)...))
		if verified.exit != 0 || len(verified.stdout) == 0 || len(verified.stderr) != 0 {
			t.Fatalf("verify=%#v", verified)
		}
	})
}

// GPK-V0-001..004, GPK-V0-007, OCM-V0-006..012.
func TestOCMReadCommandsMatchPythonOracle(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"status", "verify", "report"} {
		t.Run(action, func(t *testing.T) {
			fixture := newOCMReadFixture(t)
			extra := []string{}
			if action == "report" {
				extra = []string{"--output", "ocm-review.md"}
			}
			candidate := runOCMProcess(t, ocmReadArguments(fixture, action, extra...))
			if candidate.exit != 0 || len(candidate.stderr) != 0 {
				t.Fatalf("exit=%d stderr=%s", candidate.exit, candidate.stderr)
			}
		})
	}
}

// GPK-V0-002, OCM-V0-006..007.
func TestOCMLegacyCEMReadCommandsMatchPythonOracle(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"status", "verify", "report"} {
		t.Run(action, func(t *testing.T) {
			fixture := newLRFFixture(t, wire.Spec01)
			writeOCM(t, fixture, "change.ocm.json", false)
			arguments := []string{
				"--root", fixture.root, "ocm", action,
				"--map", "change.ocm.json", "--cem", fixture.mapPath,
			}
			if action == "report" {
				arguments = append(arguments, "--output", "ocm-review.md")
			}
			candidate := runOCMProcess(t, arguments)
			if candidate.exit != 0 || len(candidate.stderr) != 0 {
				t.Fatalf("exit=%d stderr=%s", candidate.exit, candidate.stderr)
			}
		})
	}
}

// GPK-V0-002, OCM-V0-010.
func TestOCMMaxUnknownMatchesOracleBelowAtAndAbove(t *testing.T) {
	t.Parallel()
	for _, maximum := range []string{"0", "1", "2"} {
		for _, action := range []string{"status", "verify", "report"} {
			t.Run(action+"-"+maximum, func(t *testing.T) {
				fixture := newOCMReadFixture(t)
				extra := []string{"--max-unknown", maximum}
				if action == "report" {
					extra = append(extra, "--output", "ocm-review.md")
				}
				arguments := ocmReadArguments(fixture, action, extra...)
				output := filepath.Join(fixture.root, "ocm-review.md")
				prior := []byte("prior report\n")
				if maximum == "0" && action == "report" {
					if err := os.WriteFile(output, prior, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				candidate := execute(t, candidateCommand(arguments...))
				if maximum == "0" && action == "report" {
					if data, err := os.ReadFile(output); err != nil || !bytes.Equal(data, prior) {
						t.Fatalf("candidate changed a policy-failing report: %q %v", data, err)
					}
				}
				wantExit := 0
				if maximum == "0" {
					wantExit = 1
				}
				if candidate.exit != wantExit {
					t.Fatalf("exit=%d stdout=%s", candidate.exit, candidate.stdout)
				}
			})
		}
	}
}

func TestOCMMaxUnknownDoesNotUseImpactLimit(t *testing.T) {
	t.Parallel()
	options, err := parseOCMFlags([]string{"verify", "--map", "change.ocm.json", "--max-unknown", "100"})
	if err != nil || options.maxUnknown == nil || *options.maxUnknown != 100 {
		t.Fatalf("options=%#v err=%v", options, err)
	}
}

// GPK-V0-002, GPK-V0-004, GPK-V0-008.
func TestOCMFailuresMatchOracleVerdictAndReportWritesNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*testing.T, lrfFixture)
	}{
		{"cem-binding", corruptOCMBinding},
		{"structural", func(t *testing.T, fixture lrfFixture) {
			if err := os.WriteFile(filepath.Join(fixture.root, "change.ocm.json"), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		for _, action := range []string{"status", "verify", "report"} {
			t.Run(test.name+"-"+action, func(t *testing.T) {
				fixture := newOCMReadFixture(t)
				test.mutate(t, fixture)
				extra := []string{}
				if action == "report" {
					extra = []string{"--output", "ocm-review.md"}
				}
				arguments := ocmReadArguments(fixture, action, extra...)
				output := filepath.Join(fixture.root, "ocm-review.md")
				prior := []byte("prior report\n")
				if action == "report" {
					if err := os.WriteFile(output, prior, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				candidate := execute(t, candidateCommand(arguments...))
				if action == "report" {
					if data, err := os.ReadFile(output); err != nil || !bytes.Equal(data, prior) {
						t.Fatalf("candidate changed a structurally failing report: %q %v", data, err)
					}
				}
				if candidate.exit != 1 || len(candidate.stderr) != 0 {
					t.Fatalf("exit=%d stderr=%s", candidate.exit, candidate.stderr)
				}
			})
		}
	}
}

func TestOCMCEMBindingPrecedesMalformedClosureLikeOracle(t *testing.T) {
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
	document["cem"].(map[string]any)["mapSha256"] = strings.Repeat("0", 64)
	document["claims"].([]any)[0].(map[string]any)["id"] = "claim:sha256:" + strings.Repeat("0", 64)
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := runOCMProcess(t, ocmReadArguments(fixture, "verify"))
	if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"cem-map-digest-mismatch"`)) {
		t.Fatalf("candidate=%#v", candidate)
	}
}

func corruptOCMBinding(t *testing.T, fixture lrfFixture) {
	t.Helper()
	path := filepath.Join(fixture.root, "change.ocm.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["cem"].(map[string]any)["mapSha256"] = strings.Repeat("0", 64)
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// GPK-V0-004, OCM-V0-006..007.
func TestOCMCanonicalAuthorityPrecedenceMatchesOracle(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	if err := os.WriteFile(filepath.Join(fixture.root, "change.ocm.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := []string{"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json", "--cem", fixture.mapPath}
	for _, arguments := range [][]string{
		base,
		append(append([]string{}, base...), "--expected-base", fixture.base),
		append(append([]string{}, base...), "--expected-base", fixture.base, "--target", fixture.target),
	} {
		candidate := runOCMProcess(t, arguments)
		if candidate.exit != 1 {
			t.Fatalf("exit=%d stdout=%s", candidate.exit, candidate.stdout)
		}
	}
}

func TestOCMExplicitEmptyAuthorityMatchesOracle(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	base := []string{"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json", "--cem", fixture.mapPath}
	for _, arguments := range [][]string{
		append(append([]string{}, base...), "--expected-base", "", "--target", fixture.target),
		append(append([]string{}, base...), "--expected-base", fixture.base, "--target", ""),
	} {
		candidate := runOCMProcess(t, arguments)
		if candidate.exit != 1 {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
	corruptOCMBinding(t, fixture)
	arguments := append(append([]string{}, base...), "--expected-base", fixture.base, "--target", "")
	candidate := runOCMProcess(t, arguments)
	if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"cem-map-digest-mismatch"`)) {
		t.Fatalf("candidate=%#v", candidate)
	}
}

func TestOCMLegacyTargetMismatchMatchesOracle(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec01)
	writeOCM(t, fixture, "change.ocm.json", false)
	arguments := []string{
		"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json",
		"--cem", fixture.mapPath, "--target", fixture.base,
	}
	candidate := runOCMProcess(t, arguments)
	if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"target-mismatch"`)) {
		t.Fatalf("candidate=%#v", candidate)
	}
}

// GPK-V0-001..004.
func TestOCMReadArgumentFailuresMatchOracle(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	for _, arguments := range [][]string{
		{"--root", fixture.root, "ocm", "verify", "--cem", fixture.mapPath},
		{"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json", "--max-unknown", "nope"},
		{"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json", "--max-unknown", "-1"},
		{"--root", fixture.root, "ocm", "status", "--map", "change.ocm.json", "--output", "report.md"},
	} {
		candidate := runOCMProcess(t, arguments)
		if candidate.exit != 2 || len(candidate.stdout) != 0 || len(candidate.stderr) == 0 {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
}

func TestOCMMissingInputsMatchOracle(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	for _, arguments := range [][]string{
		{"--root", fixture.root, "ocm", "verify", "--map", "missing.ocm.json", "--cem", fixture.mapPath},
		{"--root", fixture.root, "ocm", "verify", "--map", "change.ocm.json", "--cem", "missing.cem.json"},
	} {
		candidate := runOCMProcess(t, arguments)
		if candidate.exit != 2 || len(candidate.stdout) != 0 {
			t.Fatalf("candidate=%#v", candidate)
		}
	}
}

func TestOCMSymlinkInputsMatchOracle(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	for _, test := range []struct {
		name, target, link string
		arguments          func(string) []string
	}{
		{"OCM", "change.ocm.json", "linked.ocm.json", func(link string) []string {
			return ocmReadArguments(fixture, "verify", "--map", link)
		}},
		{"CEM", fixture.mapPath, "linked.cem.json", func(link string) []string {
			return ocmReadArguments(fixture, "verify", "--cem", link)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.Symlink(filepath.Join(fixture.root, filepath.FromSlash(test.target)), filepath.Join(fixture.root, test.link)); err != nil {
				t.Fatal(err)
			}
			candidate := runOCMProcess(t, test.arguments(test.link))
			if candidate.exit != 2 || !bytes.Contains(candidate.stderr, []byte("cannot read "+test.name+" map")) {
				t.Fatalf("candidate=%#v", candidate)
			}
		})
	}
}

// GPK-V0-007.
func TestOCMStatusAndVerifyAreReadOnly(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	before := repositoryBytesDigest(t, fixture.root)
	for _, action := range []string{"status", "verify"} {
		result := execute(t, candidateCommand(ocmReadArguments(fixture, action)...))
		if result.exit != 0 || len(result.stderr) != 0 {
			t.Fatalf("%s: %#v", action, result)
		}
	}
	if after := repositoryBytesDigest(t, fixture.root); after != before {
		t.Fatal("OCM status or verify changed repository bytes")
	}
}

// GPK-V0-002, GPK-V0-008, OCM-V0-011..012.
func TestOCMReportWritesExactOracleBytesAndMode(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	arguments := ocmReadArguments(fixture, "report")
	candidate := execute(t, candidateCommand(arguments...))
	path := filepath.Join(cemGit(t, fixture.root, "rev-parse", "--absolute-git-dir"), "corvint", "ocm-review.md")
	candidateBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	candidateInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateFileMode(t, "candidate private file mode", candidateInfo.Mode(), 0o600)
	if candidate.exit != 0 || len(candidate.stderr) != 0 || len(candidateBytes) == 0 {
		t.Fatalf("report result=%#v", candidate)
	}
}

func TestOCMReportExplicitEmptyOutputMatchesOracleAndWritesNothing(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	before := repositoryBytesDigest(t, fixture.root)
	arguments := ocmReadArguments(fixture, "report", "--output", "")
	candidate := runOCMProcess(t, arguments)
	if candidate.exit != 2 || len(candidate.stdout) != 0 {
		t.Fatalf("candidate=%#v", candidate)
	}
	if after := repositoryBytesDigest(t, fixture.root); after != before {
		t.Fatal("empty report output changed repository bytes")
	}
}

// GPK-V0-002.
func TestOCMOutputLocationDependenceMatchesOracle(t *testing.T) {
	t.Parallel()
	first := newOCMReadFixture(t)
	secondRoot := filepath.Join(t.TempDir(), "different", "root")
	if err := os.MkdirAll(filepath.Dir(secondRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	clone := exec.Command("git", "clone", "-q", first.root, secondRoot)
	if output, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, output)
	}
	for _, path := range []string{first.mapPath, "change.ocm.json"} {
		raw, err := os.ReadFile(filepath.Join(first.root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		full := filepath.Join(secondRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	second := first
	second.root = secondRoot
	for _, action := range []string{"status", "verify", "report"} {
		extra := []string{}
		if action == "report" {
			extra = []string{"--output", "ocm-review.md"}
		}
		firstCandidate := runOCMProcess(t, ocmReadArguments(first, action, extra...))
		secondCandidate := runOCMProcess(t, ocmReadArguments(second, action, extra...))
		equal := bytes.Equal(firstCandidate.stdout, secondCandidate.stdout)
		if action == "verify" && !equal {
			t.Fatal("verify output depends on repository location")
		}
		if action != "verify" && equal {
			t.Fatalf("%s output unexpectedly location-independent", action)
		}
	}
}

// GPK-V0-004, GPK-V0-014.
func TestOCMUnsupportedProfilesAreTyped(t *testing.T) {
	t.Parallel()
	t.Run("malformed JSON", func(t *testing.T) {
		fixture := newOCMReadFixture(t)
		if err := os.WriteFile(filepath.Join(fixture.root, "change.ocm.json"), []byte("{\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		arguments := ocmReadArguments(fixture, "verify")
		candidate := runOCMProcess(t, arguments)
		if candidate.exit != 2 || !bytes.Contains(candidate.stderr, []byte(`"code": "unsupported-ocm-json-profile"`)) {
			t.Fatalf("candidate=%#v", candidate)
		}
	})
	t.Run("Python claim", func(t *testing.T) {
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
		document["claims"].([]any)[0].(map[string]any)["path"] = "tests/widget_test.py"
		claim := document["claims"].([]any)[0].(map[string]any)
		body := make(map[string]any, len(claim)-1)
		for key, value := range claim {
			if key != "id" {
				body[key] = value
			}
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		claim["id"] = "claim:sha256:" + digestBytes(encoded)
		document["obligations"].([]any)[0].(map[string]any)["claimIds"] = []any{claim["id"]}
		raw, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		corruptOCMBinding(t, fixture)
		candidate := runOCMProcess(t, ocmReadArguments(fixture, "verify"))
		if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"cem-map-digest-mismatch"`)) {
			t.Fatalf("binding precedence candidate=%#v", candidate)
		}
		document["cem"].(map[string]any)["mapSha256"] = digestBytes(fixture.cemRaw)
		document["cem"].(map[string]any)["patchSha256"] = strings.Repeat("0", 64)
		raw, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		candidate = runOCMProcess(t, ocmReadArguments(fixture, "verify"))
		if candidate.exit != 1 || !bytes.Contains(candidate.stdout, []byte(`"code":"cem-patch-digest-mismatch"`)) {
			t.Fatalf("patch precedence candidate=%#v", candidate)
		}
		document["cem"].(map[string]any)["patchSha256"] = digestBytes(fixture.patch)
		raw, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		candidate = execute(t, candidateCommand(ocmReadArguments(fixture, "verify")...))
		if candidate.exit != 2 || !bytes.Contains(candidate.stderr, []byte(`"code": "unsupported-ocm-python-claims"`)) {
			t.Fatalf("candidate=%#v", candidate)
		}
	})
	t.Run("closure validation", func(t *testing.T) {
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
		document["claims"].([]any)[0].(map[string]any)["id"] = "claim:sha256:" + strings.Repeat("0", 64)
		raw, err = json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		candidate := runOCMProcess(t, ocmReadArguments(fixture, "verify"))
		if candidate.exit != 2 || !bytes.Contains(candidate.stderr, []byte(`"code": "unsupported-ocm-closure-profile"`)) {
			t.Fatalf("candidate=%#v", candidate)
		}
	})
}

// GPK-V0-008, OCM-V0-012.
func TestOCMReportRefusesOutsideAndSymlinkOutputs(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	outsideRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideRoot, "outside.md")
	arguments := ocmReadArguments(fixture, "report", "--output", outside)
	result := execute(t, candidateCommand(arguments...))
	if result.exit != 2 || !bytes.Contains(result.stderr, []byte(`"code": "unsupported-ocm-output-path"`)) {
		t.Fatalf("outside result=%#v", result)
	}
	if _, err := os.Lstat(outside); !os.IsNotExist(err) {
		t.Fatal("candidate wrote outside the repository root")
	}
	relativeOutside := filepath.Join("..", "outside-relative.md")
	result = execute(t, candidateCommand(ocmReadArguments(fixture, "report", "--output", relativeOutside)...))
	if result.exit != 2 || !bytes.Contains(result.stderr, []byte(`"code": "unsupported-ocm-output-path"`)) {
		t.Fatalf("relative outside result=%#v", result)
	}
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(fixture.root, "review.md")); err != nil {
		t.Fatal(err)
	}
	result = execute(t, candidateCommand(ocmReadArguments(fixture, "report", "--output", "review.md")...))
	if result.exit != 2 || !bytes.Contains(result.stderr, []byte(`"code": "unsafe-output"`)) {
		t.Fatalf("symlink result=%#v", result)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "safe" {
		t.Fatalf("sentinel=%q err=%v", data, err)
	}
	outsideDirectory := t.TempDir()
	if err := os.Symlink(outsideDirectory, filepath.Join(fixture.root, "linked")); err != nil {
		t.Fatal(err)
	}
	result = execute(t, candidateCommand(ocmReadArguments(fixture, "report", "--output", "linked/review.md")...))
	if result.exit != 2 || !bytes.Contains(result.stderr, []byte(`"code": "unsafe-output"`)) {
		t.Fatalf("ancestor symlink result=%#v", result)
	}
	if entries, err := os.ReadDir(outsideDirectory); err != nil || len(entries) != 0 {
		t.Fatalf("outside entries=%v err=%v", entries, err)
	}
}

// TestOCMUnsupportedRefusalAppendsOneObservation: SOL-V0-007 records an ocm
// unsupported-* refusal at the command boundary with only code and intent.
func TestOCMUnsupportedRefusalAppendsOneObservation(t *testing.T) {
	t.Parallel()
	fixture := newOCMReadFixture(t)
	if err := os.MkdirAll(filepath.Join(fixture.root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, ".corvint", ".gitignore"), []byte("*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	result := runOCMProcess(t, ocmReadArguments(fixture, "report", "--output", outside))
	if result.exit != 2 || !bytes.Contains(result.stderr, []byte(`"code": "unsupported-ocm-output-path"`)) {
		t.Fatalf("result=%#v", result)
	}
	ledger, err := os.ReadFile(filepath.Join(fixture.root, ".corvint", "self-observations.jsonl"))
	if want := `{"kind":"unsupported","code":"unsupported-ocm-output-path","queryIntent":"unknown"}` + "\n"; err != nil || string(ledger) != want {
		t.Fatalf("ledger=%q err=%v want %q", ledger, err, want)
	}
}

// SOL-V0-007: lrf, cem, and dogfood-ocm record their unsupported-* refusals at
// the same stderr boundary as ocm, each appending exactly one row.
func TestLRFCEMAndDogfoodOCMUnsupportedRefusalsAppendOneObservationEach(t *testing.T) {
	t.Parallel()
	fixture := newLRFFixture(t, wire.Spec01)
	writeOCM(t, fixture, "change.ocm.json", true)
	ocm, err := os.ReadFile(filepath.Join(fixture.root, "change.ocm.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fixture.root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		".gitignore": []byte("*\n"), "change.cem.json": fixture.cemRaw,
		"change.ocm-intents": []byte("docs/intent.md\n"), "change.ocm.001.json": ocm,
	} {
		if err := os.WriteFile(filepath.Join(fixture.root, ".corvint", name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	steps := []struct {
		code string
		argv []string
	}{
		{"unsupported-lrf-context", []string{"lrf", "--cem", fixture.mapPath, "--patch", fixture.patchPath, "--target", fixture.target, "--ocm", "change.ocm.json"}},
		{"unsupported-object-alternates", []string{"cem", "prepare", "--base", fixture.base, "--target", fixture.target}},
		{"unsupported-object-alternates", []string{"dogfood-ocm", "status", "--expected-base", fixture.base, "--target", fixture.target}},
	}
	want := ""
	for index, step := range steps {
		if index == 1 {
			// Alternates refuse before any object read in every CEM/OCM consumer.
			if err := os.WriteFile(filepath.Join(fixture.root, ".git", "objects", "info", "alternates"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, _, stderr := runCLI(t, append([]string{"--root", fixture.root}, step.argv...)...)
		if code != 2 || !strings.Contains(stderr, `"code": "`+step.code+`"`) {
			t.Fatalf("%s: exit=%d stderr=%q", step.argv[0], code, stderr)
		}
		want += `{"kind":"unsupported","code":"` + step.code + `","queryIntent":"unknown"}` + "\n"
		ledger, err := os.ReadFile(filepath.Join(fixture.root, ".corvint", "self-observations.jsonl"))
		if err != nil || string(ledger) != want {
			t.Fatalf("%s: ledger=%q err=%v want %q", step.argv[0], ledger, err, want)
		}
	}
}

// OIF-V0-001: --intent-form is an explicit closed choice; requirements and an
// absent flag both select the default form.
func TestOIFV0PrepareIntentFormFlag(t *testing.T) {
	base := []string{"--target", "HEAD", "--intent", "docs/adr/0224-x.md"}
	cases := map[string]string{"": "", "requirements": "", "adr-decisions": "adr-decisions", "roadmap-acceptance": "roadmap-acceptance"}
	for flag, want := range cases {
		arguments := base
		if flag != "" {
			arguments = append(append([]string{}, base...), "--intent-form", flag)
		}
		parsed, err := parseOCMPrepareFlags(ocmCLIOptions{}, arguments)
		if err != nil || parsed.intentForm != want {
			t.Fatalf("flag=%q form=%q err=%v", flag, parsed.intentForm, err)
		}
	}
	if _, err := parseOCMPrepareFlags(ocmCLIOptions{}, append(base, "--intent-form=adr")); err == nil {
		t.Fatal("unknown intent form accepted")
	}
}
