package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// sessiontestImport is the package that owns the corvint-test-run/0 receipt.
const sessiontestImport = "github.com/Beamfall/corvint/internal/sessiontest"

// receiptSymbols are the sessiontest names through which Go code can hold,
// decode or judge a receipt.
var receiptSymbols = map[string]bool{"ReadReceipt": true, "Receipt": true, "ReceiptProfile": true, "Verify": true}

// receiptLiteral matches the receipt profile and its default file name, the
// way code outside Go would find or recognise a receipt.
var receiptLiteral = regexp.MustCompile(`corvint-test-run/0|test-run[.\[]`)

// receiptScanSkip are directories that hold no product code: prose,
// retained evidence, state stores and fixtures.
var receiptScanSkip = map[string]bool{".git": true, ".corvint": true, ".taskman": true, "docs": true, "evidence": true, "node_modules": true, "testdata": true, "vendor": true}

// receiptScanExtensions are the non-Go files that can read a file. An
// extensionless file is scanned when it starts with a `#!` interpreter line.
var receiptScanExtensions = map[string]bool{".bash": true, ".cjs": true, ".jq": true, ".js": true, ".mjs": true, ".mk": true, ".py": true, ".rb": true, ".sh": true, ".ts": true, ".yaml": true, ".yml": true}

// receiptSites returns one "path decl symbol" site for every place the file
// at rel (slash-separated) names a receipt symbol or literal. Test files
// are not product readers and yield nothing.
func receiptSites(t *testing.T, rel string, source []byte) []string {
	t.Helper()
	base := path.Base(rel)
	if strings.HasSuffix(base, "_test.go") || strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return nil
	}
	if !strings.HasSuffix(rel, ".go") {
		if receiptLiteral.Match(source) {
			return []string{rel + " - literal"}
		}
		return nil
	}
	// The bare path matches a double-quoted or a raw-string import.
	if !receiptLiteral.Match(source) && !strings.Contains(string(source), sessiontestImport) && path.Dir(rel) != "internal/sessiontest" {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), rel, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	inPackage := path.Dir(rel) == "internal/sessiontest"
	qualifier := ""
	var sites []string
	for _, spec := range file.Imports {
		if value, _ := strconv.Unquote(spec.Path.Value); value == sessiontestImport {
			qualifier = "sessiontest"
			if spec.Name != nil {
				qualifier = spec.Name.Name
			}
			if qualifier == "." {
				sites = append(sites, rel+" - dot-import")
			}
		}
	}
	for _, unit := range declUnits(file) {
		name, declared := declName(unit)
		ast.Inspect(unit, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.ImportSpec:
				return false
			case *ast.Field:
				// Field and parameter names are definitions, not uses.
				for _, ident := range node.Names {
					declared[ident] = true
				}
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok {
					declared[key] = true
				}
			case *ast.BasicLit:
				if node.Kind == token.STRING && receiptLiteral.MatchString(node.Value) {
					sites = append(sites, rel+" "+name+" literal")
				}
			case *ast.SelectorExpr:
				if x, ok := node.X.(*ast.Ident); ok && qualifier != "" && x.Name == qualifier && receiptSymbols[node.Sel.Name] {
					sites = append(sites, rel+" "+name+" "+node.Sel.Name)
				}
				if inPackage {
					// A field or method selector is not the package-level name.
					ast.Inspect(node.X, func(inner ast.Node) bool {
						if ident, ok := inner.(*ast.Ident); ok && receiptSymbols[ident.Name] {
							sites = append(sites, rel+" "+name+" "+ident.Name)
						}
						return true
					})
					return false
				}
			case *ast.Ident:
				if inPackage && receiptSymbols[node.Name] && !declared[node] {
					sites = append(sites, rel+" "+name+" "+node.Name)
				}
			}
			return true
		})
	}
	return sites
}

// receiptReaderFunc is the one product function that reads receipts.
const receiptReaderFunc = "runTestVerifyShards"

