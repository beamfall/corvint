package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
)

func TestCanonicalVSIXRequiresClosedAgreeingMembers(t *testing.T) {
	t.Run("PUB-V0-013 canonical-vsix-closed-members", func(t *testing.T) {
		members := make(map[string][]byte, len(vsixMembers))
		for _, name := range vsixMembers {
			members[name] = []byte("content:" + name)
		}
		a, err := canonicalVSIX(members)
		if err != nil {
			t.Fatal(err)
		}
		b, err := canonicalVSIX(members)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("canonical VSIX bytes differ")
		}
		decoded, err := readVSIXMembers(a)
		if err != nil {
			t.Fatal(err)
		}
		if err := equalVSIXMembers(members, decoded); err != nil {
			t.Fatal(err)
		}
		changed := make(map[string][]byte, len(members))
		for name, body := range members {
			changed[name] = append([]byte(nil), body...)
		}
		changed[vsixMembers[0]][0] ^= 1
		if err := equalVSIXMembers(members, changed); err == nil {
			t.Fatal("member disagreement accepted")
		}
		delete(changed, vsixMembers[0])
		if _, err := canonicalVSIX(changed); err == nil {
			t.Fatal("missing closed member accepted")
		}
	})
}

func TestVSIXToolIdentityRefusesPathAndSymlinkSwap(t *testing.T) {
	t.Run("PUB-V0-013 vsix-tool-identity-swap-refusal", func(t *testing.T) {
		dir := t.TempDir()
		a, b, tool := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "tool")
		if err := os.WriteFile(a, []byte("a"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(b, []byte("b"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(a, tool); err != nil {
			t.Fatal(err)
		}
		want, err := fileSHA256(tool)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(tool); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(b, tool); err != nil {
			t.Fatal(err)
		}
		if err := requireFileDigest(tool, want); err == nil {
			t.Fatal("symlink swap accepted")
		}
		if err := os.Remove(tool); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tool, []byte("a"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tool, []byte("b"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := requireFileDigest(tool, want); err == nil {
			t.Fatal("path replacement accepted")
		}
	})
}

func TestBuildOnlyNPMNoticeIsExplicitlyUnshipped(t *testing.T) {
	t.Run("PUB-V0-013 build-only-npm-license-inventory", func(t *testing.T) {
		lock := `{"packages":{"":{"version":"0.1.0","license":"SEE LICENSE IN LICENSE"},"node_modules/a":{"version":"1.2.3","license":"MIT","integrity":"sha512-x"}}}`
		entry, err := buildOnlyNPMNotice(Export{Files: []SourceFile{{Path: "extensions/vscode/package-lock.json", Data: []byte(lock)}}})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(entry.Data, []byte("not shipped")) || !bytes.Contains(entry.Data, []byte("Runtime dependencies: zero")) || !bytes.Contains(entry.Data, []byte("a\t1.2.3\tMIT\tsha512-x")) {
			t.Fatalf("notice: %s", entry.Data)
		}
	})
}

func TestMCPDiscoveryRequiresExactProtocolAndIdentity(t *testing.T) {
	t.Run("PUB-V0-015 mcp-discovery-protocol-identity", func(t *testing.T) {
		good := []map[string]any{{"result": map[string]any{"supportedVersions": []any{"2026-07-28"}, "resultType": "complete", "cacheScope": "public", "ttlMs": float64(0), "capabilities": map[string]any{"tools": map[string]any{}}, "_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{"name": "corvint-mcp", "version": "0.1.0-experimental"}}}}}
		if err := requireMCPDiscovery(good, 0, "corvint-mcp", "0.1.0-experimental"); err != nil {
			t.Fatal(err)
		}
		good[0]["result"].(map[string]any)["supportedVersions"] = []any{"old"}
		if err := requireMCPDiscovery(good, 0, "corvint-mcp", "0.1.0-experimental"); err == nil {
			t.Fatal("wrong protocol accepted")
		}
	})
}

func TestRetainedSmokeReportBindsArchiveOutsideArchive(t *testing.T) {
	t.Run("PUB-V0-015 retained-smoke-report-binding", func(t *testing.T) {
		entries := []ArchiveEntry{{Path: "bin/x", Mode: 0o755, Data: []byte("x")}, {Path: "SHA256SUMS", Mode: 0o644, Data: []byte("s")}}
		archive, err := buildTarGz(entries)
		if err != nil {
			t.Fatal(err)
		}
		opts := Options{Scratch: t.TempDir(), OutputParent: t.TempDir(), BundleName: "b"}
		report, err := qualifyAndRetain(t.Context(), opts, Report{}, entries, archive, func(context.Context, string) ([]SmokeStep, error) {
			return []SmokeStep{{Name: "x", OK: true, BundleSHA256: sha256Hex(archive)}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(report.SmokeReportPath)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			BundleSHA256 string      `json:"bundleSha256"`
			Steps        []SmokeStep `json:"steps"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.BundleSHA256 != sha256Hex(archive) || len(got.Steps) != 1 {
			t.Fatalf("report not bound: %+v", got)
		}
	})
}

func TestExportedPackageTreeAndManifestReferencesAreBound(t *testing.T) {
	t.Run("PUB-V0-014 host-package-tree-manifest-binding", func(t *testing.T) {
		export := Export{HeadCommit: "commit", HeadTree: "tree", Files: []SourceFile{
			{Path: "integrations/codex/README.md", Mode: "100644", Data: []byte("FALLBACK\n")},
			{Path: "integrations/codex/plugins/corvint/run", Mode: "100755", Data: []byte("run\n")},
		}}
		source := []byte("source")
		files, artifact, err := exportedPackageTree(export, "integrations/codex/", "source/corvint-src.tar.gz", sha256Hex(source))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, ArchiveEntry{Path: "source/corvint-src.tar.gz", Mode: 0o644, Data: source})
		manifest := BundleManifest{Artifacts: []ArtifactManifest{artifact}}
		if err := validateManifestReferences(files, manifest); err != nil {
			t.Fatal(err)
		}
		files[0].Data[0] ^= 1
		if err := validateManifestReferences(files, manifest); err == nil {
			t.Fatal("mutated package tree accepted")
		}
	})
}

func TestBinaryComponentsShareOneModuleSourceReference(t *testing.T) {
	t.Run("PUB-V0-003 shared-module-source-deduplication", func(t *testing.T) {
		export := Export{HeadCommit: "commit", HeadTree: "tree", Files: []SourceFile{{Path: "LICENSE", Mode: "100644", Data: []byte("license\n")}, {Path: "PROVENANCE.md", Mode: "100644", Data: []byte("provenance\n")}}}
		sourceFiles, sourcePath, sourceDigest, err := assembleModuleSource("corvint", export)
		if err != nil {
			t.Fatal(err)
		}
		firstFiles, first := assembleBinaryComponent("corvint", "corvint", export, BuiltBinary{SHA256: sha256Hex([]byte("a")), Size: 1}, []byte("a"), sourcePath, sourceDigest)
		secondFiles, second := assembleBinaryComponent("corvint-mcp", "corvint", export, BuiltBinary{SHA256: sha256Hex([]byte("b")), Size: 1}, []byte("b"), sourcePath, sourceDigest)
		if first.SourceArchivePath != second.SourceArchivePath || first.SourceArchiveSHA256 != second.SourceArchiveSHA256 {
			t.Fatal("components do not share source identity")
		}
		all := append(sourceFiles, append(firstFiles, secondFiles...)...)
		if err := validateManifestReferences(all, BundleManifest{Components: []ComponentManifest{first, second}}); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, file := range all {
			if file.Path == sourcePath {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("shared source occurs %d times", count)
		}
	})
}

func TestMCPResponseAcceptsProductionDiscovery(t *testing.T) {
	t.Run("PUB-V0-015 production-mcp-response-framing", func(t *testing.T) {
		srv, err := server.New(server.Config{
			Name: "corvint-mcp", Version: "0.1.0-experimental", Description: "Corvint <context> & evidence",
			Capabilities: map[string]any{"tools": map[string]any{}},
			Handler: server.HandlerFunc(func(context.Context, protocol.Request, server.Notifier) (map[string]any, *protocol.RPCError) {
				t.Error("discovery reached tool handler")
				return nil, nil
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(mcpRequest(1, "server/discover", map[string]any{"_meta": mcpMeta}))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := srv.Serve(t.Context(), bytes.NewReader(append(raw, '\n')), &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(out.Bytes(), []byte("Corvint <context> & evidence")) {
			t.Fatalf("production encoder escaping changed: %s", out.Bytes())
		}
		response, err := decodeMCPResponse(out.Bytes(), 1)
		if err != nil {
			t.Fatalf("production MCP response rejected: %v; %s", err, out.Bytes())
		}
		if err := requireMCPDiscovery([]map[string]any{response}, 0, "corvint-mcp", "0.1.0-experimental"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMCPResponseRefusesInvalidFramingAndIdentity(t *testing.T) {
	good := `{"jsonrpc":"2.0","id":1,"result":{}}`
	for name, raw := range map[string][]byte{
		"empty":         nil,
		"missing LF":    []byte(good),
		"extra frame":   []byte(good + "\n" + good + "\n"),
		"invalid JSON":  []byte("{\n"),
		"non object":    []byte("[]\n"),
		"null":          []byte("null\n"),
		"invalid UTF-8": append([]byte(`{"jsonrpc":"2.0","id":1,"result":"`), 0xff, '"', '}', '\n'),
		"over limit":    []byte(good + strings.Repeat(" ", protocol.MaxMessageBytes-len(good)+1) + "\n"),
		"wrong version": []byte(`{"jsonrpc":"1.0","id":1,"result":{}}` + "\n"),
		"wrong ID":      []byte(`{"jsonrpc":"2.0","id":2,"result":{}}` + "\n"),
		"wrong ID type": []byte(`{"jsonrpc":"2.0","id":"1","result":{}}` + "\n"),
		"missing ID":    []byte(`{"jsonrpc":"2.0","result":{}}` + "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeMCPResponse(raw, 1); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
	for _, raw := range []string{good + "\r\n", good + strings.Repeat(" ", protocol.MaxMessageBytes-len(good)) + "\n"} {
		if _, err := decodeMCPResponse([]byte(raw), 1); err != nil {
			t.Fatalf("valid frame rejected: %v", err)
		}
	}
}
