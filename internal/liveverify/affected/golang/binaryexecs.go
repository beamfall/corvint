package golang

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file records the command edges of AFP-V0-037: a package that runs the
// built binary of a command (a package main in the repository) depends on the
// command's whole build, though no import edge says so. An edge comes from a
// path literal of the package that names the command's directory, as in
// `go build ./cmd/corvint`, or from the project-owned declaration
// affected.BinaryExecsPath for a package that runs a binary it is handed.

// FrontierBinaryExecsInvalid reports a command-edge declaration that is present
// but unreadable or invalid. No declared edge is then kept, so a plan may miss
// a declared runner and widens instead.
const FrontierBinaryExecsInvalid = "go:test-binary-execs-invalid"

// BinaryExecsProfile is the declaration's closed profile name.
const BinaryExecsProfile = "corvint-test-binary-execs/0"

const (
	maxBinaryExecBytes    = 1 << 20
	maxBinaryExecPackages = 4096
	maxBinaryExecEntries  = 256
)

type binaryExecsFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// applyBinaryExecs sets each unit's command edges, given each unit's
// directory and module and the command unit in each command directory: the
// literal edges always, and the declared edges unless the declaration is
// unreadable or invalid, which widens every plan instead.
func applyBinaryExecs(root *affected.Source, units []affected.Unit, directories []string, owners map[string]module, commands map[string]string, frontier map[string]bool) {
	edges := make([]map[string]bool, len(units))
	for index := range units {
		edges[index] = literalExecs(root, units[index], directories[index], owners[directories[index]], commands)
	}
	declared, err := readBinaryExecs(root)
	var declaredEdges map[int][]string
	if err == nil {
		declaredEdges, err = matchBinaryExecs(declared, directories, commands)
	}
	if err != nil {
		frontier[FrontierBinaryExecsInvalid] = true
	}
	for index := range units {
		for _, id := range declaredEdges[index] {
			edges[index][id] = true
		}
		if len(edges[index]) != 0 {
			units[index].Execs = sortedKeys(edges[index])
		}
	}
}

// literalExecs names the commands, other than the unit itself, whose directory
// one of the unit's path tokens names exactly, in a literal that some file of
// the unit uses in a position that can reach a build or a process
// (AFP-V0-402). Only files that name a command are parsed in full.
func literalExecs(root *affected.Source, unit affected.Unit, directory string, owner module, commands map[string]string) map[string]bool {
	candidates := map[string]string{}
	for _, value := range unit.PathTokens {
		target, ok := commandDirectory(value, directory, owner.dir)
		if !ok {
			continue
		}
		if id, command := commands[target]; command && id != unit.ID {
			candidates[target] = id
		}
	}
	edges := map[string]bool{}
	if len(candidates) == 0 {
		return edges
	}
	for _, relative := range slices.Concat(unit.Sources, unit.Tests) {
		for target := range usedCommandLiterals(root, relative, directory, owner, candidates) {
			edges[candidates[target]] = true
			delete(candidates, target)
		}
		if len(candidates) == 0 {
			break
		}
	}
	return edges
}

// usedCommandLiterals returns the candidate command directories that a string
// literal of one file names outside a data-only position (dataOnly). A file
// that cannot be read or parsed in full names every candidate, so no edge is
// lost to an unparsed file.
func usedCommandLiterals(root *affected.Source, relative, directory string, owner module, candidates map[string]string) map[string]bool {
	used := map[string]bool{}
	body, err := root.Read(relative)
	var file *ast.File
	if err == nil {
		file, err = parser.ParseFile(token.NewFileSet(), relative, body, parser.SkipObjectResolution)
	}
	if err != nil {
		for target := range candidates {
			used[target] = true
		}
		return used
	}
	names := unshadowedImports(file)
	var parents []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			parents = parents[:len(parents)-1]
			return false
		}
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && !dataOnly(parents, literal, names) {
			if value, err := strconv.Unquote(literal.Value); err == nil {
				for _, name := range literalTokens(value, owner.path) {
					if target, ok := commandDirectory(name, directory, owner.dir); ok && candidates[target] != "" {
						used[target] = true
					}
				}
			}
		}
		parents = append(parents, node)
		return true
	})
	return used
}

// dataPredicates are the standard-library functions whose string arguments
// are only compared, never built or run (AFP-V0-402): every argument of one
// that returns a bool or an index, and only the cut operand, the second
// argument, of one that returns the remainder of its first. A constructor of
// an error or a message (`errors.New`, `fmt.Errorf`) is not one, because its
// result carries the literal on.
var dataPredicates = map[string]bool{
	"strings.HasPrefix": true, "strings.HasSuffix": true, "strings.Contains": true,
	"strings.EqualFold": true, "strings.Index": true, "strings.LastIndex": true,
	"bytes.HasPrefix": true, "bytes.HasSuffix": true, "bytes.Contains": true,
	"path.Match": true, "path/filepath.Match": true,
}

