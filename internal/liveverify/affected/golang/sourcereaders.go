// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

// inertImports are the standard packages whose whole API, given the values
// a package can build without the classified packages below, can neither
// read a file or directory, start a process, nor observe build or source
// metadata (AFP-V0-9010). A standard import in neither this set nor
// classifiedImports, and every import outside the standard library, is
// unclassified and makes its file observing: the classifier is a closed
// allowlist and fails closed.
var inertImports = map[string]bool{
	"archive/tar": true, "bufio": true, "bytes": true, "cmp": true, "compress/bzip2": true,
	"compress/flate": true, "compress/gzip": true, "compress/lzw": true, "compress/zlib": true,
	"container/heap": true, "container/list": true, "container/ring": true, "context": true,
	"crypto/ed25519": true, "crypto/hmac": true, "crypto/md5": true, "crypto/rand": true,
	"crypto/sha1": true, "crypto/sha256": true, "crypto/sha3": true, "crypto/sha512": true,
	"crypto/subtle": true, "encoding": true, "encoding/ascii85": true, "encoding/asn1": true,
	"encoding/base32": true, "encoding/base64": true, "encoding/binary": true, "encoding/csv": true,
	"encoding/gob": true, "encoding/hex": true, "encoding/json": true, "encoding/pem": true,
	"encoding/xml": true, "errors": true, "flag": true, "fmt": true, "go/ast": true,
	"go/build/constraint": true, "go/constant": true, "go/format": true, "go/printer": true,
	"go/token": true, "go/version": true, "hash": true, "hash/adler32": true, "hash/crc32": true,
	"hash/crc64": true, "hash/fnv": true, "hash/maphash": true, "html": true, "io": true,
	"io/fs": true, "iter": true, "log": true, "log/slog": true, "maps": true, "math": true,
	"math/big": true, "math/bits": true, "math/cmplx": true, "math/rand": true, "math/rand/v2": true,
	"mime": true, "net/url": true, "os/signal": true, "path": true, "regexp": true,
	"regexp/syntax": true, "runtime": true, "slices": true, "sort": true, "strconv": true,
	"strings": true, "structs": true, "sync": true, "sync/atomic": true, "testing": true,
	"testing/fstest": true, "testing/iotest": true, "testing/quick": true, "testing/synctest": true,
	"text/scanner": true, "text/tabwriter": true, "time": true, "unicode": true,
	"unicode/utf16": true, "unicode/utf8": true, "unique": true, "weak": true,
}

// testdataMember is a classified member allowed only in a call whose first
// argument names the package's own testdata directory (ownTestdata).
const testdataMember = 2

// classifiedImports are the standard packages whose members are checked one
// by one: a member listed 1 is allowed in any position, a testdataMember only
// as a call that reads below the package's own testdata directory, and any
// other member makes its file observing. os/exec is listed with no members:
// its only allowed references are the `go list` calls goListScanFile proves,
// passed to observesSource by offset.
var classifiedImports = map[string]map[string]int{
	"os": {
		"ReadFile": testdataMember, "Open": testdataMember, "ReadDir": testdataMember,
		"Stat": testdataMember, "Lstat": testdataMember,
		"Args": 1, "Chmod": 1, "Chtimes": 1, "CopyFS": 1, "Create": 1, "CreateTemp": 1, "DevNull": 1,
		"DirEntry": 1, "Environ": 1, "ErrClosed": 1, "ErrDeadlineExceeded": 1, "ErrExist": 1,
		"ErrNotExist": 1, "ErrPermission": 1, "Executable": 1, "Exit": 1, "Expand": 1, "ExpandEnv": 1,
		"File": 1, "FileInfo": 1, "FileMode": 1, "Getegid": 1, "Getenv": 1, "Geteuid": 1, "Getgid": 1,
		"Getpid": 1, "Getppid": 1, "Getuid": 1, "Getwd": 1, "Hostname": 1, "Interrupt": 1,
		"IsExist": 1, "IsNotExist": 1, "IsPermission": 1, "IsTimeout": 1, "Kill": 1, "LinkError": 1,
		"LookupEnv": 1, "Mkdir": 1, "MkdirAll": 1, "MkdirTemp": 1, "ModeDir": 1, "ModePerm": 1,
		"ModeSymlink": 1, "O_APPEND": 1, "O_CREATE": 1, "O_EXCL": 1, "O_RDONLY": 1, "O_RDWR": 1,
		"O_TRUNC": 1, "O_WRONLY": 1, "PathError": 1, "PathListSeparator": 1, "PathSeparator": 1,
		"Pipe": 1, "Remove": 1, "RemoveAll": 1, "Rename": 1, "SameFile": 1, "Signal": 1,
		"Stderr": 1, "Stdin": 1, "Stdout": 1, "Symlink": 1, "SyscallError": 1, "TempDir": 1,
		"Truncate": 1, "UserCacheDir": 1, "UserConfigDir": 1, "UserHomeDir": 1, "WriteFile": 1,
	},
	"io/ioutil": {
		"ReadFile": testdataMember, "ReadDir": testdataMember,
		"Discard": 1, "NopCloser": 1, "ReadAll": 1, "TempDir": 1, "TempFile": 1, "WriteFile": 1,
	},
	"path/filepath": {
		"Abs": 1, "Base": 1, "Clean": 1, "Dir": 1, "ErrBadPattern": 1, "Ext": 1, "FromSlash": 1,
		"IsAbs": 1, "IsLocal": 1, "Join": 1, "ListSeparator": 1, "Localize": 1, "Match": 1, "Rel": 1,
		"Separator": 1, "SkipAll": 1, "SkipDir": 1, "Split": 1, "SplitList": 1, "ToSlash": 1,
		"VolumeName": 1,
	},
	"reflect": {"DeepEqual": 1},
	"os/exec": {},
}

