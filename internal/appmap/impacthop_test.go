package appmap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// hopRepo commits the fixture app with its settings and members states naming a template, a
// controller and a loaded helper, plus extra files.
func hopRepo(t *testing.T, extra map[string]string) prefixFixture {
	t.Helper()
	root, _ := fixtureRepo(t)
	routes, err := os.ReadFile(filepath.Join(root, "app/routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(string(routes), "url: '/settings'", "url: '/settings',\n      templateUrl: 'app/clubs/settings.html',\n      controller: 'SettingsCtrl',\n      resolve: { f: function () { return require('./util/format'); } }", 1)
	src = strings.Replace(src, "url: '/members'\n    })\n    .state('members2'", "url: '/members',\n      controller: 'MembersCtrl'\n    })\n    .state('members2'", 1)
	src = strings.Replace(src, "function ($stateProvider)", "function ($stateProvider, clubThemeProvider)", 1)
	files := map[string]string{
		"app/routes.js":           src,
		"app/util/format.js":      "module.exports = function (d) { return String(d); };\n",
		"app/clubs/settings.html": "<form>{{ row.date | format }}<member-row></member-row><div data-club-badge></div></form>\n",
		// A common stem as a function name, a property and an object key (ANL-V0-070).
		"app/list.js": "function format(x) { return x.format(); }\nvar o = { format: 1 };\nmodule.exports = o.format + format;\n",
	}
	for p, body := range extra {
		files[p] = body
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rev := commitAll(t, root, "hop")
	return prefixFixture{m: build(t, root, rev), root: root, rev: rev}
}

// hopChange commits files and runs impact against the previous HEAD.
func hopChange(t *testing.T, f prefixFixture, files map[string]string) ImpactReport {
	t.Helper()
	head := gitTest(t, f.root, "rev-parse", "HEAD")
	impactCommit(t, f, files)
	return impactRun(t, f, nil, head, Options{})
}

func hopBases(r ImpactReport) map[string]string {
	got := map[string]string{}
	for _, s := range r.Screens {
		got[s.ID] = s.Basis
	}
	return got
}

func hopOneUnknown(t *testing.T, what string, r ImpactReport, want ImpactUnknown) {
	t.Helper()
	if len(r.Unknown) == 1 {
		// AMAP-V0-210: every impact unknown states a remedy; the hop tests compare the rest.
		if rm := r.Unknown[0].Remedy; rm == nil || (rm.Key == "" && rm.Uncertainty == "") {
			t.Fatalf("%s: unknown without a remedy: %+v", what, r.Unknown[0])
		}
		r.Unknown[0].Remedy = nil
	}
	if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0] != want {
		t.Fatalf("%s: want %+v, got %+v", what, want, r)
	}
}

var hopFormatChange = map[string]string{"app/util/format.js": "module.exports = function (d) { return String(d) + ''; };\n"}

// ANL-V0-070: identifiers, properties and keys sharing a changed script's stem, and a markup filter of
// that name, are not mentions; a computed require that may load it refuses, naming file and line
// (ANL-V0-072).
func TestANLV0070CommonStemIdentifiersAreNotReferences(t *testing.T) {
	f := hopRepo(t, nil)
	settings := screenID("admin", "app.clubs.settings")
	r := hopChange(t, f, hopFormatChange)
	if got := hopBases(r); r.Verdict != VerdictTargeted || len(r.Unknown) != 0 || len(got) != 1 || got[settings] != "template-reference:app/routes.js" {
		t.Fatalf("common stem refused or widened: %+v", r)
	}
	impactCommit(t, f, map[string]string{"app/loader.js": "var format = './x';\nvar m = require(dir +\n  format);\n"})
	r = hopChange(t, f, map[string]string{"app/util/format.js": "module.exports = 3;\n"})
	hopOneUnknown(t, "computed require", r, ImpactUnknown{Path: "app/util/format.js", Reason: "unresolved-reference", Detail: "app/loader.js:3"})
	// No lookback bound: an argument opening far before the identifier still refuses.
	impactCommit(t, f, map[string]string{"app/loader.js": "var m = require(" + strings.Repeat(" ", 4097) + "format);\n"})
	r = hopChange(t, f, map[string]string{"app/util/format.js": "module.exports = 5;\n"})
	hopOneUnknown(t, "far computed require", r, ImpactUnknown{Path: "app/util/format.js", Reason: "unresolved-reference", Detail: "app/loader.js:1"})
	if s := hopLoadSpans("a(require (x, [y]), import(z"); !reflect.DeepEqual(s, [][2]int{{10, 17}, {26, 28}}) {
		t.Fatalf("load spans %v", s)
	}
}

// ANL-V0-069: an oversized file without the walked names leaves the report unchanged; one that may
// hold them reads reference-graph-bound naming the file and the bound.
func TestANLV0069OversizedFileIsANamedBound(t *testing.T) {
	plain := hopChange(t, hopRepo(t, nil), hopFormatChange)
	f := hopRepo(t, map[string]string{"app/vendor/bundle.js": strings.Repeat("var a = 1;\n", 110000)})
	big := hopChange(t, f, hopFormatChange)
	if plain.Verdict != VerdictTargeted || big.Verdict != plain.Verdict || !reflect.DeepEqual(big.Screens, plain.Screens) || !reflect.DeepEqual(big.Unknown, plain.Unknown) || !reflect.DeepEqual(big.SpecFiles, plain.SpecFiles) {
		t.Fatalf("unrelated bundle changed impact:\n%+v\n%+v", plain, big)
	}
	impactCommit(t, f, map[string]string{"app/vendor/bundle.js": strings.Repeat("var a = 1;\n", 110000) + "a.format = 2;\n"})
	r := hopChange(t, f, map[string]string{"app/util/format.js": "module.exports = 4;\n"})
	hopOneUnknown(t, "mentioning bundle", r, ImpactUnknown{Path: "app/util/format.js", Reason: "reference-graph-bound", Detail: "app/vendor/bundle.js exceeds 1048576 bytes"})
}

// ANL-V0-069: the file that would carry the read set past the total bound is named.
func TestANLV0069TotalBoundNamesTheFile(t *testing.T) {
	extra := map[string]string{}
	for i := 0; i < 32; i++ {
		body := fmt.Sprintf("v%02d;", i) + strings.Repeat("x", hopMaxFile-4) // distinct blobs, each counted
		if i == 31 {
			body = "a.format=1;" + body[11:]
		}
		extra[fmt.Sprintf("zz/b%02d.js", i)] = body
	}
	r := hopChange(t, hopRepo(t, extra), hopFormatChange)
	hopOneUnknown(t, "total", r, ImpactUnknown{Path: "app/util/format.js", Reason: "reference-graph-bound", Detail: "zz/b31.js exceeds the 33554432-byte reference graph total"})
}

// ANL-V0-069: the walked-node bound is named with the next file.
func TestANLV0069NodeBoundNamesTheBound(t *testing.T) {
	g := &referenceGraph{files: map[string][]byte{}, blobs: map[string]string{}, unread: map[string]*hopUnread{}, lexed: map[string]hopLex{}}
	for i := 0; i < 70; i++ {
		n := fmt.Sprintf("c/n%02d.js", i)
		g.files[n] = []byte(fmt.Sprintf("module.exports = require('./n%02d');\n", i-1))
		g.names = append(g.names, n)
	}
	_, why, detail := templateHop(&Map{}, g, "c/n00.js")
	if why != "reference-graph-bound" || detail != "more than 64 walked files (next c/n64.js)" {
		t.Fatalf("%s %s", why, detail)
	}
	u := &hopUnread{runs: hopRuns([]byte("x.format = 1; clubs-row"))}
	if !u.mayMention([]string{"format"}) || u.mayMention([]string{"clubs.service"}) || !u.mayMention([]string{"clubs-row"}) || !(&hopUnread{}).mayMention([]string{"q"}) {
		t.Fatal("mayMention")
	}
}

// ANL-V0-071: AngularJS DI registrations are followed through DI arrays, $inject, implicit
// parameters, a provider's NameProvider, a state's controller name and a component's or
// directive's markup forms; a registration whose name is not a literal refuses with file:line.
func TestANLV0071AngularDIRegistrationsAreReferences(t *testing.T) {
	f := hopRepo(t, map[string]string{
		"app/svc/clubs.service.js":            "angular.module('admin').factory('ClubService', function () { return {}; });\n",
		"app/clubs/settings.ctrl.js":          "angular.module('admin').controller('SettingsCtrl', ['ClubService', function (ClubService) {}]);\n",
		"app/svc/member.store.js":             "angular.module('admin').service('MemberStore', function () {});\n",
		"app/members/members.ctrl.js":         "function MembersCtrl(MemberStore) {}\nMembersCtrl.$inject = ['MemberStore'];\nangular.module('admin').controller('MembersCtrl', MembersCtrl);\n",
		"app/svc/theme.provider.js":           "angular.module('admin').provider('clubTheme', function () { this.$get = function () { return {}; }; });\n",
		"app/members/member-row.component.js": "angular.module('admin').component('memberRow', { template: '<tr></tr>' });\n",
		"app/clubs/badge.directive.js":        "angular.module('admin')\n  .directive(\"clubBadge\", function () { return {}; });\n",
	})
	settings, members := screenID("admin", "app.clubs.settings"), screenID("admin", "app.clubs.members")
	for p, want := range map[string]string{
		"app/svc/clubs.service.js":            settings,
		"app/members/member-row.component.js": settings,
		"app/clubs/badge.directive.js":        settings,
		"app/svc/member.store.js":             members,
	} {
		body, err := os.ReadFile(filepath.Join(f.root, p))
		if err != nil {
			t.Fatal(err)
		}
		r := hopChange(t, f, map[string]string{p: string(body) + "// v2\n"})
		if got := hopBases(r); r.Verdict != VerdictTargeted || len(r.Unknown) != 0 || len(got) != 1 || got[want] != "template-reference:app/routes.js" {
			t.Fatalf("%s: %+v", p, r)
		}
	}
	// A provider injected into the router's config block, outside every state span, widens to the file.
	r := hopChange(t, f, map[string]string{"app/svc/theme.provider.js": "angular.module('admin').provider('clubTheme', function () { this.$get = function () { return 1; }; });\n"})
	if r.Verdict != VerdictTargeted || len(r.Unknown) != 0 || len(r.Screens) < 5 {
		t.Fatalf("provider: %+v", r)
	}
	impactCommit(t, f, map[string]string{"app/svc/dyn.service.js": "var n = 'DynService';\nangular.module('admin')\n  .factory(n, function () {});\n"})
	r = hopChange(t, f, map[string]string{"app/svc/dyn.service.js": "var n = 'DynService';\nangular.module('admin')\n  .factory(n, function () { return 1; });\n"})
	hopOneUnknown(t, "computed registration", r, ImpactUnknown{Path: "app/svc/dyn.service.js", Reason: "unresolved-reference", Detail: "app/svc/dyn.service.js:3"})
	// A literal that is only part of the first argument is not the registered name.
	impactCommit(t, f, map[string]string{"app/svc/cat.service.js": "angular.module('admin')\n  .factory('Club' + 'Service', function () {});\n"})
	r = hopChange(t, f, map[string]string{"app/svc/cat.service.js": "angular.module('admin')\n  .factory('Club' + 'Service', function () { return 1; });\n"})
	hopOneUnknown(t, "concatenated registration", r, ImpactUnknown{Path: "app/svc/cat.service.js", Reason: "unresolved-reference", Detail: "app/svc/cat.service.js:2"})
}

// ANL-V0-071: a filter reaches the markup that pipes through it, and a directive reaches every
// markup spelling AngularJS normalizes to its name; array methods taking a function register
// nothing.
func TestANLV0071FiltersAndDirectiveSpellingsAreReferences(t *testing.T) {
	f := hopRepo(t, map[string]string{
		"app/clubs/settings.html":        "<form>{{ row.date | format }}\n{{ row.at | fmtDate }}\n<div data_club_panel></div></form>\n",
		"app/filters/fmt-date.filter.js": "var ok = [1].filter(function (x) { return x; }).filter((x) => x);\nangular.module('admin').filter('fmtDate', function () { return String; });\n",
		"app/clubs/panel.directive.js":   "angular.module('admin').directive('clubPanel', function () { return {}; });\n",
	})
	settings := screenID("admin", "app.clubs.settings")
	for _, p := range []string{"app/filters/fmt-date.filter.js", "app/clubs/panel.directive.js"} {
		body, err := os.ReadFile(filepath.Join(f.root, p))
		if err != nil {
			t.Fatal(err)
		}
		r := hopChange(t, f, map[string]string{p: string(body) + "// v2\n"})
		if got := hopBases(r); r.Verdict != VerdictTargeted || len(r.Unknown) != 0 || len(got) != 1 || got[settings] != "template-reference:app/routes.js" {
			t.Fatalf("%s: %+v", p, r)
		}
	}
	for _, run := range []string{"club-panel", "data-club-panel", "data_club_panel", "data:club:panel", "x-club-panel", "x_club_panel", "x:club-panel", "DATA-Club-Panel", "club_panel", "club:panel"} {
		if got := hopNgForms(run); got[0] != "clubPanel" {
			t.Fatalf("%s normalizes to %v", run, got)
		}
	}
	if got := hopNgForms("data-x-panel"); got[0] != "xPanel" || got[1] != "dataXPanel" {
		t.Fatalf("data-x-panel normalizes to %v", got)
	}
	for method, want := range map[string][]string{"filter": {"fmtDate", "fmtDateFilter"}, "value": {"fmtDate"}, "constant": {"fmtDate"}, "animation": {".fmtDate", "fmtDate"}} {
		n := "fmtDate"
		if method == "animation" {
			n = ".fmtDate"
		}
		if got, ng := hopDINames(method, n); !reflect.DeepEqual(got, want) || ng != "" {
			t.Fatalf("%s: %v %q", method, got, ng)
		}
	}
	u := &hopUnread{runs: hopRuns([]byte("<x:club-panel> <b data_CLUB_x>"))}
	if !u.mayMentionNg([]string{"clubPanel"}) || u.mayMentionNg([]string{"clubRow"}) || u.mayMentionNg(nil) || !(&hopUnread{}).mayMentionNg([]string{"q"}) {
		t.Fatal("mayMentionNg")
	}
}
