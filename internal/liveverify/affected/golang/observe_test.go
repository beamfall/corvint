package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// observeFixture writes a module of count packages that import their
// neighbours, carry test imports and path literals, and raise frontier
// reasons from scattered directories, so a reordered merge would show.
func observeFixture(t testing.TB, count int) string {
	root := t.TempDir()
	files := map[string]string{"go.mod": "module example.test/m\n"}
	for offset := range count {
		directory := fmt.Sprintf("p%02d/q%d", offset, offset%3)
		next := fmt.Sprintf("example.test/m/p%02d/q%d", (offset+1)%count, (offset+1)%count%3)
		files[directory+"/p.go"] = fmt.Sprintf("package q\n\nimport _ %q\n\nvar path = \"testdata/case%02d.json\"\n", next, offset)
		files[directory+"/p_test.go"] = fmt.Sprintf("package q\n\nimport _ %q\n", "example.test/m/p00/q0")
		switch offset % 7 {
		case 2:
			files[directory+"/broken.go"] = "package q\n\nvar s = \"unterminated\n"
		case 4:
			files[directory+"/c.go"] = "package q\n\nimport \"C\"\n"
		}
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The bounded pool observes a disk source to the same units, edges and
// frontier as the serial pass a single processor takes.
func TestConcurrentDirectoryObservationMatchesSerial(t *testing.T) {
	root := observeFixture(t, 40)
	units := func(procs int, source *affected.Source) affected.Result {
		previous := runtime.GOMAXPROCS(procs)
		defer runtime.GOMAXPROCS(previous)
		result, err := New().UnitsSource(source)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	serial := units(1, affected.DiskSource(root))
	if len(serial.Units) != 40 || len(serial.Frontier) < 2 {
		t.Fatalf("fixture lost its shape: %d units, frontier %v", len(serial.Units), serial.Frontier)
	}
	for _, procs := range []int{2, 8, 16} {
		if parallel := units(procs, affected.DiskSource(root)); !reflect.DeepEqual(parallel, serial) {
			t.Fatalf("GOMAXPROCS=%d observation differs from the serial pass", procs)
		}
	}
}

// BenchmarkUnitsSourceDisk times one Go observation of 400 packages.
func BenchmarkUnitsSourceDisk(b *testing.B) {
	root := observeFixture(b, 400)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := New().UnitsSource(affected.DiskSource(root)); err != nil {
			b.Fatal(err)
		}
	}
}
