package companionrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// checkCoreDiscoveryWorkflows proves that the installed core binary, rather
// than a source-tree `go run`, can reach the four read surfaces issue #44
// requires. The Playwright row qualifies retained-receipt discovery only; it
// deliberately does not claim that this fixture executed Playwright.
func checkCoreDiscoveryWorkflows(ctx context.Context, binary, scratch string) ([]SmokeStep, error) {
	checks := []struct {
		name string
		run  func(context.Context, string, string) error
	}{
		{"corvint-version-identity", smokeCoreVersion},
		{"corvint-affected-selection", smokeAffectedSelection},
		{"corvint-playwright-external-discovery", smokeExternalPlaywrightDiscovery},
		{"corvint-documentation-corpus-discovery", smokeDocumentationCorpus},
		{"corvint-work-queue-observation", smokeWorkQueueObservation},
	}
	steps := make([]SmokeStep, 0, len(checks))
	for _, check := range checks {
		root := filepath.Join(scratch, "core-smoke-"+strings.TrimPrefix(check.name, "corvint-"))
		err := check.run(ctx, binary, root)
		steps = append(steps, step(check.name, err, "installed extracted binary"))
		if err != nil {
			return steps, fmt.Errorf("%s: %w", check.name, err)
		}
	}
	return steps, nil
}

func smokeCoreVersion(ctx context.Context, binary, root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	stdout, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--version")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(stdout))
	if !strings.HasPrefix(version, "Corvint ") || !strings.Contains(version, " (build ") || !strings.HasSuffix(version, ")") {
		return fmt.Errorf("version output lacks release/build identity: %q", version)
	}
	return nil
}

func smokeAffectedSelection(ctx context.Context, binary, root string) error {
	files := map[string]string{
		"go.mod":            "module example.test/release-smoke\n\ngo 1.27.1\n",
		"calc/calc.go":      "package calc\n\nfunc Add(a, b int) int { return a + b }\n",
		"calc/calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(1, 1) != 2 { t.Fatal(\"bad sum\") } }\n",
	}
	if _, err := initCommittedFixture(ctx, root, files); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "calc", "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a+b }\n"), 0o600); err != nil {
		return err
	}
	stdout, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "affected")
	if err != nil {
		return err
	}
	var receipt struct {
		OK      bool   `json:"ok"`
		Profile string `json:"profile"`
		Plan    struct {
			Selected []json.RawMessage `json:"selected"`
		} `json:"plan"`
	}
	if err := decodeClosedJSON(stdout, &receipt); err != nil {
		return err
	}
	if !receipt.OK || receipt.Profile != "affected-plan/0" || len(receipt.Plan.Selected) == 0 {
		return fmt.Errorf("affected selection did not select the committed fixture test")
	}
	return nil
}

func smokeExternalPlaywrightDiscovery(ctx context.Context, binary, root string) error {
	config := filepath.Join(root, "playwright.config.cjs")
	test := filepath.Join(root, "release.spec.cjs")
	files := map[string]string{
		"playwright.config.cjs": "module.exports = { projects: [{ name: 'chromium', use: { browserName: 'chromium' } }] };\n",
		"release.spec.cjs":      "// retained external Playwright discovery fixture; no execution claim\n",
	}
	if _, err := initCommittedFixture(ctx, root, files); err != nil {
		return err
	}
	configDigest, err := fileDigest(config)
	if err != nil {
		return err
	}
	testDigest, err := fileDigest(test)
	if err != nil {
		return err
	}
	receipt := jstestprovider.Receipt{
		Profile: jstestprovider.ExternalProfile,
		Kind:    "e2e",
		Identity: jstestprovider.Identity{
			ConfigInputDigests: map[string]string{config: configDigest},
			TestFileDigests:    map[string]string{test: testDigest},
			ConfigFile:         config,
			ConfigDigest:       configDigest,
			NodeVersion:        "not-executed",
			RunnerName:         "playwright",
			RunnerVersion:      "1.60.0",
			Environment:        map[string]string{},
			Argv:               []string{"npx", "playwright", "test", "--config=" + config},
		},
		External: &jstestprovider.ExternalLifecycle{
			ReadyURL: "http://127.0.0.1:1", DeclaredAppIdentity: "release-discovery-fixture-not-executed",
			Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown",
			ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true,
			ConfigOverride: "retained-discovery-fixture",
		},
		Tests: []jstestprovider.TestOutcome{},
	}
	document, err := jstestprovider.EncodeQualified(receipt)
	if err != nil {
		return err
	}
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(evidence, "external.json"), document, 0o600); err != nil {
		return err
	}
	stdout, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "test-validity", "--discover")
	if err != nil {
		return err
	}
	var projected struct {
		Schema     string `json:"schema"`
		Source     string `json:"source"`
		Playwright *struct {
			Profile string `json:"profile"`
		} `json:"playwright"`
		Discovery *struct {
			Evidence string `json:"evidence"`
		} `json:"discovery"`
	}
	if err := decodeClosedJSON(stdout, &projected); err != nil {
		return err
	}
	if projected.Schema != "corvint-test-validity/0" || projected.Playwright == nil || projected.Playwright.Profile != jstestprovider.ExternalProfile || projected.Discovery == nil || projected.Discovery.Evidence == "" {
		return fmt.Errorf("external Playwright receipt was not discovered through test-validity")
	}
	return nil
}

