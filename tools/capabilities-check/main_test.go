// SPDX-License-Identifier: Apache-2.0
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
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree"
)

const (
	catalogPath = "../../docs/CAPABILITIES.md"
	readmePath  = "../../README.md"
	helpPath    = "../../cmd/corvint/help.go"
	mainPath    = "../../cmd/corvint/main.go"
	cmdPath     = "../../cmd"
	mcpPath     = "../../internal/mcp"
	internalDir = "../../internal"
	linkBase    = "../../docs"
)

func load(t *testing.T) (Inventory, []byte, []byte) {
	t.Helper()
	inv, err := Collect(helpPath, mainPath, cmdPath, mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := MergeSubcommands(&inv, "../.."); err != nil {
		t.Fatal(err)
	}
	if inv.Literals, err = ScanLiterals([]string{cmdPath, internalDir}); err != nil {
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
	writeFiles(t, root, syntheticRepository())
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

// syntheticRepository is the smallest repository the whole check passes over.
func syntheticRepository() map[string]string {
	return map[string]string{
		"cmd/corvint/help.go": "package main\n\nconst rootHelp = `Usage:\n\nCommands:\n  init  first inventory\n\n" +
			"  Core (decision 0332):\n    init\n  Experimental, no stability promise:\n    none\n`\n",
		"cmd/corvint/main.go": "package main\n\nvar topLevelCommands = []string{\"init\"}\n",
		"cmd/corvint/tables.go": "package main\n\nvar helpSubcommands = map[string]map[string]bool{}\nvar docsSubcommands = []string{}\n" +
			"var flowSubcommands = map[string]bool{}\nvar leaseActions = []string{}\n",
		"cmd/corvint-mcp/main.go": "package main\n\nconst tool = \"corvint.query\"\n",
		"internal/mcp/tools.go":   "package mcp\n",
		"docs/CAPABILITIES.md": "<!-- capability-gate:commands -->\n| Command | Maturity |\n|---|---|\n| `corvint init` | core |\n<!-- /capability-gate:commands -->\n" +
			"<!-- capability-gate:binaries -->\n| Binary | Maturity |\n|---|---|\n| `corvint` | core |\n| `corvint-mcp` | experimental |\n<!-- /capability-gate:binaries -->\n" +
			"<!-- capability-gate:mcp-tools -->\n| Tool | Maturity |\n|---|---|\n| `corvint.query` | experimental |\n<!-- /capability-gate:mcp-tools -->\n",
		"README.md":                "<!-- capability-gate:readme -->\n| Capability | Try it | Maturity |\n|---|---|---|\n| Init | `corvint init` | core |\n<!-- /capability-gate:readme -->\n",
		ratchetPath:                "binary\treason\ncorvint-mcp\tsynthetic\n",
		treesDir + "/corvint.json": syntheticCorvintTree(),
		llmsPath: "- docs/CAPABILITIES.md\n- corvint help --json\n- corvint help guide\n" +
			"- corvint help --tree (" + treesDir + "/corvint.json)\n",
	}
}

// syntheticCorvintTree is the command-tree snapshot of the synthetic repository's corvint.
func syntheticCorvintTree() string {
	builder := clitree.NewBuilder("corvint")
	builder.AddText("corvint init\ncorvint help [--json]\n", "")
	builder.Declare([]string{"init"}, "first inventory")
	builder.Declare([]string{"help"}, "help")
	encoded, err := clitree.Encode(builder.Tree("corvint help --tree", "synthetic", []string{"cmd/corvint"}, nil, nil))
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
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

// TestCAPV0002GroupedRootHelp pins the grouped Commands: layout (V1-0945): the Core group is the
// Core list, and a header or verb line that does not fit the layout fails instead of passing.
func TestCAPV0002GroupedRootHelp(t *testing.T) {
	const head = "const rootHelp = `Usage:\n\nCommands:\n"
	commands, core, experimental, err := parseCommands([]byte(head +
		"  Core (decision 0332):\n    init  first inventory\n" +
		"  Experimental, no stability promise:\n    harness event  one event (AHI)\n\n`\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(commands, ",") != "init,harness event" || !core["init"] || core["harness"] || len(core) != 1 {
		t.Fatalf("commands %v core %v", commands, core)
	}
	if !experimental["harness"] || experimental["init"] || len(experimental) != 1 {
		t.Fatalf("experimental %v", experimental)
	}
	for name, block := range map[string]string{
		"unknown header":    "  Core (x):\n    init  a\n  Deprecated:\n    old  b\n",
		"repeated header":   "  Core (x):\n    init  a\n  Core (y):\n    adopt  b\n",
		"misaligned":        "  Core (x):\n    init  a\n    Experimental, no stability promise:\n      feature  b\n",
		"not indented":      "  Core (x):\n  init  a\n",
		"header after verb": "  init  a\n  Core (x):\n    adopt  b\n",
		"empty core":        "  Experimental, no stability promise:\n    feature  b\n",
		"no description":    "  Core (x):\n    init\n",
	} {
		if _, _, _, err := parseCommands([]byte(head + block + "\n`\n")); err == nil {
			t.Errorf("%s: grouped root help accepted", name)
		}
	}
}

// replaceOnce returns doc with old replaced by new, failing when old is not present exactly once.
func replaceOnce(t *testing.T, doc []byte, old, new string) []byte {
	t.Helper()
	if n := bytes.Count(doc, []byte(old)); n != 1 {
		t.Fatalf("%q occurs %d times in the catalog", old, n)
	}
	return bytes.Replace(doc, []byte(old), []byte(new), 1)
}

// TestCAPV0007UndocumentedSubcommandFails proves the subcommand half of the gate fails without
// the fix: a subcommand dropped from its verb's row, or one registered in code but never listed.
func TestCAPV0007UndocumentedSubcommandFails(t *testing.T) {
	inv, catalog, readme := load(t)
	for _, verb := range []string{"cem", "ocm", "step", "docs", "flows", "lease"} {
		if len(inv.Subcommands[verb]) == 0 {
			t.Errorf("no subcommands collected for %q: %v", verb, inv.Subcommands)
		}
	}
	for _, sub := range []string{"gate-verdict", "anchor"} {
		if !contains(inv.Subcommands["cem"], sub) {
			t.Errorf("cem subcommand %q not followed through cemHelpActions", sub)
		}
	}
	if !contains(inv.Subcommands["flows"], "request-path") || !contains(inv.Subcommands["flows"], "appmap") {
		t.Errorf("flows init registrations not collected: %v", inv.Subcommands["flows"])
	}
	dropped := replaceOnce(t, catalog, "`gate-verdict`, ", "")
	if got := kinds(Check(dropped, readme, inv, linkBase))["subcommand-undocumented"]; !contains(got, "cem gate-verdict") || len(got) != 1 {
		t.Errorf("dropped cem subcommand: subcommand-undocumented %v", got)
	}
	inv.Subcommands["lease"] = append(inv.Subcommands["lease"], "transfer")
	if got := kinds(Check(catalog, readme, inv, linkBase))["subcommand-undocumented"]; !contains(got, "lease transfer") || len(got) != 1 {
		t.Errorf("new lease subcommand: subcommand-undocumented %v", got)
	}
}

// TestCAPV0007SubcommandTablesParse pins the table reader on a synthetic package: a nested map
// following an identifier, a slice, a map with an init registration, and a missing table.
func TestCAPV0007SubcommandTablesParse(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nvar cemActions = map[string]bool{\"verify\": true}\n" +
		"var helpSubcommands = map[string]map[string]bool{\"cem\": cemActions, \"ocm\": {\"link\": true}}\n" +
		"var docsSubcommands = []string{\"nav\"}\nvar flowSubcommands = map[string]func(){\"map\": nil}\n" +
		"func init() { flowSubcommands[\"navigate\"] = nil }\nvar leaseActions = []string{\"list\"}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package main\n\nfunc init() { flowSubcommands[\"ghost\"] = nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parseSubcommands(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"cem": "verify", "ocm": "link", "docs": "nav", "flows": "map,navigate", "lease": "list"}
	for verb, subs := range want {
		if strings.Join(got[verb], ",") != subs {
			t.Errorf("%s: got %v, want %s", verb, got[verb], subs)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(strings.Replace(src, "leaseActions", "leaseVerbs", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseSubcommands(dir); err == nil || !strings.Contains(err.Error(), "leaseActions") {
		t.Errorf("renamed table: err %v", err)
	}
}

// TestCAPV0008ExperimentalMaturityMismatchFails proves an Experimental-group verb whose row
// claims another maturity fails, where the Core-only comparison let it pass.
func TestCAPV0008ExperimentalMaturityMismatchFails(t *testing.T) {
	inv, catalog, readme := load(t)
	if !inv.Experimental["lease"] || inv.Experimental["init"] {
		t.Fatalf("experimental group not collected: %v", inv.Experimental)
	}
	drifted := replaceOnce(t, catalog, "| `corvint lease list` | Lease JSON naming whether it mutated | experimental |", "| `corvint lease list` | Lease JSON naming whether it mutated | implemented |")
	if got := kinds(Check(drifted, readme, inv, linkBase))["maturity"]; !contains(got, "lease") || len(got) != 1 {
		t.Errorf("maturity findings %v", got)
	}
}

// TestCAPV0009RemovedRefusalCodeFails proves a listed refusal code that no source literal holds
// fails: a code renamed in the row, and a code removed from the code while the row keeps it.
func TestCAPV0009RemovedRefusalCodeFails(t *testing.T) {
	inv, catalog, readme := load(t)
	stale := replaceOnce(t, catalog, "`attest-public-key-unavailable`", "`public-key-unavailable`")
	if got := kinds(Check(stale, readme, inv, linkBase))["refusal-unknown"]; !contains(got, "corvint prove") || len(got) != 1 {
		t.Errorf("renamed code: refusal-unknown %v", got)
	}
	if !inv.Literals["attest-public-key-unavailable"] {
		t.Fatal("refusal code literal not scanned")
	}
	delete(inv.Literals, "attest-public-key-unavailable")
	if got := kinds(Check(catalog, readme, inv, linkBase))["refusal-unknown"]; !contains(got, "corvint prove") || len(got) != 1 {
		t.Errorf("removed code: refusal-unknown %v", got)
	}
	for _, prose := range []string{"--mutate", "cem/0.x", "context --task TASK --limit 5", ".corvint/x.json", "exit"} {
		if refusalCode.MatchString(prose) {
			t.Errorf("%q matched as a refusal code", prose)
		}
	}
}

// TestCAPV0960TreeSubcommandUndocumentedFails proves a subcommand dispatched outside the declared
// tables is checked through the corvint command tree: `flows coverage` is in no table, yet
// dropping it from the flows row fails.
func TestCAPV0960TreeSubcommandUndocumentedFails(t *testing.T) {
	declared, err := parseSubcommands("../../cmd/corvint")
	if err != nil {
		t.Fatal(err)
	}
	if contains(declared["flows"], "coverage") {
		t.Fatal("flows coverage is now in a declared table; pick another tree-only subcommand")
	}
	inv, catalog, readme := load(t)
	if !contains(inv.Subcommands["flows"], "coverage") || !contains(inv.Subcommands["runs"], "identity") {
		t.Fatalf("tree subcommands not merged: flows %v, runs %v", inv.Subcommands["flows"], inv.Subcommands["runs"])
	}
	dropped := replaceOnce(t, catalog, "include `coverage`, ", "include ")
	if got := kinds(Check(dropped, readme, inv, linkBase))["subcommand-undocumented"]; !contains(got, "flows coverage") || len(got) != 1 {
		t.Errorf("dropped tree subcommand: subcommand-undocumented %v", got)
	}
	if err := MergeSubcommands(&Inventory{}, t.TempDir()); err == nil {
		t.Error("missing corvint tree: want an error")
	}
}

// TestCAPV0961StaleDocumentedSubcommandFails proves a subcommand a row still lists after the code
// dropped it is reported, while option spans and parenthesised detail are not read as subcommands.
func TestCAPV0961StaleDocumentedSubcommandFails(t *testing.T) {
	inv, catalog, readme := load(t)
	stale := replaceOnce(t, catalog, "Subcommands `acquire`, `renew`,", "Subcommands `acquire`, `transfer`, `renew`,")
	if got := kinds(Check(stale, readme, inv, linkBase))["subcommand-stale"]; !contains(got, "lease transfer") || len(got) != 1 {
		t.Errorf("stale lease subcommand: subcommand-stale %v", got)
	}
	inv.Subcommands["lease"] = slices.DeleteFunc(slices.Clone(inv.Subcommands["lease"]), func(s string) bool { return s == "renew" })
	got := kinds(Check(catalog, readme, inv, linkBase))
	if !contains(got["subcommand-stale"], "lease renew") || len(got["subcommand-stale"]) != 1 {
		t.Errorf("removed lease subcommand: subcommand-stale %v", got["subcommand-stale"])
	}
}

// TestCAPV0961DocumentedSubcommandsParse pins the Subcommands sentence reader.
func TestCAPV0961DocumentedSubcommandsParse(t *testing.T) {
	cells := []string{"`corvint x`", "Use it. Subcommands include `a`, `b c` (detail; `--flag F` and `d`), `-x`, `e`. Later `f` is prose."}
	if got := strings.Join(documentedSubcommands(cells), ","); got != "a,b,e" {
		t.Errorf("got %s, want a,b,e", got)
	}
	if got := documentedSubcommands([]string{"no sentence `a`"}); len(got) != 0 {
		t.Errorf("no Subcommands sentence: got %v", got)
	}
}

// TestCAPV0965SubcommandSpansAreNames (V1-3661) proves the Subcommands sentence is typed: a flag,
// an argument or an example written as a depth-zero span is subcommand-span-malformed, the same
// detail inside parentheses is not, and the committed catalog has no such span.
func TestCAPV0965SubcommandSpansAreNames(t *testing.T) {
	inv, catalog, readme := load(t)
	if got := kinds(Check(catalog, readme, inv, linkBase))["subcommand-span-malformed"]; len(got) != 0 {
		t.Fatalf("committed catalog: subcommand-span-malformed %v", got)
	}
	for _, c := range []struct{ span, want string }{
		{"`acquire --lease-minutes N`", "lease `acquire --lease-minutes N`"},
		{"`--json`", "lease `--json`"},
		{"`corvint lease acquire`", "lease `corvint lease acquire`"},
	} {
		bad := replaceOnce(t, catalog, "Subcommands `acquire`, `renew`,", "Subcommands `acquire`, "+c.span+", `renew`,")
		if got := kinds(Check(bad, readme, inv, linkBase))["subcommand-span-malformed"]; !contains(got, c.want) || len(got) != 1 {
			t.Errorf("%s: subcommand-span-malformed %v", c.span, got)
		}
	}
	parenthesised := replaceOnce(t, catalog, "Subcommands `acquire`, `renew`,", "Subcommands `acquire` (`acquire --lease-minutes N`), `renew`,")
	if got := kinds(Check(parenthesised, readme, inv, linkBase))["subcommand-span-malformed"]; len(got) != 0 {
		t.Errorf("parenthesised detail: subcommand-span-malformed %v", got)
	}
}

// TestCAPV0962RefusalCodeMatchesWholeLiteral proves a refusal code no longer passes because some
// longer literal contains it as a word; only a literal equal to the code keeps it accepted.
func TestCAPV0962RefusalCodeMatchesWholeLiteral(t *testing.T) {
	inv, catalog, readme := load(t)
	delete(inv.Literals, "attest-public-key-unavailable")
	inv.Literals["refused: attest-public-key-unavailable for this key"] = true
	if got := kinds(Check(catalog, readme, inv, linkBase))["refusal-unknown"]; !contains(got, "corvint prove") || len(got) != 1 {
		t.Errorf("code only inside prose: refusal-unknown %v", got)
	}
	dir := t.TempDir()
	src := "package p\n\nconst code = \"owned-code\"\nvar msg = `see other-code here`\n"
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	literals, err := ScanLiterals([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !literals["owned-code"] || literals["other-code"] || !literals["see other-code here"] {
		t.Errorf("literals %v", literals)
	}
}

// TestCAPV0002JobGroupedRootHelp pins the V1-3730 layout: several Experimental job headers
// follow the one Core header, a family line labels each of its verbs, and a hiddenCommands verb
// counts as documented and experimental although no Commands line lists it.
func TestCAPV0002JobGroupedRootHelp(t *testing.T) {
	src := "const rootHelp = `Usage:\n\nCommands:\n" +
		"  Core (decision 0332):\n    init  alias of adopt\n" +
		"  Experimental, context and review:\n    features | overview  guidance (RGV-V0)\n" +
		"  Experimental, tests:\n    test  affected tests (STR-V0)\n\n`\n" +
		"var hiddenCommands = map[string]string{\n\t\"lrf\": \"floor (LRF-V0)\",\n}\n"
	commands, core, experimental, err := parseCommands([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(commands, ",") != "init,features,overview,test,lrf" || len(core) != 1 || !core["init"] {
		t.Fatalf("commands %v core %v", commands, core)
	}
	for _, verb := range []string{"features", "overview", "test", "lrf"} {
		if !experimental[verb] {
			t.Errorf("%s is not experimental: %v", verb, experimental)
		}
	}
	if _, _, _, err := parseCommands([]byte(strings.Replace(src, "  Experimental, tests:\n", "  Core (again):\n", 1))); err == nil {
		t.Error("a Core header after an Experimental header passed")
	}
}