// indirectReceiptSites returns the import-graph and call-graph sites of a
// product Go file (V1-2025): "dir importer" for a package outside
// internal/sessiontest that imports it, which could reach a receipt through
// reflection or an unnamed value rather than a receipt symbol, and
// "path decl reference" for every use of the reader function, which could
// hand verify-shards' decoded result to another consumer.
func indirectReceiptSites(t *testing.T, rel string, source []byte) []string {
	t.Helper()
	base := path.Base(rel)
	if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(base, "_test.go") {
		return nil
	}
	if !strings.Contains(string(source), sessiontestImport) && !strings.Contains(string(source), receiptReaderFunc) {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), rel, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var sites []string
	for _, spec := range file.Imports {
		if value, _ := strconv.Unquote(spec.Path.Value); value == sessiontestImport && path.Dir(rel) != "internal/sessiontest" {
			sites = append(sites, path.Dir(rel)+" importer")
		}
	}
	for _, unit := range declUnits(file) {
		name, _ := declName(unit)
		ast.Inspect(unit, func(node ast.Node) bool {
			if fn, ok := node.(*ast.FuncDecl); ok {
				// The declaration's own name is not a use.
				if fn.Body != nil {
					ast.Inspect(fn.Body, func(inner ast.Node) bool {
						if ident, ok := inner.(*ast.Ident); ok && ident.Name == receiptReaderFunc {
							sites = append(sites, rel+" "+name+" reference")
						}
						return true
					})
				}
				return false
			}
			if ident, ok := node.(*ast.Ident); ok && ident.Name == receiptReaderFunc {
				sites = append(sites, rel+" "+name+" reference")
			}
			return true
		})
	}
	return sites
}

// normalizeIndirectSites sorts the sites and collapses only the per-file
// repeats of a package importer. Every reference stays its own entry, so a
// second use of the reader inside an already-listed declaration still
// changes the set (V1-2025).
func normalizeIndirectSites(sites []string) []string {
	out := slices.Clone(sites)
	slices.Sort(out)
	return slices.CompactFunc(out, func(a, b string) bool {
		return a == b && strings.HasSuffix(a, " importer")
	})
}

// declUnits splits a file into its functions and the single specs of its
// general declarations, so a site names the function, type, constant or
// variable that holds it.
func declUnits(file *ast.File) []ast.Node {
	var units []ast.Node
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				units = append(units, spec)
			}
		default:
			units = append(units, decl)
		}
	}
	return units
}

// declName names one declaration unit and returns the identifiers it
// declares, which are definitions rather than uses.
func declName(unit ast.Node) (string, map[*ast.Ident]bool) {
	declared := map[*ast.Ident]bool{}
	switch unit := unit.(type) {
	case *ast.FuncDecl:
		declared[unit.Name] = true
		if unit.Recv != nil && len(unit.Recv.List) == 1 {
			receiver := unit.Recv.List[0].Type
			if star, ok := receiver.(*ast.StarExpr); ok {
				receiver = star.X
			}
			if ident, ok := receiver.(*ast.Ident); ok {
				return ident.Name + "." + unit.Name.Name, declared
			}
		}
		return unit.Name.Name, declared
	case *ast.TypeSpec:
		declared[unit.Name] = true
		return unit.Name.Name, declared
	case *ast.ValueSpec:
		var names []string
		for _, ident := range unit.Names {
			declared[ident] = true
			names = append(names, ident.Name)
		}
		return strings.Join(names, ","), declared
	}
	return "-", declared
}

