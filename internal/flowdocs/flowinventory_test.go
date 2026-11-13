package flowdocs

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const inventoryFixtureA = "class A\n def one\n  authorize(user)\n end\n def two\n  record.save!\n end\nend\n"

func inventoryFixture(t *testing.T) (string, string) {
	t.Helper()
	return fixture(t, map[string]string{
		"app/a.rb":     inventoryFixtureA,
		"app/b.js":     "function go() {\n fetch('/api');\n}\n",
		"app/c.rb":     "class C\n def unused\n  authorize(user)\n end\nend\n",
		"app/dup.rb":   "class D\n def same\n end\n def same\n end\nend\n",
		"other/x.rb":   "class X\n def outside\n end\nend\n",
		"app/notes.md": "not source\n",
	})
}

const inventoryInput = `{"schema":"corvint-flow-inventory/1","flows":[
 {"id":"checkout","title":"Checkout","members":[{"path":"app/a.rb","symbol":"A#one"},{"path":"app/b.js"}],"entry_points":[{"path":"app/b.js"}]},
 {"id":"admin","title":"Admin","members":[{"path":"app/a.rb","symbol":"A#two"},{"path":"app/missing.rb"},{"path":"app/a.rb","symbol":"A#nope"},{"path":"app/dup.rb","symbol":"D#same"}]},
 {"id":"ghost","title":"Ghost","members":[{"path":"app/gone.rb"}]}]}`

