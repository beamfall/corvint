package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

const calcFiller = "// filler one\n// filler two\n// filler three\n// filler four\n// filler five\n// filler six\n// filler seven\n// filler eight\n"

const calcBase = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n\treturn v\n}\n\n" +
	calcFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\treturn 0\n}\n"

const calcTarget = "package calc\n\n// Clamp bounds v to [lo, hi].\nfunc Clamp(v, lo, hi int) int {\n" +
	"\tif v < lo {\n\t\treturn lo\n\t}\n\tif v > hi {\n\t\treturn hi\n\t}\n\treturn v\n}\n\n" +
	calcFiller + "\n// Sign reports -1, 0, or 1.\nfunc Sign(v int) int {\n\tif v < 0 {\n\t\treturn -1\n\t}\n\tif v > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"

const calcStrongTest = "package calc\n\nimport \"testing\"\n\nfunc TestClamp(t *testing.T) {\n" +
	"\tfor _, c := range []struct{ v, lo, hi, want int }{{5, 0, 3, 3}, {-1, 0, 3, 0}, {2, 0, 3, 2}, {0, 0, 3, 0}, {3, 0, 3, 3}} {\n" +
	"\t\tif got := Clamp(c.v, c.lo, c.hi); got != c.want {\n\t\t\tt.Fatalf(\"Clamp(%d,%d,%d) = %d, want %d\", c.v, c.lo, c.hi, got, c.want)\n\t\t}\n\t}\n}\n"

const calcWeakTest = "package calc\n\nimport \"testing\"\n\nfunc TestClampWeak(t *testing.T) {\n\t_ = Clamp(2, 0, 3)\n}\n"

// makeCalcRepo builds a Go module whose target commit changes two hunks of
// pkg/calc/calc.go; the strong and weak test files exist at the base so they
// can be cited as test-claim evidence.
func makeCalcRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.27.0\n")
	writeFile(t, root, "pkg/calc/calc.go", calcBase)
	writeFile(t, root, "pkg/calc/calc_test.go", calcStrongTest)
	writeFile(t, root, "pkg/calc/weak_test.go", calcWeakTest)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "pkg/calc/calc.go", calcTarget)
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "target")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

func selectionOf(paths ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(paths, "\n") + "\n"))
	return hex.EncodeToString(digest[:])
}
