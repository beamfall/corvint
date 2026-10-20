// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	catalogPath = "../../docs/CAPABILITIES.md"
	readmePath  = "../../README.md"
	helpPath    = "../../cmd/corvint/help.go"
	mainPath    = "../../cmd/corvint/main.go"
	cmdPath     = "../../cmd"
	mcpPath     = "../../internal/mcp"
	linkBase    = "../../docs"
)

func load(t *testing.T) (Inventory, []byte, []byte) {
	t.Helper()
	inv, err := Collect(helpPath, mainPath, cmdPath, mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	return inv, catalog, readme
}

func kinds(findings []Finding) map[string][]string {
	out := map[string][]string{}
	for _, f := range findings {
		out[f.Kind] = append(out[f.Kind], f.Name)
	}
	return out
}

// TestCAPV0002RepositoryCatalogMatchesInventory is the drift gate itself: the committed
// catalog and README must agree with the code on this revision.
func TestCAPV0002RepositoryCatalogMatchesInventory(t *testing.T) {
	inv, catalog, readme := load(t)
	for _, f := range Check(catalog, readme, inv, linkBase) {
		t.Errorf("%s (edit the docs/CAPABILITIES.md or README row; see docs/specs/capability-catalog-v0.md)", f)
	}
}

// TestCAPV0002InventoryCountsMatchCode pins the collectors to known members so a parser
// that silently returns nothing cannot pass the gate.
func TestCAPV0002InventoryCountsMatchCode(t *testing.T) {
	inv, _, _ := load(t)
	if len(inv.Commands) < 40 || len(inv.Binaries) < 40 || len(inv.Tools) < 30 {
		t.Fatalf("inventory too small: %d commands, %d binaries, %d tools", len(inv.Commands), len(inv.Binaries), len(inv.Tools))
	}
	for _, c := range []string{"init", "context", "harness event", "delta"} {
		if !contains(inv.Commands, c) {
			t.Errorf("command %q not collected", c)
		}
	}
	for _, c := range []string{"init", "adopt", "index", "query", "context", "impact", "affected", "prove", "cem", "ocm", "frontier", "dogfood"} {
		if !inv.Core[c] {
			t.Errorf("Core verb %q not collected", c)
		}
	}
	if inv.Core["feature"] || inv.Core["Experimental"] {
		t.Errorf("Core list reads past its section: %v", inv.Core)
	}
	for _, c := range []string{"init", "harness", "delta", "tests"} {
		if !contains(inv.Dispatched, c) {
			t.Errorf("dispatched verb %q not collected", c)
		}
	}
	for _, b := range []string{"corvint", "corvint-mcp", "corvint-lsp", "corvint-tasks", "corvint-analyzer-go"} {
		if !contains(inv.Binaries, b) {
			t.Errorf("binary %q not collected", b)
		}
	}
	for _, tool := range []string{"corvint.query", "corvint.cem.report", "corvint.flows.navigate", "corvint.docs_draft", "corvint.test_validity", "corvint.docs_trace", "corvint.map_plan"} {
		if !contains(inv.Tools, tool) {
			t.Errorf("MCP tool %q not collected", tool)
		}
	}
}

// TestCAPV0003SyntheticOmissionAndStaleRowFail proves the gate fails without the fix: a
// deleted row is missing and a row for something that does not exist is stale.
func TestCAPV0003SyntheticOmissionAndStaleRowFail(t *testing.T) {
	inv, catalog, _ := load(t)
	cases := []struct {
		name, drop, add, kind, want string
	}{
		{"command omitted", "| `corvint delta` |", "", "missing", "delta"},
		{"binary omitted", "| `corvint-lsp` |", "", "missing", "corvint-lsp"},
		{"tool omitted", "| `corvint.docs_trace` |", "", "missing", "corvint.docs_trace"},
		{"stale command", "", "| `corvint retired-verb` | q | u | `corvint retired-verb` | o | experimental | r | s |\n", "stale", "retired-verb"},
		{"duplicate command", "", "| `corvint delta` | q | u | `corvint delta` | o | experimental | r | s |\n", "duplicate", "delta"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := catalog
			if tc.drop != "" {
				doc = dropLine(t, doc, tc.drop)
			}
			if tc.add != "" {
				doc = insertAfterCommandHeader(t, doc, tc.add)
			}
			got := kinds(Check(doc, nil, inv, linkBase))
			if !contains(got[tc.kind], tc.want) {
				t.Fatalf("want %s %q, got %v", tc.kind, tc.want, got)
			}
		})
	}
	// An inventory item the catalog never heard of is also missing.
	inv.Binaries = append(append([]string(nil), inv.Binaries...), "corvint-new-companion")
	if got := kinds(Check(catalog, nil, inv, linkBase)); !contains(got["missing"], "corvint-new-companion") {
		t.Fatalf("new binary not reported missing: %v", got)
	}
}

