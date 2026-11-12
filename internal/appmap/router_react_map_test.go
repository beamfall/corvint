package appmap

import (
	"slices"
	"testing"
)

// reactTemplates parses a React router source and returns its resolved screen templates and the
// reasons of its unknowns.
func reactTemplates(t *testing.T, src string) ([]string, []string) {
	t.Helper()
	states, unknowns := parseReactRouter(blobEntry{path: "console.tsx"}, []byte(src))
	screens, _ := resolveScreens("test", states)
	templates, reasons := []string{}, []string{}
	for _, s := range screens {
		if s.Status == StatusResolved && !s.Abstract {
			templates = append(templates, s.Template)
		}
	}
	for _, u := range unknowns {
		reasons = append(reasons, u.Reason)
	}
	slices.Sort(templates)
	return templates, reasons
}

// AMAP-V0-970: a `.map` child over a same-file literal array of objects expands into one route per
// element; `key` is ignored, and a useMemo array and a template path resolve.
func TestAMAPV0970MapOverSameFileArray(t *testing.T) {
	for name, c := range map[string]struct {
		src  string
		want []string
	}{
		"local const": {`export function Console() {
  const sections = [{path: 'users', title: 'Users'}, {path: 'roles', title: 'Roles'}];
  return (<Routes>
    <Route path="/" element={<Home />} />
    {sections.map(s => <Route key={s.path} path={s.path} element={<Page s={s} title={s.title} />} />)}
  </Routes>);
}
`, []string{"/", "/roles", "/users"}},
		"top-level typed const, parenthesised": {`const SECTIONS: Section[] = [
  { path: 'users', id: 'console-users' },
  { path: 'roles', id: 'console-roles' },
] as const;
export function Console() {
  return (<Routes>
    {SECTIONS.map((s) => (
      <Route key={s.id} id={s.id} path={s.path} element={<Page section={s} />} />
    ))}
  </Routes>);
}
`, []string{"/roles", "/users"}},
		"useMemo and template path": {`export function Console() {
  const sections = useMemo(() => [{path: 'a'}, {path: 'b'}], []);
  return (<Routes>{sections.map(s => <Route path={` + "`${s.path}/*`" + `} element={<Page />} />)}</Routes>);
}
`, []string{"/a/{*splat}", "/b/{*splat}"}},
		"key on a plain route": {`function A() { return (<Routes><Route key="k" path="/a" element={<A />} /></Routes>); }`, []string{"/a"}},
	} {
		got, reasons := reactTemplates(t, c.src)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: screens %v, want %v (unknowns %v)", name, got, c.want, reasons)
		}
		if len(reasons) != 0 {
			t.Errorf("%s: unknowns %v, want none", name, reasons)
		}
	}
	states, _ := parseReactRouter(blobEntry{path: "console.tsx"}, []byte(`const SECTIONS = [{path: 'users'}, {path: 'roles'}];
function A() { return (<Routes>{SECTIONS.map(s => <Route path={s.path} element={<Page id={s.path} />} />)}</Routes>); }`))
	if len(states) != 2 || states[0].element == states[1].element || states[0].element == "" {
		t.Errorf("expanded elements must carry each value for the AMAP-V0-882 comparison: %+v", states)
	}
}

// AMAP-V0-971: anything the reader cannot prove literal keeps non-literal-react-children and
// expands nothing.
func TestAMAPV0971UnprovenArrayStaysUnknown(t *testing.T) {
	const routes = `return (<Routes><Route path="/" element={<Home />} />{sections.map(s => <Route path={s.path} element={<Page />} />)}</Routes>);`
	for name, src := range map[string]string{
		"mutated":              `function A() { const sections = [{path: 'users'}]; sections.push({path: 'x'}); ` + routes + ` }`,
		"index assignment":     `function A() { const sections = [{path: 'users'}]; sections[0] = {path: 'x'}; ` + routes + ` }`,
		"non-literal element":  `function A() { const sections = [{path: computePath()}]; ` + routes + ` }`,
		"spread element":       `function A() { const sections = [{...base, path: 'a'}]; ` + routes + ` }`,
		"not an object":        `function A() { const sections = ['users']; ` + routes + ` }`,
		"second declaration":   `const sections = [{path: 'a'}]; function A() { const sections = [{path: 'b'}]; ` + routes + ` }`,
		"exported":             `export const sections = [{path: 'a'}]; function A() { ` + routes + ` }`,
		"let binding":          `function A() { let sections = [{path: 'a'}]; ` + routes + ` }`,
		"parameter":            `function A({sections}) { ` + routes + ` }`,
		"imported":             `import { sections } from './sections'; function A() { ` + routes + ` }`,
		"filtered":             `function A() { const sections = all.filter(ok); ` + routes + ` }`,
		"escapes":              `function A() { const sections = [{path: 'a'}]; register(sections); ` + routes + ` }`,
		"template names it":    "function A() { const sections = [{path: 'a'}]; log(`${sections}`); " + routes + ` }`,
		"path not from member": `function A() { const sections = [{path: 'a'}]; return (<Routes><Route path="/" element={<Home />} />{sections.map(s => <Route path={s} element={<Page />} />)}</Routes>); }`,
		"member not a string":  `function A() { const sections = [{path: 'a', depth: 2}]; return (<Routes><Route path="/" element={<Home />} />{sections.map(s => <Route path={s.path} index={s.depth} element={<Page />} />)}</Routes>); }`,
		"not a Route":          `function A() { const sections = [{path: 'a'}]; return (<Routes><Route path="/" element={<Home />} />{sections.map(s => <Page path={s.path} />)}</Routes>); }`,
	} {
		got, reasons := reactTemplates(t, src)
		if !slices.Contains(reasons, "non-literal-react-children") {
			t.Errorf("%s: unknowns %v, want non-literal-react-children", name, reasons)
		}
		if !slices.Equal(got, []string{"/"}) {
			t.Errorf("%s: screens %v, want only the literal sibling /", name, got)
		}
	}
}
