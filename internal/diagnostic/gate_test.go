package diagnostic

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const importPath = "github.com/Beamfall/corvint/internal/diagnostic"

// registry is docs/specs/FIX-REGISTRY.tsv read as its two DRC-V0-004 vocabularies.
type registry struct {
	kinds []string
	fixes map[string]bool
}

func parseRegistry(text string) (registry, error) {
	parsed := registry{fixes: map[string]bool{}}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "vocabulary\tidentifier\ttarget\tadmissible" {
		return parsed, fmt.Errorf("registry header is not vocabulary/identifier/target/admissible")
	}
	for number, line := range lines[1:] {
		if err := parsed.add(strings.Split(line, "\t")); err != nil {
			return parsed, fmt.Errorf("registry row %d: %w", number+2, err)
		}
	}
	return parsed, nil
}

func (parsed *registry) add(cells []string) error {
	if len(cells) != 4 || slices.Contains(cells, "") {
		return fmt.Errorf("want four non-empty cells, got %q", cells)
	}
	if slices.Contains(parsed.kinds, cells[1]) || parsed.fixes[cells[1]] {
		return fmt.Errorf("identifier %q is repeated", cells[1])
	}
	switch cells[0] {
	case "kind":
		parsed.kinds = append(parsed.kinds, cells[1])
	case "fix":
		parsed.fixes[cells[1]] = true
	default:
		return fmt.Errorf("vocabulary %q is neither kind nor fix", cells[0])
	}
	return nil
}

// specKinds extracts the backticked kind tokens of DRC-V0-001's closed vocabulary sentence.
func specKinds(spec string) []string {
	start := strings.Index(spec, "`kind` MUST be one of the closed vocabulary")
	if start < 0 {
		return nil
	}
	sentence := spec[start:]
	sentence = sentence[:strings.Index(sentence, ";")]
	tokens := regexp.MustCompile("`([a-z-]+)`").FindAllStringSubmatch(sentence, -1)
	kinds := []string{}
	for _, token := range tokens[1:] {
		kinds = append(kinds, token[1])
	}
	return kinds
}

// site is one covered DRC-V0-011 site: a diagnostic.Refusal composite literal.
type site struct {
	position string
	fixes    []string
}

type report struct {
	sites      []site
	violations []string
}

// scanSources applies the DRC-V0-011 and DRC-V0-012 checks to non-test Go sources keyed by
// repository-relative path.
func scanSources(sources map[string][]byte, known registry) report {
	var result report
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		result.scanFile(name, sources[name], known)
	}
	return result
}

func (result *report) scanFile(name string, source []byte, known registry) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, name, source, 0)
	if err != nil {
		result.violations = append(result.violations, fmt.Sprintf("%s: unparseable: %v", name, err))
		return
	}
	local := diagnosticImportName(parsed)
	if local == "" {
		return
	}
	if local == "." {
		result.violations = append(result.violations, name+": a dot import of internal/diagnostic hides refusal literals from the gate")
		return
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		position := files.Position(node.Pos()).String()
		if literal, ok := node.(*ast.CompositeLit); ok && isSelector(literal.Type, local, "Refusal") {
			result.scanRefusal(position, literal, known)
		}
		if problem := uninspectableRefusal(node, local); problem != "" {
			result.violations = append(result.violations, position+": "+problem)
		}
		if parse := messageParse(node); parse != "" {
			result.violations = append(result.violations, fmt.Sprintf("%s: DRC-V0-012 %s parses a refusal message", position, parse))
		}
		return true
	})
}

// uninspectableRefusal names a form that emits or builds a refusal the literal scan cannot
// check: a diagnostic.Error literal with no Refusal member, a Refusal collection literal whose
// elements elide their type, or a zero-valued Refusal declaration or new(diagnostic.Refusal).
func uninspectableRefusal(node ast.Node, local string) string {
	switch typed := node.(type) {
	case *ast.CompositeLit:
		if isSelector(typed.Type, local, "Error") && keyedFields(typed)["Refusal"] == nil {
			return "diagnostic.Error literal has no Refusal member"
		}
		if isSelector(collectionElement(typed.Type), local, "Refusal") {
			return "diagnostic.Refusal collection literal elides the refusal literal type"
		}
	case *ast.ValueSpec:
		if isSelector(typed.Type, local, "Refusal") && len(typed.Values) == 0 {
			return "zero-valued diagnostic.Refusal declaration"
		}
	case *ast.CallExpr:
		if ident, ok := typed.Fun.(*ast.Ident); ok && ident.Name == "new" && len(typed.Args) == 1 && isSelector(typed.Args[0], local, "Refusal") {
			return "new(diagnostic.Refusal) builds a zero-valued refusal"
		}
	}
	return ""
}

