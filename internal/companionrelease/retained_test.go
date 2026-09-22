package companionrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRetainedBundleAndFreshExtraction(t *testing.T) {
	dir := makeRetainedFixture(t)
	verified, err := VerifyRetainedBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ArchiveSHA256 == "" || len(verified.Manifest.Components) != 9 {
		t.Fatalf("incomplete verification: %+v", verified)
	}
	target := filepath.Join(t.TempDir(), "extract")
	if err := verified.Extract(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "bin", "corvint")); err != nil {
		t.Fatal(err)
	}
	if err := verified.Extract(target); err == nil {
		t.Fatal("existing extraction target accepted")
	}
}

func TestVerifyRetainedBundleRejectsChangedSidecarAndSmoke(t *testing.T) {
	for _, mutate := range []func(string){
		func(dir string) { mustWrite(t, filepath.Join(dir, "fixture.tar.gz.sha256"), []byte("bad\n")) },
		func(dir string) {
			path := filepath.Join(dir, "fixture.smoke.json")
			body, _ := os.ReadFile(path)
			mustWrite(t, path, append(body, []byte(`{"extra":true}`)...))
		},
	} {
		dir := makeRetainedFixture(t)
		mutate(dir)
		if _, err := VerifyRetainedBundle(dir); err == nil {
			t.Fatal("tampered retained bundle accepted")
		}
	}
}

