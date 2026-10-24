package contextindex

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// setupSelectFiles is a suite whose setup projects seed data contexts that specs read through a
// declared registry key, plus one seed a spec imports directly and a hub listing every seed.
func setupSelectFiles() map[string]string {
	files := registryIssueFiles()
	for _, name := range []string{"tests/alpha.spec.ts", "tests/beta.spec.ts", "tests/gamma.spec.ts"} {
		delete(files, name)
	}
	provider := "import { DataProvider } from './data/provider';\nconst dp = new DataProvider();\n"
	files["setup/alpha.setup.ts"] = "import { AlphaContext } from '../src/contexts/alpha';\nexport default new AlphaContext();\n"
	files["setup/beta.setup.ts"] = "import Beta from '../src/contexts/beta';\nexport default Beta;\n"
	files["setup/gamma.setup.ts"] = "import type { GammaContext } from '../src/contexts/gamma';\nimport { seed } from './shared';\nexport default seed;\n"
	files["setup/shared.ts"] = "export const seed = 1;\n"
	files["setup/users.seed.ts"] = "export const users = ['ada'];\n"
	files["tests/hub.ts"] = "export * from '../setup/alpha.setup';\nexport * from '../setup/beta.setup';\n" +
		"export * from '../setup/gamma.setup';\nexport * from '../setup/users.seed';\n"
	files["tests/alpha.spec.ts"] = provider + "test('alpha', () => dp.createTestData('ctx:alpha'));\n"
	files["tests/users.spec.ts"] = "import { users } from '../setup/users.seed';\ntest('users', () => users);\n"
	files["tests/hubbed.spec.ts"] = "import * as all from './hub';\n" + provider + "test('beta', () => dp.createTestData(`ctx:beta`));\n"
	files["tests/commented.spec.ts"] = provider + "// dp.createTestData('ctx:gamma')\n/* import '../setup/users.seed'; */\n" +
		"test('alpha', () => dp.createTestData('ctx:alpha'));\n"
	files["tests/computed.spec.ts"] = provider + "const name = process.env.CTX;\ntest('x', () => dp.createTestData(name));\n"
	files["tests/unknown.spec.ts"] = provider + "test('x', () => dp.createTestData('ctx:nope'));\n"
	files["tests/dynamic.spec.ts"] = "const which = 'users';\ntest('x', async () => import(`../setup/${which}.seed`));\n"
	files["tests/required.spec.ts"] = "const { users } = require('../setup/users.seed');\ntest('x', () => users);\n"
	return files
}

func setupSelectIndex(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, setupSelectFiles())
	head := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	index, err := BuildRevisionContext(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func reasonCodes(selection SetupSelection) []string {
	codes := []string{}
	for _, reason := range selection.Reasons {
		codes = append(codes, reason.Code)
	}
	return codes
}

// TestSelectSetupClosure proves specs get exactly the setup files they reach directly, through a
// registry key's data context, or through require(), that a hub listing every seed is not
// followed, and that comment-only mentions are ignored (TSA-V0-001, TSA-V0-002, TSA-V0-004).
func TestSelectSetupClosure(t *testing.T) {
	t.Parallel()
	index := setupSelectIndex(t)
	cases := []struct {
		specs []string
		want  []string
	}{
		{[]string{"tests/alpha.spec.ts"}, []string{"setup/alpha.setup.ts"}},
		{[]string{"tests/users.spec.ts"}, []string{"setup/users.seed.ts"}},
		{[]string{"tests/required.spec.ts"}, []string{"setup/users.seed.ts"}},
		{[]string{"tests/hubbed.spec.ts"}, []string{"setup/beta.setup.ts"}},
		{[]string{"tests/commented.spec.ts"}, []string{"setup/alpha.setup.ts"}},
		{[]string{"tests/users.spec.ts", "tests/alpha.spec.ts"}, []string{"setup/alpha.setup.ts", "setup/users.seed.ts"}},
	}
	for _, c := range cases {
		selection := SelectSetup(index, c.specs)
		if selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, c.want) {
			t.Errorf("%v: mode=%s files=%v reasons=%+v, want %v", c.specs, selection.Mode, selection.SetupFiles, selection.Reasons, c.want)
		}
		if selection.Universe != 4 || selection.Revision != index.CommitRevision {
			t.Errorf("%v: universe=%d revision=%q", c.specs, selection.Universe, selection.Revision)
		}
	}
	hubbed := SelectSetup(index, []string{"tests/hubbed.spec.ts"})
	if !reflect.DeepEqual(hubbed.NotFollowed, []SetupNotFollowed{{Path: "tests/data/registry.ts", Reason: "registry"}, {Path: "tests/hub.ts", Reason: "hub"}}) {
		t.Fatalf("not followed = %+v", hubbed.NotFollowed)
	}
}