// collectionElement is the element type of an array, slice or map type, pointer elided.
func collectionElement(expression ast.Expr) ast.Expr {
	var element ast.Expr
	switch typed := expression.(type) {
	case *ast.ArrayType:
		element = typed.Elt
	case *ast.MapType:
		element = typed.Value
	}
	if star, ok := element.(*ast.StarExpr); ok {
		return star.X
	}
	return element
}

func diagnosticImportName(file *ast.File) string {
	for _, spec := range file.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); path != importPath {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "diagnostic"
	}
	return ""
}

func (result *report) scanRefusal(position string, literal *ast.CompositeLit, known registry) {
	fields := keyedFields(literal)
	covered := site{position: position}
	problems := []string{}
	problems = append(problems, subjectProblems(fields["Subject"])...)
	fixes, fixProblems := fixList(fields["SupportedFixes"], known)
	covered.fixes = fixes
	problems = append(problems, fixProblems...)
	problems = append(problems, evidenceProblems(fields["Evidence"])...)
	problems = append(problems, terminalProblems(fields, fixes)...)
	for _, problem := range problems {
		result.violations = append(result.violations, position+": "+problem)
	}
	if len(problems) == 0 {
		result.sites = append(result.sites, covered)
	}
}

func keyedFields(literal *ast.CompositeLit) map[string]ast.Expr {
	fields := map[string]ast.Expr{}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := pair.Key.(*ast.Ident); ok {
			fields[key.Name] = pair.Value
		}
	}
	return fields
}

func subjectProblems(expression ast.Expr) []string {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return []string{"refusal has no subject literal"}
	}
	fields := keyedFields(literal)
	kind, ok := stringLiteral(fields["Kind"])
	if !ok {
		return []string{"subject kind is not a string literal"}
	}
	if !slices.Contains(Kinds, kind) {
		return []string{fmt.Sprintf("subject kind %q is outside the DRC-V0-001 vocabulary", kind)}
	}
	if fields["Value"] == nil {
		return []string{"subject has no value"}
	}
	return nil
}

// fixList reads the SupportedFixes literal; an absent key is the empty list, which
// terminalProblems then requires a terminal literal for.
func fixList(expression ast.Expr, known registry) ([]string, []string) {
	if expression == nil {
		return []string{}, nil
	}
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, []string{"refusal has no supported_fixes literal"}
	}
	fixes := []string{}
	problems := []string{}
	for _, element := range literal.Elts {
		fix, ok := stringLiteral(element)
		if !ok {
			problems = append(problems, "fix identifier is not a string literal")
			continue
		}
		if slices.Contains(fixes, fix) {
			problems = append(problems, fmt.Sprintf("fix identifier %q is repeated", fix))
		}
		fixes = append(fixes, fix)
		if !known.fixes[fix] {
			problems = append(problems, fmt.Sprintf("fix identifier %q is absent from docs/specs/FIX-REGISTRY.tsv", fix))
		}
	}
	return fixes, problems
}

// evidenceProblems requires an evidence list literal whose every name is a string literal
// Validate admits: Bounded screens, escapes and cuts a name but never fills an empty one.
func evidenceProblems(expression ast.Expr) []string {
	if expression == nil {
		return nil
	}
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return []string{"refusal evidence is not a list literal"}
	}
	problems := []string{}
	for _, element := range literal.Elts {
		problems = append(problems, evidenceNameProblems(element)...)
	}
	return problems
}

