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
	files := registryIssueFiles()
	files["tests/dynamic.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const name = 'alpha';\n" +
		"test('dynamic', () => new DataProvider().createTestData(`ctx:${name}`));\n"
	files["tests/indirect.spec.ts"] = "import { DataProvider } from './data/provider';\n" +
		"const key = 'ctx:alpha';\n" +
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
	for name, entry := range map[string]string{
		"spread":       "  ...other,\n",
		"computed":     "  [prefix + 'x']: Beta,\n",
		"substituted":  "  [`ctx:${prefix}`]: Beta,\n",
		"method":       "  make() { return 1; },\n",
		"local-value":  "  'ctx:local': new LocalContext(),\n",
		"called-value": "  'ctx:call': makeBeta(),\n",
		"long-key":     "  " + strings.Repeat("k", maxRegistryKeyBytes+1) + ": Beta,\n",
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