func TestVerifyRetainedBundleRejectsArbitraryPassingSmoke(t *testing.T) {
	dir := makeRetainedFixture(t)
	archive, err := os.ReadFile(filepath.Join(dir, "fixture.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}{sha256Hex(archive), []SmokeStep{{Name: "fixture", OK: true, BundleSHA256: sha256Hex(archive)}}})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "fixture.smoke.json"), append(body, '\n'))
	if _, err := VerifyRetainedBundle(dir); err == nil {
		t.Fatal("arbitrary passing smoke accepted")
	}
}

func makeRetainedFixture(t *testing.T) string {
	t.Helper()
	corvintSourceEntries := []ArchiveEntry{{Path: "go.mod", Mode: 0o644, Data: []byte("module fixture\n")}, {Path: "LICENSE", Mode: 0o644, Data: []byte("license")}, {Path: "PROVENANCE.md", Mode: 0o644, Data: []byte("provenance")}, {Path: "extensions/vscode/package-lock.json", Mode: 0o644, Data: []byte("{\"packages\":{}}\n")}}
	taskmanSourceEntries := []ArchiveEntry{{Path: "go.mod", Mode: 0o644, Data: []byte("module taskman\n")}, {Path: "LICENSE", Mode: 0o644, Data: []byte("license")}, {Path: "PROVENANCE.md", Mode: 0o644, Data: []byte("provenance")}}
	source, err := buildTarGz(corvintSourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest := sha256Hex(source)
	taskmanSource, err := buildTarGz(taskmanSourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	taskmanDigest := sha256Hex(taskmanSource)
	var components []ComponentManifest
	var entries []ArchiveEntry
	for name, module := range map[string]string{"corvint": "corvint", "corvint-console": "corvint", "corvint-dashboard-snapshot": "corvint", "corvint-mcp": "corvint", "corvint-docs-mcp": "corvint", "corvint-test-validity-mcp": "corvint", "corvint-js-test-provider": "corvint", "corvint-go-test-provider": "corvint", "atm": "corvint-taskman"} {
		data := []byte("binary-" + name)
		path := "bin/" + name
		entries = append(entries, ArchiveEntry{Path: path, Mode: 0o755, Data: data})
		sourcePath, digest := "source/corvint-src.tar.gz", sourceDigest
		if module == "corvint-taskman" {
			sourcePath, digest = "source/corvint-taskman-src.tar.gz", taskmanDigest
		}
		components = append(components, ComponentManifest{Name: name, Module: module, BinaryPath: path, BinarySHA256: sha256Hex(data), BinarySizeBytes: int64(len(data)), Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Tree: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SourceArchivePath: sourcePath, SourceArchiveSHA256: digest})
	}
	entries = append(entries, ArchiveEntry{Path: "source/corvint-src.tar.gz", Mode: 0o644, Data: source}, ArchiveEntry{Path: "source/corvint-taskman-src.tar.gz", Mode: 0o644, Data: taskmanSource})
	for _, module := range []string{"corvint", "corvint-taskman"} {
		entries = append(entries, ArchiveEntry{Path: "notices/" + module + "/LICENSE", Mode: 0o644, Data: []byte("license")}, ArchiveEntry{Path: "notices/" + module + "/PROVENANCE.md", Mode: 0o644, Data: []byte("provenance")}, thirdPartyNoticeEntry("notices/"+module, module))
	}
	npmNotice, err := buildOnlyNPMNotice(Export{Files: []SourceFile{{Path: "extensions/vscode/package-lock.json", Mode: "100644", Data: []byte("{\"packages\":{}}\n")}}})
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries, npmNotice)
	artifacts := []ArtifactManifest{}
	vsix := []byte("vsix")
	entries = append(entries, ArchiveEntry{Path: "extensions/corvint-vscode-0.1.0.vsix", Mode: 0o644, Data: vsix})
	artifacts = append(artifacts, ArtifactManifest{Name: "corvint-vscode", Kind: "vsix", Path: "extensions/corvint-vscode-0.1.0.vsix", SHA256: sha256Hex(vsix), SizeBytes: int64(len(vsix)), Commit: components[0].Commit, Tree: components[0].Tree, SourceArchivePath: "source/corvint-src.tar.gz", SourceArchiveSHA256: sourceDigest, Support: "FALLBACK"})
	for _, name := range []string{"codex", "claude-code", "gemini-cli", "opencode"} {
		entry := ArchiveEntry{Path: "plugins/" + name + "/README.md", Mode: 0o644, Data: []byte(name)}
		entries = append(entries, entry)
		h := treeDigestForTest(entry)
		artifacts = append(artifacts, ArtifactManifest{Name: name, Kind: "host-package-tree", Path: "plugins/" + name + "/", SHA256: h, SizeBytes: int64(len(entry.Data)), Commit: components[0].Commit, Tree: components[0].Tree, SourceArchivePath: "source/corvint-src.tar.gz", SourceArchiveSHA256: sourceDigest, Support: "FALLBACK"})
	}
	manifest := BundleManifest{Target: supportedTarget, GoVersion: "go1.27.1", GitVersion: "git version fixture", Components: components, Artifacts: artifacts, VSIXTools: VSIXToolchainManifest{NodeVersion: requiredNodeVersion, NodeSHA256: hex64, NPMVersion: requiredNPMVersion, NPMSHA256: hex64, TypeScriptVersion: requiredTypeScriptVersion, TypeScriptSHA256: hex64, VSCEVersion: requiredVSCEVersion, VSCESHA256: hex64, LockfileSHA256: hex64}}
	entries, err = assembleBundleEntries(entries, manifest)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256Hex(archive)
	componentByName := map[string]ComponentManifest{}
	for _, component := range components {
		componentByName[component.Name] = component
	}
	var smokeSteps []SmokeStep
	for _, name := range []string{"atm-version", "atm-help", "corvint-mcp-version", "corvint-docs-mcp-version", "corvint-test-validity-mcp-version", "corvint-mcp-discover-list-status", "corvint-docs-mcp-draft-consume", "corvint-test-validity-mcp-list", "corvint-js-test-provider-help", "corvint-go-test-provider-help", "atm-init", "atm-ticket-create", "atm-ticket-refine", "corvint-dashboard-snapshot", "console-listen", "console-board", "console-detail", "console-create-form", "console-refuse-cross-origin", "console-refuse-missing-token", "console-refusal-no-store-effect", "console-detail-controls", "console-edit", "console-evidence", "console-requirement-links", "console-code-links", "console-link-gaps", "console-stop-no-descendants"} {
		componentName := "corvint-console"
		switch {
		case strings.HasPrefix(name, "atm-"):
			componentName = "atm"
		case strings.HasPrefix(name, "corvint-dashboard"):
			componentName = "corvint-dashboard-snapshot"
		case strings.HasPrefix(name, "corvint-mcp"):
			componentName = "corvint-mcp"
		case strings.HasPrefix(name, "corvint-docs"):
			componentName = "corvint-docs-mcp"
		case strings.HasPrefix(name, "corvint-test"):
			componentName = "corvint-test-validity-mcp"
		case strings.HasPrefix(name, "corvint-js"):
			componentName = "corvint-js-test-provider"
		case strings.HasPrefix(name, "corvint"):
			componentName = "corvint-go-test-provider"
		}
		component := componentByName[componentName]
		smokeSteps = append(smokeSteps, SmokeStep{Name: name, OK: true, BundleSHA256: digest, ComponentSHA256: component.BinarySHA256, SourceCommit: component.Commit, SourceTree: component.Tree, InvokedPath: "/private/removed/" + component.BinaryPath})
	}
	smoke, _ := json.Marshal(struct {
		BundleSHA256 string      `json:"bundleSha256"`
		Steps        []SmokeStep `json:"steps"`
	}{digest, smokeSteps})
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "fixture.tar.gz"), archive)
	mustWrite(t, filepath.Join(dir, "fixture.tar.gz.sha256"), []byte(digest+"  fixture.tar.gz\n"))
	mustWrite(t, filepath.Join(dir, "fixture.smoke.json"), append(smoke, '\n'))
	return dir
}

const hex64 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func treeDigestForTest(entry ArchiveEntry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%06o\x00%d\x00", entry.Path, entry.Mode, len(entry.Data))
	_, _ = h.Write(entry.Data)
	return hex.EncodeToString(h.Sum(nil))
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRetainedBundleRequiresEachBoundMCPVersion(t *testing.T) {
	for _, component := range []string{"corvint-mcp", "corvint-docs-mcp", "corvint-test-validity-mcp"} {
		for _, mutation := range []string{"missing", "duplicate", "wrong component"} {
			t.Run(component+"/"+mutation, func(t *testing.T) {
				dir := makeRetainedFixture(t)
				path := filepath.Join(dir, "fixture.smoke.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var smoke struct {
					BundleSHA256 string      `json:"bundleSha256"`
					Steps        []SmokeStep `json:"steps"`
				}
				if err := json.Unmarshal(raw, &smoke); err != nil {
					t.Fatal(err)
				}
				index := -1
				for i, step := range smoke.Steps {
					if step.Name == component+"-version" {
						index = i
					}
				}
				if index < 0 {
					t.Fatal("fixture lacks MCP version step")
				}
				switch mutation {
				case "missing":
					smoke.Steps = append(smoke.Steps[:index], smoke.Steps[index+1:]...)
				case "duplicate":
					for _, step := range smoke.Steps {
						if step.Name != component+"-version" {
							smoke.Steps[index] = step
						}
					}
				case "wrong component":
					smoke.Steps[index].ComponentSHA256 = strings.Repeat("0", 64)
				}
				raw, err = json.Marshal(smoke)
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, path, append(raw, '\n'))
				if _, err := VerifyRetainedBundle(dir); err == nil {
					t.Fatal("invalid MCP version evidence accepted")
				}
			})
		}
	}
}