// TestSelectSetupAll proves a computed lookup, unknown key, missing path, non-literal import() or
// empty spec list returns the whole universe with a typed reason (TSA-V0-003).
func TestSelectSetupAll(t *testing.T) {
	t.Parallel()
	index := setupSelectIndex(t)
	all := []string{"setup/alpha.setup.ts", "setup/beta.setup.ts", "setup/gamma.setup.ts", "setup/users.seed.ts"}
	cases := []struct {
		specs []string
		codes []string
	}{
		{[]string{"tests/computed.spec.ts"}, []string{"computed-lookup"}},
		{[]string{"tests/unknown.spec.ts"}, []string{"unknown-key"}},
		{[]string{"tests/dynamic.spec.ts"}, []string{"computed-lookup"}},
		{[]string{"tests/absent.spec.ts"}, []string{"missing-path"}},
		{[]string{"./tests/alpha.spec.ts"}, []string{"missing-path"}},
		{nil, []string{"empty-spec-list"}},
		{[]string{" "}, []string{"empty-spec-list"}},
	}
	for _, c := range cases {
		selection := SelectSetup(index, c.specs)
		if selection.Mode != "all" || !reflect.DeepEqual(selection.SetupFiles, all) || !reflect.DeepEqual(reasonCodes(selection), c.codes) {
			t.Errorf("%v: mode=%s files=%v reasons=%+v", c.specs, selection.Mode, selection.SetupFiles, selection.Reasons)
		}
	}
}

// TestSelectSetupDigest proves the result names the revision and a digest of the inputs read that
// is stable across calls and changes with the spec list (TSA-V0-005).
func TestSelectSetupDigest(t *testing.T) {
	t.Parallel()
	index := setupSelectIndex(t)
	first := SelectSetup(index, []string{"tests/alpha.spec.ts"})
	again := SelectSetup(index, []string{"tests/alpha.spec.ts"})
	other := SelectSetup(index, []string{"tests/users.spec.ts"})
	if !strings.HasPrefix(first.InputDigest, "sha256:") || first.InputDigest != again.InputDigest || first.InputDigest == other.InputDigest {
		t.Fatalf("digests: %s %s %s", first.InputDigest, again.InputDigest, other.InputDigest)
	}
	if len(first.Revision) != 40 || first.Schema != SetupSelectSchema {
		t.Fatalf("header = %+v", first)
	}
}