// TestCAPV0002ToolNamesWithDigitsAndHyphens proves a tool name with a digit or a hyphen is
// collected, so a catalog that omits it fails.
func TestCAPV0002ToolNamesWithDigitsAndHyphens(t *testing.T) {
	src := []byte(`package mcp
var names = []string{"corvint.docs_trace_v2", "corvint.docs-trace", "corvint.flows.navigate"}
`)
	got := toolNames(src)
	for _, want := range []string{"corvint.docs_trace_v2", "corvint.docs-trace", "corvint.flows.navigate"} {
		if !contains(got, want) {
			t.Fatalf("toolNames missed %q: %v", want, got)
		}
	}
	inv, catalog, _ := load(t)
	inv.Tools = append(append([]string(nil), inv.Tools...), "corvint.docs_trace_v2", "corvint.docs-trace")
	missing := kinds(Check(catalog, nil, inv, linkBase))["missing"]
	if !contains(missing, "corvint.docs_trace_v2") || !contains(missing, "corvint.docs-trace") {
		t.Fatalf("uncatalogued digit/hyphen tools not reported missing: %v", missing)
	}
}

// TestCAPV0002DispatchDriftFails proves a verb runContext dispatches without a root help line,
// or a help line nothing dispatches, fails the gate.
func TestCAPV0002DispatchDriftFails(t *testing.T) {
	inv, catalog, _ := load(t)
	added := inv
	added.Dispatched = append(append([]string(nil), inv.Dispatched...), "teleport")
	got := kinds(Check(catalog, nil, added, linkBase))
	if !contains(got["dispatch-undocumented"], "teleport") || !contains(got["missing"], "teleport") {
		t.Fatalf("dispatched but undocumented verb not reported: %v", got)
	}
	removed := inv
	removed.Dispatched = nil
	for _, verb := range inv.Dispatched {
		if verb != "delta" {
			removed.Dispatched = append(removed.Dispatched, verb)
		}
	}
	if got := kinds(Check(catalog, nil, removed, linkBase)); !contains(got["dispatch-absent"], "delta") {
		t.Fatalf("documented but undispatched verb not reported: %v", got)
	}
	if _, err := parseDispatched([]byte("package main\nvar other = []string{\"init\"}\n")); err == nil {
		t.Fatal("a main.go without topLevelCommands was accepted")
	}
}

// TestCAPV0003MaturityAndLinkDriftFail covers the maturity and broken-link findings.
func TestCAPV0003MaturityAndLinkDriftFail(t *testing.T) {
	inv, catalog, _ := load(t)
	line := lineWith(t, catalog, "| `corvint delta` |")
	doc := bytes.Replace(catalog, []byte(line), []byte(strings.Replace(line, "| experimental |", "| core |", 1)), 1)
	if got := kinds(Check(doc, nil, inv, linkBase)); !contains(got["maturity"], "delta") {
		t.Fatalf("experimental command labelled core not reported: %v", got)
	}
	line = lineWith(t, catalog, "| `corvint impact` |")
	doc = bytes.Replace(catalog, []byte(line), []byte(strings.Replace(line, "| core |", "| experimental |", 1)), 1)
	if got := kinds(Check(doc, nil, inv, linkBase)); !contains(got["maturity"], "impact") {
		t.Fatalf("Core command labelled experimental not reported: %v", got)
	}
	line = lineWith(t, catalog, "| `corvint delta` |")
	doc = bytes.Replace(catalog, []byte(line), []byte(strings.Replace(line, "immutable-delta-v0.md", "no-such-spec.md", 1)), 1)
	if got := kinds(Check(doc, nil, inv, linkBase)); !contains(got["broken-link"], "delta") {
		t.Fatalf("broken link not reported: %v", got)
	}
}

