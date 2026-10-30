package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// adopterAuthorityRepository is a synthetic repository shaped like an external
// Go modular-monolith adopter (V1-1977): a Go module, a root AGENTS.md, and
// numbered ADRs under docs/adr. Its content is invented for the test.
func adopterAuthorityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".corvint/\n",
		"go.mod":     "module example.test/mediahost\n\ngo 1.27.0\n",
		"AGENTS.md": "# Agent contract\n\n" +
			"Read the accepted ADRs under docs/adr before changing the plugin host.\n" +
			"Every plugin registers through the plugin registry; never import a plugin from core.\n",
		"docs/adr/0001-plugin-host-architecture.md": "# 0001. Plugin host architecture\n\n" +
			"Status: Accepted\n\n## Decision\n\nThe media server core loads every catalog plugin through one plugin registry.\n",
		"docs/adr/0002-plugin-registry-manifest.md": "# 0002. Plugin registry manifest\n\n" +
			"Status: Accepted\n\n## Decision\n\nA plugin declares its manifest; the plugin registry validates the manifest before load.\n",
		"internal/plugin/registry.go": "package plugin\n\n// Registry holds loaded plugins.\ntype Registry struct{ names []string }\n\n" +
			"// Register adds a plugin manifest to the registry.\nfunc (r *Registry) Register(name string) { r.names = append(r.names, name) }\n",
		"cmd/mediahost/main.go": "package main\n\nfunc main() {}\n",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

// TestAdopterUserPromptReturnsAndRecordsProjectAuthority is the V1-1977
// external-adopter qualification (SOL-V0-011): on a Go module with AGENTS.md and
// accepted docs/adr decisions, a user-prompt hook event returns accepted ADR
// authority, its self-observation row records the measured authoritative count
// (the decoded packet once read every count as zero), and the digest reports no
// zero-authority event. It fails when an adopter with governing docs gets zero
// authority, whether the packet or the ledger is wrong.
func TestAdopterUserPromptReturnsAndRecordsProjectAuthority(t *testing.T) {
	t.Parallel()
	root := adopterAuthorityRepository(t)
	var stdout, stderr bytes.Buffer
	input := strings.NewReader(`{"task":"how should a new plugin register its manifest with the plugin registry"}`)
	if code := run(cliArguments(root, "user-prompt"), input, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	var response struct {
		Degradations []string `json:"degradations"`
		Context      struct {
			Coverage struct {
				Authoritative int `json:"authoritative_results"`
			} `json:"coverage"`
			Results []struct {
				Kind     string `json:"kind"`
				ID       string `json:"id"`
				Evidence []struct {
					Path      string `json:"path"`
					Authority string `json:"authority"`
				} `json:"evidence"`
			} `json:"results"`
		} `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Context.Coverage.Authoritative == 0 {
		t.Fatalf("adopter with governing docs got zero authority: %s", stdout.Bytes())
	}
	governing := false
	for _, result := range response.Context.Results {
		for _, evidence := range result.Evidence {
			if strings.HasPrefix(evidence.Path, "docs/adr/") && evidence.Authority == "accepted-decision" {
				governing = true
			}
		}
	}
	if !governing {
		t.Fatalf("no accepted ADR authority among the results: %s", stdout.Bytes())
	}

	ledger, err := os.ReadFile(filepath.Join(root, ".corvint", "self-observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Event            string `json:"event"`
		Authoritative    int    `json:"authoritativeResultCount"`
		AuthorityCounted bool   `json:"authorityCounted"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(ledger), &row); err != nil {
		t.Fatalf("ledger row: %v\n%s", err, ledger)
	}
	if row.Event != "user-prompt" || !row.AuthorityCounted || row.Authoritative != response.Context.Coverage.Authoritative {
		t.Fatalf("ledger row %+v does not record the packet's %d authoritative results", row, response.Context.Coverage.Authoritative)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--root", root, "observations"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("observations exit = %d, stderr = %s", code, &stderr)
	}
	digest := stdout.String()
	if !strings.HasPrefix(digest, "SELF-OBSERVATIONS events=1 zero-authoritative-rate=0/1 ") {
		t.Fatalf("digest misreports the adopter's authority:\n%s", digest)
	}
	if !strings.Contains(digest, "DEGRADATION-BY-DESIGN count=1 key=frontier-authority-unavailable ") || strings.Contains(digest, "key=frontier-authority-unavailable STANDING") {
		t.Fatalf("the fixed frontier degradation reads as a defect:\n%s", digest)
	}
}
