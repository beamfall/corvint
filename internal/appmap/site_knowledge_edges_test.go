package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// skEdgeRoutes declares resource routes the rails-routes/0 dialect resolves to screens.
const skEdgeRoutes = `Rails.application.routes.draw do
  root to: "home#index"
  resources :orders
  resources :invoices
  resources :reports do
    member do
      get "preview"
    end
  end
  get "status", to: "status#show", as: :status
  post "test_setup/orders", to: "test_setup#orders", as: :test_setup_orders
end
`

type skEdgesDoc struct {
	Schema     string         `json:"schema"`
	Provenance map[string]int `json:"provenance"`
	Kinds      map[string]int `json:"static_kinds"`
	Reasons    map[string]int `json:"static_unknown_reasons"`
	Also       int            `json:"static_also_observed"`
	Edges      []SKEdge       `json:"edges"`
	Unknowns   []SKUnknown    `json:"unknowns"`
}

func skLinksOf(t *testing.T, root string, m *Map, sources ...string) (skEdgesDoc, []byte) {
	t.Helper()
	raw, err := Links(context.Background(), m, sources, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := Links(context.Background(), m, sources, Options{Root: root, Full: true}); !bytes.Equal(raw, again) {
		t.Fatal("links rerun differs (SKH-V0-007)")
	}
	var doc skEdgesDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	return doc, raw
}

// skEdgeKey summarises a static edge: kind, line, from, to and status with reason.
func skEdgeKey(e SKEdge) string {
	from := e.From
	if from == "" {
		from = "?" + e.FromReason
	}
	to := e.To
	if to == "" {
		to = "?" + e.Reason
	}
	return e.Kind + "@" + e.Anchor.Path + ":" + strconv.Itoa(e.Anchor.Start) + " " + from + " -> " + to
}

// SKH-V0-040 SKH-V0-041 SKH-V0-042 SKH-V0-043 SKH-V0-110 SKH-V0-111 SKH-V0-112 SKH-V0-113
// SKH-V0-114 SKH-V0-115: anchors, ui-sref, routerLink, router links, form actions, link_to,
// button_to, form_with, redirect_to, $state.go and navigate are static edges with provenance
// static; Rails views and controller actions bind the source screen; literal targets resolve
// through the screen index and state names; dynamic and relative targets are UNKNOWN at their
// line; external links are skipped; unsupported template languages are named unknowns; and the
// map's test-observed and flow-declared edges are listed beside them, never merged.
func TestSKHV0_110_115_StaticNavigationEdges(t *testing.T) {
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		files["app/views/orders/index.html.erb"] = `<h1>Orders</h1>
<%= link_to "New invoice", new_invoice_path %>
<a href="/reports/1/preview">Preview</a>
<form action="/orders" method="post"></form>
<%= link_to order_path(@order) do %>Open<% end %>
<a href="https://example.invalid/help">Help</a> <a href="#top">Top</a>
<%= button_to "Ship", ship_path, method: :post %>
<%= link_to "Back", :back %>
<a href="<%= orders_path %>">Self</a>
<link href="/assets/app.css">
`
		files["app/views/orders/_row.html.erb"] = `<%= link_to "Invoices", "/invoices" %>` + "\n"
		files["app/views/orders/show.html.haml"] = "%a{href: '/orders'} Orders\n"
		files["app/controllers/orders_controller.rb"] = strings.Replace(railsFixtureFiles()["app/controllers/orders_controller.rb"],
			"    check_lock\n", "    check_lock\n    redirect_to orders_path\n", 1)
		files["app/javascript/nav.tsx"] = `export function Nav({ id }) {
  const navigate = useNavigate();
  // navigate('/commented')
  return (<nav>
    <Link to="/orders">Orders</Link>
    <a href={` + "`/orders/${id}`" + `}>One</a>
    <a data-href="/invoices">x</a>
    <button onClick={() => navigate('/invoices/new')}>New</button>
    <button onClick={() => navigate(target)}>Go</button>
  </nav>);
}
function go($state) { $state.go('orders'); $state.go('.child'); router.navigate(['/orders', id]); }
`
		files["app/javascript/menu.html"] = `<a ui-sref="invoices">Invoices</a>
<a ui-sref="{{ state }}">Dyn</a>
<a routerLink="/status">Status</a>
<a [routerLink]="['/reports', '1', 'preview']">Preview</a>
<a routerLink="relative">Rel</a>
`
	})
	// The map's own edges: one test-observed pair the static links repeat, and one flow step.
	at := Anchor{Path: "e2e/specs/orders.spec.ts", Start: 1, End: 3}
	m.Edges = append(m.Edges, Edge{From: screenID(m.App, "orders"), To: screenID(m.App, "order"), Basis: "test-sequence", Source: "file:e2e/specs/orders.spec.ts", Anchor: at},
		Edge{From: screenID(m.App, "order"), To: screenID(m.App, "invoices"), Basis: "flow-step", Source: "step:pay", Anchor: at})
	doc, _ := skLinksOf(t, root, m, "app")
	if doc.Schema != EdgesSchema {
		t.Fatalf("schema %s", doc.Schema)
	}
	id := func(state string) string { return screenID(m.App, state) }
	orders := id("orders")
	got := map[string]bool{}
	static, observed, declared := 0, 0, 0
	for _, e := range doc.Edges {
		switch e.Provenance {
		case ProvenanceStatic:
			static++
			if e.Authority != "learned" || e.Anchor.Blob == "" || e.Anchor.SpanSHA256 == "" {
				t.Fatalf("static edge not anchored or not learned: %+v (SKH-V0-001)", e)
			}
			got[skEdgeKey(e)] = true
		case ProvenanceTestObserved:
			observed++
			if e.From != orders || e.To != id("order") {
				t.Fatalf("observed edge %+v", e)
			}
		case ProvenanceFlowDeclared:
			declared++
		default:
			t.Fatalf("unexpected provenance %+v (SKH-V0-110)", e)
		}
	}
	view := "app/views/orders/index.html.erb"
	nav := "app/javascript/nav.tsx"
	menu := "app/javascript/menu.html"
	want := []string{
		LinkRailsLink + "@" + view + ":2 " + orders + " -> " + id("new_invoice"),
		LinkHref + "@" + view + ":3 " + orders + " -> " + id("preview_report"),
		LinkFormAction + "@" + view + ":4 " + orders + " -> " + id("orders"),
		LinkRailsLink + "@" + view + ":5 " + orders + " -> " + id("order"),
		LinkRailsButton + "@" + view + ":7 " + orders + " -> ?no-matching-screen",
		LinkRailsLink + "@" + view + ":8 " + orders + " -> ?dynamic-target",
		LinkHref + "@" + view + ":9 " + orders + " -> ?dynamic-target",
		LinkRailsLink + "@app/views/orders/_row.html.erb:1 ?source-not-bound -> " + id("invoices"),
		LinkRedirect + "@app/controllers/orders_controller.rb:16 ?no-screen-for-action -> " + orders,
		LinkTo + "@" + nav + ":5 ?source-not-bound -> " + orders,
		LinkHref + "@" + nav + ":6 ?source-not-bound -> ?dynamic-target",
		LinkNavigate + "@" + nav + ":8 ?source-not-bound -> ?ambiguous-template",
		LinkNavigate + "@" + nav + ":9 ?source-not-bound -> ?dynamic-target",
		LinkStateGo + "@" + nav + ":12 ?source-not-bound -> " + orders,
		LinkStateGo + "@" + nav + ":12 ?source-not-bound -> ?relative-state",
		LinkNavigate + "@" + nav + ":12 ?source-not-bound -> ?dynamic-target",
		LinkUISref + "@" + menu + ":1 ?source-not-bound -> " + id("invoices"),
		LinkUISref + "@" + menu + ":2 ?source-not-bound -> ?dynamic-target",
		LinkRouterLink + "@" + menu + ":3 ?source-not-bound -> " + id("status"),
		LinkRouterLink + "@" + menu + ":4 ?source-not-bound -> " + id("preview_report"),
		LinkRouterLink + "@" + menu + ":5 ?source-not-bound -> ?relative-target",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing static edge %s", w)
		}
	}
	if static != len(want) || observed != 1 || declared != 1 {
		keys := []string{}
		for k := range got {
			keys = append(keys, k)
		}
		t.Fatalf("static %d (want %d), observed %d: %v", static, len(want), observed, keys)
	}
	if doc.Also != 1 || doc.Provenance[ProvenanceStatic] != len(want) || doc.Provenance[ProvenanceTestObserved] != 1 {
		t.Fatalf("counts = %+v %d (SKH-V0-110)", doc.Provenance, doc.Also)
	}
	if len(doc.Unknowns) != 1 || doc.Unknowns[0].Path != "app/views/orders/show.html.haml" || doc.Unknowns[0].Reason != "unsupported-template-language" {
		t.Fatalf("unknowns = %+v (SKH-V0-114)", doc.Unknowns)
	}
}

