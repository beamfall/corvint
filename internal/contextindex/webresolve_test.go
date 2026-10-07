package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// tsconfigPathsFixture loads the committed fixture for GitHub issue 659: a
// suite whose specs reach a page object only through tsconfig `baseUrl`,
// wildcard `paths` and a two-link `extends` chain. The files live in one JSON
// document so this repository's own index does not read the fixture's
// deliberately unresolved aliases as its own.
func tsconfigPathsFixture(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/tsconfig-paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Files
}

func webResolverFor(t *testing.T, files map[string]string) (*Index, *WebImportResolver) {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return index, NewWebImportResolver(index)
}

func impactRow(receipt map[string]any, kind, id string) map[string]any {
	for _, item := range mapsFromAny(receipt["results"]) {
		if item["kind"] == kind && item["id"] == id {
			return item
		}
	}
	return nil
}

// TestImpactResolvesTSConfigPathAliasImporters is GPK-V0-077 and GPK-V0-079
// (proposed): before the alias arm, rule (c) admitted only relative and
// profile-alias specifiers, so impact on the page object listed no spec at
// all and the selection built on it came back empty.
func TestImpactResolvesTSConfigPathAliasImporters(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, tsconfigPathsFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"src/pages/LoginPage.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	found := reverseImportIDs(receipt)
	for id, reason := range map[string]string{
		"tests/login.spec.ts":         "imports @pages/LoginPage",
		"tests/checkout.spec.ts":      "imports @pages/LoginPage.js",
		"tests/types.spec.ts":         "imports @pages/LoginPage",
		"tests/base-url.spec.ts":      "imports src/pages/LoginPage",
		"nested/tests/nested.spec.ts": "imports @pages/LoginPage",
	} {
		if !found[id][reason] {
			t.Errorf("reverse importers = %v, want %s with reason %q", found, id, reason)
		}
	}
	for _, id := range []string{"tests/unresolved.spec.ts", "tests/packages.spec.ts", "tests/shared.spec.ts"} {
		if found[id] != nil {
			t.Errorf("%s does not import the page object, yet impact lists it: %v", id, found[id])
		}
	}
	runtime, typeOnly := impactRow(receipt, "reverse-import", "tests/login.spec.ts"), impactRow(receipt, "reverse-import", "tests/types.spec.ts")
	if runtime == nil || typeOnly == nil {
		t.Fatal("fixture rows missing")
	}
	if runtime["score"] != 650 || typeOnly["score"] != 550 {
		t.Errorf("scores runtime=%v type-only=%v, want 650 and 550: a type-only edge is weaker than every runtime edge", runtime["score"], typeOnly["score"])
	}
	if !strings.HasPrefix(stringValue(typeOnly["summary"]), "type-only import of module containing src/pages/LoginPage.ts") {
		t.Errorf("type-only summary = %q", typeOnly["summary"])
	}
	evidence := mapsFromAny(typeOnly["evidence"])
	if len(evidence) != 1 || evidence[0]["confidence"] != "medium" || evidence[0]["line"] != 1 || evidence[0]["authority"] != "syntax" {
		t.Errorf("type-only evidence = %v, want one medium-confidence syntax row on line 1", evidence)
	}
	uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
	want := "reverse-import results for 1 changed paths are incomplete: 2 bare import specifiers in 1 sources resolve to no repository file or declared package"
	if !anyContains(uncertainty, want) {
		t.Errorf("uncertainty = %v, want %q", uncertainty, want)
	}

	// The directory-index, exact-key and second-substitution targets.
	for changed, importer := range map[string]string{
		"src/pages/cart/index.ts": "tests/checkout.spec.ts",
		"src/support/index.ts":    "tests/base-url.spec.ts",
		"src/shared/account.ts":   "tests/shared.spec.ts",
	} {
		other, err := Impact(index, []string{changed}, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		if reverseImportIDs(other)[importer] == nil {
			t.Errorf("impact on %s = %v, want %s", changed, reverseImportIDs(other), importer)
		}
	}
	support, _ := Impact(index, []string{"src/support/index.ts"}, maxLimit)
	if reverseImportIDs(support)["tests/shared.spec.ts"] == nil {
		t.Errorf("exact `@support` key did not resolve: %v", reverseImportIDs(support))
	}
}

// TestImpactSyntaxReportsBareImportUnresolved is GPK-V0-080 on the
// `non-go-syntax-v0` profile: the unresolved count is a structured unknown row.
func TestImpactSyntaxReportsBareImportUnresolved(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, tsconfigPathsFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ImpactSyntax(index, []string{"src/pages/LoginPage.ts", "tests/login.spec.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	for _, row := range mapsFromAny(receipt["unknowns"]) {
		if row["code"] == "bare-import-unresolved" {
			rows[stringValue(row["path"])] = row
		}
	}
	if row := rows["src/pages/LoginPage.ts"]; row == nil || row["count"] != 2 || row["language"] != "typescript" {
		t.Errorf("unknowns = %v, want bare-import-unresolved count 2 for the page object", receipt["unknowns"])
	}
	if rows["tests/login.spec.ts"] != nil {
		t.Error("a test changed path runs no reverse-import stage, so it carries no bare-import row")
	}
}

// TestImpactKeepsFrozenBareReadingWithoutManifest is GPK-V0-080's carve-out:
// with no package.json, tsconfig.json or jsconfig.json indexed, the default
// profile keeps GPK-V0-027's frozen reading of a bare specifier as a
// dependency, so the parity corpus stays byte-identical, while the syntax
// profile still reports the unknown.
func TestImpactKeepsFrozenBareReadingWithoutManifest(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, withBeamfallSignals(webImpactFiles())))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"internal/web/app/src/widget/button.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"]); anyContains(uncertainty, "bare import specifiers") {
		t.Errorf("uncertainty = %v, want the frozen default reading without a manifest", uncertainty)
	}
	syntax, err := ImpactSyntax(index, []string{"internal/web/app/src/widget/button.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	reported := false
	for _, row := range mapsFromAny(syntax["unknowns"]) {
		reported = reported || row["code"] == "bare-import-unresolved" && row["count"] == 1
	}
	if !reported {
		t.Errorf("unknowns = %v, want bare-import-unresolved count 1 for `widget/button`", syntax["unknowns"])
	}
}

func TestWebImportResolverFixture(t *testing.T) {
	_, resolver := webResolverFor(t, tsconfigPathsFixture(t))
	for _, test := range []struct {
		importer, specifier string
		want                WebImportResolution
	}{
		{"tests/login.spec.ts", "@pages/LoginPage", WebImportResolution{"src/pages/LoginPage.ts", WebImportRepository}},
		{"tests/login.spec.ts", "@pages/LoginPage.js", WebImportResolution{"src/pages/LoginPage.ts", WebImportRepository}},
		{"tests/login.spec.ts", "@pages/cart", WebImportResolution{"src/pages/cart/index.ts", WebImportRepository}},
		{"tests/login.spec.ts", "@support", WebImportResolution{"src/support/index.ts", WebImportRepository}},
		{"tests/login.spec.ts", "@shared/account", WebImportResolution{"src/shared/account.ts", WebImportRepository}},
		{"tests/login.spec.ts", "src/support", WebImportResolution{"src/support/index.ts", WebImportRepository}},
		{"nested/tests/nested.spec.ts", "@pages/LoginPage", WebImportResolution{"src/pages/LoginPage.ts", WebImportRepository}},
		{"tests/login.spec.ts", "../src/pages/LoginPage", WebImportResolution{"src/pages/LoginPage.ts", WebImportRepository}},
		{"tests/login.spec.ts", "./missing", WebImportResolution{State: WebImportUnresolved}},
		{"tests/login.spec.ts", "@playwright/test", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "@scope/kit/sub", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "left-pad", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "chalk", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "fs/promises", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "node:path", WebImportResolution{State: WebImportPackage}},
		{"tests/login.spec.ts", "@pages/Gone", WebImportResolution{State: WebImportUnresolved}},
		{"tests/login.spec.ts", "@widgets/Widget", WebImportResolution{State: WebImportUnresolved}},
		{"tests/login.spec.ts", "fixture-suite", WebImportResolution{State: WebImportUnresolved}},
		{"tests/login.spec.ts", "/abs/path", WebImportResolution{State: WebImportUnresolved}},
	} {
		if got := resolver.Resolve(test.importer, test.specifier); got != test.want {
			t.Errorf("Resolve(%s, %s) = %+v, want %+v", test.importer, test.specifier, got, test.want)
		}
	}
}

// TestWebImportResolverFailsClosed is GPK-V0-078 (proposed): a config the
// resolver cannot read in full never yields a guessed target.
func TestWebImportResolverFailsClosed(t *testing.T) {
	page := "export const Page = 1;\n"
	deep := map[string]string{"src/lib/page.ts": page, "app/a.ts": "", "tsconfig.json": `{"extends": "./c1.json"}`}
	for depth := 1; depth <= maxWebConfigDepth+1; depth++ {
		next := `{"extends": "./c` + itoa(depth+1) + `.json"}`
		if depth == maxWebConfigDepth+1 {
			next = `{"compilerOptions": {"baseUrl": "src"}}`
		}
		deep["c"+itoa(depth)+".json"] = next
	}
	for name, test := range map[string]struct {
		files     map[string]string
		specifier string
		want      WebImportResolution
	}{
		"cycle": {map[string]string{
			"tsconfig.json": `{"extends": "./a.json", "compilerOptions": {"baseUrl": "src"}}`,
			"a.json":        `{"extends": "./tsconfig.json"}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"too deep": {deep, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"missing relative extends": {map[string]string{
			"tsconfig.json": `{"extends": "./absent.json", "compilerOptions": {"baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"package extends under leaf baseUrl and paths": {map[string]string{
			"tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json", "compilerOptions": {"baseUrl": "src", "paths": {}, "moduleResolution": "bundler"}}`,
		}, "lib/page", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
		"package extends may declare the module resolution": {map[string]string{
			"tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json", "compilerOptions": {"baseUrl": "src", "paths": {}}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"package extends may declare paths": {map[string]string{
			"tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json", "compilerOptions": {"baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"package extends with nothing declared": {map[string]string{
			"tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json"}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"module suffixes": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": "src", "moduleSuffixes": [".ios", ""]}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"ambiguous wildcard": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"lib/*": ["src/lib/*"], "lib/*e": ["src/lib/*e"]}}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"two wildcards in one key": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"*/*": ["src/*/*"]}}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"package directory": {map[string]string{
			"tsconfig.json":        `{"compilerOptions": {"baseUrl": "src"}}`,
			"src/lib/package.json": `{"name": "inner", "main": "page.ts"}`,
		}, "lib", WebImportResolution{State: WebImportUnresolved}},
		"malformed config": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": "src"`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"duplicate key": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": "src", "baseUrl": "other"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"baseUrl escapes the repository": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": "../outside"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"configDir substitution": {map[string]string{
			"base/tsconfig.json": `{"compilerOptions": {"paths": {"~/*": ["${configDir}/src/*"]}}}`,
			"tsconfig.json":      `{"extends": "./base/tsconfig.json"}`,
		}, "~/lib/page", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
		"unknown paths may claim a declared package": {map[string]string{
			"tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json", "compilerOptions": {"baseUrl": "src"}}`,
			"package.json":  `{"dependencies": {"left-pad": "1"}}`,
		}, "left-pad", WebImportResolution{State: WebImportUnresolved}},
		"explicit substitution extension": {map[string]string{
			"tsconfig.json":   `{"compilerOptions": {"paths": {"alias": ["src/lib/page.js"]}}}`,
			"src/lib/page.js": page,
		}, "alias", WebImportResolution{"src/lib/page.js", WebImportRepository}},
		"matched key skips baseUrl": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": "src", "paths": {"lib/*": ["missing/*"]}}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"jsx specifier names a ts source": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias/*": ["src/lib/*"]}}}`,
		}, "alias/page.jsx", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
		"absolute substitution": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias/*": ["/src/lib/*"]}}}`,
		}, "alias/page", WebImportResolution{State: WebImportUnresolved}},
		"escaping substitution": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias/*": ["../outside/*", "src/lib/*"]}}}`,
		}, "alias/page", WebImportResolution{State: WebImportUnresolved}},
		"repository root directory": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"root": ["."]}}}`,
			"index.ts":      page,
		}, "root", WebImportResolution{"index.ts", WebImportRepository}},
		"node10 tries typed files first": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias": ["first/foo", "second/foo"]}}}`,
			"first/foo.js":  page,
			"second/foo.ts": page,
		}, "alias", WebImportResolution{"second/foo.ts", WebImportRepository}},
		"bundler tries every form at once": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"moduleResolution": "Bundler", "paths": {"alias": ["first/foo", "second/foo"]}}}`,
			"first/foo.js":  page,
			"second/foo.ts": page,
		}, "alias", WebImportResolution{"first/foo.js", WebImportRepository}},
		"relative node10 takes a typed index": {map[string]string{
			"app/foo.js":       page,
			"app/foo/index.ts": page,
		}, "./foo", WebImportResolution{"app/foo/index.ts", WebImportRepository}},
		"relative bundler takes the file": {map[string]string{
			"tsconfig.json":    `{"compilerOptions": {"module": "preserve"}}`,
			"app/foo.js":       page,
			"app/foo/index.ts": page,
		}, "./foo", WebImportResolution{"app/foo.js", WebImportRepository}},
		"mts is a typed form": {map[string]string{
			"tsconfig.json":  `{"compilerOptions": {"paths": {"alias": ["first/foo", "second/foo.mjs"]}}}`,
			"first/foo.js":   page,
			"second/foo.mts": page,
		}, "alias", WebImportResolution{"second/foo.mts", WebImportRepository}},
		"trailing slash names a directory": {map[string]string{
			"tsconfig.json":    `{"compilerOptions": {"paths": {"lib": ["src/lib/"]}}}`,
			"src/lib.ts":       page,
			"src/lib/index.ts": page,
		}, "lib", WebImportResolution{"src/lib/index.ts", WebImportRepository}},
		"declared types may win over node10 JavaScript": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias": ["src/alias"]}}}`,
			"package.json":  `{"devDependencies": {"@types/alias": "1.0.0"}}`,
			"src/alias.js":  page,
		}, "alias", WebImportResolution{State: WebImportUnresolved}},
		"bundler takes JavaScript over declared types": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"moduleResolution": "bundler", "paths": {"alias": ["src/alias"]}}}`,
			"package.json":  `{"devDependencies": {"@types/alias": "1.0.0"}}`,
			"src/alias.js":  page,
		}, "alias", WebImportResolution{"src/alias.js", WebImportRepository}},
		"nodenext needs an explicit extension": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"module": "NodeNext", "paths": {"alias": ["src/lib/page"]}}}`,
		}, "alias", WebImportResolution{State: WebImportUnresolved}},
		"nodenext explicit extension": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"moduleResolution": "node16", "paths": {"alias": ["src/lib/page.js"]}}}`,
		}, "alias", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
		"classic module is not modelled": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"module": "esnext", "baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"modern target without module is classic": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"target": "ES2020", "baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"invalid module resolution": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"moduleResolution": 2, "baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{State: WebImportUnresolved}},
		"empty baseUrl is the config directory": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"baseUrl": ""}}`,
		}, "src/lib/page", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
		"drive-letter substitution": {map[string]string{
			"tsconfig.json": `{"compilerOptions": {"paths": {"alias/*": ["C:/external/*", "src/lib/*"]}}}`,
		}, "alias/page", WebImportResolution{State: WebImportUnresolved}},
		"empty wildcard capture": {map[string]string{
			"tsconfig.json":    `{"compilerOptions": {"paths": {"alias/*": ["src/lib/*"]}}}`,
			"src/lib/index.ts": page,
		}, "alias/", WebImportResolution{State: WebImportUnresolved}},
		"jsconfig": {map[string]string{
			"jsconfig.json": `{"compilerOptions": {"baseUrl": "src"}}`,
		}, "lib/page", WebImportResolution{"src/lib/page.ts", WebImportRepository}},
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"src/lib/page.ts": page, "app/a.ts": ""}
			for key, value := range test.files {
				files[key] = value
			}
			_, resolver := webResolverFor(t, files)
			if got := resolver.Resolve("app/a.ts", test.specifier); got != test.want {
				t.Errorf("Resolve(%s) = %+v, want %+v", test.specifier, got, test.want)
			}
		})
	}
}

func itoa(value int) string {
	digits := ""
	for {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
		if value == 0 {
			return digits
		}
	}
}

func TestJSONCToJSON(t *testing.T) {
	for _, test := range []struct {
		in, want string
		ok       bool
	}{
		{`{"a": 1, // note` + "\n}", `{"a": 1  ` + "\n}", true},
		{`{"a": [1, 2,], /* b */ "c": "//not a comment",}`, `{"a": [1, 2 ],   "c": "//not a comment" }`, true},
		{`{"a": "x\"/*y"}`, `{"a": "x\"/*y"}`, true},
		{`{"a": 1 /* open`, "", false},
		{`{"a": "open`, "", false},
	} {
		got, ok := jsoncToJSON(test.in)
		if ok != test.ok || got != test.want {
			t.Errorf("jsoncToJSON(%q) = %q, %v; want %q, %v", test.in, got, ok, test.want, test.ok)
		}
	}
}

func TestWebImportKinds(t *testing.T) {
	kinds, _ := WebImportKinds(strings.Join([]string{
		`import type { A } from "type-only";`,
		`import type * as N from "type-namespace";`,
		`export type { B } from "type-export";`,
		`import type from "default-named-type";`,
		`import type, { c } from "default-and-named";`,
		`import { type D } from "inline-type";`,
		`import type { E } from "mixed";`,
		`import { F } from "mixed";`,
		`import "side-effect";`,
		`const lazy = import("dynamic");`,
	}, "\n"))
	for specifier, want := range map[string]bool{
		"type-only": true, "type-namespace": true, "type-export": true,
		"default-named-type": false, "default-and-named": false, "inline-type": false,
		"mixed": false, "side-effect": false, "dynamic": false,
	} {
		got, present := kinds[specifier]
		if !present || got != want {
			t.Errorf("WebImportKinds[%q] = %v (present %v), want %v", specifier, got, present, want)
		}
	}
}
