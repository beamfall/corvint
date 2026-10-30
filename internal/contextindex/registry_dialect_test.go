package contextindex

import (
	"context"
	"strings"
	"testing"
)

// registryManifest declares registries the way an adopter's committed
// application-map-manifest/0 document does (NGI-V0-007).
func registryManifest(declarations ...string) string {
	return `{"schema": "application-map-manifest/0", "app": "shop", "registries": [` + strings.Join(declarations, ",") + `]}`
}

func registryDeclaration(path, export string, accessors ...string) string {
	return `{"path": "` + path + `", "export": "` + export + `", "dialect": "string-key-registry/0", "accessors": ["` +
		strings.Join(accessors, `", "`) + `"]}`
}

// registryIssueFiles is the fixture shaped like GitHub #871: a registry of
// several context classes reached by specs only through a literal key passed
// to the declared accessor.
func registryIssueFiles() map[string]string {
	return map[string]string{
		"corvint/appmap.json":   registryManifest(registryDeclaration("tests/data/registry.ts", "registry", "createTestData")),
		"src/contexts/alpha.ts": "export class AlphaContext {}\n",
		"src/contexts/beta.ts":  "export default class BetaContext {}\n",
		"src/contexts/gamma.ts": "export class GammaContext {}\n",
		"tests/data/registry.ts": "import { AlphaContext } from '../../src/contexts/alpha';\n" +
			"import Beta from '../../src/contexts/beta';\n" +
			"import * as ctx from '../../src/contexts/gamma';\n" +
			"\n" +
			"export const registry: Record<string, unknown> = {\n" +
			"  'ctx:alpha': new AlphaContext(),\n" +
			"  \"ctx:beta\": Beta,\n" +
			"  [`ctx:gamma`]: new ctx.GammaContext() as unknown,\n" +
			"};\n",
		"tests/data/provider.ts": "import { registry } from './registry';\n" +
			"export class DataProvider {\n" +
			"  createTestData(key: string): unknown {\n" +
			"    return registry[key];\n" +
			"  }\n" +
			"}\n",
		"tests/alpha.spec.ts": "import { DataProvider } from './data/provider';\n" +
			"const dp = new DataProvider();\n" +
			"\n" +
			"test('alpha', () => dp.createTestData('ctx:alpha'));\n",
		"tests/beta.spec.ts": "import { DataProvider } from './data/provider';\n" +
			"const dp = new DataProvider();\n" +
			"test('beta', () => dp.createTestData(`ctx:beta`));\n",
		"tests/gamma.spec.ts": "import { DataProvider } from './data/provider';\n" +
			"const dp = new DataProvider();\n" +
			"test('gamma', () => dp.createTestData(\"ctx:gamma\", { seed: 1 }));\n",
	}
}

func registryImpact(t *testing.T, files map[string]string, changed string) map[string]any {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ImpactSyntax(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

// registryRows maps each test or reference result id to its evidence.
func registryRows(receipt map[string]any) map[string][]map[string]any {
	rows := map[string][]map[string]any{}
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] == "test" || item["kind"] == "reference" {
			rows[stringValue(item["id"])] = mapsFromAny(item["evidence"])
		}
	}
	return rows
}

func registryUnknownRows(receipt map[string]any, code string) []map[string]any {
	found := []map[string]any{}
	for _, row := range mapsFromAny(receipt["unknowns"]) {
		if row["code"] == code {
			found = append(found, row)
		}
	}
	return found
}

func hasEvidence(rows []map[string]any, path string, line int, confidence string) bool {
	for _, row := range rows {
		if row["path"] == path && row["line"] == line && row["confidence"] == confidence && row["authority"] == "syntax" &&
			stringValue(row["blob_hash"]) != "" {
			return true
		}
	}
	return false
}

// TestRegistryDialectSelectsTestsPassingTheKey is NGI-V0-009 on the issue
// fixture: each registered module selects exactly the specs that pass its key,
// with the registry entry line and the call-site line as evidence, for a
// quoted, double-quoted, substitution-free template and single-literal
// computed key alike.
func TestRegistryDialectSelectsTestsPassingTheKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		changed, spec string
		entry, call   int
	}{
		{"src/contexts/alpha.ts", "tests/alpha.spec.ts", 6, 4},
		{"src/contexts/beta.ts", "tests/beta.spec.ts", 7, 3},
		{"src/contexts/gamma.ts", "tests/gamma.spec.ts", 8, 3},
	} {
		receipt := registryImpact(t, registryIssueFiles(), tc.changed)
		rows := registryRows(receipt)
		if len(rows) != 1 || rows[tc.spec] == nil {
			t.Fatalf("%s: rows = %v, want only %s", tc.changed, rows, tc.spec)
		}
		evidence := rows[tc.spec]
		if !hasEvidence(evidence, "tests/data/registry.ts", tc.entry, "high") || !hasEvidence(evidence, tc.spec, tc.call, "high") {
			t.Errorf("%s: evidence = %v, want registry line %d and call line %d", tc.changed, evidence, tc.entry, tc.call)
		}
		for _, code := range []string{"accessor-key-nonliteral", "registry-key-unresolved", "registry-export-unresolved"} {
			if found := registryUnknownRows(receipt, code); len(found) != 0 {
				t.Errorf("%s: unexpected %s %v", tc.changed, code, found)
			}
		}
		if len(registryUnknownRows(receipt, "dynamic-dispatch-unresolved")) != 1 {
			t.Errorf("%s: the baseline frontier must remain", tc.changed)
		}
	}
}

