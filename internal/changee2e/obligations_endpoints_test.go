package changee2e

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// CCE-V0-152: with the endpoint join inputs, a changed Rails controller reaches the screen whose
// controller calls its endpoint, with the endpoint in the chain, and the join is named consulted.
// A join with an unknown keeps the placed controller an unknown span (map-unknown), so the
// obligations never read complete over a caller the join could not see; without the inputs the
// controller stays an unplaced path and the join is named not consulted.
func TestCCEV0152ObligationsThroughEndpoints(t *testing.T) {
	root, _ := fixtureRepo(t)
	routes := *overlayText(t, "app/routes.js")
	routes = strings.Replace(routes, "templateUrl: 'app/components/teesheet.html'", "templateUrl: 'app/components/teesheet.html',\n      controller: 'TeesheetsCtrl'", 1)
	ctrl := "angular.module('admin').controller('TeesheetsCtrl', ['$http', function ($http) {\n  $http.get('/api/slots');\n}]);\n"
	base := commitFiles(t, root, map[string]*string{
		"app/routes.js":                       text(routes),
		"app/clubs/teesheets.ctrl.js":         text(ctrl),
		"config/routes.rb":                    text("Rails.application.routes.draw do\n  get \"api/slots\", to: \"slots#index\"\nend\n"),
		"app/controllers/slots_controller.rb": text("class SlotsController\nend\n"),
	}, "endpoint base")
	head := commitFiles(t, root, map[string]*string{"app/controllers/slots_controller.rb": text("class SlotsController\n  def index; end\nend\n")}, "controller change")
	controller := "app/controllers/slots_controller.rb"
	eo := &appmap.EndpointsOptions{RailsRoutes: []string{"config/routes.rb"}, Sources: []string{"app/clubs"}}

	plain := obligationsFor(t, diffOpts(root, base, head))
	if unknownWith(plain, ReasonUnplacedPath, controller) == nil || !slices.Contains(plain.Relations.NotConsulted, EndpointsRelation) {
		t.Fatalf("without the join: %s", plain.Encoded())
	}

	o := diffOpts(root, base, head)
	o.Endpoints = eo
	r := obligationsFor(t, o)
	requireClosedReasons(t, r)
	reached := false
	for _, ob := range r.Obligations {
		if ob.Screen == teesheetsScreen && strings.Contains(strings.Join(ob.Chain, "|"), "endpoint:GET /api/slots") {
			reached = true
		}
	}
	if !reached || len(r.Unknown) != 0 || !slices.Contains(r.Relations.Consulted, EndpointsRelation) {
		t.Fatalf("through the join: %s", r.Encoded())
	}

	base2 := commitFiles(t, root, map[string]*string{"app/clubs/other.js": text("fetch(url);\n")}, "dynamic caller")
	head2 := commitFiles(t, root, map[string]*string{controller: text("class SlotsController\nend\n")}, "controller change again")
	o = diffOpts(root, base2, head2)
	o.Endpoints = eo
	r = obligationsFor(t, o)
	requireClosedReasons(t, r)
	u := unknownWith(r, ReasonMapUnknown, controller)
	if u == nil || !strings.HasPrefix(u.Detail, "endpoint-callers-incomplete") || r.State != Unknown {
		t.Fatalf("incomplete join: %s", r.Encoded())
	}
}

// AMAP-V0-223: obligations bind the aliased roots for the endpoint join. With the server in --root,
// one caller placed in --root and one in the aliased test root, a changed controller reaches the
// --root caller's screen through the endpoint and keeps a map-unknown span naming the aliased
// caller with its root, so the obligations are never complete over a caller no screen holds.
func TestAMAPV0223ObligationsAliasedEndpointSource(t *testing.T) {
	app, _, e2e, _ := twoRepoFixture(t)
	routes := *overlayText(t, "app/routes.js")
	routes = strings.Replace(routes, "templateUrl: 'app/components/teesheet.html'", "templateUrl: 'app/components/teesheet.html',\n      controller: 'TeesheetsCtrl'", 1)
	ctrl := "angular.module('admin').controller('TeesheetsCtrl', ['$http', function ($http) {\n  $http.get('/api/slots');\n}]);\n"
	commitFiles(t, e2e, map[string]*string{"client/slots.js": text("export const slots = () => fetch('/api/slots');\n")}, "aliased caller")
	controller := "app/controllers/slots_controller.rb"
	base := commitFiles(t, app, map[string]*string{
		"app/routes.js":               text(routes),
		"app/clubs/teesheets.ctrl.js": text(ctrl),
		"config/routes.rb":            text("Rails.application.routes.draw do\n  get \"api/slots\", to: \"slots#index\"\nend\n"),
		controller:                    text("class SlotsController\nend\n"),
	}, "endpoint base")
	head := commitFiles(t, app, map[string]*string{controller: text("class SlotsController\n  def index; end\nend\n")}, "controller change")
	o := diffOpts(app, base, head)
	o.Roots = appmap.Roots{Repos: map[string]string{"e2e": e2e}}
	o.Endpoints = &appmap.EndpointsOptions{RailsRoutes: []string{"config/routes.rb"}, Sources: []string{"app/clubs", "e2e:client"}}
	r := obligationsFor(t, o)
	requireClosedReasons(t, r)
	reached := false
	for _, ob := range r.Obligations {
		if ob.Screen == teesheetsScreen && strings.Contains(strings.Join(ob.Chain, "|"), "endpoint:GET /api/slots") {
			reached = true
		}
	}
	u := unknownWith(r, ReasonMapUnknown, controller)
	if !reached || u == nil || u.Span.Repo != "" || r.State != Unknown ||
		u.Detail != "endpoint-callers-incomplete: 1 endpoint join unknowns, first e2e:client/slots.js aliased-root-caller-not-placed" {
		t.Fatalf("aliased caller: %s", r.Encoded())
	}
}
