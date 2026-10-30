package gateselect

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// parallelFixture spans more files than scanWorkers, with literals, unresolved
// reads, embeds and cross-package imports, so the merge order matters.
func parallelFixture() map[string]string {
	files := map[string]string{}
	for i := range 48 {
		files[fmt.Sprintf("p%02d/p.go", i)] = fmt.Sprintf("package p%02d\n\nimport _ %q\n\nvar a = \"docs/specs/s%02d.md\"\n", i, fmt.Sprintf("%s/p%02d", fixtureModule, (i+1)%48), i%7)
		files[fmt.Sprintf("p%02d/p_test.go", i)] = fmt.Sprintf("package p%02d\n\nvar b = \"testdata/t%02d.txt\"\nvar c = \"../p%02d/x\"\n", i, i, (i+3)%48)
	}
	files["walker/walker.go"] = "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n"
	files["walker/walker_test.go"] = "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n"
	files["embedder/e.go"] = "package embedder\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n"
	return files
}

// TestAFPV0316ParallelIndexMatchesSerial: the bounded scan pool builds the
// same index as the serial pass (GOMAXPROCS 1 takes the serial path).
func TestAFPV0316ParallelIndexMatchesSerial(t *testing.T) {
	root := writeFixture(t, parallelFixture())
	previous := runtime.GOMAXPROCS(1)
	serial, err := indexRepository(root, fixtureModule)
	runtime.GOMAXPROCS(max(previous, scanWorkers))
	defer runtime.GOMAXPROCS(previous)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		parallel, err := indexRepository(root, fixtureModule)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(serial, parallel) {
			t.Fatalf("parallel index differs from the serial one:\nserial   %+v\nparallel %+v", serial, parallel)
		}
	}
	if pkg := serial.packages["walker"]; pkg == nil || !strings.Contains(pkg.unresolved, "walker/walker.go calls os.Getwd") {
		t.Fatalf("walker = %+v", pkg)
	}
}

// TestAFPV0316ParallelIndexReportsTheFirstErrorInWalkOrder: with several
// unreadable sources, the error is the one the serial walk meets first.
func TestAFPV0316ParallelIndexReportsTheFirstErrorInWalkOrder(t *testing.T) {
	files := parallelFixture()
	files["p05/broken.go"] = "package p05\n\nimport (\n"
	files["p30/broken.go"] = "package p30\n\nimport (\n"
	root := writeFixture(t, files)
	previous := runtime.GOMAXPROCS(max(runtime.GOMAXPROCS(0), scanWorkers))
	defer runtime.GOMAXPROCS(previous)
	for range 5 {
		_, err := indexRepository(root, fixtureModule)
		if err == nil || err.Error() != "p05/broken.go: imports do not parse" {
			t.Fatalf("error = %v, want the walk-first p05/broken.go", err)
		}
	}
}

// BenchmarkIndexRepository measures indexing a 1,000-file fixture (V1-2169).
func BenchmarkIndexRepository(b *testing.B) {
	files := map[string]string{}
	body := strings.Repeat("// filler line of commentary for the lexer\n", 200)
	for i := range 1000 {
		files[fmt.Sprintf("d%02d/f%03d.go", i%50, i)] = fmt.Sprintf("package d%02d\n\nimport \"strings\"\n\nvar _ = strings.ToUpper(\"docs/specs/s%03d.md\")\n%s", i%50, i, body)
	}
	root := writeFixture(b, files)
	for b.Loop() {
		if _, err := indexRepository(root, fixtureModule); err != nil {
			b.Fatal(err)
		}
	}
}
