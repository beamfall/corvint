// SPDX-License-Identifier: Apache-2.0
package contextindex

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TCP-V0-8200: selected slot evidence adds to BM25; a lookup carries source
// before its paired test, and neither path disappears.
func TestTaskContextSelectedSlotScores(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":         "module example.test/slots\n\ngo 1.27.0\n",
		"engine.go":      "package slots\n// RunEngine canonical binding implementation.\nfunc RunEngine() {}\n",
		"engine_test.go": "package slots\n// RunEngine canonical binding tests.\nfunc TestRunEngine() { RunEngine() }\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	c := newTaskContextCompiler(index, "where is `RunEngine` canonical binding", "")
	rows := c.compile(10)
	paths := []string{}
	band := c.lexicalBand()
	for _, row := range rows {
		paths = append(paths, row.path)
		if row.kind != "definition" && row.kind != "test" {
			continue
		}
		want := min(contextLexicalBase+band[row.path]+contextSlotBonus[row.kind]+50*len(c.support[row.path]), contextAdditiveMaxScore)
		if row.score != want || !strings.Contains(row.reason, "; additive ") {
			t.Fatalf("%s score=%d want=%d: %s", row.path, row.score, want, row.reason)
		}
	}
	a, b := slices.Index(paths, "engine.go"), slices.Index(paths, "engine_test.go")
	if a < 0 || b < 0 || a >= b {
		t.Fatalf("source/test order: %v", paths)
	}
}
