package gitstatus

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Decision 0383: every refusal site names a class of the closed set other
// than "unclassified", fixed in source where the refusal is built.
func TestEveryRefusalSiteCarriesAClosedClass(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	classes := map[string]string{}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if ok && fmt.Sprint(spec.Type) == "reasonClass" {
				value, _ := strconv.Unquote(spec.Values[0].(*ast.BasicLit).Value)
				classes[spec.Names[0].Name] = value
			}
			return true
		})
	}
	values := make([]string, 0, len(classes))
	for _, value := range classes {
		values = append(values, value)
	}
	slices.Sort(values)
	want := []string{"attributes-file", "config-include", "config-malformed", "git-filter", "gitdir-pointer",
		"metadata-directory", "metadata-drift", "metadata-limit", "metadata-unreadable", "ref-storage",
		"root-unresolved", "scratch-dir", "split-index", "submodule", "unclassified", "worktree-config"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("reason classes=%v want the closed set %v", values, want)
	}
	// A site passes a class constant, or forwards the class returned by
	// unsafeConfig or captureReason, whose own returns are checked below.
	classified := func(expression ast.Expr, forwarded ...string) bool {
		switch value := expression.(type) {
		case *ast.Ident:
			constant, known := classes[value.Name]
			return known && constant != "unclassified" || slices.Contains(forwarded, value.Name)
		case *ast.SelectorExpr:
			return slices.Contains(forwarded, fmt.Sprint(value.X)+"."+value.Sel.Name)
		}
		return false
	}
	sites := 0
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				if name, ok := value.Fun.(*ast.Ident); ok && name.Name == "unsupported" {
					sites++
					if !classified(value.Args[0], "class") {
						t.Errorf("%s: refusal without a closed class", fileSet.Position(value.Pos()))
					}
				}
			case *ast.FuncDecl:
				if value.Name.Name != "unsafeConfig" && value.Name.Name != "captureReason" {
					return true
				}
				ast.Inspect(value.Body, func(node ast.Node) bool {
					result, ok := node.(*ast.ReturnStmt)
					if !ok {
						return true
					}
					sites++
					reason, literal := result.Results[1].(*ast.BasicLit)
					safe := literal && reason.Value == `""`
					if !safe && !classified(result.Results[0], "refused.class") {
						t.Errorf("%s: refusal reason without a closed class", fileSet.Position(result.Pos()))
					}
					return true
				})
				return false
			}
			return true
		})
	}
	if sites < 40 {
		t.Fatalf("found only %d refusal sites; the inspection lost its targets", sites)
	}
}

// The class is read from the typed refusal only: text that looks like a
// refusal is unclassified, and a wrapped refusal keeps its own class whatever
// its reason says.
func TestRefusalClassComesFromTypedRefusalOnly(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("unrelated"), "unclassified"},
		{errors.New(errUnsafe.Error() + ": repository config sets filter.hostile.clean"), "unclassified"},
		{errors.New(RefusalMessage(unsupported(classGitFilter, "repository config sets filter.hostile.clean"))), "unclassified"},
		{fmt.Errorf("wrapped: %w", unsupported(classSplitIndex, "repository config sets filter.hostile.clean")), "split-index"},
		{errUnsafe, "unclassified"},
		{errDrift, "metadata-drift"},
		{context.Canceled, "unclassified"},
		{metadataProbeError(os.ErrPermission), "unclassified"},
		{errTooLarge, "metadata-limit"},
		{errRoot, "root-unresolved"},
		{errScratchInside, "scratch-dir"},
	}
	for _, test := range cases {
		if got := RefusalClass(test.err); got != test.want {
			t.Errorf("RefusalClass(%v)=%q want %q", test.err, got, test.want)
		}
	}
	for raw, want := range map[string]reasonClass{
		"include.path\n/x\x00":                   classConfigInclude,
		"filter.x.process\ncat\x00":              classGitFilter,
		"core.attributesfile\n/x\x00":            classAttributesFile,
		"extensions.refstorage\nreftable\x00":    classRefStorage,
		"extensions.refstorage\nother\x00":       classRefStorage,
		"core.bare\ntrue\x00":                    classWorktreeConfig,
		"core.worktree\n/elsewhere\x00":          classWorktreeConfig,
		"user.name\nfirst\nsecond\x00":           classConfigMalformed,
		"user.\x01name\nvalue\x00":               classConfigMalformed,
		"includeif.gitdir:/.path\n/x\x00":        classConfigInclude,
		"filter.hostile-driver.clean\ntouch\x00": classGitFilter,
	} {
		if class, _ := unsafeConfig([]byte(raw), "/repo", "/repo/.git"); class != want {
			t.Errorf("unsafeConfig(%q) class=%q want %q", raw, class, want)
		}
	}
}
