package breakagemap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/extevidence"
)

// TestJSDeclarationSpans pins BKM-V0-011: top-level JS/TS declarations
// resolve to their physical lines, and every shape the token extractor cannot
// bound fails closed.
func TestJSDeclarationSpans(t *testing.T) {
	for name, test := range map[string]struct {
		source     string
		start, end int
	}{
		"function":           {"import x from 'x'\nexport async function checkout(a: number): Promise<void> {\n  if (a) { return }\n}\nfunction other() {}\n", 2, 4},
		"generator":          {"function* checkout() {\n  yield 1\n}\n", 1, 3},
		"generic":            {"export function checkout<T extends { id: string }>(item: T): T {\n  return item\n}\n", 1, 3},
		"class":              {"export default class checkout extends Base<Item> {\n  run() { return 1 }\n}\n", 1, 3},
		"arrow":              {"const a = 1\nexport const checkout = async (cart: Cart): Promise<void> => {\n  await pay(cart)\n};\n", 2, 4},
		"single param arrow": {"const checkout = cart => {\n  return cart\n}\nexport default checkout\n", 1, 3},
		"function value":     {"var checkout = function named() {\n  return 1\n}\n", 1, 3},
		"interface":          {"export interface checkout {\n  id: string\n}\n", 1, 3},
	} {
		start, end, ok := jsDeclaration(test.source, "checkout")
		if !ok || start != test.start || end != test.end {
			t.Errorf("%s = %d..%d %v, want %d..%d", name, start, end, ok, test.start, test.end)
		}
	}
	for name, source := range map[string]string{
		"absent":             "function other() {}\n",
		"overloads":          "export function checkout(a: string): void;\nexport function checkout(a: any) {\n  return a\n}\n",
		"reassigned":         "function checkout() {}\ncheckout = other\n",
		"compound assign":    "let checkout = () => {}\ncheckout += 1\n",
		"decorated class":    "@Injectable()\nclass checkout {\n}\n",
		"expression arrow":   "const checkout = (a) => a + 1\n",
		"typed binding":      "const checkout: Handler = () => {}\n",
		"object return type": "function checkout(): { id: string } {\n  return { id: '' }\n}\n",
		"called after body":  "const checkout = function () {\n  return 1\n}.bind(null)\n",
		"function value use": "const checkout = function () {\n  return 1\n}\n(other)()\n",
		"expression":         "const handler = function checkout() {}\n",
		"unbalanced":         "function checkout() {\n",
		"unsure lexer":       "function checkout() { return 'open\n}\n",
		"type alias":         "type checkout = { id: string }\n",
	} {
		if start, end, ok := jsDeclaration(source, "checkout"); ok {
			t.Errorf("%s resolved %d..%d", name, start, end)
		}
	}
	if _, _, ok := declarationSpan("view.tsx", []byte("export function checkout() {\n  return <a>{'x'}</a>\n}\n"), "checkout"); ok {
		t.Error("TSX resolved; JSX text is outside the trusted extractor")
	}
	if _, _, ok := declarationSpan("checkout.py", []byte("def checkout():\n  pass\n"), "checkout"); ok {
		t.Error("Python resolved without an extractor")
	}
}