// TestSelectSetupTraversesRegistrySeeders proves a setup file selected through a registry key is
// itself traversed: a setup it imports is selected, and an unresolved import there returns ALL
// (TSA-V0-001, TSA-V0-003).
func TestSelectSetupTraversesRegistrySeeders(t *testing.T) {
	t.Parallel()
	files := setupSelectFiles()
	files["setup/alpha.setup.ts"] = "import { AlphaContext } from '../src/contexts/alpha';\nimport './base.setup';\nexport default new AlphaContext();\n"
	files["setup/base.setup.ts"] = "export const base = 1;\n"
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	selection := SelectSetup(index, []string{"tests/alpha.spec.ts"})
	if selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, []string{"setup/alpha.setup.ts", "setup/base.setup.ts"}) {
		t.Fatalf("mode=%s files=%v reasons=%+v", selection.Mode, selection.SetupFiles, selection.Reasons)
	}
	files["setup/base.setup.ts"] = "import './absent.setup';\nexport const base = 1;\n"
	root = impactRepositoryWithFiles(t, files)
	if index, err = BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))); err != nil {
		t.Fatal(err)
	}
	selection = SelectSetup(index, []string{"tests/alpha.spec.ts"})
	if selection.Mode != "all" || !reflect.DeepEqual(reasonCodes(selection), []string{"import-unresolved"}) {
		t.Fatalf("mode=%s reasons=%+v", selection.Mode, selection.Reasons)
	}
}

// TestSelectSetupUniverseIncludesUnindexedFiles proves ALL names every tracked setup file,
// including a generated one the index does not read (TSA-V0-003).
func TestSelectSetupUniverseIncludesUnindexedFiles(t *testing.T) {
	t.Parallel()
	files := setupSelectFiles()
	files["setup/gen/omega.setup.ts"] = "export const omega = 1;\n"
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	if _, indexed := index.Sources["setup/gen/omega.setup.ts"]; indexed {
		t.Fatal("the generated setup file was indexed; the fixture no longer exercises an unindexed path")
	}
	selection := SelectSetup(index, nil)
	if selection.Mode != "all" || selection.Universe != 5 || selection.SetupFiles[len(selection.SetupFiles)-1] != "setup/users.seed.ts" ||
		!strings.Contains(strings.Join(selection.SetupFiles, ","), "setup/gen/omega.setup.ts") {
		t.Fatalf("mode=%s universe=%d files=%v", selection.Mode, selection.Universe, selection.SetupFiles)
	}
}

// sharedConstantFiles is the GitHub #1082 shape: two setup files seed different literal keys of one
// exported constant object, and data contexts read those keys without importing any setup file.
func sharedConstantFiles() map[string]string {
	read := func(body string) string { return "import { SHARED } from '../shared_map';\n" + body }
	return map[string]string{
		"lib/data/shared_map.ts":         "export const SHARED = { scenario_a: { id: 'a' }, scenario_b: { id: 'b' } } as const;\n",
		"lib/data/barrel.ts":             "export * from './shared_map';\n",
		"lib/data/renamed.ts":            "export { SHARED as DATA } from './shared_map';\n",
		"setup/scenario_a.setup.ts":      "import { SHARED } from '../lib/data/shared_map';\nexport default async () => db.insert(SHARED.scenario_a);\n",
		"setup/scenario_b.setup.ts":      "import { SHARED } from '../lib/data/shared_map';\nexport default async () => db.insert(SHARED['scenario_b']);\n",
		"lib/data/contexts/a_context.ts": read("type Key = keyof typeof SHARED;\n// SHARED.scenario_b is not read here\nexport const aContext = () => SHARED.scenario_a;\n"),
		"lib/data/contexts/k_context.ts": read("export const kContext = (k: string) => SHARED[k];\n"),
		"lib/data/contexts/w_context.ts": read("export const wContext = () => use(SHARED);\n"),
		"lib/data/contexts/t_context.ts": read("export const tContext = () => `/users/${SHARED.scenario_b.id}`;\n"),
		"lib/data/contexts/d_context.ts": "import { SHARED } from '../barrel';\nconst { scenario_a } = SHARED;\nexport const dContext = () => scenario_a;\n",
		"lib/data/contexts/r_context.ts": "import { DATA as D } from '../renamed';\nexport const rContext = () => D?.scenario_b;\n",
		"lib/data/contexts/n_context.ts": "import * as data from '../shared_map';\nexport const nContext = () => data.SHARED.scenario_a;\n",
		"lib/data/contexts/m_context.ts": "import * as data from '../shared_map';\nexport const mContext = () => Object.keys(data);\n",
		"tests/x.spec.ts":                "import { aContext } from '../lib/data/contexts/a_context';\ntest('x', () => aContext());\n",
		"tests/fixture.ts":               "import { aContext } from '../lib/data/contexts/a_context';\nexport const fixture = aContext;\n",
		"tests/fixtured.spec.ts":         "import { fixture } from './fixture';\ntest('f', () => fixture());\n",
		"tests/computed.spec.ts":         "import { kContext } from '../lib/data/contexts/k_context';\ntest('k', () => kContext('scenario_a'));\n",
		"tests/whole.spec.ts":            "import { wContext } from '../lib/data/contexts/w_context';\ntest('w', () => wContext());\n",
		"tests/template.spec.ts":         "import { tContext } from '../lib/data/contexts/t_context';\ntest('t', () => tContext());\n",
		"tests/destructured.spec.ts":     "import { dContext } from '../lib/data/contexts/d_context';\ntest('d', () => dContext());\n",
		"tests/renamed.spec.ts":          "import { rContext } from '../lib/data/contexts/r_context';\ntest('r', () => rContext());\n",
		"tests/namespace.spec.ts":        "import { nContext } from '../lib/data/contexts/n_context';\ntest('n', () => nContext());\n",
		"tests/namespace-whole.spec.ts":  "import { mContext } from '../lib/data/contexts/m_context';\ntest('m', () => mContext());\n",
		"tests/plain.spec.ts":            "test('plain', () => expect(1).toBe(1));\n",
		"tests/unrelated.spec.ts":        "import { other } from '../lib/other';\ntest('u', () => other.a);\n",
		"lib/other.ts":                   "export const other = { a: 1 };\n",
	}
}

