package appmap

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// AMAP-V0-371 (V1-3058, #1965): a block with block parameters -- a loop over a list, a `direct`
// helper -- is one unread region at its line in both Rails readers, and the rest of the file is
// read; an optional-segment scope with a regexp constraint on that segment reads its drawn files
// under the optional parameter, recording the constraint.
func TestAMAPV0371RailsBlockParameters(t *testing.T) {
	f := hopRepo(t, map[string]string{
		"config/application.rb": "module Shop\nend\n",
		"config/routes.rb": "Rails.application.routes.draw do\n  direct :public_blob do |model|\n    route_for(:rails_blob, model)\n  end\n" +
			"  scope \"(:locale)\", locale: /en|fr/ do\n    draw :api\n  end\n  get \"help\", to: \"pages#help\"\nend\n",
		"config/routes/api.rb": "namespace :api, constraints: {format: /json/} do\n  %w[a b].each do |ns|\n    namespace ns do\n" +
			"      post \"verify\", to: \"verify#create\"\n    end\n  end\n  resources :rounds, only: [] do\n    member do\n      post :confirm\n    end\n  end\nend\n",
		"app/items/items.js": "function c($http, id) {\n  return $http.post('/api/rounds/' + id + '/confirm');\n}\n",
	})
	d := railsDrawEndpoints(t, f, "config/routes.rb")
	ids := []string{}
	for _, e := range d.Rows {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	if got := strings.Join(ids, ","); got != "GET /help,POST /api/rounds/{}/confirm,POST /{}/api/rounds/{}/confirm" {
		t.Fatalf("endpoints %s", got)
	}
	var notes []string
	for _, u := range d.Unknown {
		if u.Reason != "unmapped-path" {
			notes = append(notes, u.At+" "+u.Reason)
		}
	}
	sort.Strings(notes)
	if got := strings.Join(notes, ","); got != "config/routes.rb:2 rails-block-parameters,config/routes/api.rb:2 rails-block-parameters" {
		t.Fatalf("unknowns %s", got)
	}
	if len(d.Calls) != 1 || !strings.Contains(strings.Join(d.Calls[0].Endpoints, ","), "POST /api/rounds/{}/confirm") {
		t.Fatalf("join %+v", d.Calls)
	}

	// The screen reader skips the same region.
	p := &railsParser{}
	p.parse(blobEntry{path: "config/routes.rb"}, []byte("Rails.application.routes.draw do\n  %w[a b].each do |ns, (x, y)|\n    get ns, to: \"x#y\"\n  end\n  get \"help\", to: \"pages#help\"\nend\n"), railsScope{}, 0)
	if len(p.states) != 1 || len(p.unknowns) != 1 || p.unknowns[0].Reason != "rails-block-parameters" || p.unknowns[0].Line != 2 {
		t.Fatalf("screens %+v unknowns %+v", p.states, p.unknowns)
	}
}

// AMAP-V0-371: a block body that may transfer control, or whose end the reader cannot place,
// still refuses the file, now naming the refusing statement's line; a regexp option naming no
// segment of the scope's path is still a non-literal scope.
func TestAMAPV0371RailsBlockFailClosed(t *testing.T) {
	for name, c := range map[string]struct {
		src  string
		want string
	}{
		"return":      {"Rails.application.routes.draw do\n  get 'a', to: 'a#a'\n  [1].each do |x|\n    return if x\n  end\n  get 'b', to: 'b#b'\nend\n", "unsupported-rails-syntax:3"},
		"one line":    {"Rails.application.routes.draw do\n  [1].each do |x| get 'b', to: 'b#b' end\nend\n", "unsupported-rails-syntax:2"},
		"stray end":   {"get 'a', to: 'a#a'\nend\n", "unsupported-rails-syntax:2"},
		"other regex": {"scope \"(:locale)\", lang: /en/ do\n  get 'a', to: 'a#a'\nend\n", "non-literal-rails-scope:1,unresolved-parent:2"},
	} {
		p := &routeActionParser{}
		p.parse("config/routes.rb", []byte(c.src), actionScope{}, 0)
		var notes []string
		for _, n := range p.abstain {
			notes = append(notes, n.reason+":"+strconv.Itoa(n.line))
		}
		if got := strings.Join(notes, ","); got != c.want || len(p.routes) != 0 {
			t.Errorf("%s: %s routes %+v, want %s", name, got, p.routes, c.want)
		}
	}
}