// TestFlowDocsInventoryAggregatesDeclaredFlows (AFU-V1-101..104, V1-2313):
// --flows produces exactly the declared flows, each aggregating its members'
// observations; unresolved members and unassigned declarations are reported,
// and Open rederives the manifest byte for byte.
func TestFlowDocsInventoryAggregatesDeclaredFlows(t *testing.T) {
	ctx := context.Background()
	root, rev := inventoryFixture(t)
	r, e := Generate(ctx, root, Options{Revision: rev, Scope: "app", Flows: []byte(inventoryInput)})
	if e != nil {
		t.Fatal(e)
	}
	m := r.Manifest
	if len(m.Flows) != 2 || m.Flows[0].ID != identity("flow", "inventory:checkout") || m.Flows[0].Name != "Checkout" || m.Flows[1].ID != identity("flow", "inventory:admin") || m.Flows[1].Name != "Admin" {
		t.Fatalf("flows are not exactly the declared ones: %+v", m.Flows)
	}
	claims := map[string]int{}
	for _, c := range m.Provider.Claims {
		claims[c.Subject]++
	}
	if claims[m.Flows[0].ID] != 2 || claims[m.Flows[1].ID] != 1 || len(m.Provider.Claims) != 3 {
		t.Fatalf("member observations not aggregated: %v", claims)
	}
	paths := map[string]bool{}
	for _, c := range m.Flows[0].Calls {
		paths[c.Anchor.Path] = true
	}
	if !paths["app/a.rb"] || !paths["app/b.js"] {
		t.Fatalf("calls not aggregated across members: %+v", m.Flows[0].Calls)
	}
	entry := false
	for _, p := range m.Flows[0].Paragraphs {
		entry = entry || p.Section == "entry-points" && p.Anchor.Path == "app/b.js" && strings.HasPrefix(p.Text, "Declared entry point")
	}
	if !entry {
		t.Fatal("declared entry point not rendered in entry-points")
	}
	inv := m.Inventory
	if inv == nil || len(inv.Flows) != 3 {
		t.Fatalf("inventory report: %+v", inv)
	}
	admin, ghost := inv.Flows[1], inv.Flows[2]
	if !inv.Flows[0].Generated || len(inv.Flows[0].Unresolved) != 0 || len(inv.Flows[0].Resolved) != 2 {
		t.Fatalf("checkout report: %+v", inv.Flows[0])
	}
	reasons := []string{}
	for _, u := range admin.Unresolved {
		reasons = append(reasons, u.Path+" "+u.Symbol+": "+u.Reason)
	}
	want := []string{
		"app/missing.rb : path has no unambiguous supported declaration at this revision",
		"app/a.rb A#nope: symbol is not a supported declaration in this path at this revision",
		"app/dup.rb D#same: symbol declaration is ambiguous in this path at this revision",
	}
	if strings.Join(reasons, "|") != strings.Join(want, "|") || !admin.Generated {
		t.Fatalf("admin unresolved members: %q", reasons)
	}
	if ghost.Generated || len(ghost.Unresolved) != 1 || ghost.Flow != identity("flow", "inventory:ghost") {
		t.Fatalf("ghost flow with no resolved member: %+v", ghost)
	}
	if len(inv.Unassigned) != 1 || inv.Unassigned[0] != (InventoryMember{Path: "app/c.rb", Symbol: "C#unused"}) {
		t.Fatalf("unassigned declarations: %+v", inv.Unassigned)
	}
	readme := string(r.Files[strings.TrimPrefix(m.Flows[1].ID, "flowdocs:flow:")+"/README.md"])
	if !strings.Contains(readme, "Unresolved member app/missing\\.rb") {
		t.Fatalf("admin README omits unresolved member:\n%s", readme)
	}
	if _, e = Open(ctx, root, r.Files["generation.json"]); e != nil {
		t.Fatalf("inventory manifest does not rederive: %v", e)
	}
	if e = os.WriteFile(filepath.Join(root, "generated.json"), r.Files["provider.json"], 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "generated.json")
	git(t, root, "commit", "-qm", "generated provider")
	if _, e = Finalize(ctx, root, r, git(t, root, "rev-parse", "HEAD"), "generated.json"); e != nil {
		t.Fatalf("inventory provider does not finalize: %v", e)
	}
	plain, e := Generate(ctx, root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	if plain.Manifest.Inventory != nil || strings.Contains(string(plain.Files["generation.json"]), `"inventory"`) || len(plain.Manifest.Flows) != 4 {
		t.Fatalf("absent --flows changed the declaration-flow behaviour: %d flows", len(plain.Manifest.Flows))
	}
}

// TestFlowDocsInventoryCheckIdentities (AFU-V1-105, V1-2313): check keeps
// per-claim fresh/stale/unresolved against the declared inventory identities.
func TestFlowDocsInventoryCheckIdentities(t *testing.T) {
	ctx := context.Background()
	root, rev := inventoryFixture(t)
	r, e := Generate(ctx, root, Options{Revision: rev, Scope: "app", Flows: []byte(inventoryInput)})
	if e != nil {
		t.Fatal(e)
	}
	checkout, admin := identity("flow", "inventory:checkout"), identity("flow", "inventory:admin")
	jsClaim, rbClaim, adminClaim := "", "", ""
	for _, c := range r.Manifest.Provider.Claims {
		switch {
		case c.Subject == checkout && c.Evidence.Anchors[0].Path == "app/b.js":
			jsClaim = c.ID
		case c.Subject == checkout:
			rbClaim = c.ID
		case c.Subject == admin:
			adminClaim = c.ID
		}
	}
	if jsClaim == "" || rbClaim == "" || adminClaim == "" || rbClaim != identity("claim", checkout+":app/a.rb:A#one:"+oneKey(t)) {
		t.Fatalf("claim identities are not inventory-scoped: js=%s rb=%s admin=%s", jsClaim, rbClaim, adminClaim)
	}
	// Change A#one's authorization span and remove A#two entirely.
	edited := "# moved\nclass A\n def one\n  authorize(admin)\n end\nend\n"
	if e = os.WriteFile(filepath.Join(root, "app", "a.rb"), []byte(edited), 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "commit", "-qam", "edit")
	checked, e := CheckRevision(ctx, root, r.Files["generation.json"], git(t, root, "rev-parse", "HEAD"), "")
	if e != nil {
		t.Fatal(e)
	}
	states := map[string]string{}
	for _, b := range checked.Bindings {
		states[b.ID] = b.State
	}
	if states[jsClaim] != "fresh" || states[rbClaim] != "stale" || states[adminClaim] != "unresolved" || checked.Clean {
		t.Fatalf("js=%s rb=%s admin=%s clean=%v", states[jsClaim], states[rbClaim], states[adminClaim], checked.Clean)
	}
	if _, ok := states[checkout]; !ok {
		t.Fatal("declared flow identity not checked")
	}
	retired := strings.Join(checked.Retired, " ")
	if !strings.Contains(retired, adminClaim) {
		t.Fatalf("removed member claim not retired: %v", checked.Retired)
	}
}

// oneKey is the lexical claim key of A#one's single observation.
func oneKey(t *testing.T) string {
	t.Helper()
	ds, _ := extract(contextindex.Source{Path: "app/a.rb", Data: []byte(inventoryFixtureA)})
	for _, d := range ds {
		if d.name == "A#one" && len(d.observations) == 1 {
			return d.observations[0].key
		}
	}
	t.Fatal("A#one observation missing")
	return ""
}

// TestFlowDocsInventoryRefusals (AFU-V1-101, AFU-V1-106): malformed,
// duplicate, overlapping and out-of-scope inventories are refused, never
// repaired.
func TestFlowDocsInventoryRefusals(t *testing.T) {
	head := `{"schema":"corvint-flow-inventory/1","flows":[`
	cases := map[string]string{
		"unknown member":      head + `{"id":"a","title":"A","members":[{"path":"app/a.rb"}],"extra":1}]}`,
		"schema":              `{"schema":"other/1","flows":[{"id":"a","title":"A","members":[{"path":"app/a.rb"}]}]}`,
		"no flows":            head + `]}`,
		"duplicate id":        head + `{"id":"a","title":"A","members":[{"path":"app/a.rb"}]},{"id":"a","title":"B","members":[{"path":"app/b.js"}]}]}`,
		"bad id":              head + `{"id":"A b","title":"A","members":[{"path":"app/a.rb"}]}]}`,
		"empty title":         head + `{"id":"a","title":"","members":[{"path":"app/a.rb"}]}]}`,
		"no members":          head + `{"id":"a","title":"A","members":[]}]}`,
		"escaping path":       head + `{"id":"a","title":"A","members":[{"path":"../a.rb"}]}]}`,
		"duplicate member":    head + `{"id":"a","title":"A","members":[{"path":"app/a.rb"},{"path":"app/a.rb"}]}]}`,
		"overlapping member":  head + `{"id":"a","title":"A","members":[{"path":"app/a.rb"},{"path":"app/a.rb","symbol":"A#one"}]}]}`,
		"foreign entry point": head + `{"id":"a","title":"A","members":[{"path":"app/a.rb"}],"entry_points":[{"path":"app/b.js"}]}]}`,
	}
	for name, raw := range cases {
		if _, e := ParseFlowInventory([]byte(raw)); e == nil || !strings.Contains(e.Error(), "flowdocs-refused") {
			t.Errorf("%s: accepted: %v", name, e)
		}
	}
	root, rev := inventoryFixture(t)
	outside := head + `{"id":"a","title":"A","members":[{"path":"other/x.rb"}]}]}`
	if _, e := Generate(context.Background(), root, Options{Revision: rev, Scope: "app", Flows: []byte(outside)}); e == nil || !strings.Contains(e.Error(), "outside --scope") {
		t.Fatalf("out-of-scope member accepted: %v", e)
	}
}

// AFU-V1-108: an entry-point refusal names the flow id and the entry point's path and symbol, and
// says whether it is foreign to the flow or a repeat.
func TestAFUV1108EntryPointRefusalNamesFlowAndMember(t *testing.T) {
	head := `{"schema":"corvint-flow-inventory/1","flows":[`
	cases := map[string]struct{ raw, want string }{
		"foreign": {head + `{"id":"checkout","title":"A","members":[{"path":"app/a.rb"}],"entry_points":[{"path":"app/b.js","symbol":"pay"}]}]}`,
			`flow "checkout" entry point path "app/b.js" symbol "pay" is not a member of the flow`},
		"repeat": {head + `{"id":"checkout","title":"A","members":[{"path":"app/a.rb"}],"entry_points":[{"path":"app/a.rb"},{"path":"app/a.rb"}]}]}`,
			`flow "checkout" entry point path "app/a.rb" symbol "" repeats an earlier entry point`},
	}
	for name, c := range cases {
		if _, e := ParseFlowInventory([]byte(c.raw)); e == nil || !strings.Contains(e.Error(), c.want) {
			t.Errorf("%s: refusal %v does not contain %q", name, e, c.want)
		}
	}
}

const topLevelWeb = `export interface Props {
  id: string;
}
export type Mode = "a" | "b";
export const Status = { ok: "ok" } as const;
export type Status = (typeof Status)[keyof typeof Status];
export class Client {
  load() {
    return fetch("/api");
  }
}
export abstract class Base {}
export const enum Kind { A }
enum Plain { B }
let counter = 0;
var legacy = 1;
export const api = {
  load() {
    return fetch("/x");
  },
};
export interface Button {
  label: string;
}
export const Button = () => {
  render();
};
function helper() {
  const inner = 1;
}
`

// AFU-V1-107: with a declared inventory, top-level classes, enums, consts, lets, vars,
// interfaces and type aliases resolve as members; a value outranks a same-named type and a
// function or arrow outranks both; nested declarations and object members do not resolve and
// the refusal names how to reference them. Without an inventory none of them becomes a flow.
func TestAFUV1107TopLevelWebDeclarationsResolveAsMembers(t *testing.T) {
	source := contextindex.Source{Path: "app/web.ts", Data: []byte(topLevelWeb)}
	plain, _ := extract(source)
	names := []string{}
	for _, d := range plain {
		names = append(names, d.name)
	}
	if strings.Join(names, ",") != "Button,helper" {
		t.Fatalf("extract without an inventory = %q, want only the arrow and the function", names)
	}
	ds, _ := extractMembers(source)
	lines := map[string][]int{}
	for _, d := range ds {
		lines[d.name] = append(lines[d.name], d.line)
	}
	want := map[string][]int{"Props": {1}, "Mode": {4}, "Status": {5}, "Client": {7}, "Base": {12}, "Kind": {13}, "Plain": {14}, "counter": {15}, "legacy": {16}, "api": {17}, "Button": {25}, "helper": {28}}
	if len(lines) != len(want) {
		t.Fatalf("member declarations %v, want %v", lines, want)
	}
	for name, at := range want {
		if !slices.Equal(lines[name], at) {
			t.Errorf("%s at %v, want %v", name, lines[name], at)
		}
	}
	for _, d := range ds {
		if d.name == "Client" && d.end != 11 || d.name == "api" && d.end != 21 {
			t.Errorf("%s extent %d..%d", d.name, d.line, d.end)
		}
	}

	root, rev := fixture(t, map[string]string{"app/web.ts": topLevelWeb})
	members := []string{}
	for name := range want {
		members = append(members, `{"path":"app/web.ts","symbol":"`+name+`"}`)
	}
	slices.Sort(members)
	members = append(members, `{"path":"app/web.ts","symbol":"load"}`, `{"path":"app/web.ts","symbol":"inner"}`)
	flows := `{"schema":"corvint-flow-inventory/1","flows":[{"id":"web","title":"Web","members":[` + strings.Join(members, ",") + `]}]}`
	ctx := context.Background()
	r, e := Generate(ctx, root, Options{Revision: rev, Scope: "app", Flows: []byte(flows)})
	if e != nil {
		t.Fatal(e)
	}
	report := r.Manifest.Inventory.Flows[0]
	if len(report.Resolved) != len(want) || len(report.Unresolved) != 2 {
		t.Fatalf("resolved %d unresolved %+v", len(report.Resolved), report.Unresolved)
	}
	for _, u := range report.Unresolved {
		if u.Reason != "symbol is not a top-level declaration in this path at this revision; name a class or object member's enclosing declaration" {
			t.Errorf("unresolved %s reason %q", u.Symbol, u.Reason)
		}
	}
	if _, e = Open(ctx, root, r.Files["generation.json"]); e != nil {
		t.Fatalf("inventory manifest does not rederive: %v", e)
	}
	bare, e := Generate(ctx, root, Options{Revision: rev, Scope: "app"})
	if e != nil {
		t.Fatal(e)
	}
	flowNames := []string{}
	for _, f := range bare.Manifest.Flows {
		flowNames = append(flowNames, f.Name)
	}
	slices.Sort(flowNames)
	if strings.Join(flowNames, ",") != "Button,helper" {
		t.Fatalf("flows without an inventory = %q", flowNames)
	}
}