// cutPredicates return their first argument's text, so only the second is data.
var cutPredicates = map[string]bool{
	"strings.CutPrefix": true, "strings.CutSuffix": true, "strings.TrimPrefix": true, "strings.TrimSuffix": true,
}

// testMessages are the methods of the testing package's T, B, F and TB whose
// arguments only form a test message.
var testMessages = map[string]bool{
	"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true,
	"Log": true, "Logf": true, "Skip": true, "Skipf": true,
}

// dataOnly reports a literal used only as data (AFP-V0-402): an operand of ==
// or !=, a switch case, the index of a map or slice read (not one assigned
// to, whose key may be iterated later), an argument of a dataPredicates
// function, the second argument of a cutPredicates function, or an argument
// of a testMessages method called on a parameter declared as a testing T, B,
// F or TB (testingReceiver). A package qualifier counts only through names,
// the file's imports that no declaration of the file shadows. Every other
// position, a variable initializer, a helper's argument and a method of any
// other receiver among them, may reach a build or a process and keeps the
// edge.
func dataOnly(parents []ast.Node, literal ast.Expr, names map[string]string) bool {
	var child ast.Node = literal
	for index := len(parents) - 1; index >= 0; index-- {
		switch parent := parents[index].(type) {
		case *ast.ParenExpr:
			child = parent
			continue
		case *ast.BinaryExpr:
			return parent.Op == token.EQL || parent.Op == token.NEQ
		case *ast.CaseClause:
			return true
		case *ast.IndexExpr:
			return parent.Index == child && !assigned(parents[:index], parent)
		case *ast.CallExpr:
			if parent.Fun == child {
				return false
			}
			selector, ok := parent.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return false
			}
			if names[qualifier.Name] != "" {
				name := names[qualifier.Name] + "." + selector.Sel.Name
				return dataPredicates[name] || (cutPredicates[name] && len(parent.Args) > 1 && parent.Args[1] == child)
			}
			return testMessages[selector.Sel.Name] && testingReceiver(parents[:index], qualifier.Name, names)
		}
		return false
	}
	return false
}

// testingReceiver reports a name that the innermost enclosing function
// declaring it declares as a parameter (or receiver) of type *testing.T,
// *testing.B, *testing.F or testing.TB, through the file's unshadowed import
// of "testing", and that no declaration in that function's body redeclares.
// A name declared nowhere in an enclosing function is not proven.
func testingReceiver(parents []ast.Node, name string, names map[string]string) bool {
	testing := ""
	for local, value := range names {
		if value == "testing" && local != "_" && local != "." {
			testing = local
		}
	}
	if testing == "" {
		return false
	}
	for index := len(parents) - 1; index >= 0; index-- {
		var fields []*ast.Field
		var body *ast.BlockStmt
		switch function := parents[index].(type) {
		case *ast.FuncLit:
			fields, body = signatureFields(nil, function.Type), function.Body
		case *ast.FuncDecl:
			fields, body = signatureFields(function.Recv, function.Type), function.Body
		default:
			continue
		}
		for _, field := range fields {
			if !slices.ContainsFunc(field.Names, func(ident *ast.Ident) bool { return ident.Name == name }) {
				continue
			}
			return testingType(field.Type, testing) && !redeclares(body, name)
		}
	}
	return false
}

func signatureFields(receiver *ast.FieldList, signature *ast.FuncType) []*ast.Field {
	var fields []*ast.Field
	for _, list := range []*ast.FieldList{receiver, signature.TypeParams, signature.Params, signature.Results} {
		if list != nil {
			fields = append(fields, list.List...)
		}
	}
	return fields
}

// testingType reports *testing.T, *testing.B, *testing.F or testing.TB.
func testingType(expression ast.Expr, testing string) bool {
	if star, ok := expression.(*ast.StarExpr); ok {
		return qualifiedName(star.X, testing, "T", "B", "F")
	}
	return qualifiedName(expression, testing, "TB")
}

func qualifiedName(expression ast.Expr, qualifier string, names ...string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == qualifier && slices.Contains(names, selector.Sel.Name)
}

// redeclares reports a declaration of name anywhere in node: a short variable
// declaration, a var or const, a range variable, a function literal's
// parameter or result, or a type, so a call on that name may not reach the
// declaration it is checked against.
func redeclares(node ast.Node, name string) bool {
	if node == nil {
		return false
	}
	return slices.Contains(declaredNames(node), name)
}

