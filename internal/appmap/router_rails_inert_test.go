package appmap

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// gh1954Routes is the issue's routes file: a library require, a begin/rescue region that logs, and
// a namespace whose constraints hash holds a regexp literal.
const gh1954Routes = "require \"x/web\"\n\nRails.application.routes.draw do\n  begin\n    X::Web.mount(self)\n" +
	"  rescue => e\n    Rails.logger.error \"failed: #{e.message}\"\n  end\n  get \"help\", to: \"pages#help\"\n" +
	"  namespace :api, constraints: {format: /(json|csv)/} do\n    resources :items do\n      member do\n" +
	"        post :confirm\n      end\n    end\n  end\nend\n"

// AMAP-V0-366 (V1-2968, #1954): the Rails screen reader skips a non-route statement and an
// unread region at their lines, and a namespace with a constraints option stays unknown at its
// line, instead of refusing the whole file.
func TestAMAPV0366RailsScreensSkipStatements(t *testing.T) {
	p := &railsParser{}
	p.parse(blobEntry{path: "config/routes.rb"}, []byte(gh1954Routes), railsScope{}, 0)
	var names, notes []string
	for _, s := range p.states {
		names = append(names, s.name+" "+s.url)
	}
	for _, u := range p.unknowns {
		notes = append(notes, u.Reason)
	}
	if want := []string{"help /help"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("states %v, want %v", names, want)
	}
	if got := strings.Join(notes, ","); got != "non-route-statement,rails-region-not-read,rails-constraints,unresolved-parent,unresolved-parent,unresolved-parent" {
		t.Fatalf("unknowns %s", got)
	}
	// Without the constraint the namespace's screens compile.
	p = &railsParser{}
	p.parse(blobEntry{path: "config/routes.rb"}, []byte(strings.Replace(gh1954Routes, ", constraints: {format: /(json|csv)/}", "", 1)), railsScope{}, 0)
	if len(p.states) != 6 {
		t.Fatalf("states %+v unknowns %+v", p.states, p.unknowns)
	}
}

// AMAP-V0-366 (V1-2968, #1954): the endpoint reader compiles the issue's routes file itself, not
// only a drawn one, to its endpoints with per-statement notes.
func TestAMAPV0366RailsEndpointsIssueFile(t *testing.T) {
	f := hopRepo(t, map[string]string{"config/application.rb": "module Shop\nend\n", "config/routes.rb": gh1954Routes,
		"app/items/items.js": "function c($http, id) {\n  return $http.post('/api/items/' + id + '/confirm');\n}\n"})
	d := railsDrawEndpoints(t, f, "config/routes.rb")
	ids := map[string]bool{}
	for _, e := range d.Rows {
		ids[e.ID] = true
	}
	if !ids["POST /api/items/{}/confirm"] || !ids["GET /help"] || len(ids) != 9 {
		t.Fatalf("endpoints %v", ids)
	}
	var notes []string
	for _, u := range d.Unknown {
		if u.Reason != "unmapped-path" {
			notes = append(notes, u.At+" "+u.Reason)
		}
	}
	sort.Strings(notes)
	if got := strings.Join(notes, ","); got != "config/routes.rb:1 non-route-statement,config/routes.rb:4 rails-region-not-read" {
		t.Fatalf("unknowns %s", got)
	}
	if len(d.Calls) != 1 || strings.Join(d.Calls[0].Endpoints, ",") != "POST /api/items/{}/confirm" {
		t.Fatalf("join %+v", d.Calls)
	}
}

// AMAP-V0-366 (review): a region that may transfer control refuses the file, and a region whose
// end the reader cannot place (an `end` or `do` inside a statement, an expression opener) is not
// skipped, so a route inside it is never read as unconditional; `as: :do` opens no block.
func TestAMAPV0366RailsRegionsFailClosed(t *testing.T) {
	for name, src := range map[string]string{
		"next":       "Rails.application.routes.draw do\n  get 'home', to: 'pages#home'\n  unless ENV['BETA']\n    next\n  end\n  get 'beta', to: 'beta#show'\nend\n",
		"member end": "Rails.application.routes.draw do\n  if ENV['A']\n    y = r.end\n    get 'secret', to: 's#s'\n  end\n  get 'x', to: 'x#y', as: :do\nend\n",
		"def end":    "Rails.application.routes.draw do\n  if ENV['A']\n    def helper; end\n    get 'secret', to: 's#s'\n  end\n  get 'x', to: 'x#y', as: :do\nend\n",
	} {
		p := &railsParser{}
		p.parse(blobEntry{path: "config/routes.rb"}, []byte(src), railsScope{}, 0)
		for _, s := range p.states {
			if s.name == "beta" || s.name == "secret" {
				t.Errorf("%s: screen reader read %s as certain: %+v", name, s.name, p.unknowns)
			}
		}
		a := &routeActionParser{}
		a.parse("config/routes.rb", []byte(src), actionScope{}, 0)
		for _, r := range a.routes {
			if strings.Contains(r.path, "beta") || strings.Contains(r.path, "secret") {
				t.Errorf("%s: endpoint reader read %s as certain", name, r.path)
			}
		}
	}
}
