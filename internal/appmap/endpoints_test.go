package appmap

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type endpointsDoc struct {
	Schema   string            `json:"schema"`
	Complete bool              `json:"complete"`
	Total    int               `json:"endpoints_total"`
	Calls    []endpointCall    `json:"calls"`
	Unknown  []endpointUnknown `json:"unknown"`
	Rows     []struct {
		ID         string   `json:"id"`
		Handler    string   `json:"handler"`
		Controller string   `json:"controller"`
		Operation  string   `json:"operation_id"`
		Calls      []string `json:"calls"`
		Screens    []string `json:"screens"`
	} `json:"endpoints"`
}

const endpointRoutes = `Rails.application.routes.draw do
  get "api/clubs/:club_id/settings", to: "settings#show"
  patch "api/clubs/:club_id/settings", to: "settings#update"
  resources :orders, only: [:index, :show]
  get "dynamic", to: DynamicApp
end
`

// endpointFixture is the hop fixture app with a Rails backend, a JSON and a YAML OpenAPI document,
// and controllers calling them through $http, axios and fetch.
func endpointFixture(t *testing.T) prefixFixture {
	t.Helper()
	return hopRepo(t, map[string]string{
		"config/routes.rb":                       endpointRoutes,
		"app/controllers/settings_controller.rb": "class SettingsController < ApplicationController\n  def show\n  end\n\n  def update\n  end\nend\n",
		"api/openapi.json":                       `{"openapi":"3.0.0","servers":[{"url":"https://api.example.com/v2"}],"paths":{"/teesheets/{date}":{"get":{"operationId":"getTeesheet"}}}}`,
		"api/openapi.yaml":                       "openapi: 3.0.0\npaths: {}\n",
		"app/clubs/settings.ctrl.js": "angular.module('admin').controller('SettingsCtrl', ['$http', '$stateParams', function ($http, $stateParams) {\n" +
			"  $http.get('/api/clubs/' + $stateParams.clubId + '/settings');\n" +
			"  $http({ method: 'PATCH', url: `/api/clubs/${$stateParams.clubId}/settings` });\n" +
			"  fetch('/v2/teesheets/' + day + '?x=1');\n" +
			"  $http.post('https://other.example.com/x');\n" +
			"  $http.get(buildUrl());\n" +
			"  fetch('/nowhere');\n" +
			"}]);\n",
		"app/members/members.ctrl.js": "function MembersCtrl(MemberStore) {\n  axios.get('/orders');\n  axios.get('/orders/7.json');\n}\nangular.module('admin').controller('MembersCtrl', MembersCtrl);\n",
		"app/orphan.js":               "fetch('/orders/7');\n",
	})
}