// TestRegistryDialectNonLiteralKeyWidens is NGI-V0-010: one spec passing a
// template with a substitution makes every caller of that accessor selected
// and names the accessor in an unknown with the count of such calls.
func TestRegistryDialectNonLiteralKeyWidens(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["tests/dynamic.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const name = 'alpha';\n" +
		"test('dynamic', () => new DataProvider().createTestData(`ctx:${name}`));\n"
	files["tests/indirect.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"let key = 'ctx:alpha';\n" +
		"test('indirect', () => new DataProvider().createTestData(key));\n"
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	rows := registryRows(receipt)
	for _, spec := range []string{"tests/alpha.spec.ts", "tests/beta.spec.ts", "tests/gamma.spec.ts", "tests/dynamic.spec.ts", "tests/indirect.spec.ts"} {
		if rows[spec] == nil {
			t.Errorf("rows = %v, want widened caller %s", rows, spec)
		}
	}
	if !hasEvidence(rows["tests/alpha.spec.ts"], "tests/data/registry.ts", 6, "high") {
		t.Errorf("the precise caller keeps its registry evidence: %v", rows["tests/alpha.spec.ts"])
	}
	if !hasEvidence(rows["tests/dynamic.spec.ts"], "tests/dynamic.spec.ts", 3, "medium") {
		t.Errorf("a widened caller carries its own call line at medium confidence: %v", rows["tests/dynamic.spec.ts"])
	}
	found := registryUnknownRows(receipt, "accessor-key-nonliteral")
	if len(found) != 1 || found[0]["count"] != 2 || found[0]["accessor"] != "createTestData" ||
		found[0]["registry"] != "tests/data/registry.ts" || found[0]["path"] != "src/contexts/alpha.ts" {
		t.Errorf("unknowns = %v, want accessor-key-nonliteral count 2 for createTestData", found)
	}
}

// TestRegistryDialectChangedRegistrySelectsEveryCaller is NGI-V0-010's
// registry-file arm: a change to the registry itself selects every caller.
func TestRegistryDialectChangedRegistrySelectsEveryCaller(t *testing.T) {
	t.Parallel()
	rows := registryRows(registryImpact(t, registryIssueFiles(), "tests/data/registry.ts"))
	for _, spec := range []string{"tests/alpha.spec.ts", "tests/beta.spec.ts", "tests/gamma.spec.ts"} {
		if rows[spec] == nil {
			t.Errorf("rows = %v, want %s", rows, spec)
		}
	}
	if rows["tests/data/provider.ts"] != nil {
		t.Errorf("the accessor's own definition is not a call site: %v", rows["tests/data/provider.ts"])
	}
	// A registered module under the test tree is still resolved precisely.
	files := registryIssueFiles()
	files["tests/contexts/delta.ts"] = "export class DeltaContext {}\n"
	files["tests/data/registry.ts"] = "import { DeltaContext } from '../contexts/delta';\n" + strings.Replace(files["tests/data/registry.ts"], "};\n", "  'ctx:delta': new DeltaContext(),\n};\n", 1)
	files["tests/delta.spec.ts"] = "test('delta', () => dp.createTestData('ctx:delta'));\n"
	rows = registryRows(registryImpact(t, files, "tests/contexts/delta.ts"))
	if len(rows) != 1 || !hasEvidence(rows["tests/delta.spec.ts"], "tests/data/registry.ts", 10, "high") {
		t.Errorf("rows = %v, want only the delta spec", rows)
	}
}

// TestRegistryDialectUnresolvedRegistryWidens covers a spread entry, a
// computed non-literal key, a method entry and a value that is not an
// imported binding: each is counted, never dropped, and widens selection.
func TestRegistryDialectUnresolvedRegistryWidens(t *testing.T) {
	t.Parallel()
	for name, entry := range map[string]string{
		"spread":       "  ...other,\n",
		"computed":     "  [prefix + 'x']: Beta,\n",
		"substituted":  "  [`ctx:${prefix}`]: Beta,\n",
		"method":       "  make() { return 1; },\n",
		"local-value":  "  'ctx:local': new LocalContext(),\n",
		"called-value": "  'ctx:call': makeBeta(),\n",
		"long-key":     "  " + strings.Repeat("k", registryKeyBound.limit+1) + ": Beta,\n",
	} {
		files := registryIssueFiles()
		files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "};\n", entry+"};\n", 1)
		receipt := registryImpact(t, files, "src/contexts/alpha.ts")
		found := registryUnknownRows(receipt, "registry-key-unresolved")
		if len(found) != 1 || found[0]["count"] != 1 || found[0]["registry"] != "tests/data/registry.ts" {
			t.Errorf("%s: unknowns = %v, want registry-key-unresolved count 1", name, receipt["unknowns"])
		}
		rows := registryRows(receipt)
		if rows["tests/beta.spec.ts"] == nil || rows["tests/gamma.spec.ts"] == nil || !hasEvidence(rows["tests/alpha.spec.ts"], "tests/data/registry.ts", 6, "high") {
			t.Errorf("%s: rows = %v, want the precise caller plus every widened caller", name, rows)
		}
	}
}

// TestRegistryDialectReexportedRegistryIsUnresolved: a declaration naming a
// file that only re-exports the registry has no readable entries, so every
// changed path it could reach is widened and carries the unknown.
func TestRegistryDialectReexportedRegistryIsUnresolved(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["tests/data/index.ts"] = "export { registry } from './registry';\n"
	files["corvint/appmap.json"] = registryManifest(registryDeclaration("tests/data/index.ts", "registry", "createTestData"))
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	found := registryUnknownRows(receipt, "registry-export-unresolved")
	if len(found) != 1 || found[0]["registry"] != "tests/data/index.ts" {
		t.Errorf("unknowns = %v, want registry-export-unresolved", receipt["unknowns"])
	}
	rows := registryRows(receipt)
	for _, spec := range []string{"tests/alpha.spec.ts", "tests/beta.spec.ts", "tests/gamma.spec.ts"} {
		if rows[spec] == nil {
			t.Errorf("rows = %v, want widened %s", rows, spec)
		}
	}
}

// TestRegistryDialectAliasedAccessorImport: an accessor imported under a local
// alias is still its declared accessor; a local function of the same alias
// name in an unrelated file is not.
func TestRegistryDialectAliasedAccessorImport(t *testing.T) {
	files := registryIssueFiles()
	files["tests/data/helpers.ts"] = "import { registry } from './registry';\n" +
		"export function createTestData(key: string) { return registry[key]; }\n"
	files["tests/aliased.spec.ts"] = "import { createTestData as make } from './data/helpers';\n" +
		"test('aliased', () => make('ctx:alpha'));\n"
	files["tests/unrelated.spec.ts"] = "const make = (key: string) => key;\n" +
		"test('unrelated', () => make('ctx:alpha'));\n"
	rows := registryRows(registryImpact(t, files, "src/contexts/alpha.ts"))
	if !hasEvidence(rows["tests/aliased.spec.ts"], "tests/aliased.spec.ts", 2, "high") {
		t.Errorf("rows = %v, want the aliased caller with its call line", rows)
	}
	if rows["tests/unrelated.spec.ts"] != nil || rows["tests/data/helpers.ts"] != nil || rows["tests/beta.spec.ts"] != nil {
		t.Errorf("rows = %v, want neither the unrelated alias, the definition nor another key", rows)
	}
}

// TestRegistryDialectKeyCollisionAcrossRegistries: the same key in two
// registries binds each accessor to its own registry's module only.
func TestRegistryDialectKeyCollisionAcrossRegistries(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["corvint/appmap.json"] = registryManifest(
		registryDeclaration("tests/data/registry.ts", "registry", "createTestData"),
		registryDeclaration("tests/data/admin.ts", "adminRegistry", "createAdminData"))
	files["src/contexts/admin-alpha.ts"] = "export class AdminAlpha {}\n"
	files["tests/data/admin.ts"] = "import { AdminAlpha } from '../../src/contexts/admin-alpha';\n" +
		"export const adminRegistry = { 'ctx:alpha': new AdminAlpha() };\n"
	files["tests/admin.spec.ts"] = "import { admin } from './data/admin-provider';\n" +
		"test('admin', () => admin.createAdminData('ctx:alpha'));\n"
	rows := registryRows(registryImpact(t, files, "src/contexts/admin-alpha.ts"))
	if len(rows) != 1 || !hasEvidence(rows["tests/admin.spec.ts"], "tests/data/admin.ts", 2, "high") {
		t.Errorf("rows = %v, want only the admin spec", rows)
	}
	rows = registryRows(registryImpact(t, files, "src/contexts/alpha.ts"))
	if len(rows) != 1 || rows["tests/alpha.spec.ts"] == nil {
		t.Errorf("rows = %v, want only the alpha spec", rows)
	}
}

// TestRegistryDialectDeclarationFailures: an invalid declaration is a closed
// unknown on every JS/TS changed path, and the default profile reads nothing.
func TestRegistryDialectDeclarationFailures(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["corvint/appmap.json"] = registryManifest(`{"path": "tests/data/registry.ts", "export": "registry", "dialect": "other/0", "accessors": ["createTestData"]}`)
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	if found := registryUnknownRows(receipt, "registry-declaration-invalid"); len(found) != 1 || found[0]["count"] != 1 {
		t.Errorf("unknowns = %v, want registry-declaration-invalid", receipt["unknowns"])
	}
	if rows := registryRows(receipt); len(rows) != 0 {
		t.Errorf("rows = %v, want none from an invalid declaration", rows)
	}

	index, err := Build(context.Background(), impactRepositoryWithFiles(t, registryIssueFiles()))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Impact(index, []string{"src/contexts/alpha.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if rows := registryRows(plain); len(rows) != 0 || plain["unknowns"] != nil {
		t.Errorf("default profile rows = %v, want the historical receipt without dialect rows", rows)
	}
}

func TestValidateRegistryDeclarations(t *testing.T) {
	valid := RegistryDeclaration{Path: "tests/data/registry.ts", Export: "registry", Dialect: RegistryDialect, Accessors: []string{"createTestData"}}
	if err := ValidateRegistryDeclarations([]RegistryDeclaration{valid}); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RegistryDeclaration){
		"dialect":       func(d *RegistryDeclaration) { d.Dialect = "string-key-registry/1" },
		"escape":        func(d *RegistryDeclaration) { d.Path = "../registry.ts" },
		"absolute":      func(d *RegistryDeclaration) { d.Path = "/registry.ts" },
		"unclean":       func(d *RegistryDeclaration) { d.Path = "tests/./registry.ts" },
		"language":      func(d *RegistryDeclaration) { d.Path = "tests/registry.rb" },
		"export":        func(d *RegistryDeclaration) { d.Export = "registry.x" },
		"no accessors":  func(d *RegistryDeclaration) { d.Accessors = nil },
		"bad accessor":  func(d *RegistryDeclaration) { d.Accessors = []string{"create-data"} },
		"same accessor": func(d *RegistryDeclaration) { d.Accessors = []string{"a", "a"} },
		"git dir":       func(d *RegistryDeclaration) { d.Path = ".GIT/registry.ts" },
		"backslash":     func(d *RegistryDeclaration) { d.Path = `tests\registry.ts` },
	} {
		declaration := valid
		mutate(&declaration)
		if ValidateRegistryDeclarations([]RegistryDeclaration{declaration}) == nil {
			t.Errorf("%s: admitted %+v", name, declaration)
		}
	}
	if ValidateRegistryDeclarations([]RegistryDeclaration{valid, valid}) == nil {
		t.Error("admitted a repeated registry")
	}
}

// TestRegistryDialectStrictManifest: a manifest the application map would
// refuse (a duplicate member) is counted, never read as its last member, and
// an escaped solidus in the schema name does not hide a declaration.
func TestRegistryDialectStrictManifest(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	declaration := registryDeclaration("tests/data/registry.ts", "registry", "createTestData")
	files["corvint/appmap.json"] = `{"schema": "application-map-manifest/0", "app": "shop", "registries": [], "registries": [` + declaration + `]}`
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	if found := registryUnknownRows(receipt, "registry-declaration-invalid"); len(found) != 1 || found[0]["count"] != 1 {
		t.Errorf("duplicate member: unknowns = %v, want registry-declaration-invalid", receipt["unknowns"])
	}
	if rows := registryRows(receipt); len(rows) != 0 {
		t.Errorf("duplicate member: rows = %v, want none", rows)
	}

	files["corvint/appmap.json"] = `{"schema": "application-map-manifest\/0", "app": "shop", "registries": [` + declaration + `]}`
	receipt = registryImpact(t, files, "src/contexts/alpha.ts")
	if !hasEvidence(registryRows(receipt)["tests/alpha.spec.ts"], "tests/alpha.spec.ts", 4, "high") {
		t.Errorf("escaped schema: rows = %v, want the declared registry read", registryRows(receipt))
	}
}

// TestRegistryDialectDefaultReceiptUnchanged: the default profile renders the
// same bytes whether or not the committed manifest declares registries.
func TestRegistryDialectDefaultReceiptUnchanged(t *testing.T) {
	t.Parallel()
	render := func(files map[string]string) []byte {
		t.Helper()
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := Impact(index, []string{"src/contexts/alpha.ts", "tests/data/registry.ts"}, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := CanonicalJSON(receipt)
		if err != nil {
			t.Fatal(err)
		}
		// The fixtures are different commits; everything else must match.
		return []byte(strings.ReplaceAll(string(encoded), index.Revision, "REVISION"))
	}
	declared := registryIssueFiles()
	plain := registryIssueFiles()
	plain["corvint/appmap.json"] = `{"schema": "application-map-manifest/0", "app": "shop"}`
	if got, want := render(declared), render(plain); string(got) != string(want) {
		t.Errorf("default receipt changed with a registries declaration:\n got %s\nwant %s", got, want)
	}
}

// registryLazyFiles is the fixture shaped like GitHub #945 and #946: one
// eager entry, and lazy entries that reach their module only through a
// require() or import() inside an arrow function.
func registryLazyFiles() map[string]string {
	return map[string]string{
		"corvint/appmap.json":   registryManifest(registryDeclaration("tests/data/registry.ts", "registry", "createTestData")),
		"src/contexts/alpha.ts": "export class Calpha {}\n",
		"src/contexts/zeta.ts":  "export class Czeta {}\n",
		"src/contexts/eta.ts":   "export default class Ceta {}\n",
		"tests/data/registry.ts": "import { Calpha } from '../../src/contexts/alpha';\n" +
			"export const registry = {\n" +
			"  'ctx:alpha': new Calpha(),\n" +
			"  'ctx:zeta': () => new (require('../../src/contexts/zeta').Czeta)(),\n" +
			"  'ctx:eta': () => import('../../src/contexts/eta').then(m => new m.default()),\n" +
			"};\n" +
			"export function createTestData<T>(key: string): T { return (registry as any)[key]; }\n",
		"tests/alpha.spec.ts": "import { createTestData } from './data/registry';\n" +
			"test('alpha', async () => { await createTestData('ctx:alpha'); });\n",
		"tests/zeta.spec.ts": "import { createTestData } from './data/registry';\n" +
			"test('zeta', async () => { await createTestData('ctx:zeta'); });\n",
		"tests/eta.spec.ts": "import { createTestData } from './data/registry';\n" +
			"test('eta', async () => { await createTestData('ctx:eta'); });\n",
	}
}

// TestRegistryDialectLazyRequireNeverFailsOpen is GitHub #945 (V1-1455): a
// module the registry reaches only through a require() inside an entry must
// select the spec passing its key, or widen every caller; it never returns
// no test rows.
func TestRegistryDialectLazyRequireNeverFailsOpen(t *testing.T) {
	receipt := registryImpact(t, registryLazyFiles(), "src/contexts/zeta.ts")
	if rows := registryRows(receipt); rows["tests/zeta.spec.ts"] == nil {
		t.Fatalf("rows = %v, unknowns = %v: the spec passing ctx:zeta is not selected", rows, receipt["unknowns"])
	}
}

// TestRegistryDialectUnresolvedEntryAppliesToEveryPath is NGI-V0-012: an
// entry the dialect cannot read may reach any module, so the registry's
// registry-key-unresolved row and its widening apply to every JS/TS changed
// path, not only to the paths the registry imports.
func TestRegistryDialectUnresolvedEntryAppliesToEveryPath(t *testing.T) {
	t.Parallel()
	files := registryLazyFiles()
	files["tests/data/load.ts"] = "export const load = (name: string) => require('../../src/contexts/' + name);\n"
	files["tests/data/registry.ts"] = "import { Calpha } from '../../src/contexts/alpha';\n" +
		"import { load } from './load';\n" +
		"export const registry = {\n" +
		"  'ctx:alpha': new Calpha(),\n" +
		"  'ctx:zeta': () => load('zeta'),\n" +
		"};\n" +
		"export function createTestData<T>(key: string): T { return (registry as any)[key]; }\n"
	receipt := registryImpact(t, files, "src/contexts/zeta.ts")
	found := registryUnknownRows(receipt, "registry-key-unresolved")
	if len(found) != 1 || found[0]["count"] != 1 || found[0]["registry"] != "tests/data/registry.ts" || found[0]["path"] != "src/contexts/zeta.ts" {
		t.Errorf("unknowns = %v, want registry-key-unresolved count 1 on the changed path", receipt["unknowns"])
	}
	rows := registryRows(receipt)
	for _, spec := range []string{"tests/alpha.spec.ts", "tests/zeta.spec.ts", "tests/eta.spec.ts"} {
		if !hasEvidence(rows[spec], spec, 2, "medium") {
			t.Errorf("rows = %v, want widened caller %s with its call line", rows, spec)
		}
	}
	// A Ruby path cannot be reached by a JavaScript registry entry.
	files["lib/other.rb"] = "class Other; end\n"
	if found := registryUnknownRows(registryImpact(t, files, "lib/other.rb"), "registry-key-unresolved"); len(found) != 0 {
		t.Errorf("a Ruby changed path carries %v", found)
	}
}

// registryLazyFormFiles registers one module per lazy loader form of GitHub
// #946, each reached by one spec through its literal key.
func registryLazyFormFiles() map[string]string {
	files := registryLazyFiles()
	files["src/contexts/theta.ts"] = "export class Ctheta {}\n"
	files["src/contexts/iota/index.ts"] = "export class Ciota {}\n"
	files["tests/data/registry.ts"] = "import { Calpha } from '../../src/contexts/alpha';\n" +
		"export const registry: Record<string, () => unknown> = {\n" +
		"  'ctx:alpha': Calpha,\n" +
		"  'ctx:zeta': () => new (require('../../src/contexts/zeta').Czeta)(),\n" +
		"  'ctx:eta': () => new (require('../../src/contexts/eta.js').default)(),\n" +
		"  'ctx:theta': () => require(`../../src/contexts/theta`).Ctheta as unknown,\n" +
		"  'ctx:iota': () => import('../../src/contexts/iota').then(m => new m.Ciota()),\n" +
		"};\n" +
		"export function createTestData<T>(key: string): T { return (registry as any)[key](); }\n"
	files["tests/theta.spec.ts"] = "import { createTestData } from './data/registry';\n" +
		"test('theta', async () => { await createTestData('ctx:theta'); });\n"
	files["tests/iota.spec.ts"] = "import { createTestData } from './data/registry';\n" +
		"test('iota', async () => { await createTestData('ctx:iota'); });\n"
	return files
}

// TestRegistryDialectLazyEntriesResolve is NGI-V0-013 (GitHub #946): each
// lazy loader form binds its key to the module its closed specifier names,
// so a change selects only the spec passing that key.
func TestRegistryDialectLazyEntriesResolve(t *testing.T) {
	t.Parallel()
	files := registryLazyFormFiles()
	for _, tc := range []struct {
		form, changed, spec string
		entry               int
	}{
		{"new (require(S).X)()", "src/contexts/zeta.ts", "tests/zeta.spec.ts", 4},
		{"new (require(S).default)()", "src/contexts/eta.ts", "tests/eta.spec.ts", 5},
		{"require(S).X", "src/contexts/theta.ts", "tests/theta.spec.ts", 6},
		{"import(S).then(...)", "src/contexts/iota/index.ts", "tests/iota.spec.ts", 7},
	} {
		receipt := registryImpact(t, files, tc.changed)
		rows := registryRows(receipt)
		if len(rows) != 1 || !hasEvidence(rows[tc.spec], "tests/data/registry.ts", tc.entry, "high") || !hasEvidence(rows[tc.spec], tc.spec, 2, "high") {
			t.Errorf("%s: rows = %v, want only %s with registry line %d", tc.form, rows, tc.spec, tc.entry)
		}
		for _, code := range []string{"registry-key-unresolved", "accessor-key-nonliteral", "registry-export-unresolved"} {
			if found := registryUnknownRows(receipt, code); len(found) != 0 {
				t.Errorf("%s: unexpected %v", tc.form, found)
			}
		}
	}
}

// TestRegistryDialectLazyEntriesStayClosed: an arrow the lazy rule does not
// read, or whose specifier names no indexed JS/TS source, is counted as
// registry-key-unresolved and widens every caller. So is an entry that may
// reach a second module (a constructor argument, a callback, an expression
// after an assertion) or a require the file may rebind.
func TestRegistryDialectLazyEntriesStayClosed(t *testing.T) {
	t.Parallel()
	rebound := map[string]string{
		"shadowed require": "const require = (name: string) => ({ Czeta: Calpha });\n",
		"declared require": "function require(name: string) { return { Czeta: Calpha }; }\n",
		"escaped require":  "const \\u0072equire = (name: string) => ({ Czeta: Calpha });\n",
	}
	for name, prelude := range rebound {
		files := registryLazyFormFiles()
		files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "};\n", "  'ctx:x': () => require('../../src/contexts/zeta').Czeta,\n};\n", 1) + prelude
		receipt := registryImpact(t, files, "src/contexts/alpha.ts")
		// The zeta, eta and theta require entries and the new one are open.
		if found := registryUnknownRows(receipt, "registry-key-unresolved"); len(found) != 1 || found[0]["count"] != 4 {
			t.Errorf("%s: unknowns = %v, want registry-key-unresolved count 4", name, receipt["unknowns"])
		}
	}
	for name, entry := range map[string]string{
		"callback reads another binding":   "  'ctx:x': () => import('../../src/contexts/zeta').then(() => Calpha),\n",
		"callback argument":                "  'ctx:x': () => import('../../src/contexts/zeta').then(m => new m.Czeta(Calpha)),\n",
		"constructor argument":             "  'ctx:x': () => new (require('../../src/contexts/zeta').Czeta)(new Calpha()),\n",
		"expression after assertion":       "  'ctx:x': () => require('../../src/contexts/zeta').Czeta as unknown && Calpha,\n",
		"eager expression after assertion": "  'ctx:x': Calpha as unknown && Other,\n",
		"computed specifier":               "  'ctx:x': () => require('../../src/contexts/' + name).X,\n",
		"substituted":                      "  'ctx:x': () => require(`../../src/contexts/${name}`).X,\n",
		"bare specifier":                   "  'ctx:x': () => require('contexts/zeta').X,\n",
		"missing module":                   "  'ctx:x': () => require('../../src/contexts/missing').X,\n",
		"parameter":                        "  'ctx:x': (seed) => new (require('../../src/contexts/zeta').Czeta)(seed),\n",
		"async arrow":                      "  'ctx:x': async () => require('../../src/contexts/zeta').Czeta,\n",
		"block body":                       "  'ctx:x': () => { return require('../../src/contexts/zeta').Czeta; },\n",
		"called module":                    "  'ctx:x': () => require('../../src/contexts/zeta')(),\n",
		"import then catch":                "  'ctx:x': () => import('../../src/contexts/zeta').catch(fail),\n",
		"trailing call":                    "  'ctx:x': () => require('../../src/contexts/zeta').Czeta.make(),\n",
	} {
		files := registryLazyFormFiles()
		files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "};\n", entry+"};\n", 1)
		receipt := registryImpact(t, files, "src/contexts/alpha.ts")
		found := registryUnknownRows(receipt, "registry-key-unresolved")
		if len(found) != 1 || found[0]["count"] != 1 {
			t.Errorf("%s: unknowns = %v, want registry-key-unresolved count 1", name, receipt["unknowns"])
		}
		if rows := registryRows(receipt); rows["tests/zeta.spec.ts"] == nil || rows["tests/iota.spec.ts"] == nil {
			t.Errorf("%s: rows = %v, want every caller widened", name, rows)
		}
	}
}

