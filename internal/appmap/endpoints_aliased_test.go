package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// aliasedEndpointRoots is the two-root map of twoRoots with the settings screen's controller in
// --root calling the settings API, a --root Rails server for /api/members, and, in the aliased
// "e2e" root the map pins for its tests, a Rails server for the settings API and a client file
// calling /api/members, also at app/clubs/settings.ctrl.js, a path --root holds too.
func aliasedEndpointRoots(t *testing.T) (m *Map, app, e2e string) {
	t.Helper()
	app, _, e2e, _ = twoRoots(t, false)
	routes := readText(t, app, "app/routes.js")
	writeFile(t, app, "app/routes.js", strings.Replace(routes, "url: '/settings'", "url: '/settings',\n      controller: 'SettingsCtrl'", 1))
	writeFile(t, app, "app/clubs/settings.ctrl.js", "angular.module('admin').controller('SettingsCtrl', ['$http', '$stateParams', function ($http, $stateParams) {\n"+
		"  $http.get('/api/clubs/' + $stateParams.clubId + '/settings');\n}]);\n")
	writeFile(t, app, "config/routes.rb", "Rails.application.routes.draw do\n  get \"api/members\", to: \"members#index\"\nend\n")
	writeFile(t, app, "app/controllers/members_controller.rb", "class MembersController\nend\n")
	appRev := commitAll(t, app, "client and members server")
	writeFile(t, e2e, "server/config/routes.rb", "Rails.application.routes.draw do\n  get \"api/clubs/:club_id/settings\", to: \"settings#show\"\n"+
		"  patch \"api/clubs/:club_id/settings\", to: \"settings#update\"\nend\n")
	writeFile(t, e2e, "server/app/controllers/settings_controller.rb", "class SettingsController\nend\n")
	writeFile(t, e2e, "client/members.js", "export const members = () => fetch('/api/members');\n")
	writeFile(t, e2e, "app/clubs/settings.ctrl.js", "export const members = () => fetch('/api/members');\n")
	commitAll(t, e2e, "settings server and members client")
	m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	return m, app, e2e
}

type aliasedJoinDoc struct {
	Complete  bool              `json:"complete"`
	Unknown   []endpointUnknown `json:"unknown"`
	Endpoints []endpointRow     `json:"endpoints"`
	Calls     []endpointCall    `json:"calls"`
}

func aliasedJoin(t *testing.T, m *Map, eo EndpointsOptions, o Options) (aliasedJoinDoc, string) {
	t.Helper()
	o.Full = true
	raw, err := Endpoints(context.Background(), m, eo, o)
	if err != nil {
		t.Fatalf("%+v: %v", eo, err)
	}
	var doc aliasedJoinDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc, string(raw)
}

