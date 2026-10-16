// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// up climbs one directory. It is built so that this file carries no
// parent-only literal, which would make the package a rule (d) reader.
var up = "." + "./"

// repository writes and indexes a module whose declared internal/cem/verify
// tests reach the root and name the tamper matrix, the V1-1340 shape, and whose
// declared fixtures/f tests name a tracked file without reaching the root.
func repository(t *testing.T, entries, literals string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                                "module example.test/m\n\ngo 1.27\n",
		"Makefile":                              "gate:\n",
		"protocol/cem-1.0/stable/manifest.json": "{}\n",
		"protocol/cem-1.0/stable/tamper-matrix/case.json": "{}\n",
		"testdata/case.json":                              "{}\n",
		".corvint/change.cem.json":                        "{}\n",
		"internal/cem/verify/testdata/case.json":          "{}\n",
		"internal/cem/verify/v.go":                        "package verify\n",
		"internal/cem/verify/root_test.go":                "package verify\n\nvar top = \"" + strings.Repeat(up, 3) + "\"\n\nvar parent = \"" + strings.TrimSuffix(up, "/") + "\"\n",
		"internal/cem/verify/matrix_test.go":              "package verify\n\nvar matrix = top + \"protocol/cem-1.0/stable/tamper-matrix\"\n\nvar folded = \"makefile\"\n\nvar shadowed = \"testdata/case.json\"\n\nvar evidence = \".corvint/change.cem.json\"\n",
		"fixtures/f/f.go":                                 "package f\n",
		"fixtures/f/f_test.go":                            "package f\n\nvar fixture = \"protocol/cem-1.0/stable/manifest.json\"\n",
		".corvint/test-read-scopes.json":                  `{"profile":"corvint-test-read-scopes/0","packages":{"fixtures/f":[],"internal/cem/verify":[` + entries + `]}}`,
	}
	if literals != "" {
		files[literalsPath] = literals
	}
	for name, content := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestUndeclaredRepositoryLiteralFailsNamingPackageAndPath_V1_1351(t *testing.T) {
	const matrix = `"protocol/cem-1.0/stable/tamper-matrix/"`
	// The root testdata/case.json is named by a literal that also names the
	// package's own testdata/case.json; every case but one declares it.
	const shadow = `,"testdata/case.json"`
	const head = `{"profile":"corvint-test-read-scope-literals/0","literals":`
	recorded := head + `{"internal/cem/verify":{"protocol/cem-1.0/stable/tamper-matrix":"fixture"}}}`
	for name, c := range map[string]struct {
		entries, literals string
		code              int
		want              string
	}{
		"declared entry":      {matrix + shadow, "", 0, "test-read-scopes-check: ok"},
		"declared parent":     {`"protocol/cem-1.0/stable/"` + shadow, "", 0, "test-read-scopes-check: ok"},
		"declared file below": {`"protocol/cem-1.0/stable/tamper-matrix/case.json"` + shadow, "", 0, "test-read-scopes-check: ok"},
		"entry removed":       {`"testdata/case.json"`, "", 1, "package internal/cem/verify: internal/cem/verify/matrix_test.go names protocol/cem-1.0/stable/tamper-matrix, outside its entries"},
		"sibling only":        {`"protocol/cem-1.0/stable/manifest.json"` + shadow, "", 1, "names protocol/cem-1.0/stable/tamper-matrix"},
		"root shadowed":       {matrix, "", 1, "internal/cem/verify/matrix_test.go names testdata/case.json, outside its entries"},
		"recorded literal":    {`"testdata/case.json"`, recorded, 0, "test-read-scopes-check: ok"},
		"stale record":        {matrix + shadow, recorded, 0, "records internal/cem/verify protocol/cem-1.0/stable/tamper-matrix, which no test file of the package names"},
		"empty reason":        {"", head + `{"internal/cem/verify":{"protocol/cem-1.0/stable/tamper-matrix":" "}}}`, 2, "want exactly"},
		"empty package":       {"", head + `{"internal/cem/verify":{}}}`, 2, "want exactly"},
		"unknown member":      {"", head + `{},"extra":1}`, 2, "want exactly"},
		"invalid scopes":      {`"/abs"`, "", 2, "is invalid"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(repository(t, c.entries, c.literals), &stdout, &stderr)
		output := stdout.String() + stderr.String()
		if code != c.code || !strings.Contains(output, c.want) {
			t.Errorf("%s: code=%d want %d; output %q lacks %q", name, code, c.code, output, c.want)
		}
		// fixtures/f does not reach the root, the climb to internal/cem is the
		// root lookup, a case-folded literal names no tracked path, and the
		// change-evidence map is tracked only until its seal.
		for _, never := range []string{"package fixtures/f", "names internal/cem,", "names Makefile", "names .corvint/change.cem.json"} {
			if strings.Contains(output, never) {
				t.Errorf("%s: output %q reports %q", name, output, never)
			}
		}
	}
}
