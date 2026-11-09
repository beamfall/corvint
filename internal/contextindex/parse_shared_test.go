// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"testing"
)

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