// TestSelectSetupSharedConstantKeys proves the GitHub #1082 shape selects exactly the setup file
// that reads the same literal key of the same exported constant, through a fixture, an import
// alias, a re-export, `export *`, a namespace member, destructuring and a template substitution,
// and that a spec reaching no shared constant still selects nothing (TSA-V0-006).
func TestSelectSetupSharedConstantKeys(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, sharedConstantFiles())
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	a, b := []string{"setup/scenario_a.setup.ts"}, []string{"setup/scenario_b.setup.ts"}
	cases := []struct {
		spec string
		want []string
	}{
		{"tests/x.spec.ts", a},
		{"tests/fixtured.spec.ts", a},
		{"tests/template.spec.ts", b},
		{"tests/destructured.spec.ts", a},
		{"tests/renamed.spec.ts", b},
		{"tests/namespace.spec.ts", a},
		{"tests/plain.spec.ts", []string{}},
		{"tests/unrelated.spec.ts", []string{}},
	}
	for _, c := range cases {
		selection := SelectSetup(index, []string{c.spec})
		if selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, c.want) || len(selection.Reasons) != 0 {
			t.Errorf("%s: mode=%s files=%v reasons=%+v, want %v", c.spec, selection.Mode, selection.SetupFiles, selection.Reasons, c.want)
		}
	}
}

// TestSelectSetupSharedConstantFailsClosed proves a computed key, a whole-object use and a
// namespace used other than as NAMESPACE.EXPORT return ALL with shared-constant-key-unresolved
// when an unselected setup file reads the same constant (TSA-V0-007).
func TestSelectSetupSharedConstantFailsClosed(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, sharedConstantFiles())
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	all := []string{"setup/scenario_a.setup.ts", "setup/scenario_b.setup.ts"}
	for _, spec := range []string{"tests/computed.spec.ts", "tests/whole.spec.ts", "tests/namespace-whole.spec.ts"} {
		selection := SelectSetup(index, []string{spec})
		if selection.Mode != "all" || !reflect.DeepEqual(selection.SetupFiles, all) ||
			!reflect.DeepEqual(reasonCodes(selection), []string{"shared-constant-key-unresolved"}) || selection.Reasons[0].Line != 2 {
			t.Errorf("%s: mode=%s files=%v reasons=%+v", spec, selection.Mode, selection.SetupFiles, selection.Reasons)
		}
	}
}

