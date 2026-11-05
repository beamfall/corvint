// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestGPKV08000SharedGoParseMatchesIndependent(t *testing.T) {
	t.Run("GPK-V0-8000 shared", func(t *testing.T) {
		check := func(name, text string) {
			t.Helper()
			source := Source{Path: name, BlobHash: "fixture", Data: []byte(text)}
			symbols, fields, refusal, file := goSymbolsParsed(source, text)
			wantSymbols, wantFields, wantRefusal := goSymbols(source, text)
			gotImports, gotWhy := goImportsAfterSymbols(file, text)
			wantImports, wantWhy := goImports(text)
			if !reflect.DeepEqual(symbols, wantSymbols) || !reflect.DeepEqual(fields, wantFields) || refusal != wantRefusal || !reflect.DeepEqual(gotImports, wantImports) || gotWhy != wantWhy {
				t.Fatalf("%s: shared parse differs from independent symbol/import extraction", name)
			}
			var chunk compiledSources
			if _, valid, _ := source.Text(); !valid {
				return
			}
			chunk.collect(&Index{Sources: map[string]Source{name: source}}, []string{name}, nil)
			if !slices.Equal(chunk.symbols, symbols) || !slices.Equal(chunk.fields, fields) || !reflect.DeepEqual(chunk.imports[name], nonemptyImports(wantImports)) {
				t.Fatalf("%s: collector differs from independent extraction", name)
			}
			var importWhy string
			for _, u := range chunk.unparsed {
				if u.Facts == "imports" {
					importWhy = u.Detail
				}
			}
			if importWhy != wantWhy {
				t.Fatalf("%s: import refusal %q != %q", name, importWhy, wantWhy)
			}
		}
		for _, text := range []string{
			"package p\n", "package p\nvar importWord = 1\n",
			"package p\nimport(\"x/y\"; _ `a/b`)\nfunc F(){}\n",
			"package p\nimport \"x/y\"\nfunc F( {\n", // imports valid, body invalid
			"import (\n\"x/y\"\n)\n", "package p\nimport (\"unterminated\n",
			"package p\nimport \"x\\u002fy\"\n", "package p\nimport \"x\\q\"\n",
		} {
			check("edge.go", text)
		}
		if _, err := exec.LookPath("git"); err != nil {
			t.Skipf("tracked Go source parity requires git on PATH: %v", err)
		}
		root := repositoryRoot(t)
		listed, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go").Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range strings.Split(string(listed), "\x00") {
			if name == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if len(data) <= maxSourceBytes {
				check(name, string(data))
			}
		}
	})
}

func nonemptyImports(m map[string]struct{}) map[string]struct{} {
	if len(m) == 0 {
		return nil
	}
	return m
}

func TestGPKV08000SharedImportAllocationBound(t *testing.T) {
	t.Run("GPK-V0-8000 allocation", func(t *testing.T) {
		text := "package p\nimport(\"x/y\"; _ `a/b`)\nfunc F(){}\n"
		_, _, _, file := goSymbolsParsed(Source{}, text)
		shared := testing.AllocsPerRun(20, func() { goImportsAfterSymbols(file, text) })
		independent := testing.AllocsPerRun(20, func() { goImports(text) })
		if shared*2 >= independent {
			t.Fatalf("shared imports %.0f allocations, independent %.0f; redundant parse retained", shared, independent)
		}
		t.Run("collect", func(t *testing.T) {
			// Compare the real collector with the same collector excluding imports
			// plus an independent import parse. A helper-only guard cannot detect
			// the collector accidentally returning to the independent parser.
			index := &Index{Sources: map[string]Source{
				"fixture.go": {Path: "fixture.go", BlobHash: "fixture", Data: []byte(text)},
			}}
			paths := []string{"fixture.go"}
			collected := testing.AllocsPerRun(20, func() {
				var chunk compiledSources
				chunk.collect(index, paths, nil)
			})
			separate := testing.AllocsPerRun(20, func() {
				var chunk compiledSources
				chunk.collect(index, paths, func(string) bool { return false })
				imported, why := goImports(text)
				chunk.recordImportExtraction("fixture.go", importExtraction{imports: imported, refusal: why, reason: UnparsedGoGrammar})
			})
			t.Logf("collector %.0f allocations, separate %.0f, independent imports %.0f", collected, separate, independent)
			if separate-collected < independent/2 {
				t.Fatalf("collector saves %.0f allocations, want at least half of independent imports %.0f; redundant parse retained", separate-collected, independent)
			}
		})
	})
}

func BenchmarkGPKV08000GoFacts(b *testing.B) {
	text := "package p\nimport(\"x/y\"; _ `a/b`)\nfunc F(){}\n"
	for _, shared := range []bool{false, true} {
		name := "Independent"
		if shared {
			name = "Shared"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				_, _, _, file := goSymbolsParsed(Source{}, text)
				if shared {
					goImportsAfterSymbols(file, text)
				} else {
					goImports(text)
				}
			}
		})
	}
}