// AMAP-V0-212..214: server endpoints from Rails routes and OpenAPI, client calls from $http, axios
// and fetch, and the join placing each endpoint on the screens whose source calls it; every call,
// route or file the join cannot read is unknown with its reason.
func TestAMAPV0212EndpointJoin(t *testing.T) {
	f := endpointFixture(t)
	eo := EndpointsOptions{RailsRoutes: []string{"config/routes.rb"}, OpenAPI: []string{"api/openapi.json", "api/openapi.yaml"}, Sources: []string{"app"}}
	raw, err := Endpoints(context.Background(), f.m, eo, Options{Root: f.root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var d endpointsDoc
	if err = json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Schema != EndpointsSchema || d.Complete {
		t.Fatalf("head: %s", raw)
	}
	settings, members := screenID("admin", "app.clubs.settings"), screenID("admin", "app.clubs.members")
	screens := map[string][]string{}
	for _, e := range d.Rows {
		screens[e.ID] = e.Screens
		if e.ID == "PATCH /api/clubs/{}/settings" && (e.Handler != "settings#update" || e.Controller != "app/controllers/settings_controller.rb") {
			t.Fatalf("rails handler: %+v", e)
		}
		if e.ID == "GET /v2/teesheets/{}" && e.Operation != "getTeesheet" {
			t.Fatalf("openapi operation: %+v", e)
		}
	}
	for id, want := range map[string][]string{
		"GET /api/clubs/{}/settings":   {settings},
		"PATCH /api/clubs/{}/settings": {settings},
		"GET /v2/teesheets/{}":         {settings},
		"GET /orders":                  {members},
		"GET /orders/{}":               {members},
	} {
		if got, ok := screens[id]; !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: screens %v, want %v (%s)", id, got, want, raw)
		}
	}
	reasons := map[string]string{}
	for _, u := range d.Unknown {
		reasons[u.At] = u.Reason
	}
	for at, want := range map[string]string{
		"api/openapi.yaml":             "openapi-yaml-unsupported",
		"app/clubs/settings.ctrl.js:5": "absolute-url",
		"app/clubs/settings.ctrl.js:6": "non-literal-url",
		"app/clubs/settings.ctrl.js:7": "no-declared-endpoint",
	} {
		if reasons[at] != want {
			t.Errorf("%s: reason %q, want %q (%+v)", at, reasons[at], want, d.Unknown)
		}
	}
	if reasons["app/orphan.js"] == "" || reasons["config/routes.rb:5"] == "" {
		t.Fatalf("orphan placement and opaque route must be unknown: %+v", d.Unknown)
	}
	// Bad queries refuse.
	for _, bad := range []EndpointsOptions{{Sources: []string{"app"}}, {RailsRoutes: []string{"config/routes.rb"}}, {RailsRoutes: []string{"../x"}, Sources: []string{"app"}}} {
		if _, err := Endpoints(context.Background(), f.m, bad, Options{Root: f.root}); codeOf(err) != "appmap-invalid-query" {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
}

// AMAP-V0-213: which client spellings are calls, with what verb and URL, and why the rest are
// unknown.
func TestAMAPV0213CallSites(t *testing.T) {
	for src, want := range map[string]string{
		"$http.get('/a/' + id);":                             "GET /a/{}",
		"$http.jsonp(`/a/${x}/b?q=1`);":                      "GET /a/{}/b",
		"$http({ url: '/a', method: 'DELETE' });":            "DELETE /a",
		"$http({ url: '/a', method: m });":                   "* /a",
		"axios('/a', { method: 'put' });":                    "PUT /a",
		"axios.request({ url: '/a' });":                      "GET /a",
		"axios.patch('/a', data, { headers: {} });":          "PATCH /a",
		"window.fetch('/a#top');":                            "GET /a",
		"fetch('/a', { method: 'POST', body: b });":          "POST /a",
		"fetch('/a', opts);":                                 "* /a",
		"this.fetch('/a'); obj.$http;":                       "! client-reference-not-a-call",
		"import axios from 'axios'; function f($http) {}":    "",
		"axios.get('/a', { baseURL: '/b' });":                "! config-base-url",
		"axios.create({});":                                  "! client-instance-not-modelled",
		"axios.defaults.baseURL = '/api';":                   "! client-defaults-not-read",
		"fetch(base + '/a');":                                "! url-base-not-literal",
		"fetch('a/b');":                                      "! relative-url",
		"fetch('//cdn.example.com/a');":                      "! absolute-url",
		"fetch(url);":                                        "! non-literal-url",
		"fetch(make('/a'));":                                 "! non-literal-url",
		"$http.foo('/a');":                                   "! client-method-not-modelled",
		"var get = $http.get;":                               "! client-reference-not-a-call",
		"const o = { fetch: 1 }; axios.interceptors.request": "! request-interceptor-not-modelled",
	} {
		calls, notes := scanCalls("f.js", []byte(src))
		got := []string{}
		for _, c := range calls {
			got = append(got, c.Verb+" "+c.URL)
		}
		for _, n := range notes {
			got = append(got, "! "+n.Reason)
		}
		if strings.Join(got, ";") != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
	}
}

// AMAP-V0-214: a call reaches an endpoint whose verb and every segment it may spell match.
func TestAMAPV0214Reaches(t *testing.T) {
	rails := func(verb, p string) *endpointRow {
		segs, ok := templateSegments(p, ':', '*')
		if !ok {
			t.Fatalf("template %s", p)
		}
		return &endpointRow{Verb: verb, segs: segs, format: true}
	}
	call := func(u string) []string {
		c := &endpointCall{}
		if r := c.setURL(strings.ReplaceAll(u, "{}", substMark)); r != "" {
			t.Fatalf("%s: %s", u, r)
		}
		return c.segs
	}
	for _, tc := range []struct {
		e    *endpointRow
		verb string
		url  string
		want bool
	}{
		{rails("GET", "/orders/:id"), "GET", "/orders/7", true},
		{rails("GET", "/orders/:id"), "HEAD", "/orders/7.json", true},
		{rails("GET", "/orders/:id"), "POST", "/orders/7", false},
		{rails("PATCH|PUT", "/orders/:id"), "PUT", "/orders/{}", true},
		{rails("GET", "/orders/new"), "GET", "/orders/{}", true},
		{rails("GET", "/orders/new"), "GET", "/orders/x{}", false},
		{rails("GET", "/files/*rest"), "*", "/files/a/b", true},
		{rails("GET", "/files/*rest"), "GET", "/files", false},
		{rails("ANY", "/"), "DELETE", "/", true},
		{rails("GET", "/orders"), "GET", "/orders/7", false},
	} {
		if got := tc.e.reaches(tc.verb, call(tc.url)); got != tc.want {
			t.Errorf("%s %v <- %s %s: %v", tc.e.Verb, tc.e.segs, tc.verb, tc.url, got)
		}
	}
	for doc, want := range map[string]string{
		`{"swagger":"2.0","basePath":"/v1","paths":{"/a/{id}":{"get":{},"post":{}}}}`: "",
		`{"openapi":"3.1.0","servers":[{"url":"/x"},{"url":"/y"}],"paths":{}}`:        "openapi-servers-differ",
		`{"openapi":"3.1.0","servers":[{"url":"https://h/{base}"}],"paths":{}}`:       "openapi-server-url-not-literal",
		`{"paths":{}}`: "openapi-unreadable",
		`{"openapi":"3.1.0","paths":{"/a/{id}.{ext}":{"get":{}}}}`: "openapi-path-not-template",
	} {
		rows, reason, _ := openAPIEndpoints("o.json", []byte(doc))
		if reason != want || (want == "" && (len(rows) != 2 || rows[0].Path != "/v1/a/{id}" || rows[1].Verb != "POST")) {
			t.Errorf("%s: %q %d", doc, reason, len(rows))
		}
	}
}