// TestSelectSetupSharedConstantExportForms proves the other export forms and access spellings that
// TSA-V0-006 names: a default-exported object, a local object exported under an alias, `!.` and
// `?.["key"]`.
func TestSelectSetupSharedConstantExportForms(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, map[string]string{
		"lib/defaults.ts":        "export default { one: 1, two: 2 };\n",
		"lib/local.ts":           "const LOCAL: Record<string, number> = { one: 1, two: 2 };\nexport { LOCAL as ALIASED };\n",
		"setup/one.setup.ts":     "import D from '../lib/defaults';\nimport { ALIASED } from '../lib/local';\nexport default () => [D.one, ALIASED.one];\n",
		"setup/two.setup.ts":     "import D from '../lib/defaults';\nimport { ALIASED } from '../lib/local';\nexport default () => [D?.['two'], ALIASED['two']];\n",
		"tests/default.spec.ts":  "import Defaults from '../lib/defaults';\ntest('d', () => Defaults!.one);\n",
		"tests/aliased.spec.ts":  "import { ALIASED as A } from '../lib/local';\ntest('a', () => A?.[\"two\"]);\n",
		"tests/computed.spec.ts": "import Defaults from '../lib/defaults';\ntest('c', () => Defaults[process.env.K]);\n",
	})
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	for spec, want := range map[string][]string{"tests/default.spec.ts": {"setup/one.setup.ts"}, "tests/aliased.spec.ts": {"setup/two.setup.ts"}} {
		if selection := SelectSetup(index, []string{spec}); selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, want) {
			t.Errorf("%s: mode=%s files=%v reasons=%+v, want %v", spec, selection.Mode, selection.SetupFiles, selection.Reasons, want)
		}
	}
	if selection := SelectSetup(index, []string{"tests/computed.spec.ts"}); selection.Mode != "all" || !reflect.DeepEqual(reasonCodes(selection), []string{"shared-constant-key-unresolved"}) {
		t.Errorf("computed default: mode=%s reasons=%+v", selection.Mode, selection.Reasons)
	}
}

// sharedHops returns a chain lib/hopN.ts -> ... -> lib/hop8.ts -> lib/shared.ts of `export *` edges.
func sharedHops(files map[string]string, first int) {
	for hop := first; hop <= 8; hop++ {
		next := "./hop" + strconv.Itoa(hop+1)
		if hop == 8 {
			next = "./shared"
		}
		files["lib/hop"+strconv.Itoa(hop)+".ts"] = "export * from '" + next + "';\n"
	}
}

// sharedTemplate nests read in depth interpolated template literals.
func sharedTemplate(read string, depth int) string {
	for ; depth > 0; depth-- {
		read = "`${" + read + "}`"
	}
	return read
}