func smokeDocumentationCorpus(ctx context.Context, binary, root string) error {
	files := map[string]string{
		"go.mod":          "module example.test/release-docs\n\ngo 1.27.1\n",
		"docs/release.md": "# ReleaseSmoke\n\nInstalled documentation corpus discovery.\n",
	}
	revision, err := initCommittedFixture(ctx, root, files)
	if err != nil {
		return err
	}
	manifest, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "docs", "corpus", "manifest", "--revision", revision, "--scope", "docs/release.md", "--timestamp", "2026-09-20T00:00:00Z")
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(root, "corpus-input.json")
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		return err
	}
	artifact, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "docs", "corpus", "build", "--manifest", "corpus-input.json")
	if err != nil {
		return err
	}
	artifactPath := filepath.Join(root, "corpus.json")
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		return err
	}
	stdout, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "docs", "corpus", "search", "--artifact", "corpus.json", "--query", "ReleaseSmoke")
	if err != nil {
		return err
	}
	var receipt struct {
		Schema    string            `json:"schema"`
		Operation string            `json:"operation"`
		Results   []json.RawMessage `json:"results"`
	}
	if err := decodeClosedJSON(stdout, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "corvint-corpus-receipt/1" || receipt.Operation != "search" || len(receipt.Results) == 0 {
		return fmt.Errorf("documentation corpus search returned no installed discovery result")
	}
	return nil
}

func smokeWorkQueueObservation(ctx context.Context, binary, root string) error {
	boundBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	if _, err := initCommittedFixture(ctx, root, map[string]string{"README.md": "# queue fixture\n"}); err != nil {
		return err
	}
	if _, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "work", "init", "--repository", "release-smoke", "--corvint-executable", boundBinary); err != nil {
		return err
	}
	gitPath, err := lookGit()
	if err != nil {
		return err
	}
	for _, args := range [][]string{{"add", ".corvint"}, {"-c", "user.name=Corvint Smoke", "-c", "user.email=smoke@corvint.invalid", "commit", "-qm", "adopt work queue"}} {
		if _, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, append([]string{gitPath}, args...)...); err != nil {
			return err
		}
	}
	stdout, _, err := runCaptured(ctx, root, minimalRunEnv(root), subprocessTimeout, binary, "--root", root, "work", "observe")
	if err != nil {
		return fmt.Errorf("%w (stdout=%s)", err, trimForError(stdout))
	}
	var result struct {
		Profile     string          `json:"profile"`
		State       string          `json:"state"`
		Observation json.RawMessage `json:"observation"`
	}
	if err := decodeClosedJSON(stdout, &result); err != nil {
		return err
	}
	if result.Profile != "work-command-result/0" || result.State != "OK" || len(result.Observation) == 0 || string(result.Observation) == "null" {
		return fmt.Errorf("work queue observation did not return an observation")
	}
	return nil
}

func initCommittedFixture(ctx context.Context, root string, files map[string]string) (string, error) {
	if err := writeFiles(root, files, 0o600); err != nil {
		return "", err
	}
	gitPath, err := lookGit()
	if err != nil {
		return "", err
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"-c", "user.name=Corvint Smoke", "-c", "user.email=smoke@corvint.invalid", "commit", "-qm", "fixture"}} {
		if _, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, append([]string{gitPath}, args...)...); err != nil {
			return "", err
		}
	}
	stdout, _, err := runCaptured(ctx, root, closedGitEnv(root), subprocessTimeout, gitPath, "rev-parse", "HEAD")
	return strings.TrimSpace(string(stdout)), err
}

func decodeClosedJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func fileDigest(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
