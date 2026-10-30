package appmap

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// railsDrawFixture is a Rails backend whose routes file requires a library, wraps a logger call in
// begin/rescue, declares an optional locale scope and draws files under config/routes/, one of
// which draws a nested file inside a namespace with a regex constraint.
func railsDrawFixture(t *testing.T) prefixFixture {
	t.Helper()
	return hopRepo(t, map[string]string{
		"config/application.rb": "module Shop\nend\n",
		"config/routes.rb": "require \"shop/web\"\nRails.application.routes.draw do\n  begin\n    Rails.logger.info(\"routes\")\n" +
			"  rescue => e\n    warn \"routes not logged\"\n  end\n  scope \"(:locale)\" do\n    get \"help\", to: \"pages#help\"\n  end\n  draw :api\n  draw :admin\nend\n",
		"config/routes/api.rb":       "namespace :api, constraints: {format: /(json|csv)/} do\n  draw \"api/items\"\nend\n",
		"config/routes/api/items.rb": "resources :items, only: [:index] do\n  member do\n    post :send_confirmation\n  end\nend\n",
		"config/routes/admin.rb":     "namespace :admin do\n  get \"sessions\", to: \"sessions#index\"\nend\n",
		"app/items/items.js":         "function confirm($http, id) {\n  return $http.post('/api/items/' + id + '/send_confirmation');\n}\n",
	})
}

func railsDrawEndpoints(t *testing.T, f prefixFixture, routes ...string) endpointsDoc {
	t.Helper()
	raw, err := Endpoints(context.Background(), f.m, EndpointsOptions{RailsRoutes: routes, Sources: []string{"app/items"}}, Options{Root: f.root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var d endpointsDoc
	if err = json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// AMAP-V0-351..354: issues #1938 and #1939. Non-route statements and an inert begin/rescue region
// are per-statement unknowns, a namespace with a regex constraint hash compiles, nested draws keep
// the drawing scope, an optional scope segment yields both paths, a listed drawn file is read only
// through its draw, and controllers resolve under the Rails application root.
func TestAMAPV0351RailsDrawsAndRoot(t *testing.T) {
	f := railsDrawFixture(t)
	d := railsDrawEndpoints(t, f, "config/routes.rb", "config/routes/api.rb", "config/routes/admin.rb", "config/routes/api/items.rb")
	got := map[string]string{}
	for _, e := range d.Rows {
		got[e.ID] = e.Handler + " " + e.Controller
	}
	want := map[string]string{
		"GET /{}/help":                         "pages#help app/controllers/pages_controller.rb",
		"GET /help":                            "pages#help app/controllers/pages_controller.rb",
		"GET /api/items":                       "api/items#index app/controllers/api/items_controller.rb",
		"POST /api/items/{}/send_confirmation": "api/items#send_confirmation app/controllers/api/items_controller.rb",
		"GET /admin/sessions":                  "admin/sessions#index app/controllers/admin/sessions_controller.rb",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("endpoints\n got %v\nwant %v", got, want)
	}
	notes := []string{}
	for _, u := range d.Unknown {
		if u.Reason != "unmapped-path" { // the client file is on no screen
			notes = append(notes, u.At+" "+u.Reason)
		}
	}
	sort.Strings(notes)
	if strings.Join(notes, ",") != "config/routes.rb:1 non-route-statement,config/routes.rb:3 non-route-statement" {
		t.Fatalf("unknowns: %v", notes)
	}
	if len(d.Calls) != 1 || strings.Join(d.Calls[0].Endpoints, ",") != "POST /api/items/{}/send_confirmation" {
		t.Fatalf("join: %+v", d.Calls)
	}
	// A drawn file listed alone resolves controllers under the application root, not its
	// grandparent directory (#1939).
	d = railsDrawEndpoints(t, f, "config/routes/admin.rb")
	if len(d.Rows) != 1 || d.Rows[0].Controller != "app/controllers/admin/sessions_controller.rb" {
		t.Fatalf("drawn file root: %+v", d.Rows)
	}
}

// AMAP-V0-351 AMAP-V0-352: a region declaring a route is not mined and stays unknown, a relative
// require stays unsupported, and nested optional segments are a stated unknown.
func TestAMAPV0352RailsRegionsAndOptionals(t *testing.T) {
	src := "require_relative \"extra\"\nRails.application.routes.draw do\n  if ENV[\"X\"]\n    get \"x\", to: \"x#y\"\n  end\n" +
		"  scope \"(:a)\" do\n    scope \"(:b)\" do\n      get \"z\", to: \"z#show\"\n    end\n  end\n  get \"kept\", to: \"k#show\"\nend\n"
	p := &routeActionParser{}
	p.parse("config/routes.rb", []byte(src), actionScope{}, 0)
	notes := []string{}
	for _, n := range p.abstain {
		notes = append(notes, n.reason)
	}
	if strings.Join(notes, ",") != "unsupported-rails-statement,rails-region-not-read,rails-optional-segments" {
		t.Fatalf("notes: %v", notes)
	}
	paths := []string{}
	for _, rt := range p.routes {
		paths = append(paths, rt.verb+" "+rt.path)
	}
	if strings.Join(paths, ",") != "GET /:a/:b/z,GET /kept" {
		t.Fatalf("routes: %v", paths)
	}
	// Only an inert expression keeps a statement inert: an assignment, a call argument or a
	// region condition that calls a bare helper, or passes self, may declare routes.
	for src, want := range map[string]string{
		"x = \"lit\"\ny = Foo::BAR\nz = ENV[\"A\"]\nn = 10\nRails.logger.info(\"r\")\n": "non-route-statement,non-route-statement,non-route-statement,non-route-statement,non-route-statement",
		"ignored = install_routes\n":                                      "unsupported-rails-statement",
		"Helper.install(self)\n":                                          "unsupported-rails-statement",
		"puts install_routes\n":                                           "unsupported-rails-statement",
		"x = \"#{install_routes}\"\n":                                     "unsupported-rails-statement",
		"if install_routes\n  x = 1\nend\n":                               "rails-region-not-read",
		"if ENV[\"A\"] == \"1\"\n  x = 1\nend\n":                          "non-route-statement",
		"begin\n  x = 1\nrescue StandardError => e\n  warn \"no\"\nend\n": "non-route-statement",
	} {
		p := &routeActionParser{}
		p.parse("config/routes.rb", []byte(src), actionScope{}, 0)
		notes := []string{}
		for _, n := range p.abstain {
			notes = append(notes, n.reason)
		}
		if strings.Join(notes, ",") != want {
			t.Errorf("%q: %v, want %s", src, notes, want)
		}
	}
}