// TestSelectSetupSharedConstantReviewForms proves the forms the PR #1096 review found failing
// open now select the right setup file or return ALL (TSA-V0-006, TSA-V0-007): an
// `export * as` namespace re-export, an `export *` cycle resolved after a cached traversal, a
// chain of exactly 8 hops, reads inside the declaring module, `require("m").NAME`, literal
// require() and import() bindings, templates nested past the bound, and shadowing parameters.
func TestSelectSetupSharedConstantReviewForms(t *testing.T) {
	t.Parallel()
	setup := func(from, read string) string {
		return "import { SHARED } from '" + from + "';\nexport default async () => db.insert(" + read + ");\n"
	}
	files := map[string]string{
		"lib/shared.ts":        "export const SHARED = { scenario_a: 1, scenario_b: 2, scenario_c: 3, scenario_d: 4 };\n",
		"lib/ns_barrel.ts":     "export * as data from './shared';\n",
		"lib/cyc_a.ts":         "export * from './cyc_b';\nexport * from './shared';\n",
		"lib/cyc_b.ts":         "export * from './cyc_a';\n",
		"lib/self.ts":          "export const SELF = { one: 1, two: 2 };\nexport function context() { return SELF.one; }\n",
		"setup/a.setup.ts":     setup("../lib/shared", "SHARED.scenario_a"),
		"setup/b.setup.ts":     setup("../lib/shared", "SHARED['scenario_b']"),
		"setup/c.setup.ts":     setup("../lib/cyc_a", "SHARED.scenario_c"),
		"setup/h8.setup.ts":    setup("../lib/hop1", "SHARED.scenario_d"),
		"setup/one.setup.ts":   "import { SELF } from '../lib/self';\nexport default () => SELF.one;\n",
		"setup/two.setup.ts":   "import { SELF } from '../lib/self';\nexport default () => SELF.two;\n",
		"tests/ns.spec.ts":     "import { data } from '../lib/ns_barrel';\ntest('n', () => data.SHARED.scenario_a);\n",
		"tests/nested.spec.ts": "import * as all from '../lib/ns_barrel';\ntest('n', () => all.data.SHARED.scenario_b);\n",
		"tests/cycle.spec.ts":  "import { SHARED } from '../lib/cyc_b';\ntest('c', () => SHARED.scenario_c);\n",
		"tests/hops.spec.ts":   "import { SHARED } from '../lib/shared';\ntest('h', () => SHARED.scenario_d);\n",
		"tests/self.spec.ts":   "import { context } from '../lib/self';\ntest('s', () => context());\n",
		"tests/member.spec.ts": "test('m', () => require('../lib/shared').SHARED.scenario_a);\n",
		"tests/require.spec.ts": "const { SHARED } = require('../lib/shared');\n" +
			"test('r', () => SHARED.scenario_a);\n",
		"tests/dynamic.spec.ts":   "test('d', async () => {\n  const { SHARED } = await import('../lib/shared');\n  return SHARED.scenario_a;\n});\n",
		"tests/template8.spec.ts": "import { SHARED } from '../lib/shared';\ntest('t', () => " + sharedTemplate("SHARED.scenario_a", 8) + ");\n",
		"tests/template9.spec.ts": "import { SHARED } from '../lib/shared';\ntest('t', () => " + sharedTemplate("SHARED.scenario_a", 9) + ");\n",
		"tests/shadow.spec.ts": "import { SHARED } from '../lib/shared';\nfunction value(SHARED) { return SHARED.scenario_a; }\n" +
			"const pick = (SHARED: Data) => SHARED.scenario_a;\nconst bare = SHARED => SHARED.scenario_a;\ntest('s', () => [value({}), pick({}), bare({})]);\n",
	}
	sharedHops(files, 1)
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	a := []string{"setup/a.setup.ts"}
	for spec, want := range map[string][]string{
		"tests/ns.spec.ts":        a,
		"tests/nested.spec.ts":    {"setup/b.setup.ts"},
		"tests/cycle.spec.ts":     {"setup/c.setup.ts"},
		"tests/hops.spec.ts":      {"setup/h8.setup.ts"},
		"tests/self.spec.ts":      {"setup/one.setup.ts"},
		"tests/member.spec.ts":    a,
		"tests/template8.spec.ts": a,
		"tests/shadow.spec.ts":    a,
	} {
		if selection := SelectSetup(index, []string{spec}); selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, want) || len(selection.Reasons) != 0 {
			t.Errorf("%s: mode=%s files=%v reasons=%+v, want %v", spec, selection.Mode, selection.SetupFiles, selection.Reasons, want)
		}
	}
	for spec, line := range map[string]int{"tests/require.spec.ts": 1, "tests/dynamic.spec.ts": 2, "tests/template9.spec.ts": 2} {
		selection := SelectSetup(index, []string{spec})
		if selection.Mode != "all" || len(selection.Reasons) != 1 || selection.Reasons[0].Code != "shared-constant-key-unresolved" ||
			selection.Reasons[0].Path != spec || selection.Reasons[0].Line != line {
			t.Errorf("%s: mode=%s reasons=%+v, want shared-constant-key-unresolved at line %d", spec, selection.Mode, selection.Reasons, line)
		}
	}
}