// STR-V0-011: receipts are evidence only. The one product reader is
// `corvint test verify-shards`; everything else that names a receipt writes
// it. The test fixes the exact set of receipt sites in the repository's
// product code (Go outside tests, and scripts, workflows and Makefiles by the
// receipt's profile and file name), so a second reader, which could feed
// ranking, selection, learning or authority, fails it until a reviewed
// requirement admits it.
func TestSTRV0011ReceiptsHaveNoOtherReader(t *testing.T) {
	// The spy must see each kind of second reader.
	for rel, source := range map[string]string{
		"internal/rank/rank.go":       "package rank\nimport st \"" + sessiontestImport + "\"\nfunc Weight(n string) { r, _ := st.ReadReceipt(n); _ = r }\n",
		"internal/rank/decode.go":     "package rank\nimport \"" + sessiontestImport + "\"\nvar last sessiontest.Receipt\n",
		"internal/rank/dot.go":        "package rank\nimport . \"" + sessiontestImport + "\"\n",
		"internal/rank/raw_import.go": "package rank\nimport st `" + sessiontestImport + "`\nfunc Weight(n string) { _, _ = st.ReadReceipt(n) }\n",
		"script/learn":                "#!/bin/sh\njq .result \"$GIT_DIR/corvint/test-run.json\"\n",
		"internal/rank/raw.go":        "package rank\nconst receipt = \".git/corvint/test-run.json\"\n",
		"internal/sessiontest/x.go":   "package sessiontest\nfunc widen() { _, _ = ReadReceipt(\"r\") }\n",
		"script/learn.sh":             "jq .result \"$GIT_DIR/corvint/test-run.0.json\"\n",
		".github/workflows/learn.yml": "run: grep corvint-test-run/0 receipts\n",
	} {
		if len(receiptSites(t, rel, []byte(source))) == 0 {
			t.Fatalf("SPY_INEFFECTIVE: %s names a receipt but yields no site", rel)
		}
	}
	for rel, source := range map[string]string{
		"internal/rank/rank_test.go":         "package rank\nimport \"" + sessiontestImport + "\"\nvar r sessiontest.Receipt\n",
		"internal/rank/runner.go":            "package rank\nconst tool = \"corvint-test-runner-receipt/0\"\n",
		"internal/sessiontest/field.go":      "package sessiontest\nfunc f(x struct{ Receipt int }) int { return x.Receipt }\n",
		"internal/sessiontest/definition.go": "package sessiontest\nfunc Verify() {}\n",
	} {
		if sites := receiptSites(t, rel, []byte(source)); len(sites) != 0 {
			t.Fatalf("SPY_FALSE_POSITIVE: %s yields %v", rel, sites)
		}
	}

	// V1-2025: the import-graph and call-graph spies.
	for rel, source := range map[string]string{
		"internal/rank/reflect.go":  "package rank\nimport (\"reflect\"; st \"" + sessiontestImport + "\")\nvar T = reflect.TypeOf(st.Options{})\n",
		"cmd/corvint/learn.go":      "package main\nfunc learn() { _ = " + receiptReaderFunc + "(nil, nil, nil) }\n",
		"cmd/corvint/learn_hook.go": "package main\nvar hook = " + receiptReaderFunc + "\n",
	} {
		if len(indirectReceiptSites(t, rel, []byte(source))) == 0 {
			t.Fatalf("SPY_INEFFECTIVE: %s reaches receipts indirectly but yields no site", rel)
		}
	}
	// Two references in one declaration are two sites, and normalization
	// keeps both, so a second handoff inside runTest is not hidden.
	twice := indirectReceiptSites(t, "cmd/corvint/test.go", []byte("package main\nfunc runTest() { _ = "+receiptReaderFunc+"(nil, nil, nil); hook := "+receiptReaderFunc+"; _ = hook }\n"))
	if got := normalizeIndirectSites(twice); len(got) != 2 || got[0] != got[1] {
		t.Fatalf("SPY_INEFFECTIVE: two references in one declaration normalize to %q", got)
	}
	if got := normalizeIndirectSites([]string{"cmd/corvint importer", "cmd/corvint importer"}); len(got) != 1 {
		t.Fatalf("SPY_FALSE_POSITIVE: per-file importer repeats normalize to %q", got)
	}
	for rel, source := range map[string]string{
		"internal/sessiontest/own.go": "package sessiontest\nimport \"" + sessiontestImport + "\"\n",
		"cmd/corvint/decl.go":         "package main\nfunc " + receiptReaderFunc + "() {}\n",
		"cmd/corvint/learn_test.go":   "package main\nfunc learn() { _ = " + receiptReaderFunc + "(nil, nil, nil) }\n",
	} {
		if sites := indirectReceiptSites(t, rel, []byte(source)); len(sites) != 0 {
			t.Fatalf("SPY_FALSE_POSITIVE: %s yields %v", rel, sites)
		}
	}

	root := moduleRoot(t)
	var sites, indirect []string
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name != root && (receiptScanSkip[entry.Name()] || (strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(name)
		if !entry.Type().IsRegular() || (ext != ".go" && ext != "" && !receiptScanExtensions[ext]) {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if ext == "" && entry.Name() != "Makefile" && !bytes.HasPrefix(source, []byte("#!")) {
			return nil
		}
		sites = append(sites, receiptSites(t, filepath.ToSlash(rel), source)...)
		indirect = append(indirect, indirectReceiptSites(t, filepath.ToSlash(rel), source)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(sites)
	sites = slices.Compact(sites)
	want := []string{
		// The one reader: verify-shards decodes the receipts and judges them.
		"cmd/corvint/test.go runTestVerifyShards ReadReceipt",
		"cmd/corvint/test.go runTestVerifyShards Receipt",
		"cmd/corvint/test.go runTestVerifyShards Verify",
		// Writers: the run that produces a receipt, its default path, the
		// output-path refusal that protects it, and the help text.
		"cmd/corvint/test.go testCheckOutputPath literal",
		"cmd/corvint/test.go testDefaultReceiptPath literal",
		"cmd/corvint/test.go testExecute Receipt",
		"cmd/corvint/test.go testHelp literal",
		"cmd/corvint/test.go testWriteReceipt Receipt",
		// The owning package: the profile, the strict decoder verify-shards
		// calls, Verify itself, and the shard execution that builds a receipt.
		"internal/sessiontest/adapter.go refuseUnmanifested Receipt",
		"internal/sessiontest/manifest.go ReadReceipt Receipt",
		"internal/sessiontest/manifest.go ReadReceipt ReceiptProfile",
		"internal/sessiontest/manifest.go ReceiptProfile literal",
		"internal/sessiontest/run.go Finish Receipt",
		"internal/sessiontest/run.go NewReceipt Receipt",
		"internal/sessiontest/run.go NewReceipt ReceiptProfile",
		"internal/sessiontest/run.go Runner.Execute Receipt",
		"internal/sessiontest/verify.go Verify Receipt",
	}
	var extra, missing []string
	for _, site := range sites {
		if !slices.Contains(want, site) {
			extra = append(extra, site)
		}
	}
	for _, site := range want {
		if !slices.Contains(sites, site) {
			missing = append(missing, site)
		}
	}
	if len(extra)+len(missing) != 0 {
		t.Fatalf("STR-V0-011 receipt sites changed; a receipt reader other than verify-shards needs its own accepted requirement first.\nnew sites: %q\ngone sites: %q", extra, missing)
	}
	indirect = normalizeIndirectSites(indirect)
	wantIndirect := []string{
		// The reader and its one dispatch.
		"cmd/corvint importer",
		"cmd/corvint/test.go runTest reference",
		// Tools that read sessiontest's command declarations, not receipts.
		"tools/test-commands-check importer",
		"tools/test-commands-run importer",
	}
	if !slices.Equal(indirect, wantIndirect) {
		t.Fatalf("STR-V0-011 indirect receipt reach changed (V1-2025): a new sessiontest importer or a second use of %s needs review that it reads no receipt.\ngot  %q\nwant %q", receiptReaderFunc, indirect, wantIndirect)
	}
}