// AMAP-V0-222: the endpoint join reads a declaration or a client source directory named ALIAS:PATH
// from the root the map pinned under ALIAS. A server in the aliased root joins to calls in --root
// and their screens, its endpoints carrying repo; a call in the aliased root links to the endpoints
// it reaches, carries repo and is never placed on a --root screen by path: it is the unknown
// aliased-root-caller-not-placed, so the join is not complete. Unbound, unread and unsafe aliased
// inputs refuse; a --root-only join carries no repo member.
func TestAMAPV0222AliasedRootJoin(t *testing.T) {
	m, app, e2e := aliasedEndpointRoots(t)
	repos := map[string]string{"e2e": e2e}
	settings := screenID("admin", "app.clubs.settings")

	doc, _ := aliasedJoin(t, m, EndpointsOptions{RailsRoutes: []string{"e2e:server/config/routes.rb"}, Sources: []string{"app/clubs"}}, Options{Root: app, Repos: repos})
	if !doc.Complete || len(doc.Endpoints) != 2 || len(doc.Calls) != 1 {
		t.Fatalf("server in the aliased root: %+v", doc)
	}
	get := doc.Endpoints[0]
	if get.ID != "GET /api/clubs/{}/settings" || get.Repo != "e2e" || get.Controller != "server/app/controllers/settings_controller.rb" ||
		get.Declaration != "server/config/routes.rb:2" || strings.Join(get.Calls, ",") != "app/clubs/settings.ctrl.js:2" || strings.Join(get.Screens, ",") != settings {
		t.Fatalf("aliased endpoint: %+v", get)
	}
	if c := doc.Calls[0]; c.Repo != "" || strings.Join(c.Screens, ",") != settings || doc.Endpoints[1].Repo != "e2e" || len(doc.Endpoints[1].Calls) != 0 {
		t.Fatalf("--root call: %+v %+v", c, doc.Endpoints[1])
	}

	doc, _ = aliasedJoin(t, m, EndpointsOptions{RailsRoutes: []string{"config/routes.rb", "e2e:server/config/routes.rb"}, Sources: []string{"app/clubs", "e2e:client"}},
		Options{Root: app, Repos: repos})
	var members *endpointRow
	for i := range doc.Endpoints {
		if doc.Endpoints[i].ID == "GET /api/members" {
			members = &doc.Endpoints[i]
		}
	}
	if doc.Complete || members == nil || members.Repo != "" || members.Controller != "app/controllers/members_controller.rb" ||
		strings.Join(members.Calls, ",") != "e2e:client/members.js:1" || len(members.Screens) != 0 {
		t.Fatalf("client in the aliased root: %+v", doc)
	}
	var aliasedCall *endpointCall
	for i := range doc.Calls {
		if doc.Calls[i].Repo == "e2e" {
			aliasedCall = &doc.Calls[i]
		}
	}
	if aliasedCall == nil || aliasedCall.At != "client/members.js:1" || strings.Join(aliasedCall.Endpoints, ",") != "GET /api/members" ||
		len(aliasedCall.Screens) != 0 || aliasedCall.Placement != "aliased-root-caller-not-placed" {
		t.Fatalf("aliased call: %+v", doc.Calls)
	}
	if len(doc.Unknown) != 1 || doc.Unknown[0] != (endpointUnknown{Repo: "e2e", At: "client/members.js", Reason: "aliased-root-caller-not-placed",
		Detail: "screens are anchored in --root and the reference hop does not cross roots"}) {
		t.Fatalf("unknowns: %+v", doc.Unknown)
	}

	// An aliased call no declared endpoint reaches keeps its root on the unknown.
	_, raw := aliasedJoin(t, m, EndpointsOptions{RailsRoutes: []string{"e2e:server/config/routes.rb"}, Sources: []string{"e2e:client"}}, Options{Root: app, Repos: repos})
	if !strings.Contains(raw, `{"repo":"e2e","at":"client/members.js:1","reason":"no-declared-endpoint","detail":"GET /api/members"}`) {
		t.Fatalf("aliased unmatched call: %s", raw)
	}

	// One path read from both roots keeps both calls: the --root file is placed on its screen and the
	// aliased file of the same path is unknown, never placed through the --root file's anchor.
	doc, _ = aliasedJoin(t, m, EndpointsOptions{RailsRoutes: []string{"config/routes.rb", "e2e:server/config/routes.rb"}, Sources: []string{"app/clubs", "e2e:app/clubs"}},
		Options{Root: app, Repos: repos})
	if len(doc.Calls) != 2 || doc.Calls[0].Repo != "" || strings.Join(doc.Calls[0].Screens, ",") != settings ||
		doc.Calls[1].Repo != "e2e" || doc.Calls[1].At != "app/clubs/settings.ctrl.js:1" || len(doc.Calls[1].Screens) != 0 ||
		doc.Calls[1].Placement != "aliased-root-caller-not-placed" || doc.Complete {
		t.Fatalf("one path in two roots: %+v", doc)
	}

	// A --root-only join is byte-for-byte free of repo members.
	if _, raw = aliasedJoin(t, m, EndpointsOptions{RailsRoutes: []string{"config/routes.rb"}, Sources: []string{"app/clubs"}}, Options{Root: app}); strings.Contains(raw, `"repo"`) {
		t.Fatalf("--root join carries repo: %s", raw)
	}

	ctx := context.Background()
	for name, tc := range map[string]struct {
		eo    EndpointsOptions
		repos map[string]string
		code  string
	}{
		"unbound alias":   {EndpointsOptions{RailsRoutes: []string{"e2e:server/config/routes.rb"}, Sources: []string{"app/clubs"}}, nil, "appmap-root-unavailable"},
		"unread alias":    {EndpointsOptions{OpenAPI: []string{"docs:openapi.json"}, Sources: []string{"app/clubs"}}, map[string]string{"docs": e2e}, "appmap-root-unavailable"},
		"unsafe path":     {EndpointsOptions{RailsRoutes: []string{"e2e:../routes.rb"}, Sources: []string{"app/clubs"}}, repos, "appmap-invalid-query"},
		"unbound sources": {EndpointsOptions{RailsRoutes: []string{"config/routes.rb"}, Sources: []string{"e2e:client"}}, nil, "appmap-root-unavailable"},
	} {
		if _, err := Endpoints(ctx, m, tc.eo, Options{Root: app, Repos: tc.repos}); codeOf(err) != tc.code {
			t.Errorf("%s: %v, want %s", name, err, tc.code)
		}
	}
}

// AMAP-V0-222: impact matches only endpoints declared in --root against --root's changed paths. A
// changed --root file at the path of an aliased root's controller is not placed through the join
// (it stays non-ui-path); a changed --root controller whose only caller is in the aliased root
// stays endpoint-callers-incomplete naming that caller with its root.
func TestAMAPV0222ImpactAliasedRoots(t *testing.T) {
	m, app, e2e := aliasedEndpointRoots(t)
	f := prefixFixture{m: m, root: app, rev: m.Revision}
	impactCommit(t, f, map[string]string{
		"server/app/controllers/settings_controller.rb": "class SettingsController\nend\n",
		"app/controllers/members_controller.rb":         "class MembersController\n  def index; end\nend\n",
	})
	eo := &EndpointsOptions{RailsRoutes: []string{"config/routes.rb", "e2e:server/config/routes.rb"}, Sources: []string{"app/clubs", "e2e:client"}}
	r := impactRun(t, f, nil, m.Revision, Options{Endpoints: eo, Repos: map[string]string{"e2e": e2e}})
	got := map[string][]string{}
	for _, u := range r.Unknown {
		got[u.Path] = append(got[u.Path], u.Reason+" "+u.Detail)
	}
	if strings.Join(got["server/app/controllers/settings_controller.rb"], ",") != "non-ui-path " {
		t.Fatalf("same-named path matched an aliased controller: %+v", r)
	}
	if strings.Join(got["app/controllers/members_controller.rb"], ",") != "endpoint-callers-incomplete 1 endpoint join unknowns, first e2e:client/members.js aliased-root-caller-not-placed" {
		t.Fatalf("aliased caller: %+v", r)
	}
	for _, s := range r.Screens {
		if strings.HasPrefix(s.Basis, "endpoint:") {
			t.Fatalf("an endpoint basis without a placed caller: %+v", s)
		}
	}
	if r.Verdict != VerdictFullSuite {
		t.Fatalf("verdict %s", r.Verdict)
	}
}