// TestSelectSetupSharedConstantOpenSetups proves a setup file whose shared-constant read the
// selector cannot attribute to one constant (a namespace used as a value, a chain past the 8-hop
// bound) returns ALL naming that setup whenever the closure reads a shared constant, and changes
// nothing for a spec that reads none (TSA-V0-007). No other setup resolves SHARED first, so the
// 9-hop chain meets the bound rather than a cached resolution.
func TestSelectSetupSharedConstantOpenSetups(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"lib/shared.ts":       "export const SHARED = { scenario_a: 1, scenario_b: 2 };\n",
		"setup/far.setup.ts":  "import { SHARED } from '../lib/hop0';\nexport default async () => db.insert(SHARED.scenario_a);\n",
		"setup/ns.setup.ts":   "import * as data from '../lib/shared';\nexport default async () => seedNamespace(data);\n",
		"tests/x.spec.ts":     "import { SHARED } from '../lib/shared';\ntest('x', () => SHARED.scenario_a);\n",
		"tests/plain.spec.ts": "test('plain', () => expect(1).toBe(1));\n",
	}
	sharedHops(files, 0)
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	selection := SelectSetup(index, []string{"tests/x.spec.ts"})
	paths := []string{}
	for _, reason := range selection.Reasons {
		if reason.Code != "shared-constant-key-unresolved" || reason.Line != 2 {
			t.Errorf("reason %+v", reason)
		}
		paths = append(paths, reason.Path)
	}
	if selection.Mode != "all" || !reflect.DeepEqual(paths, []string{"setup/far.setup.ts", "setup/ns.setup.ts"}) {
		t.Errorf("mode=%s reasons=%+v", selection.Mode, selection.Reasons)
	}
	if selection := SelectSetup(index, []string{"tests/plain.spec.ts"}); selection.Mode != "selected" || len(selection.SetupFiles) != 0 {
		t.Errorf("plain: mode=%s files=%v reasons=%+v", selection.Mode, selection.SetupFiles, selection.Reasons)
	}
}

// TestSelectSetupRegistryKeyInSubstitution is TSA-V0-646: a literal key passed to an accessor
// called inside a template substitution selects its setup file, while a registry written after
// its declaration, or an entry whose constructor arguments are open, returns ALL.
func TestSelectSetupRegistryKeyInSubstitution(t *testing.T) {
	t.Parallel()
	files := setupSelectFiles()
	files["tests/templated.spec.ts"] = "import { DataProvider } from './data/provider';\nconst dp = new DataProvider();\n" +
		"test('t', () => `seeded ${dp.createTestData('ctx:alpha')}`);\n"
	build := func() *Index {
		root := impactRepositoryWithFiles(t, files)
		index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
		if err != nil {
			t.Fatal(err)
		}
		return index
	}
	selection := SelectSetup(build(), []string{"tests/templated.spec.ts"})
	if selection.Mode != "selected" || !reflect.DeepEqual(selection.SetupFiles, []string{"setup/alpha.setup.ts"}) {
		t.Fatalf("mode=%s files=%v reasons=%+v", selection.Mode, selection.SetupFiles, selection.Reasons)
	}
	registry := files["tests/data/registry.ts"]
	for _, text := range []string{
		registry + "registry['ctx:alpha'] = 1;\n",
		strings.Replace(registry, "new AlphaContext()", "new AlphaContext(seed)", 1),
	} {
		files["tests/data/registry.ts"] = text
		selection := SelectSetup(build(), []string{"tests/templated.spec.ts"})
		if selection.Mode != "all" || !reflect.DeepEqual(reasonCodes(selection), []string{"registry-unresolved"}) {
			t.Errorf("mode=%s reasons=%+v", selection.Mode, selection.Reasons)
		}
	}
}
