// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// AFP-V0-9712 (V1-5465): fault cases for the read-scope declaration
// narrowing. Each edit injects a fault into one or more packages' declared
// entries (a confined test of a package whose scope changed can start or
// stop failing); the narrowed run must hold every faulted package, and no
// package whose entries are unchanged unless its own scope holds the
// declaration. A declaration the selector cannot read runs everything.
func TestReadScopeDeclarationNarrowingFaults_AFPV09712(t *testing.T) {
	files := map[string]string{}
	for _, name := range []string{"alpha", "beta", "gamma", "delta", "epsilon", "keeper"} {
		files[name+"/"+name+".go"] = "package " + name + "\n"
		files[name+"/"+name+"_test.go"] = "package " + name + "\n\nimport \"testing\"\n\nfunc TestRead(t *testing.T) {}\n"
	}
	baseDeclaration := `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/","spec/"],"epsilon":[],"keeper":[".corvint/"]`
	files[readScopesPath] = declaration(baseDeclaration)
	root, base := baseRepository(t, files, nil)
	cases := []struct {
		name    string
		current string // declaration body; "" deletes the file
		faulted []string
	}{
		{"entry changed", `"alpha":["docs/"],"beta":["spec/"],"delta":["docs/","spec/"],"epsilon":[],"keeper":[".corvint/"]`, []string{"beta"}},
		{"entry added", `"alpha":["docs/","spec/"],"beta":["docs/"],"delta":["docs/","spec/"],"epsilon":[],"keeper":[".corvint/"]`, []string{"alpha"}},
		{"entry removed", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/"],"epsilon":[],"keeper":[".corvint/"]`, []string{"delta"}},
		{"empty scope widened", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/","spec/"],"epsilon":["docs/"],"keeper":[".corvint/"]`, []string{"epsilon"}},
		{"key added", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/","spec/"],"epsilon":[],"gamma":["docs/"],"keeper":[".corvint/"]`, []string{"gamma"}},
		{"key removed", `"alpha":["docs/"],"beta":["docs/"],"delta":["docs/","spec/"],"keeper":[".corvint/"]`, []string{"epsilon"}},
		{"key renamed", `"alpha":["docs/"],"delta":["docs/","spec/"],"epsilon":[],"gamma":["docs/"],"keeper":[".corvint/"]`, []string{"beta", "gamma"}},
		{"two keys changed", `"alpha":["spec/"],"beta":["docs/"],"delta":["spec/"],"epsilon":[],"keeper":[".corvint/"]`, []string{"alpha", "delta"}},
		{"reformatted only", strings.ReplaceAll(baseDeclaration, ",", ", "), nil},
		{"declaration deleted", "", []string{"alpha", "beta", "delta", "epsilon", "keeper"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(root, filepath.FromSlash(readScopesPath))
			if tc.current == "" {
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
				defer os.WriteFile(name, []byte(files[readScopesPath]), 0o644)
			} else if err := os.WriteFile(name, []byte(declaration(tc.current)), 0o644); err != nil {
				t.Fatal(err)
			}
			plan := ncmPlan([]string{readScopesPath})
			plan.Revision = base
			// keeper's own scope holds the declaration (AFP-V0-050).
			want := append(slices.Clone(tc.faulted), "keeper")
			slices.Sort(want)
			want = slices.Compact(want)
			var packages []string
			for _, pkg := range want {
				packages = append(packages, fixtureModule+"/"+pkg)
			}
			requireVerdict(t, plan, root, "run "+strings.Join(packages, " "), "noncode read-scope-declaration "+readScopesPath)
		})
	}
	t.Run("invalid current declaration", func(t *testing.T) {
		name := filepath.Join(root, filepath.FromSlash(readScopesPath))
		if err := os.WriteFile(name, []byte(declaration(`"beta":["spec/","docs/"]`)), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(name, []byte(files[readScopesPath]), 0o644)
		plan := ncmPlan([]string{readScopesPath})
		plan.Revision = base
		requireVerdict(t, plan, root, "FALLBACK corvint-config "+readScopesPath, "noncode corvint-config "+readScopesPath)
	})
	t.Run("another configuration path still runs everything", func(t *testing.T) {
		name := filepath.Join(root, filepath.FromSlash(readScopesPath))
		if err := os.WriteFile(name, []byte(declaration(cases[0].current)), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(name, []byte(files[readScopesPath]), 0o644)
		plan := ncmPlan([]string{readScopesPath, ".corvint/runtime-readers.json"})
		plan.Revision = base
		requireVerdict(t, plan, root, "FALLBACK corvint-config .corvint/runtime-readers.json", "noncode read-scope-declaration "+readScopesPath)
	})
}

// AFP-V0-9713 (V1-5465): an edit of the literal exemptions, which only the
// declaration tools read, keeps the selector's attribution of its readers
// while the current file is absent or exactly the tools' closed record; any
// other content runs everything, and an edit no package reads is never
// narrowed to nothing.
func TestReadScopeLiteralsNarrowToReaders_AFPV09713(t *testing.T) {
	const literals = readScopeLiteralsPath
	reader := map[string]string{
		"checker/checker.go":      "package checker\n",
		"checker/checker_test.go": "package checker\n\nvar literals = \"" + literals + "\"\n",
	}
	valid := `{"profile":"corvint-test-read-scope-literals/0","literals":{"app":{"app/testdata/seed.txt":"fixture text, not a read"}}}`
	cases := []struct{ name, body, verdict string }{
		{"valid edit", valid, "run " + fixtureModule + "/checker"},
		{"deleted", "", "run " + fixtureModule + "/checker"},
		{"wrong profile", strings.Replace(valid, "literals/0", "literals/1", 1), "FALLBACK corvint-config " + literals},
		{"empty reason", strings.Replace(valid, "fixture text, not a read", " ", 1), "FALLBACK corvint-config " + literals},
		{"unknown member", strings.Replace(valid, `"literals":`, `"extra":1,"literals":`, 1), "FALLBACK corvint-config " + literals},
		{"no paths", `{"profile":"corvint-test-read-scope-literals/0","literals":{"app":{}}}`, "FALLBACK corvint-config " + literals},
		{"climbing path", strings.Replace(valid, "app/testdata/seed.txt", "../seed.txt", 1), "FALLBACK corvint-config " + literals},
		{"not json", "{", "FALLBACK corvint-config " + literals},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := map[string]string{}
			for name, body := range reader {
				extra[name] = body
			}
			if tc.body != "" {
				extra[literals] = tc.body
			}
			root := ncmFixture(t, extra)
			audit := "noncode corvint-config " + literals
			if strings.HasPrefix(tc.verdict, "run ") {
				audit = "noncode read-scope-declaration " + literals
			}
			requireVerdict(t, ncmPlan([]string{literals}), root, tc.verdict, audit)
		})
	}
	t.Run("symlink", func(t *testing.T) {
		extra := map[string]string{"real.json": valid}
		for name, body := range reader {
			extra[name] = body
		}
		root := ncmFixture(t, extra)
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../real.json", filepath.Join(root, filepath.FromSlash(literals))); err != nil {
			t.Fatal(err)
		}
		requireVerdict(t, ncmPlan([]string{literals}), root, "FALLBACK corvint-config "+literals, "noncode corvint-config "+literals)
	})
	t.Run("no reader", func(t *testing.T) {
		root := ncmFixture(t, map[string]string{literals: valid})
		for name, lines := range bothSelectors(ncmPlan([]string{literals}), root) {
			if verdict := lines[len(lines)-1]; !strings.HasPrefix(verdict, "FALLBACK ") {
				t.Fatalf("%s verdict = %q, want a full run for an edit no package reads:\n%s", name, verdict, strings.Join(lines, "\n"))
			}
		}
	})
}
