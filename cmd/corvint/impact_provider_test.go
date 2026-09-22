package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func providerRecord(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "extevidence", "testdata", "mock-provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	data = bytes.Replace(data, []byte(strings.Repeat("0", 40)), []byte(head), 1)
	path := filepath.Join(t.TempDir(), "mock-provider.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImpactProviderFlagParsing(t *testing.T) {
	t.Parallel()
	parsed, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--provider", "a.json", "--provider=b.json", "pkg/main.go"}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(parsed.impactProviders, ",") != "a.json,b.json" {
		t.Fatalf("providers = %v", parsed.impactProviders)
	}
	refusals := map[string][]string{
		"missing value":        {"pkg/main.go", "--provider"},
		"empty inline value":   {"--provider=", "pkg/main.go"},
		"fifth provider":       {"--provider", "1", "--provider", "2", "--provider", "3", "--provider", "4", "--provider", "5", "pkg/main.go"},
		"with --base":          {"--provider", "a.json", "--base", strings.Repeat("a", 40)},
		"with working tree":    {"--provider", "a.json", "--working-tree-untracked", "pkg/main.go"},
		"still requires paths": {"--provider", "a.json"},
	}
	for name, arguments := range refusals {
		if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, arguments, "darwin"); err == nil {
			t.Errorf("%s: expected an argument error", name)
		}
	}
}

func TestImpactProviderSectionSeparation(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record := providerRecord(t, root)
	code, plain, stderr := runCLI(t, "--root", root, "impact", "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("plain impact: exit %d stderr %q", code, stderr)
	}
	code, withProvider, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider: exit %d stderr %q", code, stderr)
	}
	var plainPayload, providerPayload map[string]any
	if err := json.Unmarshal([]byte(plain), &plainPayload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(withProvider), &providerPayload); err != nil {
		t.Fatal(err)
	}
	plainContext := plainPayload["context"].(map[string]any)
	providerContext := providerPayload["context"].(map[string]any)
	if _, present := plainContext["external"]; present {
		t.Fatal("external must be absent without --provider")
	}
	external, ok := providerContext["external"].(map[string]any)
	if !ok {
		t.Fatalf("external section missing: %s", withProvider)
	}
	delete(providerContext, "external")
	left, _ := contextindex.CanonicalJSON(plainPayload)
	right, _ := contextindex.CanonicalJSON(providerPayload)
	if !bytes.Equal(left, right) {
		t.Fatalf("core receipt must be byte-identical:\n%s\n%s", left, right)
	}
	provider := external["providers"].([]any)[0].(map[string]any)
	if provider["state"] != "loaded" || provider["freshness"] != "equal" || provider["id"] != "mockdocs" {
		t.Fatalf("provider row = %v", provider)
	}
	results := external["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["entity"] != "mockdocs:cap-stable-value" {
		t.Fatalf("results = %v", results)
	}
	if results[0].(map[string]any)["authority"] != "external-provider" {
		t.Fatalf("Core must assign the external-provider authority: %v", results[0])
	}
	if got := len(external["verification"].([]any)); got != 1 {
		t.Fatalf("expected one verification relation from pkg/main_test.go, got %d", got)
	}
	for _, entry := range plainContext["results"].([]any) {
		if strings.HasPrefix(entry.(map[string]any)["id"].(string), "mockdocs:") {
			t.Fatal("external entities must never enter core results")
		}
	}
	// A missing record is structured, not fatal (EEP-V0-005).
	code, degraded, stderr := runCLI(t, "--root", root, "impact", "--provider", filepath.Join(t.TempDir(), "absent.json"), "pkg/main.go")
	if code != 0 || stderr != "" || !strings.Contains(degraded, `"state":"unavailable"`) {
		t.Fatalf("missing record: exit %d stderr %q stdout %s", code, stderr, degraded)
	}
}

func TestImpactProviderReadOnly(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record := providerRecord(t, root)
	before, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	status := affectedGit(t, root, "status", "--porcelain")
	code, stdout, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "pkg/main.go", "pkg/main_test.go")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, `"mutates":false`) {
		t.Fatalf("receipt must report mutates false: %s", stdout)
	}
	if after := affectedGit(t, root, "status", "--porcelain"); after != status {
		t.Fatalf("worktree status changed:\n%s\n%s", status, after)
	}
	after, err := os.ReadFile(record)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("provider record bytes must be untouched")
	}
}

// providerRecord1 fills the two-repository V1 conformance fixture against root
// and a second, independent e2e repository it creates.
func providerRecord1(t *testing.T, root string) (record, e2e string) {
	t.Helper()
	return providerRecordFixture(t, root, filepath.Join("conformance-v1", "two-repository.json"))
}

