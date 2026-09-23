package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
)

var structuralClasses = []string{"formatter-only", "import-reorder", "move", "rename"}

func fixture(t *testing.T, class, name string) []byte {
	t.Helper()
	// Fixtures are Go source kept under .txt so gofmt gates leave the
	// intentionally unformatted bases alone.
	data, err := os.ReadFile(filepath.Join("testdata", "structural", class, name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestStructuralTruePositives proves each fixture class from its base and
// positive files (CEM-SM-007..010).
func TestStructuralTruePositives(t *testing.T) {
	for _, class := range structuralClasses {
		if !structuralPredicates[class](fixture(t, class, "base"), fixture(t, class, "positive")) {
			t.Errorf("%s: positive fixture not proven", class)
		}
	}
}

// TestStructuralNearMisses refuses a semantic change hidden inside each
// class, and refuses every class claimed against another class's positive.
// The one admitted overlap is gofmt's own import sorting: the import-reorder
// positive is also formatter-only.
func TestStructuralNearMisses(t *testing.T) {
	overlap := map[string]bool{"import-reorder/formatter-only": true}
	for _, class := range structuralClasses {
		base := fixture(t, class, "base")
		if structuralPredicates[class](base, fixture(t, class, "nearmiss")) {
			t.Errorf("%s: near-miss fixture accepted", class)
		}
		for _, other := range structuralClasses {
			if other == class || overlap[class+"/"+other] {
				continue
			}
			if structuralPredicates[other](base, fixture(t, class, "positive")) {
				t.Errorf("%s positive accepted as %s", class, other)
			}
		}
	}
}

// TestStructuralRefusesUnprovableInputs covers the CEM-SM-005 refusals and
// the rename and move rules that the fixtures do not exercise.
func TestStructuralRefusesUnprovableInputs(t *testing.T) {
	cases := []struct{ name, class, old, new string }{
		{"non-go", "formatter-only", "not go\n", "not  go\n"},
		{"unparsable-new", "rename", "package p\n\nvar a = 1\n", "package p\n\nvar b = \n"},
		{"selector-rename", "rename",
			"package p\n\ntype c struct{ timeout int }\n\nfunc f(cfg c) int { return cfg.timeout }\n",
			"package p\n\ntype c struct{ deadline int }\n\nfunc f(cfg c) int { return cfg.deadline }\n"},
		{"exported-rename", "rename", "package p\n\nfunc Old() {}\n", "package p\n\nfunc New() {}\n"},
		{"rename-to-existing", "rename", "package p\n\nvar a, b = 1, 2\n\nfunc f() int { return a }\n",
			"package p\n\nvar a, b = 1, 2\n\nfunc f() int { return b }\n"},
		{"directive-comment", "rename",
			"package p\n\n//go:noinline\nfunc a() {}\n",
			"package p\n\n//go:inline\nfunc b() {}\n"},
		{"var-order", "move",
			"package p\n\nfunc g() int { return 1 }\n\nvar a = g()\n\nvar b = g()\n",
			"package p\n\nfunc g() int { return 1 }\n\nvar b = g()\n\nvar a = g()\n"},
		{"init-order", "move",
			"package p\n\nfunc init() { println(1) }\n\nfunc init() { println(2) }\n",
			"package p\n\nfunc init() { println(2) }\n\nfunc init() { println(1) }\n"},
		{"import-alias", "import-reorder",
			"package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nvar _ = fmt.Sprint(strings.ToUpper(\"\"))\n",
			"package p\n\nimport (\n\tstrings \"strings\"\n\t\"fmt\"\n)\n\nvar _ = fmt.Sprint(strings.ToUpper(\"\"))\n"},
	}
	for _, tc := range cases {
		if structuralPredicates[tc.class]([]byte(tc.old), []byte(tc.new)) {
			t.Errorf("%s: %s accepted", tc.name, tc.class)
		}
	}
	if !structuralPredicates["move"]([]byte("package p\n\nfunc g() int { return 1 }\n\nvar a = g()\n\nfunc h() {}\n"),
		[]byte("package p\n\nfunc h() {}\n\nfunc g() int { return 1 }\n\nvar a = g()\n")) {
		t.Error("move that keeps var order refused")
	}
}

type blobMap map[string][]byte

func (m blobMap) BaseBlob(path string) ([]byte, string, bool, error) {
	data, ok := m[path]
	return data, "100644", ok, nil
}

var _ sim.BlobSource = blobMap{}

// replacementPatch renders a single full-file replacement hunk.
func replacementPatch(path string, old, new []byte) []byte {
	var b strings.Builder
	oldLines := strings.SplitAfter(strings.TrimSuffix(string(old), "\n"), "\n")
	newLines := strings.SplitAfter(strings.TrimSuffix(string(new), "\n"), "\n")
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", path, path, path, path, len(oldLines), len(newLines))
	for _, line := range oldLines {
		b.WriteString("-" + strings.TrimSuffix(line, "\n") + "\n")
	}
	for _, line := range newLines {
		b.WriteString("+" + strings.TrimSuffix(line, "\n") + "\n")
	}
	return []byte(b.String())
}

func parseSingle(t *testing.T, raw []byte, source sim.BlobSource) (*patch.Group, *patch.Hunk) {
	t.Helper()
	parsed, err := patch.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Simulate(parsed, source); err != nil {
		t.Fatal(err)
	}
	return parsed.Groups[0], parsed.Hunks[0]
}

// TestStructuralProvenFromBaseBlobAndPatch proves every class from the base
// blob and the hunk alone (CEM-SM-002..004) and refuses a missing blob and a
// wrong class.
func TestStructuralProvenFromBaseBlobAndPatch(t *testing.T) {
	for _, class := range structuralClasses {
		base, positive := fixture(t, class, "base"), fixture(t, class, "positive")
		source := blobMap{"x.go": base}
		group, hunk := parseSingle(t, replacementPatch("x.go", base, positive), source)
		if !structuralProven(group, hunk, class, source) {
			t.Errorf("%s: not proven from base blob and patch", class)
		}
		if structuralProven(group, hunk, class, blobMap{}) {
			t.Errorf("%s: proven without a base blob", class)
		}
		other := structuralClasses[(indexOf(class)+1)%len(structuralClasses)]
		if structuralProven(group, hunk, other, source) {
			t.Errorf("%s hunk proven as %s", class, other)
		}
	}
}

func indexOf(class string) int {
	for i, c := range structuralClasses {
		if c == class {
			return i
		}
	}
	return -1
}

// TestStructuralProvenIsolatesHunksExceptMove applies a non-move hunk in
// isolation so a semantic sibling hunk does not taint it, and requires the
// whole group for move.
func TestStructuralProvenIsolatesHunksExceptMove(t *testing.T) {
	base := []byte("package p\n\nimport (\n\t\"strings\"\n\t\"fmt\"\n)\n\nvar _ = fmt.Sprint(strings.ToUpper(\"\"))\n" +
		strings.Repeat("\n", 5) + "func z() int { return 1 }\n")
	raw := []byte("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n" +
		"@@ -3,4 +3,4 @@\n import (\n-\t\"strings\"\n \t\"fmt\"\n+\t\"strings\"\n )\n" +
		"@@ -14,1 +14,1 @@\n-func z() int { return 1 }\n+func z() int { return 2 }\n")
	source := blobMap{"x.go": base}
	parsed, err := patch.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Simulate(parsed, source); err != nil {
		t.Fatal(err)
	}
	group := parsed.Groups[0]
	if !structuralProven(group, group.Hunks[0], "import-reorder", source) {
		t.Error("isolated import-reorder hunk refused beside a semantic hunk")
	}
	if structuralProven(group, group.Hunks[1], "import-reorder", source) {
		t.Error("semantic hunk proven as import-reorder")
	}
	if structuralProven(group, group.Hunks[0], "move", source) {
		t.Error("move proven from a group whose image changes semantics")
	}
}

// TestStructuralProvenRefusesCreateAndDelete pins that structural reasons
// need a base blob and an image that differs from it.
func TestStructuralProvenRefusesCreateAndDelete(t *testing.T) {
	path := "x.go"
	hunk := &patch.Hunk{}
	for _, kind := range []patch.GroupKind{patch.KindCreate, patch.KindDelete} {
		group := &patch.Group{Kind: kind, OldPath: &path, NewPath: &path, Hunks: []*patch.Hunk{hunk}}
		if structuralProven(group, hunk, "formatter-only", blobMap{path: []byte("package p\n")}) {
			t.Errorf("%v group proven", kind)
		}
	}
	if structuralProven(nil, hunk, "formatter-only", blobMap{}) {
		t.Error("nil group proven")
	}
}
