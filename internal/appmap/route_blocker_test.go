package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const blockerRoutes = `Rails.application.routes.draw do
  scope "api" do
    resources :widgets, only: [] do
      member do
        post :archive
      end
    end
  end
  resources :orders, only: [:index, :show, :create]
  mount Sidekiq::Web => "/sidekiq"
  get "status", to: StatusApp
end
`

type blockerDoc struct {
	Schema     string       `json:"schema"`
	Verdict    string       `json:"verdict"`
	Cited      int          `json:"cited"`
	Incomplete bool         `json:"route_table_incomplete"`
	Requests   []blockerRow `json:"requests"`
}

func blockerFixture(t *testing.T, routes string) (string, *Map) {
	t.Helper()
	return railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = routes
		files["app/controllers/widgets_controller.rb"] = "class WidgetsController < ApplicationController\n  def archive\n  end\nend\n"
	})
}

func checkBlocker(t *testing.T, root string, m *Map, note string) (blockerDoc, *BlockerReport) {
	t.Helper()
	r, err := CheckBlocker(context.Background(), m, note, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc blockerDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil {
		t.Fatalf("%v\n%s", err, r.Document)
	}
	return doc, r
}

// AMAP-V0-143 AMAP-V0-144: a cited 404 no route matches is flagged BLOCKER-404-NO-ROUTE naming
// the nearest route; a matched route with a defined action is accepted unchanged; a matched route
// whose handler lacks the action reuses ROUTE-ACTION-MISSING (GitHub #1215 acceptance).
func TestAMAPV0143BlockerVerdicts(t *testing.T) {
	root, m := blockerFixture(t, blockerRoutes)
	doc, r := checkBlocker(t, root, m, "blocked: missing fixture\nE2E failed: 404 POST /api/v2/widgets/42/archive")
	if doc.Schema != BlockerSchema || doc.Verdict != "flagged" || r.Flagged != 1 || len(doc.Requests) != 1 {
		t.Fatalf("no-route: %+v", doc)
	}
	row := doc.Requests[0]
	if row.Rule != RuleBlocker404 || row.NoteLine != 2 || row.Request != "POST /api/v2/widgets/42/archive" || len(row.Nearest) == 0 ||
		row.Nearest[0].Verb != "POST" || row.Nearest[0].Path != "/api/widgets/:id/archive" || row.Nearest[0].Declaration != "config/routes.rb:5" {
		t.Fatalf("no-route row: %+v", row)
	}
	doc, r = checkBlocker(t, root, m, "fixture-gap: HTTP 404 from POST https://staging.example.test/api/widgets/42/archive?force=1.")
	if doc.Verdict != "accepted" || r.Flagged != 0 || doc.Requests[0].Route == nil || doc.Requests[0].Route.Action != "archive" {
		t.Fatalf("matched: %+v", doc)
	}
	doc, _ = checkBlocker(t, root, m, "404 GET /orders/7.json and 404 HEAD /orders/")
	if doc.Verdict != "accepted" || len(doc.Requests) != 2 || doc.Requests[1].Request != "HEAD /orders" {
		t.Fatalf("format and HEAD: %+v", doc)
	}
	doc, _ = checkBlocker(t, root, m, "blocked (404): POST /orders")
	if row := doc.Requests[0]; doc.Verdict != "flagged" || row.Rule != RuleRouteActionMissing || row.Basis != "non-public-method" ||
		row.HandlerFile != "app/controllers/orders_controller.rb" {
		t.Fatalf("action missing: %+v", doc)
	}
	doc, _ = checkBlocker(t, root, m, "blocked: environment down, GET /orders returned 500")
	if doc.Verdict != "not-applicable" || doc.Cited != 0 {
		t.Fatalf("no 404: %+v", doc)
	}
}

// AMAP-V0-142 AMAP-V0-143: mounted applications, rack endpoints, non-literal request paths and an
// incomplete route table make the cited request uncertain, never flagged.
func TestAMAPV0142BlockerUncertain(t *testing.T) {
	root, m := blockerFixture(t, blockerRoutes)
	doc, r := checkBlocker(t, root, m, "404 GET /sidekiq/queues\n404 GET /status\n404 GET /orders/${id}")
	got := []string{}
	for _, row := range doc.Requests {
		got = append(got, row.Verdict+":"+row.Reason+":"+row.At)
	}
	want := "uncertain:mounted-application:config/routes.rb:10 uncertain:rack-endpoint:config/routes.rb:11 uncertain:non-literal-request-path:"
	if doc.Verdict != "uncertain" || r.Flagged != 0 || strings.Join(got, " ") != want {
		t.Fatalf("uncertain: %q %+v", got, doc)
	}
	root, m = blockerFixture(t, strings.Replace(blockerRoutes, "  get \"status\"", "  get \"x\", to: handler_for(:x)\n  get \"status\"", 1))
	doc, r = checkBlocker(t, root, m, "404 POST /api/v2/widgets/42/archive")
	if row := doc.Requests[0]; doc.Verdict != "uncertain" || !doc.Incomplete || r.Flagged != 0 || row.Reason != "route-table-incomplete" ||
		row.At != "config/routes.rb:11 non-literal-rails-route" || len(row.Nearest) == 0 || row.Nearest[0].Path != "/api/widgets/:id/archive" {
		t.Fatalf("incomplete: %+v", doc)
	}
}

// AMAP-V0-140 AMAP-V0-141: the note is bounded, requests are deduplicated and at most 16
// distinct cited requests are read; anything more refuses.
func TestAMAPV0141BlockerNoteBounds(t *testing.T) {
	root, m := blockerFixture(t, blockerRoutes)
	doc, _ := checkBlocker(t, root, m, "404 GET /orders 404 GET /orders/")
	if doc.Cited != 1 {
		t.Fatalf("dedupe: %+v", doc)
	}
	many := []string{}
	for i := 0; i < 17; i++ {
		many = append(many, "404 GET /p"+string(rune('a'+i)))
	}
	for _, note := range []string{"", "  ", strings.Repeat("x", maxBlockerNote+1), strings.Join(many, "\n")} {
		if _, err := CheckBlocker(context.Background(), m, note, Options{Root: root}); codeOf(err) != "appmap-invalid-query" {
			t.Fatalf("admitted note of %d bytes: %v", len(note), err)
		}
	}
	if _, err := CheckBlocker(context.Background(), m, "404 GET /orders", Options{Root: root, Budget: 1}); codeOf(err) != "appmap-invalid-query" {
		t.Fatalf("admitted budget: %v", err)
	}
}

// AMAP-V0-142: route matching follows Rails segment rules.
func TestAMAPV0142RouteMatching(t *testing.T) {
	for _, tc := range []struct {
		route, req string
		want       bool
	}{
		{"/widgets/:id/archive", "/widgets/42/archive", true},
		{"/widgets/:id/archive", "/v2/widgets/42/archive", false},
		{"/widgets/:id", "/widgets/4.2", true}, // Rails reads .2 as the format,
		{"/widgets/:id", "/widgets/42.json", true},
		{"/files/*path", "/files/a/b", true},
		{"/files/*path", "/files", false},
		{"/", "/", true},
	} {
		if got := routeMatches(tc.route, requestSegments(tc.req)); got != tc.want {
			t.Fatalf("%s %s: %v", tc.route, tc.req, got)
		}
	}
}

// AMAP-V0-142: a mount or rack endpoint whose path is not literal covers every unmatched request,
// so it reads uncertain rather than flagged; a matched route still decides first.
func TestAMAPV0142UnknownMountCoversAll(t *testing.T) {
	root, m := blockerFixture(t, strings.Replace(blockerRoutes, `mount Sidekiq::Web => "/sidekiq"`, "mount Sidekiq::Web, at: mount_path", 1))
	doc, r := checkBlocker(t, root, m, "404 POST /api/v2/widgets/42/archive\n404 GET /orders")
	if r.Flagged != 0 || doc.Requests[0].Verdict != "uncertain" || doc.Requests[0].Reason != "mounted-application" || doc.Requests[1].Verdict != "accepted" {
		t.Fatalf("unknown mount: %+v", doc)
	}
}

// AMAP-V0-146: the same committed inputs and note produce a byte-identical report.
func TestAMAPV0146BlockerDeterministic(t *testing.T) {
	root, m := blockerFixture(t, blockerRoutes)
	note := "404 POST /api/v2/widgets/42/archive\n404 GET /sidekiq/x\n404 POST /orders"
	_, a := checkBlocker(t, root, m, note)
	_, b := checkBlocker(t, root, m, note)
	if string(a.Document) != string(b.Document) || a.Verdict != "flagged" || a.Flagged != 2 {
		t.Fatalf("not deterministic or wrong verdict:\n%s\n%s", a.Document, b.Document)
	}
}