// TestRegistryDialectNonExportedRegistry is NGI-V0-014: a top-level const
// registry the file does not export, read through a declared accessor,
// resolves like an exported one; a nested const of that name does not.
func TestRegistryDialectNonExportedRegistry(t *testing.T) {
	t.Parallel()
	files := registryLazyFormFiles()
	files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "export const registry", "const registry", 1)
	receipt := registryImpact(t, files, "src/contexts/zeta.ts")
	if rows := registryRows(receipt); len(rows) != 1 || !hasEvidence(rows["tests/zeta.spec.ts"], "tests/data/registry.ts", 4, "high") {
		t.Errorf("rows = %v, unknowns = %v, want only the zeta spec", rows, receipt["unknowns"])
	}
	private := files["tests/data/registry.ts"]
	for name, text := range map[string]string{
		"nested": "export function createTestData<T>(key: string): T {\n" +
			"  const registry = { 'ctx:zeta': () => require('../../src/contexts/zeta').Czeta };\n" +
			"  return (registry as any)[key]();\n" +
			"}\n",
		// The module's export named registry is another binding.
		"export clause": private + "const other = { 'ctx:zeta': Calpha };\nexport { other as registry };\n",
		"export star":   private + "export * from './more';\n",
		"commonjs":      private + "module.exports.registry = {};\n",
	} {
		files["tests/data/registry.ts"] = text
		receipt = registryImpact(t, files, "src/contexts/zeta.ts")
		if found := registryUnknownRows(receipt, "registry-export-unresolved"); len(found) != 1 {
			t.Errorf("%s: unknowns = %v, want registry-export-unresolved", name, receipt["unknowns"])
		}
		if rows := registryRows(receipt); rows["tests/alpha.spec.ts"] == nil || rows["tests/zeta.spec.ts"] == nil {
			t.Errorf("%s: rows = %v, want every caller widened", name, rows)
		}
	}
}