// providerRecordFixture fills one extevidence two-repository fixture against
// root and a fresh e2e history; a path fixture is filled as V2.
func providerRecordFixture(t *testing.T, root, fixture string) (record, e2e string) {
	t.Helper()
	// An independent history: cliRepository's first commit is deterministic,
	// so reusing it would give both repositories the same origin.
	e2e = t.TempDir()
	affectedGit(t, e2e, "init", "-q")
	affectedGit(t, e2e, "config", "user.email", "corvint@example.test")
	affectedGit(t, e2e, "config", "user.name", "Corvint Test")
	for _, name := range []string{"account", "journey"} {
		file := filepath.Join(e2e, "tests", name+".spec.ts")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("test('"+name+"')\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, e2e, "add", ".")
	affectedGit(t, e2e, "commit", "-qm", "add e2e tests")
	trim := func(repository string, arguments ...string) string {
		return strings.TrimSpace(affectedGit(t, repository, arguments...))
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "extevidence", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"APP_ORIGIN": trim(root, "rev-list", "--max-parents=0", "HEAD"), "APP_REVISION": trim(root, "rev-parse", "HEAD"),
		"E2E_ORIGIN": trim(e2e, "rev-list", "--max-parents=0", "HEAD"), "E2E_REVISION": trim(e2e, "rev-parse", "HEAD"),
		"E2E_TREE": trim(e2e, "rev-parse", "HEAD^{tree}"), "SCHEMA": "external-evidence-provider/2",
	}
	for key, value := range values {
		data = bytes.ReplaceAll(data, []byte("{{"+key+"}}"), []byte(value))
	}
	record = filepath.Join(t.TempDir(), "two-repository.json")
	if err := os.WriteFile(record, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return record, e2e
}

func TestImpactRepositoryFlagParsing(t *testing.T) {
	t.Parallel()
	parsed, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, []string{"--provider", "a.json", "--repository", "e2e=../e2e", "--repository=docs=/srv/docs", "pkg/main.go"}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.impactCheckouts) != 2 || parsed.impactCheckouts[0].ID != "e2e" || parsed.impactCheckouts[1].Source != "/srv/docs" {
		t.Fatalf("checkouts = %+v", parsed.impactCheckouts)
	}
	ninth := []string{"--provider", "a.json"}
	for index := 0; index < 9; index++ {
		ninth = append(ninth, "--repository", "r"+strconv.Itoa(index)+"=d")
	}
	refusals := map[string][]string{
		"missing value":      {"--provider", "a.json", "pkg/main.go", "--repository"},
		"no separator":       {"--provider", "a.json", "--repository", "e2e", "pkg/main.go"},
		"empty directory":    {"--provider", "a.json", "--repository", "e2e=", "pkg/main.go"},
		"bound twice":        {"--provider", "a.json", "--repository", "e2e=a", "--repository", "e2e=b", "pkg/main.go"},
		"without --provider": {"--repository", "e2e=../e2e", "pkg/main.go"},
		"ninth checkout":     append(ninth, "pkg/main.go"),
	}
	for name, arguments := range refusals {
		if _, err := parseImpactArgumentsForPlatform(options{impactLimit: 10}, arguments, "darwin"); err == nil {
			t.Errorf("%s: expected an argument error", name)
		}
	}
}

func TestImpactProviderV1CrossRepository(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record, e2e := providerRecord1(t, root)
	code, plain, stderr := runCLI(t, "--root", root, "impact", "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("plain impact: exit %d stderr %q", code, stderr)
	}
	code, withProvider, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "--repository", "e2e="+e2e, "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider --repository: exit %d stderr %q", code, stderr)
	}
	var plainPayload, providerPayload map[string]any
	if err := json.Unmarshal([]byte(plain), &plainPayload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(withProvider), &providerPayload); err != nil {
		t.Fatal(err)
	}
	providerContext := providerPayload["context"].(map[string]any)
	external := providerContext["external"].(map[string]any)
	delete(providerContext, "external")
	left, _ := contextindex.CanonicalJSON(plainPayload)
	right, _ := contextindex.CanonicalJSON(providerPayload)
	if !bytes.Equal(left, right) {
		t.Fatalf("core receipt must be byte-identical with a V1 provider:\n%s\n%s", left, right)
	}
	var cross map[string]any
	for _, raw := range external["verification"].([]any) {
		if entry := raw.(map[string]any); entry["path"] == "tests/account.spec.ts" {
			cross = entry
		}
	}
	if cross["repository"] != "e2e" || cross["crosses_repositories"] != true || cross["relation_state"] != "fresh" || cross["authority"] != "external-provider" {
		t.Fatalf("cross-repository verification = %v", cross)
	}
	if rows := external["checkouts"].([]any); len(rows) != 1 {
		t.Fatalf("checkouts = %v", rows)
	}
}

// EEP-V2-003, EEP-V2-005: a V2 record adds path_relations inside
// context.external and leaves every core member byte-identical.
func TestImpactProviderV2PathRelations(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record, e2e := providerRecordFixture(t, root, filepath.Join("conformance-path", "two-repository.json"))
	code, plain, stderr := runCLI(t, "--root", root, "impact", "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("plain impact: exit %d stderr %q", code, stderr)
	}
	code, withProvider, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "--repository", "e2e="+e2e, "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider V2: exit %d stderr %q", code, stderr)
	}
	var plainPayload, providerPayload map[string]any
	if err := json.Unmarshal([]byte(plain), &plainPayload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(withProvider), &providerPayload); err != nil {
		t.Fatal(err)
	}
	providerContext := providerPayload["context"].(map[string]any)
	external := providerContext["external"].(map[string]any)
	delete(providerContext, "external")
	left, _ := contextindex.CanonicalJSON(plainPayload)
	right, _ := contextindex.CanonicalJSON(providerPayload)
	if !bytes.Equal(left, right) {
		t.Fatalf("core receipt must be byte-identical with a V2 provider:\n%s\n%s", left, right)
	}
	var cross map[string]any
	for _, raw := range external["path_relations"].([]any) {
		if entry := raw.(map[string]any); entry["crosses_repositories"] == true {
			cross = entry
		}
	}
	if cross["relation_state"] != "fresh" || cross["authority"] != "external-provider" || len(cross["endpoints"].([]any)) != 2 {
		t.Fatalf("cross-repository path relation = %v", cross)
	}
}