// SKH-V0-111 SKH-V0-113 SKH-V0-114: review regressions. A literal Ruby target in a .rb file keeps
// its text; `link_to(...) do` takes the block-form target; a routerLink array with a root segment
// resolves instead of being dropped; a script `href` assignment is no anchor; and a secret-shaped
// literal target is screened, never printed.
func TestSKHV0_114_StaticLinkRegressions(t *testing.T) {
	rb := parseRubyStructure("app/controllers/a_controller.rb", []byte("class AController\n  def show\n    redirect_to \"/orders\" # \"/nope\"\n  end\nend\n"))
	sites := rubyLinks(rb.code, rb.raw)
	if len(sites) != 1 || sites[0].dynamic || sites[0].target != "/orders" || sites[0].space != "url" {
		t.Fatalf("ruby literal = %+v", sites)
	}
	erb := []string{`<%= link_to(order_path(o), class: "btn") do %>Open<% end %>`}
	if s := rubyLinks(erb, erb); len(s) != 1 || s[0].dynamic || s[0].target != "order" || s[0].space != "helper" {
		t.Fatalf("parenthesised block form = %+v", s)
	}
	if s := templateLinks(`<a [routerLink]="['/', 'orders']">x</a>`); len(s) != 1 || s[0].target != "/orders" {
		t.Fatalf("root-segment routerLink = %+v", s)
	}
	if s := templateLinks("const href = '/orders';\n"); len(s) != 0 {
		t.Fatalf("script assignment read as an anchor: %+v", s)
	}
	secret := "/reset?token=ghp_" + strings.Repeat("a", 36)
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		files["app/views/orders/index.html.erb"] = `<a href="` + secret + `">Reset</a>` + "\n"
	})
	doc, raw := skLinksOf(t, root, m, "app/views")
	if strings.Contains(string(raw), "ghp_") || len(doc.Edges) != 1 || doc.Edges[0].Reason != "screened-target" || doc.Edges[0].Target != "" {
		t.Fatalf("secret target printed or unscreened: %s (SKH-V0-006)", raw)
	}
}

