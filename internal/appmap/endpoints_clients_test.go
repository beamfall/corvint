package appmap

import (
	"strings"
	"testing"
)

func scannedCalls(file, src string) string {
	calls, notes := scanCalls(file, []byte(src))
	got := []string{}
	for _, c := range calls {
		got = append(got, c.Client+" "+c.Verb+" "+c.URL)
	}
	for _, n := range notes {
		got = append(got, "! "+n.At+" "+n.Reason+" "+n.Detail)
	}
	return strings.Join(got, ";")
}

// AMAP-V0-356: a .tsx or .jsx file is lexed with JSX, so element text with an apostrophe does not
// unbalance the lexer, a call inside an expression container is read, and a TypeScript generic
// arrow's `<T,>` is not an element; a .ts file keeps the plain lexer.
func TestAMAPV0356ClientsJSX(t *testing.T) {
	src := "const pick = <T,>(x: T) => x;\nexport function Item({ id }: { id: string }) {\n" +
		"  return <div className=\"row\">Don't <b>panic</b>\n" +
		"    <button onClick={() => fetch(`/api/items/${id}`, { method: 'DELETE' })}>x</button>\n  </div>;\n}\n"
	for _, file := range []string{"app/item.tsx", "app/item.jsx"} {
		if got := scannedCalls(file, src); got != "fetch DELETE /api/items/{}" {
			t.Errorf("%s: %q", file, got)
		}
	}
	if got := scannedCalls("app/item.ts", "const n = <number>x;\nfetch('/api/a');\n"); got != "fetch GET /api/a" {
		t.Errorf("ts: %q", got)
	}
	// A defaulted type parameter is a generic arrow too, not an element.
	if got := scannedCalls("app/id.tsx", "const id = <T = unknown>(x: T) => x;\nfetch('/api/a');\n"); got != "fetch GET /api/a" {
		t.Errorf("defaulted generic: %q", got)
	}
	if got := scannedCalls("app/broken.tsx", "const a = <div>{fetch('/api/a')</div>;\n"); got != "! app/broken.tsx source-not-lexed " {
		t.Errorf("unbounded element: %q", got)
	}
}

// AMAP-V0-357: an AngularJS $resource with a literal url template yields its default actions and
// its literal declared actions at the declaration, with and without a trailing parameter; an
// action not read and a parameter that may be omitted mid-path are unknown.
func TestAMAPV0357Resource(t *testing.T) {
	src := "app.factory('Item', function ($resource) {\n  return $resource('/api/items/:id', { id: '@id' }, {\n" +
		"    update: { method: 'PUT' },\n    send: { method: 'POST', url: '/api/items/:id/send' },\n    odd: { method: verb }\n  });\n});\n"
	want := "$resource GET /api/items/{};$resource GET /api/items;$resource POST /api/items/{};$resource POST /api/items;" +
		"$resource DELETE /api/items/{};$resource DELETE /api/items;$resource POST /api/items/{}/send;" +
		"$resource PUT /api/items/{};$resource PUT /api/items;" +
		"! app/items.js:2 non-literal-resource-action odd;! app/items.js:2 resource-param-may-be-omitted /api/items/:id/send"
	if got := scannedCalls("app/items.js", src); got != want {
		t.Errorf("resource:\n got %q\nwant %q", got, want)
	}
	if got := scannedCalls("app/items.js", "$resource(base + '/items');\n"); got != "! app/items.js:1 non-literal-url " {
		t.Errorf("non-literal: %q", got)
	}
}

// AMAP-V0-358: an axios instance created in the file with a literal baseURL prefixes its relative
// calls; a non-literal baseURL, a reassigned name and an unbound create are stated once at the
// create, and an exported instance is read but stated as escaping.
func TestAMAPV0358AxiosInstances(t *testing.T) {
	for src, want := range map[string]string{
		"const api = axios.create({ baseURL: '/api/', timeout: 5 });\napi.get('/items/' + id);\napi.post('orders');\n": "axios-instance GET /api/items/{};axios-instance POST /api/orders",
		"const api = axios.create();\napi.delete('/a');\n":                                                             "axios-instance DELETE /a",
		"const api = axios.create({ baseURL: host });\napi.get('/items');\n":                                           "! f.js:1 client-instance-not-modelled api",
		"let api = axios.create({ baseURL: '/x' });\napi = other;\napi.get('/items');\n":                               "! f.js:1 client-instance-shadowed api",
		"export const api = axios.create({ baseURL: '/x' });\napi.get('/items');\n":                                    "axios-instance GET /x/items;! f.js:1 client-instance-escapes api",
		"wrap(axios.create({ baseURL: '/x' }));\n":                                                                     "! f.js:1 client-instance-not-modelled axios.create",
		"const api = axios.create({ baseURL: '/x' });\napi.get('/a', { baseURL: '/y' });\n":                            "! f.js:2 config-base-url /x/a",
		// A config key that may change the URL or method, and an initializer chained past the
		// create, are stated, never read as a plain GET.
		"const api = axios.create({ method: 'POST' });\napi('/items');\n":                     "! f.js:1 client-instance-not-modelled api",
		"const api = axios.create({ headers: { a: 'b' }, url: '/y' });\napi.get('/items');\n": "! f.js:1 client-instance-not-modelled api",
		"const request = axios.create({ baseURL: '/api' }).get('/items');\n":                  "! f.js:1 client-instance-not-modelled request",
		"const request = axios.create()\n  .get('/items');\n":                                 "! f.js:1 client-instance-not-modelled request",
		"const api = axios.create({ baseURL: '/x' })\napi.get('/a');\n":                       "axios-instance GET /x/a",
	} {
		if got := scannedCalls("f.js", src); got != want {
			t.Errorf("%s\n got %q\nwant %q", src, got, want)
		}
	}
}