// allowedDirectives are the `//go:` lines that cannot change what a file
// reads or runs; any other one (`embed`, `linkname`, `wasmimport`, `cgo_`
// and every unknown directive) makes the file observing.
var allowedDirectives = map[string]bool{"build": true, "generate": true, "noinline": true, "fix": true}

// observingMethods are member names that observe or redirect what a later
// call sees whatever their receiver: a working-directory change, which
// moves every relative read and `go list` target, and an environment
// change, which can set GOFLAGS or GOENV by a computed name for every
// `go list` call of the process.
var observingMethods = map[string]bool{"Chdir": true, "Setenv": true, "Unsetenv": true, "Clearenv": true}

// observesSource reports a file of an import-graph reader that is not
// proven to observe the import graph alone (AFP-V0-9010). The classifier is a
// closed allowlist that fails closed: the file is proven only when it parses;
// every `//go:` line is an allowedDirective; every function has a body (one
// without is implemented in assembly); every import is an inertImport, a
// blank import of an inert or classified package, or a classifiedImport
// named by a local name no declaration of the file shadows; every reference
// to a classified package's member is an allowed member in an allowed
// position, and every reference to an os/exec member is a `go list` call
// whose selector offset goListScanFile proved (proven); and no selector
// names an observingMethod. Anything else, including an alias or function
// value of exec.Command or exec.CommandContext, any exec.Cmd reference, a dot
// import, cgo, reflection beyond reflect.DeepEqual and a standard package
// this file does not classify, is observing.
func observesSource(relative string, body []byte, proven map[int]bool) bool {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, relative, body, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return true
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if directive, ok := strings.CutPrefix(comment.Text, "//go:"); ok {
				name, _, _ := strings.Cut(directive, " ")
				if !allowedDirectives[strings.TrimSpace(name)] {
					return true
				}
			}
		}
	}
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Body == nil {
			return true
		}
	}
	names := unshadowedImports(file)
	classified := map[string]string{}
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return true
		}
		_, checked := classifiedImports[value]
		if !inertImports[value] && !checked {
			return true
		}
		local := path.Base(value)
		if value == "math/rand/v2" {
			local = "rand"
		}
		if spec.Name != nil {
			local = spec.Name.Name
		}
		switch {
		case local == "_":
		case local == ".":
			return true
		case checked && names[local] != value:
			return true
		case checked:
			classified[local] = value
		}
	}
	allowed := map[*ast.SelectorExpr]bool{}
	observes := false
	ast.Inspect(file, func(node ast.Node) bool {
		if observes {
			return false
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			value, ok := classified[qualifier.Name]
			if ok && classifiedImports[value][selector.Sel.Name] == testdataMember && len(node.Args) != 0 && ownTestdata(node.Args[0], names) {
				allowed[selector] = true
			}
		case *ast.SelectorExpr:
			if observingMethods[node.Sel.Name] {
				observes = true
				return false
			}
			qualifier, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			value, ok := classified[qualifier.Name]
			if !ok {
				return true
			}
			switch {
			case value == "os/exec":
				observes = !proven[files.Position(node.Pos()).Offset]
			case classifiedImports[value][node.Sel.Name] == 1:
			case classifiedImports[value][node.Sel.Name] == testdataMember && allowed[node]:
			default:
				observes = true
			}
		}
		return true
	})
	return observes
}