// AMAP-V0-191, SKH-V0-113: with a router base path, a router-relative link (`to`, `routerLink`,
// a router or history call, a bare navigate()) resolves without the base path, while an href and
// location.assign are browser URLs that carry it.
func TestSKHV0_113_BasePathRouterRelativeLinks(t *testing.T) {
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		files["app/javascript/nav.tsx"] = "export const Nav = () => (\n  <nav>\n    <Link to=\"/orders\">A</Link>\n    <a href=\"/web/orders\">B</a>\n    <a href=\"/orders\">C</a>\n  </nav>\n);\n" +
			"export function go() {\n  navigate(\"/orders\");\n  location.assign(\"/web/orders\");\n  location.assign(\"/orders\");\n}\n"
	})
	m.BasePath = "/web"
	doc, raw := skLinksOf(t, root, m, "app/javascript")
	got := map[int]string{}
	for _, e := range doc.Edges {
		got[e.Anchor.Start] = e.Status + ":" + e.Reason
	}
	for line, want := range map[int]string{3: StatusResolved + ":", 4: StatusResolved + ":", 5: StatusUnknown + ":outside-base-path",
		9: StatusResolved + ":", 10: StatusResolved + ":", 11: StatusUnknown + ":outside-base-path"} {
		if got[line] != want {
			t.Errorf("line %d: %q, want %q in %s", line, got[line], want, raw)
		}
	}
}