// TestRegistryDialectConstAccessorKey is NGI-V0-015 (GitHub #947): a lone
// accessor argument naming a top-level const bound to a closed literal in the
// same file is that literal key.
func TestRegistryDialectConstAccessorKey(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["tests/delta.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const SCENARIO_CONTEXT = 'ctx:delta';\n" +
		"test('d', async () => { await new DataProvider().createTestData(SCENARIO_CONTEXT); });\n"
	files["tests/beta-const.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"export const KEY = `ctx:beta` as const\n" +
		"const dp = new DataProvider();\n" +
		"test('b1', () => dp.createTestData(KEY));\n" +
		"test('b2', () => dp.createTestData(KEY, { seed: 2 }));\n"
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	if rows := registryRows(receipt); len(rows) != 1 || rows["tests/alpha.spec.ts"] == nil {
		t.Errorf("rows = %v, want only tests/alpha.spec.ts", rows)
	}
	if found := registryUnknownRows(receipt, "accessor-key-nonliteral"); len(found) != 0 {
		t.Errorf("unknowns = %v, want accessor-key-nonliteral count 0", found)
	}
	rows := registryRows(registryImpact(t, files, "src/contexts/beta.ts"))
	if len(rows) != 2 || !hasEvidence(rows["tests/beta-const.spec.ts"], "tests/beta-const.spec.ts", 4, "high") ||
		!hasEvidence(rows["tests/beta-const.spec.ts"], "tests/beta-const.spec.ts", 5, "high") || rows["tests/beta.spec.ts"] == nil {
		t.Errorf("rows = %v, want the beta spec and both const-key calls", rows)
	}
}

// TestRegistryDialectConstAccessorKeyStaysNonLiteral: an identifier the rule
// cannot prove bound to one literal at the call stays accessor-key-nonliteral
// and widens every caller.
func TestRegistryDialectConstAccessorKeyStaysNonLiteral(t *testing.T) {
	t.Parallel()
	call := "test('k', () => new DataProvider().createTestData(KEY));\n"
	head := "import { DataProvider } from './data/provider';\n"
	for name, body := range map[string]string{
		"let":                 "let KEY = 'ctx:alpha';\n" + call,
		"var":                 "var KEY = 'ctx:alpha';\n" + call,
		"reassigned":          "let KEY = 'ctx:beta';\nKEY = 'ctx:alpha';\n" + call,
		"imported":            "import { KEY } from './keys';\n" + call,
		"substituted":         "const KEY = `ctx:${name}`;\n" + call,
		"concatenated":        "const KEY = 'ctx:' + name;\n" + call,
		"continued line":      "const KEY = 'ctx:alpha'\n  + suffix;\n" + call,
		"nested":              "function f() {\n  const KEY = 'ctx:alpha';\n  return new DataProvider().createTestData(KEY);\n}\n",
		"parameter shadow":    "const KEY = 'ctx:alpha';\nconst run = (KEY: string) => new DataProvider().createTestData(KEY);\n",
		"block shadow":        "const KEY = 'ctx:alpha';\ntest('k', () => { const KEY = 'ctx:beta'; return new DataProvider().createTestData(KEY); });\n",
		"catch shadow":        "const KEY = 'ctx:alpha';\ntry { x(); } catch (KEY) { new DataProvider().createTestData(KEY); }\n",
		"repeated":            "const KEY = 'ctx:alpha';\nconst KEY = 'ctx:beta';\n" + call,
		"other read":          "const KEY = 'ctx:alpha';\nlog(KEY);\n" + call,
		"with statement":      "const KEY = 'ctx:alpha';\nwith (scope) { new DataProvider().createTestData(KEY); }\n",
		"typed":               "const KEY: string = 'ctx:alpha';\n" + call,
		"accessor definition": "const KEY = 'ctx:alpha';\nclass P { createTestData(KEY) { return KEY; } }\n" + call,
		"rest parameter":      "const KEY = 'ctx:alpha';\nconst run = (...KEY) => new DataProvider().createTestData(KEY);\n",
		"escaped shadow":      "const KEY = 'ctx:alpha';\nconst run = (\\u004bEY) => new DataProvider().createTestData(KEY);\n",
	} {
		files := registryIssueFiles()
		files["tests/key.spec.ts"] = head + body
		receipt := registryImpact(t, files, "src/contexts/alpha.ts")
		found := registryUnknownRows(receipt, "accessor-key-nonliteral")
		if len(found) != 1 || found[0]["count"] != 1 {
			t.Errorf("%s: unknowns = %v, want accessor-key-nonliteral count 1", name, receipt["unknowns"])
		}
		if rows := registryRows(receipt); rows["tests/beta.spec.ts"] == nil || rows["tests/key.spec.ts"] == nil {
			t.Errorf("%s: rows = %v, want every caller widened", name, rows)
		}
	}
}

// TestRegistryDialectExportUnresolvedIsWebOnly is V1-1479 (NGI-V0-010): an
// unresolved registry export applies to every JS/TS changed path, and to no
// Ruby path, which no JS/TS registry can map a key to.
func TestRegistryDialectExportUnresolvedIsWebOnly(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["tests/data/index.ts"] = "export { registry } from './registry';\n"
	files["corvint/appmap.json"] = registryManifest(registryDeclaration("tests/data/index.ts", "registry", "createTestData"))
	files["app/models/widget.rb"] = "class Widget\nend\n"
	if found := registryUnknownRows(registryImpact(t, files, "src/contexts/alpha.ts"), "registry-export-unresolved"); len(found) != 1 {
		t.Errorf("typescript path: want one registry-export-unresolved, got %v", found)
	}
	receipt := registryImpact(t, files, "app/models/widget.rb")
	if found := registryUnknownRows(receipt, "registry-export-unresolved"); len(found) != 0 {
		t.Errorf("ruby path: unknowns = %v, want no registry-export-unresolved", receipt["unknowns"])
	}
	for spec := range registryRows(receipt) {
		if strings.HasPrefix(spec, "tests/") && strings.HasSuffix(spec, ".spec.ts") {
			t.Errorf("ruby path: registry widened to %s", spec)
		}
	}
}

// TestRegistryDialectAmbientRequireIsLoader is V1-1479: a TypeScript ambient
// `declare const require: ...` only types the loader and binds no value, so
// lazy require entries stay closed.
func TestRegistryDialectAmbientRequireIsLoader(t *testing.T) {
	t.Parallel()
	for name, prelude := range map[string]string{
		"declare const": "declare const require: (name: string) => any;\n",
		"declare var":   "declare var require: NodeRequire;\n",
	} {
		files := registryLazyFormFiles()
		files["tests/data/registry.ts"] = prelude + files["tests/data/registry.ts"]
		receipt := registryImpact(t, files, "src/contexts/zeta.ts")
		if found := registryUnknownRows(receipt, "registry-key-unresolved"); len(found) != 0 {
			t.Errorf("%s: unknowns = %v, want no registry-key-unresolved", name, receipt["unknowns"])
		}
		if rows := registryRows(receipt); len(rows) != 1 || rows["tests/zeta.spec.ts"] == nil {
			t.Errorf("%s: rows = %v, want only the zeta spec", name, rows)
		}
	}
}

// TestRegistryDialectAccessorInsideSubstitution is NGI-V0-065: an accessor
// called inside a template substitution is a call site like any other, a
// literal key there selects precisely, and a template argument whose own
// substitution supplies the key stays non-literal, nested or not.
func TestRegistryDialectAccessorInsideSubstitution(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["tests/report.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const dp = new DataProvider();\n" +
		"test('report', () => console.log(`made ${\n  dp.createTestData('ctx:beta')\n} ok`));\n"
	receipt := registryImpact(t, files, "src/contexts/beta.ts")
	rows := registryRows(receipt)
	if len(rows) != 2 || !hasEvidence(rows["tests/report.spec.ts"], "tests/report.spec.ts", 4, "high") ||
		!hasEvidence(rows["tests/report.spec.ts"], "tests/data/registry.ts", 7, "high") {
		t.Fatalf("rows = %v, want beta.spec and report.spec with the substitution's call line", rows)
	}
	if found := registryUnknownRows(receipt, "accessor-key-nonliteral"); len(found) != 0 {
		t.Errorf("a literal key inside a substitution is not open: %v", found)
	}
	if rows := registryRows(registryImpact(t, files, "src/contexts/alpha.ts")); rows["tests/report.spec.ts"] != nil {
		t.Errorf("rows = %v: report.spec passes ctx:beta only", rows)
	}
	files["tests/nested.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const dp = new DataProvider();\n" +
		"const n = 'alpha';\n" +
		"test('n', () => `${dp.createTestData(`ctx:${n}`)}`);\n" +
		"test('m', () => `${dp.createTestData(`${'ctx'}:alpha`)}`);\n" +
		"test('o', () => dp.createTestData(`${'ctx:alpha'}`));\n"
	receipt = registryImpact(t, files, "src/contexts/gamma.ts")
	if !hasEvidence(registryRows(receipt)["tests/nested.spec.ts"], "tests/nested.spec.ts", 4, "medium") {
		t.Errorf("rows = %v, want nested.spec widened", registryRows(receipt))
	}
	found := registryUnknownRows(receipt, "accessor-key-nonliteral")
	if len(found) != 1 || found[0]["count"] != 3 {
		t.Errorf("unknowns = %v, want accessor-key-nonliteral count 3", found)
	}
}

// TestRegistryDialectWrittenAfterDeclarationWidens is NGI-V0-066: an
// assignment, compound assignment, `delete`, `++`, or write call on the
// registry after its declaration, in the declaring file or through an
// import, widens every caller with registry-written-after-declaration; reads,
// comparisons and type queries do not.
func TestRegistryDialectWrittenAfterDeclarationWidens(t *testing.T) {
	t.Parallel()
	declared := registryIssueFiles()["tests/data/registry.ts"]
	reads := "import { registry } from './data/registry';\n" +
		"type K = keyof typeof registry;\n" +
		"const same = registry == null || registry['ctx:alpha'] === undefined || registry.x <= 1;\n" +
		"const f = (registry: unknown) => registry;\n" +
		"test('reads', () => ({ ...registry, registry }));\n"
	files := registryIssueFiles()
	files["tests/reads.spec.ts"] = reads
	receipt := registryImpact(t, files, "src/contexts/alpha.ts")
	if found := registryUnknownRows(receipt, "registry-written-after-declaration"); len(found) != 0 {
		t.Fatalf("reads count as writes: %v", found)
	}
	if rows := registryRows(receipt); len(rows) != 1 {
		t.Fatalf("rows = %v, want only alpha.spec", rows)
	}
	for _, tc := range []struct {
		name, path, text string
		count            int
	}{
		{"member assign", "tests/data/registry.ts", declared + "registry['ctx:alpha'] = new GammaContext();\n", 1},
		{"compound and delete", "tests/data/registry.ts", declared + "registry.extra ??= 1;\ndelete registry['ctx:beta'];\n", 2},
		{"object assign", "tests/write.spec.ts", "import { registry as r } from './data/registry';\nObject.assign(r, { 'ctx:alpha': 1 });\n", 1},
		{"namespace and increment", "tests/write.spec.ts", "import * as data from './data/registry';\ndata.registry.count++;\nReflect.set(data.registry, 'k', 1);\n", 2},
		// A default import spells the module, not the export (Codex review).
		{"default import of a differently named export", "tests/write.spec.ts", "import r from './data/registry';\nr.extra = 1;\n", 1},
	} {
		files := registryIssueFiles()
		if strings.HasPrefix(tc.name, "default import") {
			renameRegistryExport(files, "table")
		}
		files[tc.path] = tc.text
		files["tests/reads.spec.ts"] = reads
		receipt := registryImpact(t, files, "src/contexts/alpha.ts")
		found := registryUnknownRows(receipt, "registry-written-after-declaration")
		if len(found) != 1 || found[0]["count"] != tc.count || found[0]["registry"] != "tests/data/registry.ts" {
			t.Errorf("%s: unknowns = %v, want registry-written-after-declaration count %d", tc.name, found, tc.count)
		}
		rows := registryRows(receipt)
		for _, spec := range []string{"tests/alpha.spec.ts", "tests/beta.spec.ts", "tests/gamma.spec.ts"} {
			if rows[spec] == nil {
				t.Errorf("%s: rows = %v, want widened caller %s", tc.name, rows, spec)
			}
		}
		if !hasEvidence(rows["tests/beta.spec.ts"], "tests/beta.spec.ts", 3, "medium") {
			t.Errorf("%s: beta.spec evidence = %v", tc.name, rows["tests/beta.spec.ts"])
		}
		// The written registry may reach any JS/TS path, and no Ruby path.
		files["lib/other.rb"] = "class Other; end\n"
		if found := registryUnknownRows(registryImpact(t, files, "lib/other.rb"), "registry-written-after-declaration"); len(found) != 0 {
			t.Errorf("%s: a Ruby changed path carries %v", tc.name, found)
		}
	}
}

// TestRegistryDialectConstructorArgumentsOpen is NGI-V0-067: an eager
// `new X(args)` entry whose arguments are identifiers may hand another module
// to the instance, so a change to any JS/TS path selects the callers passing
// that key and names the registry; literal arguments stay closed and the
// precise X edge stays.
func TestRegistryDialectConstructorArgumentsOpen(t *testing.T) {
	t.Parallel()
	files := registryIssueFiles()
	files["src/config.ts"] = "export const config = { url: 'x' };\n"
	files["src/other.ts"] = "export const other = 1;\n"
	files["tests/data/registry.ts"] = "import { AlphaContext } from '../../src/contexts/alpha';\n" +
		"import Beta from '../../src/contexts/beta';\n" +
		"import * as ctx from '../../src/contexts/gamma';\n" +
		"import { config } from '../../src/config';\n" +
		"export const registry: Record<string, unknown> = {\n" +
		"  'ctx:alpha': new AlphaContext(config),\n" +
		"  \"ctx:beta\": Beta,\n" +
		"  [`ctx:gamma`]: new ctx.GammaContext('seed', 2, true, null) as unknown,\n" +
		"};\n"
	receipt := registryImpact(t, files, "src/other.ts")
	found := registryUnknownRows(receipt, "registry-constructor-arguments-open")
	if len(found) != 1 || found[0]["count"] != 1 || found[0]["registry"] != "tests/data/registry.ts" {
		t.Fatalf("unknowns = %v, want registry-constructor-arguments-open count 1", receipt["unknowns"])
	}
	rows := registryRows(receipt)
	if len(rows) != 1 || !hasEvidence(rows["tests/alpha.spec.ts"], "tests/alpha.spec.ts", 4, "medium") {
		t.Errorf("rows = %v, want only alpha.spec widened through its key", rows)
	}
	rows = registryRows(registryImpact(t, files, "src/contexts/alpha.ts"))
	if !hasEvidence(rows["tests/alpha.spec.ts"], "tests/data/registry.ts", 6, "high") {
		t.Errorf("the precise X edge stays: %v", rows["tests/alpha.spec.ts"])
	}
	files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "AlphaContext(config)", "AlphaContext('x', 1)", 1)
	if found := registryUnknownRows(registryImpact(t, files, "src/other.ts"), "registry-constructor-arguments-open"); len(found) != 0 {
		t.Errorf("literal arguments are closed: %v", found)
	}
}

// renameRegistryExport renames the fixture registry's export so no importer
// spells it through the module path.
func renameRegistryExport(files map[string]string, export string) {
	files["corvint/appmap.json"] = registryManifest(registryDeclaration("tests/data/registry.ts", export, "createTestData"))
	files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "export const registry:", "export const "+export+":", 1) +
		"export default " + export + ";\n"
	files["tests/data/provider.ts"] = strings.ReplaceAll(strings.Replace(files["tests/data/provider.ts"], "{ registry }", "{ "+export+" }", 1), "return registry[", "return "+export+"[")
}