// declaredNames lists every name a declaration within node introduces, struct
// fields included (an over-approximation that only keeps edges).
func declaredNames(node ast.Node) []string {
	var declared []string
	add := func(idents ...*ast.Ident) {
		for _, ident := range idents {
			if ident != nil {
				declared = append(declared, ident.Name)
			}
		}
	}
	ast.Inspect(node, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, expression := range node.Lhs {
					if ident, ok := expression.(*ast.Ident); ok {
						add(ident)
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				for _, expression := range []ast.Expr{node.Key, node.Value} {
					if ident, ok := expression.(*ast.Ident); ok {
						add(ident)
					}
				}
			}
		case *ast.ValueSpec:
			add(node.Names...)
		case *ast.TypeSpec:
			add(node.Name)
		case *ast.Field:
			add(node.Names...)
		case *ast.FuncDecl:
			add(node.Name)
		}
		return true
	})
	return declared
}

// unshadowedImports maps each local package name of a file's imports to its
// path, leaving out a name that a declaration of the file also introduces, so
// that a qualifier is resolved only when it can name the import alone.
func unshadowedImports(file *ast.File) map[string]string {
	names := importNames(file.Imports)
	for _, name := range declaredNames(file) {
		delete(names, name)
	}
	return names
}

// assigned reports an index expression that is assigned or incremented,
// through any parentheses; its parent is the last of parents.
func assigned(parents []ast.Node, index *ast.IndexExpr) bool {
	var child ast.Expr = index
	for position := len(parents) - 1; position >= 0; position-- {
		switch statement := parents[position].(type) {
		case *ast.ParenExpr:
			child = statement
			continue
		case *ast.AssignStmt:
			return slices.Contains(statement.Lhs, child)
		case *ast.IncDecStmt:
			return statement.X == child
		}
		return false
	}
	return false
}

// importNames maps each local package name of a file's imports to its path:
// the name the import gives it, else the last path element that is not a
// major version suffix (`math/rand/v2` is `rand`).
func importNames(imports []*ast.ImportSpec) map[string]string {
	names := map[string]string{}
	for _, spec := range imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(value)
		if majorSuffix.MatchString(name) && path.Dir(value) != "." {
			name = path.Base(path.Dir(value))
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = value
	}
	return names
}

// majorSuffix is a module major version path element.
var majorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// commandDirectory resolves a path token to the repository-relative directory
// it names: an anchored token or a plain one against the module's directory
// (`go build ./cmd/x` runs there), a climbing one against the unit's own
// directory. A token naming no directory below the module root names none.
func commandDirectory(value, directory, moduleDir string) (string, bool) {
	var target string
	switch {
	case strings.HasPrefix(value, "/"):
		target = path.Join(moduleDir, value[1:])
	case value == ".." || strings.HasPrefix(value, "../"):
		target = path.Join(directory, value)
	default:
		target = path.Join(moduleDir, value)
	}
	if target == "." || target == moduleDir || target == ".." || strings.HasPrefix(target, "../") {
		return "", false
	}
	return target, true
}

// readBinaryExecs returns the declared command directories by package
// directory: nil with no declaration, an error for one that is unreadable,
// oversized, not a regular file, or invalid in any member.
func readBinaryExecs(root *affected.Source) (map[string][]string, error) {
	body, err := root.ReadBounded(affected.BinaryExecsPath, maxBinaryExecBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared binaryExecsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != BinaryExecsProfile || declared.Packages == nil || len(declared.Packages) > maxBinaryExecPackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", affected.BinaryExecsPath, BinaryExecsProfile, maxBinaryExecPackages)
	}
	for directory, entries := range declared.Packages {
		if !affected.ValidRelativePath(directory) || len(entries) == 0 || len(entries) > maxBinaryExecEntries {
			return nil, fmt.Errorf("%s: package %q", affected.BinaryExecsPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !affected.ValidRelativePath(entry) || entry <= previous || entry == directory {
				return nil, fmt.Errorf("%s: package %q entry %q", affected.BinaryExecsPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// matchBinaryExecs returns the declared command units by unit index. Every
// declared directory must hold an observed package and every entry an observed
// command; otherwise no declared edge is kept.
func matchBinaryExecs(declared map[string][]string, directories []string, commands map[string]string) (map[int][]string, error) {
	edges := make(map[int][]string, len(declared))
	for index, directory := range directories {
		entries, ok := declared[directory]
		if !ok {
			continue
		}
		for _, entry := range entries {
			id, command := commands[entry]
			if !command {
				return nil, fmt.Errorf("%s: package %q entry %q holds no command", affected.BinaryExecsPath, directory, entry)
			}
			edges[index] = append(edges[index], id)
		}
	}
	if len(edges) != len(declared) {
		return nil, fmt.Errorf("%s declares a directory that holds no Go package", affected.BinaryExecsPath)
	}
	return edges, nil
}
