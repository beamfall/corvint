package appmap

import (
	"strings"
	"testing"
)

// v13935Repo is the VanillaTwisted legal-page shape: a standalone <Routes> tree in one component
// and the main app tree after render guards, both declaring /about and /privacy; /privacy renders
// a different element in the main tree when differ is set.
func v13935Repo(t *testing.T, differ bool) *Map {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeFile(t, root, "package.json", `{"name":"web","devDependencies":{"@playwright/test":"1.50.0"}}`)
	writeFile(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"web",`+
		`"routers":[{"path":"apps/web/src/App.tsx","dialect":"react-router/0"}],`+
		`"tests":{"root":"apps/web","specs":["apps/web/e2e"],"page_objects":[]}}`)
	privacy := "<Privacy />"
	if differ {
		privacy = "<PrivacyV2 />"
	}
	writeFile(t, root, "apps/web/src/App.tsx", `import { Routes, Route } from "react-router-dom";
function StandaloneLegalRoute() {
  return (
    <main>
      <Routes>
        <Route path="/about" element={<About />} />
        <Route path="/privacy" element={<Privacy />} />
      </Routes>
    </main>
  );
}
export function App() {
  if (legalEscape) return <StandaloneLegalRoute />;
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/about" element={ <About/> } />
      <Route path="/privacy" element={`+privacy+`} />
    </Routes>
  );
}
`)
	writeFile(t, root, "apps/web/e2e/legal.spec.ts", `import { test } from "@playwright/test";
test("legal", async ({ page }) => {
  await page.goto("/about");
});
`)
	return build(t, root, commitAll(t, root, "fixture"))
}

// AMAP-V0-882 (V1-3935): one path with one element in two <Routes> trees is one screen and no
// ambiguous-template unknown; the merged screen keeps the render guard the main tree had.
func TestAMAPV0882SameElementAcrossTreesMerges_V13935(t *testing.T) {
	m := v13935Repo(t, false)
	byTemplate := map[string][]*Screen{}
	for i := range m.Screens {
		byTemplate[m.Screens[i].Template] = append(byTemplate[m.Screens[i].Template], &m.Screens[i])
	}
	for _, tpl := range []string{"/about", "/privacy"} {
		got := byTemplate[tpl]
		if len(got) != 1 || got[0].Status != StatusResolved {
			t.Fatalf("%s: screens %+v, want one resolved", tpl, got)
		}
		if got[0].Entry == nil || strings.Join(got[0].Entry.Constraints, ",") != "render-guarded" {
			t.Fatalf("%s: entry %+v, want the main tree's render-guarded constraint", tpl, got[0].Entry)
		}
	}
	for _, u := range m.Unknowns {
		if u.Reason == "ambiguous-template" {
			t.Fatalf("unexpected %+v", u)
		}
	}
	if s := byTemplate["/about"][0]; specFiles(s)["apps/web/e2e/legal.spec.ts"] != "goto" {
		t.Fatalf("/about specs %+v, want the literal goto", s.Specs)
	}
}

// AMAP-V0-882: the same path with different elements stays ambiguous, and the unknown's detail
// names both declarations; repeats inside one tree and explicit route ids never merge.
func TestAMAPV0882DifferentElementStaysAmbiguous_V13935(t *testing.T) {
	m := v13935Repo(t, true)
	var hit *Unknown
	for i, u := range m.Unknowns {
		if u.Reason == "ambiguous-template" {
			if hit != nil {
				t.Fatalf("second ambiguity %+v", u)
			}
			hit = &m.Unknowns[i]
		}
	}
	if hit == nil || hit.Ref != "/privacy" || hit.Detail != "declared by screen:web:react:apps/web/src/App.tsx:2, screen:web:react:apps/web/src/App.tsx:5" {
		t.Fatalf("ambiguity %+v, want /privacy naming both declarations", hit)
	}
	same := func(name string, tree int) rawState {
		return rawState{name: name, url: "^/a", element: "< A / >", tree: tree}
	}
	gen := "react:r.tsx:"
	if got := reactMergeAlternates([]rawState{same(gen+"1", 1), same(gen+"2", 1)}, gen); len(got) != 2 {
		t.Fatalf("one tree's repeat merged: %+v", got)
	}
	if got := reactMergeAlternates([]rawState{same(gen+"1", 1), same("legal-a", 2)}, gen); len(got) != 2 {
		t.Fatalf("an explicit id merged: %+v", got)
	}
	unread := same(gen+"2", 2)
	unread.element = ""
	if got := reactMergeAlternates([]rawState{same(gen+"1", 1), unread}, gen); len(got) != 2 {
		t.Fatalf("an unread element merged: %+v", got)
	}
	if got := reactMergeAlternates([]rawState{same(gen+"1", 1), same(gen+"2", 2)}, gen); len(got) != 1 || got[0].entry != nil {
		t.Fatalf("alternate trees: %+v, want one state with no entry", got)
	}
}
