// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestDCWV0402RelationListMatchesDocsAndCite pins the DOGFOOD.md relation
// sentence and the `cem cite --relation` choices to the registered basis
// relations, so the documented list cannot drift from the code (DCW-V0-402).
func TestDCWV0402RelationListMatchesDocsAndCite(t *testing.T) {
	want := RelationChoices()
	if len(want) == 0 || slices.Contains(want, "tests") {
		t.Fatalf("RelationChoices() = %q", want)
	}
	doc, err := os.ReadFile("../../docs/DOGFOOD.md")
	if err != nil {
		t.Fatal(err)
	}
	sentence := regexp.MustCompile("(?s)The relation is one of (.*?)\\.").FindSubmatch(doc)
	if sentence == nil {
		t.Fatal("docs/DOGFOOD.md lacks the sentence \"The relation is one of ...\"")
	}
	var documented []string
	for _, m := range regexp.MustCompile("`([a-z-]+)`").FindAllSubmatch(sentence[1], -1) {
		documented = append(documented, string(m[1]))
	}
	slices.Sort(documented)
	if !slices.Equal(documented, want) {
		t.Fatalf("docs/DOGFOOD.md relations = %q, want %q", documented, want)
	}

	var stdout, stderr bytes.Buffer
	cemcli.Run(context.Background(), t.TempDir(), []string{"cite", "--map", "m.json", "--hunk", "h", "--evidence-path", "a.go", "--lines", "1:1", "--relation", "tests"}, &stdout, &stderr)
	quoted := make([]string, len(want))
	for i, relation := range want {
		quoted[i] = "'" + relation + "'"
	}
	if choices := "(choose from " + strings.Join(quoted, ", ") + ")"; !strings.Contains(stdout.String()+stderr.String(), choices) {
		t.Fatalf("cem cite --relation tests output lacks %s:\n%s%s", choices, stdout.String(), stderr.String())
	}
}
