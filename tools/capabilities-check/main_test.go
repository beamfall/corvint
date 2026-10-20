// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const (
	catalogPath = "../../docs/CAPABILITIES.md"
	readmePath  = "../../README.md"
	helpPath    = "../../cmd/corvint/help.go"
	cmdPath     = "../../cmd"
	mcpPath     = "../../internal/mcp"
	linkBase    = "../../docs"
)

func load(t *testing.T) (Inventory, []byte, []byte) {
	t.Helper()
	inv, err := Collect(helpPath, cmdPath, mcpPath)
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
