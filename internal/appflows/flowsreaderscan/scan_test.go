// SPDX-License-Identifier: Apache-2.0

package flowsreaderscan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// flowsReaderPackage is the one package that may name the flows declaration
// path (FSV-V1-003).
const flowsReaderPackage = "internal/appflows"

// secondFlowsReaders walks the tops under root and lists, sorted, each
// package directory other than internal/appflows whose non-test Go source
// outside testdata names the flows declaration path: a string literal ending
// in it, or the literals ".corvint" and "flows.json" in one file, as a
// filepath.Join of the parts would. A path inside longer text, such as help,
// is not a reader.
func secondFlowsReaders(t *testing.T, root string, tops ...string) []string {
	t.Helper()
	found := map[string]bool{}
	for _, top := range tops {
		err := filepath.WalkDir(filepath.Join(root, top), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && (entry.Name() == "testdata" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, name)
			if err != nil {
				return err
			}
			dir := path.Dir(filepath.ToSlash(relative))
			if dir == flowsReaderPackage || found[dir] {
				return nil
			}
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			if namesFlowsDeclaration(t, name, data) {
				found[dir] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	out := []string{}
	for dir := range found {
		out = append(out, dir)
	}
	slices.Sort(out)
	return out
}

func namesFlowsDeclaration(t *testing.T, name string, data []byte) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	direct, dir, base := false, false, false
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		switch {
		case value == appflows.FlowsDeclarationPath || strings.HasSuffix(value, "/"+appflows.FlowsDeclarationPath):
			direct = true
		case value == path.Dir(appflows.FlowsDeclarationPath):
			dir = true
		case value == path.Base(appflows.FlowsDeclarationPath):
			base = true
		}
		return true
	})
	return direct || dir && base
}

// FSV-V1-003 (V1-5410): only internal/appflows names `.corvint/flows.json`
// in the source of cmd/ and internal/; every other package reads the file
// through ReadFlowsDeclaration.
func TestFSVV1003OneFlowsReader_V15410(t *testing.T) {
	if got := secondFlowsReaders(t, filepath.Join("..", "..", ".."), "cmd", "internal"); len(got) != 0 {
		t.Fatalf("packages outside %s name %s; read it through appflows.ReadFlowsDeclaration (FSV-V1-003): %v", flowsReaderPackage, appflows.FlowsDeclarationPath, got)
	}
}

// FSV-V1-003 (V1-5410): the scan fails on a fixture package that opens the
// file directly, by literal or by joined parts, and names each package;
// help text, test files and the reader package itself do not count.
func TestFSVV1003ScanNamesSecondReader_V15410(t *testing.T) {
	got := secondFlowsReaders(t, filepath.Join("testdata", "flows-scan"), "cmd", "internal")
	if want := []string{"internal/joinedreader", "internal/secondreader"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("scan found %v, want %v", got, want)
	}
}