func evidenceNameProblems(element ast.Expr) []string {
	pair, ok := element.(*ast.CompositeLit)
	if !ok {
		return []string{"evidence pair is not a literal"}
	}
	name, ok := stringLiteral(keyedFields(pair)["Name"])
	if !ok {
		return []string{"evidence name is not a string literal"}
	}
	if err := singleLine("evidence name", name); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func terminalProblems(fields map[string]ast.Expr, fixes []string) []string {
	terminal, hasTerminal := stringLiteral(fields["Terminal"])
	if len(fixes) != 0 && fields["Terminal"] != nil {
		return []string{"terminal is set beside a non-empty supported_fixes"}
	}
	if len(fixes) == 0 && (!hasTerminal || !slices.Contains(TerminalReasons, terminal)) {
		return []string{"empty supported_fixes has no DRC-V0-005 terminal literal"}
	}
	return nil
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func isSelector(expression ast.Expr, pkg, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

// messageParse names a strings/bytes/regexp call or an equality comparison over a .Message
// field or an .Error() result, the bounded DRC-V0-012 form.
func messageParse(node ast.Node) string {
	switch typed := node.(type) {
	case *ast.CallExpr:
		selector, ok := typed.Fun.(*ast.SelectorExpr)
		if !ok || !slices.ContainsFunc(typed.Args, isMessage) {
			return ""
		}
		if ident, ok := selector.X.(*ast.Ident); ok && slices.Contains([]string{"strings", "bytes", "regexp"}, ident.Name) {
			return ident.Name + "." + selector.Sel.Name
		}
	case *ast.BinaryExpr:
		if (typed.Op == token.EQL || typed.Op == token.NEQ) && (isMessage(typed.X) || isMessage(typed.Y)) {
			return "comparison"
		}
	}
	return ""
}

func isMessage(expression ast.Expr) bool {
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		return selector.Sel.Name == "Message"
	}
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Error"
}

// treeSources reads every non-test Go file under cmd/ and internal/ of the repository.
func treeSources(t *testing.T) map[string][]byte {
	t.Helper()
	root := filepath.Join("..", "..")
	sources := map[string][]byte{}
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(name)
			relative, _ := filepath.Rel(root, name)
			sources[filepath.ToSlash(relative)] = data
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return sources
}

func readRepositoryFile(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sortedCopy(values []string) []string {
	copied := slices.Clone(values)
	slices.Sort(copied)
	return copied
}

// TestFixIdentifiersResolveInRegistry checks DRC-V0-004: the registry parses, its kind rows,
// DRC-V0-001's clause and Kinds agree exactly, every identifier the tree emits resolves, and a
// planted unregistered identifier and a planted kind-row disagreement both fail.
func TestFixIdentifiersResolveInRegistry(t *testing.T) {
	registryText := readRepositoryFile(t, "docs/specs/FIX-REGISTRY.tsv")
	known, err := parseRegistry(registryText)
	if err != nil {
		t.Fatal(err)
	}
	clause := specKinds(readRepositoryFile(t, "docs/specs/diagnostic-repair-contract-v0.md"))
	if !kindsAgree(known.kinds, clause) {
		t.Fatalf("registry kinds %v, DRC-V0-001 kinds %v, Kinds %v disagree", known.kinds, clause, Kinds)
	}
	tree := scanSources(treeSources(t), known)
	if len(tree.violations) != 0 {
		t.Fatalf("tree violations:\n%s", strings.Join(tree.violations, "\n"))
	}

	planted := map[string][]byte{"internal/planted/planted.go": []byte(`package planted

import "github.com/Beamfall/corvint/internal/diagnostic"

var refusal = diagnostic.Refusal{Subject: diagnostic.Subject{Kind: "value", Value: "a"}, SupportedFixes: []string{"planted.unregistered-fix"}}
`)}
	if report := scanSources(planted, known); len(report.violations) != 1 || !strings.Contains(report.violations[0], "planted.unregistered-fix") {
		t.Fatalf("planted unregistered identifier: %v", report.violations)
	}
	disagreeing, err := parseRegistry(strings.Replace(registryText, "kind\trequest\t-\t-\n", "", 1))
	if err != nil {
		t.Fatal(err)
	}
	if kindsAgree(disagreeing.kinds, clause) {
		t.Fatal("a registry missing a kind row agreed with DRC-V0-001")
	}
	if kindsAgree(known.kinds, append(slices.Clone(clause), "path")) {
		t.Fatal("a clause with an extra kind agreed with the registry")
	}
	if _, err := parseRegistry(registryText + "fix\tworktree-impact.remove-path\tx\ty\n"); err == nil {
		t.Fatal("a repeated registry identifier parsed")
	}
}

func kindsAgree(registryKinds, clause []string) bool {
	return len(clause) != 0 && slices.Equal(sortedCopy(registryKinds), sortedCopy(clause)) && slices.Equal(sortedCopy(Kinds), sortedCopy(clause))
}

// ratchetStart is DRC-V0-011's floor: the seven unsupported-working-tree-impact-* sites.
const ratchetStart = 7

// ratchetProblem compares the recorded covered-site count with the measured one.
func ratchetProblem(recorded, measured int) string {
	if recorded < ratchetStart {
		return fmt.Sprintf("recorded count %d is below the DRC-V0-011 start of %d", recorded, ratchetStart)
	}
	if recorded != measured {
		return fmt.Sprintf("recorded count %d differs from the measured %d covered sites; update script/diagnostic-coverage.count in the converting change", recorded, measured)
	}
	return ""
}

// TestDiagnosticCoverageRatchet checks DRC-V0-011 and DRC-V0-012 over the tree: no violation,
// the measured covered-site count equals script/diagnostic-coverage.count and is at least seven,
// and a planted uncovered site, a start below seven, a lost covered site, a planted message
// parse, each refusal form the literal scan cannot inspect (an Error without a Refusal, an
// elided or zero-valued Refusal, a dot import), and each literal Validate would reject after
// Bounded (a repeated fix, an empty or non-literal evidence name) each fail.
func TestDiagnosticCoverageRatchet(t *testing.T) {
	known, err := parseRegistry(readRepositoryFile(t, "docs/specs/FIX-REGISTRY.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := strconv.Atoi(strings.TrimSpace(readRepositoryFile(t, "script/diagnostic-coverage.count")))
	if err != nil {
		t.Fatal(err)
	}
	sources := treeSources(t)
	tree := scanSources(sources, known)
	if len(tree.violations) != 0 {
		t.Fatalf("tree violations:\n%s", strings.Join(tree.violations, "\n"))
	}
	if problem := ratchetProblem(recorded, len(tree.sites)); problem != "" {
		t.Fatal(problem)
	}
	t.Logf("covered diagnostic sites: %d", len(tree.sites))

	if ratchetProblem(ratchetStart-1, ratchetStart-1) == "" {
		t.Fatal("a ratchet started below seven passed")
	}
	delete(sources, "internal/worktreeimpact/diagnostics.go")
	if ratchetProblem(recorded, len(scanSources(sources, known).sites)) == "" {
		t.Fatal("losing the converted family's sites passed the ratchet")
	}
	for name, planted := range map[string]string{
		"uncovered site":        `var refusal = diagnostic.Refusal{Evidence: []diagnostic.Evidence{{Name: "a", Value: "b"}}}`,
		"message parse":         `func parse(err *diagnostic.Error) bool { return strings.Contains(err.Error(), "captured") }`,
		"error without refusal": `var failure error = &diagnostic.Error{Err: nil}`,
		"elided refusal":        `var refusals = []diagnostic.Refusal{{Subject: diagnostic.Subject{Kind: "value", Value: "v"}, Terminal: "absent-evidence"}}`,
		"zero refusal":          `var zero diagnostic.Refusal`,
		"new refusal":           `var zero = new(diagnostic.Refusal)`,
		"repeated fix":          `var refusal = diagnostic.Refusal{Subject: diagnostic.Subject{Kind: "value", Value: "v"}, SupportedFixes: []string{"worktree-impact.remove-path", "worktree-impact.remove-path"}}`,
		"empty evidence name":   `var refusal = diagnostic.Refusal{Subject: diagnostic.Subject{Kind: "value", Value: "v"}, Evidence: []diagnostic.Evidence{{Name: "", Value: "b"}}, Terminal: "absent-evidence"}`,
		"variable evidence name": `var name = strings.ToLower("")
var refusal = diagnostic.Refusal{Subject: diagnostic.Subject{Kind: "value", Value: "v"}, Evidence: []diagnostic.Evidence{{Name: name, Value: "b"}}, Terminal: "absent-evidence"}`,
	} {
		source := "package planted\n\nimport (\n\t\"strings\"\n\n\t\"github.com/Beamfall/corvint/internal/diagnostic\"\n)\n\nvar _ = strings.Contains\n\n" + planted + "\n"
		report := scanSources(map[string][]byte{"internal/planted/planted.go": []byte(source)}, known)
		if len(report.violations) == 0 {
			t.Fatalf("planted %s passed the gate", name)
		}
	}
	dotImport := "package planted\n\nimport . \"github.com/Beamfall/corvint/internal/diagnostic\"\n\nvar refusal = Refusal{}\n"
	if report := scanSources(map[string][]byte{"internal/planted/planted.go": []byte(dotImport)}, known); len(report.violations) == 0 {
		t.Fatal("planted dot import passed the gate")
	}
}