// TestBreakageTypeScriptDeclarationChange pins BKM-V0-011 end to end: a
// TypeScript target resolves selected-declaration-changed/unchanged instead
// of the file-level bound, and an ambiguous declaration stays file-level.
func TestBreakageTypeScriptDeclarationChange(t *testing.T) {
	root := initRepo(t)
	put(t, root, "src/api.ts", "export function checkout(cart: Cart) {\n  return pay(cart)\n}\n\nexport function refund() {\n  return 0\n}\n")
	commit(t, root)
	base := command(t, root, "rev-parse", "HEAD")
	compile := func(contents, symbol string) Report {
		t.Helper()
		put(t, root, "src/api.ts", contents)
		if command(t, root, "status", "--porcelain") != "" {
			commit(t, root)
		}
		repo := pinRepo(t, "web", root)
		m := Manifest{Schema: "corvint-breakage-manifest/0", Repositories: []Repository{repo}, Sources: []Source{pinSource(t, repo, root, "src/api.ts")}, Providers: []json.RawMessage{}}
		r, err := Compile(context.Background(), encode(t, m), map[string]string{"web": root}, "web:src/api.ts:"+symbol, base)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	refundChanged := "export function checkout(cart: Cart) {\n  return pay(cart)\n}\n\nexport function refund() {\n  return 1\n}\n"
	if r := compile(refundChanged, "checkout"); r.Change != "selected-declaration-unchanged" {
		t.Fatalf("unchanged checkout = %+v", r)
	}
	if r := compile(refundChanged, "refund"); r.Change != "selected-declaration-changed" {
		t.Fatalf("changed refund = %+v", r)
	}
	overloaded := "export function checkout(cart: Cart): void;\nexport function checkout(cart: any) {\n  return pay(cart)\n}\n"
	r := compile(overloaded, "checkout")
	if r.Change != "declaration-unresolved-file-changed" {
		t.Fatalf("overloaded checkout = %+v", r)
	}
	unresolved, unsupported := false, false
	for _, u := range r.Unknowns {
		unresolved = unresolved || strings.Contains(u.Reason, "not inferred")
		unsupported = unsupported || strings.HasPrefix(u.Reason, "unsupported-language")
	}
	if !unresolved || !unsupported {
		t.Fatalf("unknowns = %+v", r.Unknowns)
	}
}

// TestBreakageProviderV3SpanSymbol pins BKM-V0-012: an EEP /3 endpoint span or
// symbol narrows its anchor through the pinned bytes only, and a symbol that
// does not resolve, or lies outside its span or the supplied span, is an
// unresolved edge with an unknown rather than a guess.
func TestBreakageProviderV3SpanSymbol(t *testing.T) {
	m, bindings, base := fixture(t)
	var record extevidence.Record1
	if err := json.Unmarshal(m.Providers[0], &record); err != nil {
		t.Fatal(err)
	}
	compile := func(from, to extevidence.Endpoint1) (Report, Edge) {
		t.Helper()
		v3 := record
		v3.Schema = extevidence.Schema3
		v3.Relations = []extevidence.Relation1{record.Relations[0]}
		v3.Relations[0].From, v3.Relations[0].To = from, to
		raw, err := json.Marshal(v3)
		if err != nil {
			t.Fatal(err)
		}
		manifest := m
		manifest.Providers = []json.RawMessage{raw}
		r, err := Compile(context.Background(), encode(t, manifest), bindings, "api:pkg/api.go:Changed", base)
		if err != nil {
			t.Fatal(err)
		}
		for _, edge := range r.Edges {
			if edge.Relation != nil {
				return r, edge
			}
		}
		t.Fatalf("no provider edge: %+v", r.Edges)
		return r, Edge{}
	}
	from, to := record.Relations[0].From, record.Relations[0].To
	symbol, span := from, to
	symbol.Symbol = "Changed"
	span.Span = &extevidence.Span1{Start: 1, End: 1}
	_, edge := compile(symbol, span)
	if edge.ByteState != "verified" || edge.From.Start != 2 || edge.From.End != 2 || edge.To.Start != 1 || edge.To.End != 1 || !strings.Contains(edge.Reason, "narrowed to a pinned span or declared symbol") {
		t.Fatalf("narrowed edge = %+v from=%+v to=%+v", edge, edge.From, edge.To)
	}
	missing, outside, beyond := from, from, to
	missing.Symbol = "Missing"
	outside.Symbol, outside.Span = "Changed", &extevidence.Span1{Start: 3, End: 3}
	beyond.Span = &extevidence.Span1{Start: 1, End: 99}
	for name, test := range map[string]struct {
		from, to extevidence.Endpoint1
		reason   string
	}{
		"unresolved symbol": {missing, to, "symbol not resolved to one declaration"},
		"symbol off span":   {outside, to, "declaration outside its declared span"},
		"beyond source":     {from, beyond, "outside supplied physical span"},
	} {
		r, edge := compile(test.from, test.to)
		hit := false
		for _, u := range r.Unknowns {
			hit = hit || strings.Contains(u.Reason, test.reason)
		}
		if edge.ByteState != "unresolved" || !hit {
			t.Errorf("%s: edge=%+v unknowns=%+v", name, edge, r.Unknowns)
		}
	}
	v2 := record
	v2.Relations = []extevidence.Relation1{record.Relations[0]}
	v2.Relations[0].From = symbol
	raw, _ := json.Marshal(v2)
	manifest := m
	manifest.Providers = []json.RawMessage{raw}
	if _, err := Compile(context.Background(), encode(t, manifest), bindings, "api:pkg/api.go:Changed", base); err == nil {
		t.Fatal("a /2 record carrying a symbol must be refused")
	}
}
