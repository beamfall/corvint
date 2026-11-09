// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9600 (V1-5071): internal/closureguard holds this repository's
// `go list` import-graph guards and reads nothing else, so it must be a
// bounded unit and the AFP-V0-9010 rule must classify it as a proven reader.
// A body-only edit then selects none of its tests; an import-set change still
// selects them (TestClosureGuardShapeIsProven_AFP_V0_9600). A guard that
// observes more (an exec.ExitError, a shared Caller root, a reused command
// output, a `..` or `/...` literal) fails here.
func TestClosureGuardIsProvenImportGraphReader_AFPV09600(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const directory = "internal/closureguard"
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".go") {
			files = append(files, directory+"/"+entry.Name())
		}
	}
	source := affected.DiskSource(root)
	owner := module{dir: ".", path: "github.com/Beamfall/corvint", listed: true}
	unit, _, _, _, _, err := Language{}.observeDirectory(source, owner, directory, files, []string{directory}, map[string]bool{}, &readKindAliases{})
	if err != nil {
		t.Fatal(err)
	}
	if unit.UnboundedReads != "" || unit.LocatesRoot {
		t.Fatalf("internal/closureguard is an unbounded reader: %s", unit.UnboundedReads)
	}
	if len(unit.Tests) == 0 {
		t.Fatal("internal/closureguard has no tests")
	}
	units := []affected.Unit{unit}
	scans := goListUnits(source, units, []string{directory}, map[string]module{directory: owner})
	if units[0].ImportGraphReadsFiles {
		t.Fatalf("internal/closureguard is an unproven import-graph reader; scans=%+v", scans[0])
	}
	for _, test := range unit.Tests {
		scan, ok := scans[0][test]
		if !ok || !scan.reader || scan.unproven {
			t.Errorf("%s: reader=%v unproven=%v, want a proven go list reader", test, scan.reader, scan.unproven)
		}
	}
}