// TestCAPV0004ReadmeRowsMapToCatalog checks that each README capability row resolves to a
// catalog row with the same maturity, and that an unmapped or overstated row fails.
func TestCAPV0004ReadmeRowsMapToCatalog(t *testing.T) {
	inv, catalog, readme := load(t)
	rows, err := parseRows(readme, sectionReadme)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 8 {
		t.Fatalf("README capability table has %d rows", len(rows))
	}
	header := "| Capability | What you get | Try it | Maturity |\n|---|---|---|---|\n"
	unmapped := []byte("<!-- capability-gate:readme -->\n" + header + "| X | y | `corvint teleport --now` | experimental |\n<!-- /capability-gate:readme -->\n")
	if got := kinds(Check(catalog, unmapped, inv, linkBase)); !contains(got["readme-unmapped"], "corvint teleport --now") {
		t.Fatalf("unmapped README row not reported: %v", got)
	}
	oversold := []byte("<!-- capability-gate:readme -->\n" + header + "| X | y | `corvint harness event --host codex` | core |\n| Z | w | `corvint-mcp --root /r` | implemented |\n<!-- /capability-gate:readme -->\n")
	got := kinds(Check(catalog, oversold, inv, linkBase))
	if !contains(got["readme-maturity"], "corvint harness event --host codex") || !contains(got["readme-maturity"], "corvint-mcp --root /r") {
		t.Fatalf("overstated README maturity not reported: %v", got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func lineWith(t *testing.T, doc []byte, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("no line starting %q", prefix)
	return ""
}

func dropLine(t *testing.T, doc []byte, prefix string) []byte {
	t.Helper()
	line := lineWith(t, doc, prefix)
	return bytes.Replace(doc, []byte(line+"\n"), nil, 1)
}

func insertAfterCommandHeader(t *testing.T, doc []byte, add string) []byte {
	t.Helper()
	sep := "<!-- capability-gate:commands -->\n| Command |"
	at := bytes.Index(doc, []byte(sep))
	if at < 0 {
		t.Fatal("commands region not found")
	}
	rest := doc[at:]
	// Skip the region marker, the header and the separator line.
	idx := 0
	for range 3 {
		idx += bytes.IndexByte(rest[idx:], '\n') + 1
	}
	out := append([]byte(nil), doc[:at+idx]...)
	out = append(out, add...)
	return append(out, doc[at+idx:]...)
}

// TestCAPV0005RoutingPointersNameTheCatalog checks that every routing surface and every
// shipped skill points agents to docs/CAPABILITIES.md.
func TestCAPV0005RoutingPointersNameTheCatalog(t *testing.T) {
	for _, file := range []string{"../../AGENTS.md", "../../docs/README.md", "../../README.md"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(src, []byte("CAPABILITIES.md")) {
			t.Errorf("%s does not route to docs/CAPABILITIES.md", file)
		}
	}
	routes, err := os.ReadFile("../../docs/AGENT-ROUTES.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lineWith(t, routes, "| What capability exists?"), "CAPABILITIES.md") {
		t.Error(`docs/AGENT-ROUTES.md "What capability exists?" row does not route to docs/CAPABILITIES.md`)
	}
	skills := 0
	for _, dir := range []string{"../../integrations", "../../protocol/integrations"} {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() != "SKILL.md" && d.Name() != "GEMINI.md" {
				return nil
			}
			skills++
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if !bytes.Contains(src, []byte("docs/CAPABILITIES.md")) {
				t.Errorf("%s does not route to docs/CAPABILITIES.md", p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if skills < 10 {
		t.Fatalf("found %d shipped skill files, want at least 10", skills)
	}
}

// TestCAPV0006CheckIsReadOnly runs the whole check over a small synthetic repository and
// asserts that no file or directory was created, removed or changed.
func TestCAPV0006CheckIsReadOnly(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"cmd/corvint/help.go": "package main\n\nconst rootHelp = `Usage:\n\nCommands:\n  init  first inventory\n\n" +
			"  Core (decision 0332):\n    init\n  Experimental, no stability promise:\n    none\n`\n",
		"cmd/corvint/main.go":     "package main\n\nvar topLevelCommands = []string{\"init\"}\n",
		"cmd/corvint-mcp/main.go": "package main\n\nconst tool = \"corvint.query\"\n",
		"internal/mcp/tools.go":   "package mcp\n",
		"docs/CAPABILITIES.md": "<!-- capability-gate:commands -->\n| Command | Maturity |\n|---|---|\n| `corvint init` | core |\n<!-- /capability-gate:commands -->\n" +
			"<!-- capability-gate:binaries -->\n| Binary | Maturity |\n|---|---|\n| `corvint` | core |\n| `corvint-mcp` | experimental |\n<!-- /capability-gate:binaries -->\n" +
			"<!-- capability-gate:mcp-tools -->\n| Tool | Maturity |\n|---|---|\n| `corvint.query` | experimental |\n<!-- /capability-gate:mcp-tools -->\n",
		"README.md": "<!-- capability-gate:readme -->\n| Capability | Try it | Maturity |\n|---|---|---|\n| Init | `corvint init` | core |\n<!-- /capability-gate:readme -->\n",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotTree(t, root)
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 0 {
		t.Fatalf("synthetic repository: exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok (1 commands, 2 binaries, 1 MCP tools)") {
		t.Fatalf("synthetic repository: stdout %q", stdout.String())
	}
	if after := snapshotTree(t, root); after != before {
		t.Fatalf("check changed the tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestCAPV0006CheckImportsNoProcessOrNetwork pins the no-command, no-network half of CAP-V0-006:
// the check's non-test sources import no package that can start a process or open a socket.
func TestCAPV0006CheckImportsNoProcessOrNetwork(t *testing.T) {
	for _, file := range []string{"check.go", "main.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			name := strings.Trim(spec.Path.Value, `"`)
			if name == "os/exec" || name == "syscall" || name == "net" || strings.HasPrefix(name, "net/") {
				t.Errorf("%s imports %s", file, name)
			}
		}
	}
}

// snapshotTree describes every entry under root by path, mode, modification time and content digest.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		digest := ""
		if info.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			h := sha256.New()
			_, err = io.Copy(h, f)
			f.Close()
			if err != nil {
				return err
			}
			digest = fmt.Sprintf("%x", h.Sum(nil))
		}
		fmt.Fprintf(&out, "%s %v %d %s\n", p, info.Mode(), info.ModTime().UnixNano(), digest)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}
