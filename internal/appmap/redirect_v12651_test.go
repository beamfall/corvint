// SPDX-License-Identifier: Apache-2.0
package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// AMAP-V0-8206: literal absolute/relative redirects resolve across router
// files, carrying every redirect's anchor into target freshness. Computed,
// cyclic, external and ambiguous targets stay unknown after serialization.
func TestAMAPV08206NavigateTargetsAndFreshness(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"redirects","base_path":"/web","routers":[{"path":"src/aliases.tsx","dialect":"react-router/0"},{"path":"src/pages.tsx","dialect":"react-router/0"}],"tests":{"root":"e2e","specs":["e2e"],"page_objects":[]}}`)
	writeFile(t, root, "src/aliases.tsx", `function RoutesA(){return <Routes>
 <Route path="/manage" element={<Layout />}>
   <Route index element={<Navigate to="libraries" replace />} />
   <Route path="old" element={<Navigate to="../libraries" />} />
 </Route>
 <Route path="/absolute" element={<Navigate to="/manage/libraries" />} />
 <Route path="/ambiguous" element={<Navigate to="/dupe" />} />
 <Route path="/expression" element={<Navigate to={"/manage/libraries"} />} />
 <Route path="/chain" element={<Navigate to="/absolute" />} />
 <Route path="/dynamic" element={<Navigate to={target} />} />
 <Route path="/spread" element={<Navigate to="/manage/libraries" {...props} />} />
 <Route path="/cycle" element={<Navigate to="/cycle" />} />
 <Route path="/external" element={<Navigate to="https://example.test/x" />} />
 <Route path="/danger" element={<Navigate to="javascript:alert(1)" />} />
 <Route path="/missing" element={<Navigate to="/absent" />} />
 <Route path="/multi/segment" element={<Layout />}><Route path="child" element={<Navigate to="../libraries" />} /></Route>
 </Routes>}`)
	writeFile(t, root, "src/pages.tsx", `function RoutesB(){return <Routes>
 <Route path="/manage/libraries" element={<Libraries />} />
 <Route path="/multi/segment/libraries" element={<Other />} />
 <Route path="/dupe" element={<First />} />
 <Route path="/dupe" element={<Second />} />
 </Routes>}`)
	writeFile(t, root, "e2e/manage.spec.ts", `import {test} from '@playwright/test'; test('manage',async({page})=>{await page.goto('/web/manage');});`)
	rev := commitAll(t, root, "redirect fixture")
	m := build(t, root, rev)
	check := func(m *Map) {
		t.Helper()
		idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
		target, reason := idx.byURL("/web/manage/libraries")
		if target == "" {
			t.Fatalf("target: %s", reason)
		}
		for _, url := range []string{"/web/manage", "/web/manage/old", "/web/absolute", "/web/chain", "/web/expression"} {
			if got, why := idx.byURL(url); got != target || why != "" {
				t.Fatalf("%s -> %s %s, want %s", url, got, why, target)
			}
		}
		if got, why := idx.byURL("/web/multi/segment/child"); got == "" || why != "" {
			t.Fatalf("route relative: %s %s", got, why)
		}
		for _, url := range []string{"/web/dynamic", "/web/spread", "/web/cycle", "/web/external", "/web/missing", "/web/ambiguous", "/web/danger"} {
			if got, why := idx.byURL(url); got != "" || why == "" {
				t.Fatalf("unsafe %s -> %s %s", url, got, why)
			}
		}
		s := m.screen(target)
		if len(s.Specs) != 1 || s.Specs[0].File != "e2e/manage.spec.ts" {
			t.Fatalf("redirect attribution: %+v", s.Specs)
		}
		found := false
		for _, a := range m.lineage(s) {
			if a.Path == "src/aliases.tsx" {
				found = true
			}
		}
		if !found {
			t.Fatal("redirect anchor missing from target lineage")
		}
	}
	check(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Map
	if err = json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	check(&loaded)
	target, _ := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens).byURL("/web/manage")
	writeFile(t, root, "src/aliases.tsx", strings.Repeat("// changed redirect\n", 2))
	newer := commitAll(t, root, "redirect changed")
	if got := LineageFreshness(context.Background(), root, newer, m, []string{target})[target]; got != Stale {
		t.Fatalf("changed redirect freshness: %s", got)
	}
}

// AMAP-V0-8206: redirect chains retain the router fragment under a base path.
func TestAMAPV08206HashRedirect(t *testing.T) {
	to := "/target"
	chain := "/alias"
	screens := []Screen{
		{ID: "target", Status: StatusResolved, Template: "/target", Key: "/target"},
		{ID: "alias", Anchor: Anchor{Path: "alias.tsx"}, Status: StatusResolved, Template: "/alias", Key: "/alias", Redirect: &RouteRedirect{To: &to}},
		{ID: "chain", Anchor: Anchor{Path: "chain.tsx"}, Status: StatusResolved, Template: "/chain", Key: "/chain", Redirect: &RouteRedirect{To: &chain}},
	}
	for _, prefix := range []string{"#", "#!"} {
		for _, base := range []string{"", "/web"} {
			for _, target := range []string{"/target", "/" + prefix + "/target", prefix + "/target"} {
				to = target
				x := newScreenIndex(prefix, base, screens)
				fresh := append([]Screen(nil), screens...)
				attachRedirectFreshness(prefix, fresh)
				if len(fresh[0].RedirectFrom) != 2 {
					t.Fatalf("%s %s redirect freshness: %+v", prefix, target, fresh[0].RedirectFrom)
				}
				for _, route := range []string{"/alias", "/chain"} {
					url := base + "/" + prefix + route
					if got, why := x.byURL(url); got != "target" || why != "" {
						t.Fatalf("%s -> %q %s", url, got, why)
					}
				}
			}
		}
	}
}