// commandMethods are the exec.Cmd methods a proven command may be called
// with; commandSetFields the fields it may be assigned (Env only to a
// literalEnvironment; a Dir target is resolved by AFP-V0-2200's module-root
// rule, as before); and commandReadFields the fields it may be read through.
var (
	commandMethods = map[string]bool{
		"CombinedOutput": true, "Environ": true, "Output": true, "Run": true, "Start": true,
		"StderrPipe": true, "StdinPipe": true, "StdoutPipe": true, "String": true, "Wait": true,
	}
	commandSetFields  = map[string]bool{"Cancel": true, "Dir": true, "Env": true, "Stderr": true, "Stdin": true, "Stdout": true, "WaitDelay": true}
	commandReadFields = map[string]bool{"Process": true, "ProcessState": true, "Stderr": true, "Stdin": true, "Stdout": true}
)

// confinedCommand reports that the exec.Cmd a `go list` call builds stays a
// confined local of outer, its outermost enclosing function (AFP-V0-9010):
// the call is the receiver of a commandMethods call, or the sole value of a
// `name := call` or `var name = call` whose every other use in outer is a
// commandMethods call, a commandSetFields assignment, a commandReadFields read
// or another such definition by an os/exec call. Any other use, such as an
// argument, a stored or returned value, an address, a Path or Args field or
// an assignment of name, is not confined.
func confinedCommand(outer ast.Node, call *ast.CallExpr, exec string) bool {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(outer, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	method := func(selector *ast.SelectorExpr) bool {
		parent, ok := parents[selector].(*ast.CallExpr)
		return ok && parent.Fun == selector && commandMethods[selector.Sel.Name]
	}
	if selector, ok := parents[call].(*ast.SelectorExpr); ok && selector.X == call {
		return method(selector)
	}
	execCall := func(expression ast.Expr) bool {
		call, ok := expression.(*ast.CallExpr)
		if !ok {
			return false
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		return ok && qualifier.Name == exec && (selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext")
	}
	// definition reports a single-name definition by an os/exec call.
	definition := func(ident *ast.Ident) bool {
		switch parent := parents[ident].(type) {
		case *ast.AssignStmt:
			return parent.Tok == token.DEFINE && len(parent.Lhs) == 1 && len(parent.Rhs) == 1 && parent.Lhs[0] == ident && execCall(parent.Rhs[0])
		case *ast.ValueSpec:
			return parent.Type == nil && len(parent.Names) == 1 && len(parent.Values) == 1 && parent.Names[0] == ident && execCall(parent.Values[0])
		}
		return false
	}
	name := ""
	switch parent := parents[call].(type) {
	case *ast.AssignStmt:
		if ident, ok := parent.Lhs[0].(*ast.Ident); ok && definition(ident) {
			name = ident.Name
		}
	case *ast.ValueSpec:
		if definition(parent.Names[0]) {
			name = parent.Names[0].Name
		}
	}
	if name == "" || name == "_" {
		return false
	}
	confined := true
	ast.Inspect(outer, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !confined || !ok || ident.Name != name || definition(ident) {
			return confined
		}
		selector, ok := parents[ident].(*ast.SelectorExpr)
		if ok && selector.Sel == ident {
			return true // a field or method of another value with the same name
		}
		if !ok || selector.X != ident {
			confined = false
			return false
		}
		field := selector.Sel.Name
		assignment, assigned := parents[selector].(*ast.AssignStmt)
		target := assigned && slices.Contains(assignment.Lhs, ast.Expr(selector))
		switch {
		case method(selector):
		case target && commandSetFields[field] && assignment.Tok == token.ASSIGN && len(assignment.Lhs) == len(assignment.Rhs):
			if field == "Env" {
				confined = literalEnvironment(assignment.Rhs[slices.Index(assignment.Lhs, ast.Expr(selector))])
			}
		case !target && commandReadFields[field]:
			if address, ok := parents[selector].(*ast.UnaryExpr); ok && address.Op == token.AND {
				confined = false
			}
			if _, ok := parents[selector].(*ast.IncDecStmt); ok {
				confined = false
			}
		default:
			confined = false
		}
		return confined
	})
	return confined
}

// ownTestdata reports a path argument that is a literal, or a path/filepath
// Join of literals, naming the package's own testdata directory or a path
// below it: a test runs in its package directory.
func ownTestdata(argument ast.Expr, names map[string]string) bool {
	var parts []string
	if value, ok := literalString(argument); ok {
		parts = []string{value}
	} else {
		call, ok := argument.(*ast.CallExpr)
		if !ok || call.Ellipsis.IsValid() {
			return false
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Join" {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || names[qualifier.Name] != "path/filepath" && names[qualifier.Name] != "path" {
			return false
		}
		for _, element := range call.Args {
			value, ok := literalString(element)
			if !ok {
				return false
			}
			parts = append(parts, value)
		}
	}
	joined := path.Clean(strings.ReplaceAll(path.Join(parts...), `\`, "/"))
	return joined == "testdata" || strings.HasPrefix(joined, "testdata/")
}
