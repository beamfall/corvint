package typedreach

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// node is one unit of code in the reach graph: a function or method body
// (with every function literal inside it), the always-run initialization of
// one package, or the code a package variable's initializer holds without
// running it (AFP-V0-2371).
type node struct {
	label string
	pkg   *Package
	// calls are the functions the code calls or takes as a value.
	calls map[*types.Func]bool
	// converted are the types whose values the code converts to an
	// interface, or passes as a type argument.
	converted map[types.Type]bool
	// reads are the package variables the code mentions.
	reads map[*types.Var]bool
	// writes are the package variables the code may write.
	writes map[*types.Var]bool
	// mentions are the other objects the code names (constants, types,
	// fields), and exprTypes the types of its expressions.
	mentions  map[types.Object]bool
	exprTypes map[types.Type]bool
	// paramMethods are the method names called on a type parameter.
	paramMethods map[string]bool
	// starts label each process start whose program is not a proven
	// external tool, and process each write of process-wide state.
	starts  []string
	process []string
	// resets marks a body calling signal.Ignore or signal.Reset, which
	// signal.Stop does not undo.
	resets bool
	// unknown labels each standard-library function the body uses that is
	// neither modelled nor on the pure list (AFP-V0-2374).
	unknown []string
}

func newNode(label string, pkg *Package) *node {
	return &node{label: label, pkg: pkg, calls: map[*types.Func]bool{}, converted: map[types.Type]bool{}, reads: map[*types.Var]bool{}, writes: map[*types.Var]bool{}, mentions: map[types.Object]bool{}, exprTypes: map[types.Type]bool{}, paramMethods: map[string]bool{}}
}

// graph is the reach graph of one program.
type graph struct {
	program *Program
	funcs   map[*types.Func]*node
	held    map[*types.Var]*node
	// always are the nodes every test process runs before any test: each
	// package's variable initialization and init functions, and TestMain.
	always []*node
	tests  []testRoot
	// children are the TestMain statements left to child processes.
	children []string
	// called marks the identifiers in call position, and plainRead the
	// package-variable uses that yield no shared reference.
	called    map[*ast.Ident]bool
	plainRead map[*ast.Ident]bool
	// private are the local variables naming a private temporary path.
	private map[*types.Var]bool
	// pureUse marks the identifiers of standard-library calls the call site
	// proves act only on in-memory buffers.
	pureUse map[*ast.Ident]bool
	// derived maps a constant or type to the constants, types and variables
	// declared from it.
	derived map[types.Object][]types.Object
	// methods indexes every module method by name, for calls on a type
	// parameter.
	methods map[string][]*types.Func
}

type testRoot struct {
	name string
	node *node
}

// externalTools are the programs a process start may name as a constant and
// still be proven to run no module code (AFP-V0-2373). It is empty: a
// constant git or shell can still run module code through configuration,
// aliases or hooks a test writes, so every process start keeps its test.
var externalTools = map[string]bool{}

// processState are standard-library functions that change state every later
// test of the process observes (AFP-V0-2374).
var processState = map[string]bool{
	"os.Chdir":        true,
	"syscall.Chdir":   true,
	"net/http.Handle": true, "net/http.HandleFunc": true,
	"runtime.GOMAXPROCS": true, "runtime/debug.SetGCPercent": true, "runtime/debug.SetMemoryLimit": true,
	"runtime/debug.SetMaxStack": true, "runtime/debug.SetMaxThreads": true,
	"runtime/debug.SetTraceback": true, "flag.Set": true, "flag.Parse": true, "math/rand.Seed": true,
	"os.Setuid": true, "syscall.Umask": true, "syscall.Setuid": true, "syscall.Setgid": true,
}

// The process environment and signal dispositions are modelled as two
// pseudo package variables, so a dropped test that reads either is kept
// when a kept test writes it (AFP-V0-2374).
var (
	processPackage = types.NewPackage("process", "process")
	environment    = types.NewVar(token.NoPos, processPackage, "environment", types.Typ[types.String])
	signals        = types.NewVar(token.NoPos, processPackage, "signals", types.Typ[types.Int])
	// files is the file system outside the paths a body derives from a
	// temporary-directory call (AFP-V0-2374).
	files = types.NewVar(token.NoPos, processPackage, "files", types.Typ[types.String])
	// logDefault and slogDefault are the log and log/slog packages' default
	// loggers.
	logDefault  = types.NewVar(token.NoPos, processPackage, "logDefault", types.Typ[types.String])
	slogDefault = types.NewVar(token.NoPos, processPackage, "slogDefault", types.Typ[types.String])
)

// unknown is the state of everything the model does not follow: a closure
// that uses an unmodelled standard-library function, starts a process or
// writes a standard-library variable reads and writes it, and it conflicts
// with every pseudo or standard-library variable (AFP-V0-2374).
var unknown = types.NewVar(token.NoPos, processPackage, "unknown", types.Typ[types.String])

// purePackages are standard-library packages whose functions and methods act
// only on their arguments and receivers (and on state modelled elsewhere:
// fmt's and log's output, time.Local), less impureFunctions.
var purePackages = map[string]bool{
	"strings": true, "strconv": true, "bytes": true, "unicode": true, "unicode/utf8": true, "unicode/utf16": true,
	"errors": true, "sort": true, "slices": true, "maps": true, "cmp": true, "iter": true,
	"container/list": true, "container/heap": true, "container/ring": true, "math": true, "math/bits": true,
	"math/big": true, "math/cmplx": true, "path": true, "path/filepath": true, "regexp": true, "regexp/syntax": true,
	"encoding": true, "encoding/json": true, "encoding/hex": true, "encoding/base64": true, "encoding/base32": true,
	"encoding/binary": true, "encoding/csv": true, "encoding/xml": true, "encoding/pem": true, "encoding/asn1": true,
	"hash": true, "hash/crc32": true, "hash/crc64": true, "hash/fnv": true, "hash/adler32": true, "hash/maphash": true,
	"crypto/sha256": true, "crypto/sha1": true, "crypto/sha512": true, "crypto/md5": true,
	"crypto/hmac": true, "crypto/subtle": true, "crypto/ed25519": true,
	"fmt": true, "io": true, "io/fs": true, "bufio": true, "context": true, "sync": true, "sync/atomic": true,
	"time": true, "testing": true, "testing/fstest": true, "testing/iotest": true, "reflect": true,
	"text/template": true, "text/template/parse": true, "html/template": true, "html": true, "text/tabwriter": true,
	"text/scanner": true, "go/ast": true, "go/token": true, "go/parser": true, "go/printer": true, "go/format": true,
	"go/scanner": true, "go/types": true, "go/constant": true, "go/doc/comment": true, "go/build/constraint": true,
	"archive/tar": true, "compress/gzip": true, "compress/flate": true, "compress/zlib": true,
	"net/url": true, "net/netip": true, "unsafe": true, "embed": true, "log": true, "log/slog": true,
	"os/signal": true,
}

// impureFunctions are functions of purePackages that touch state the model
// does not follow.
var impureFunctions = map[string]bool{
	"path/filepath.Abs": true, "time.AfterFunc": true, "time.LoadLocation": true,
	"text/template.ParseFiles": true, "text/template.ParseGlob": true, "text/template.ParseFS": true,
	"html/template.ParseFiles": true, "html/template.ParseGlob": true, "html/template.ParseFS": true,
	"(*text/template.Template).ParseFiles": true, "(*text/template.Template).ParseGlob": true,
	"(*text/template.Template).ParseFS": true, "(*html/template.Template).ParseFiles": true,
	"(*html/template.Template).ParseGlob": true, "(*html/template.Template).ParseFS": true,
	"go/parser.ParseFile": true, "go/parser.ParseDir": true,
	"fmt.Fprint": true, "fmt.Fprintf": true, "fmt.Fprintln": true, "fmt.Fscan": true, "fmt.Fscanf": true,
	"fmt.Fscanln": true,
}

// pureRuntime are the runtime functions that read or change no state
// another test observes.
var pureRuntime = map[string]bool{
	"Caller": true, "Callers": true, "CallersFrames": true, "FuncForPC": true, "Gosched": true,
	"KeepAlive": true, "Version": true, "NumCPU": true,
}

// pureBuffers are in-memory writers and readers whose methods act only on
// their receiver; an io argument of another type may be a file or a
// connection (AFP-V0-2374).
var pureBuffers = map[string]bool{
	"bytes.Buffer": true, "strings.Builder": true, "bytes.Reader": true, "strings.Reader": true,
}

// opaqueInterfaces are the standard-library packages whose interfaces
// (io.Writer, fs.File, net.Conn, ...) standard-library types with state
// implement: calling one of their methods may reach a file or a connection.
var opaqueInterfaces = map[string]bool{
	"io": true, "io/fs": true, "net": true, "os": true, "net/http": true, "database/sql": true,
	"database/sql/driver": true, "bufio": true, "mime/multipart": true,
}

// pureOS are the os functions that read no state a test can change, or only
// state modelled elsewhere.
var pureOS = map[string]bool{
	"Exit": true, "Getpid": true, "Getppid": true, "Getuid": true, "Geteuid": true, "Getgid": true,
	"Getegid": true, "Getpagesize": true, "Hostname": true, "Getwd": true, "TempDir": true, "Executable": true,
	"IsExist": true, "IsNotExist": true, "IsPermission": true, "IsTimeout": true, "IsPathSeparator": true,
	"NewSyscallError": true, "Expand": true, "SameFile": true, "UserHomeDir": true, "UserCacheDir": true,
	"UserConfigDir": true,
}

// moduleFunc reports a function of a module package.
func (g *graph) moduleFunc(f *types.Func) bool {
	pkg := g.program.byPath[f.Pkg().Path()]
	return pkg != nil && pkg.Module
}

// stdModelled reports a standard-library function whose effect the model
// records itself.
func stdModelled(f *types.Func) bool {
	name := f.Pkg().Path() + "." + f.Name()
	if f.Signature().Recv() != nil {
		return strings.HasPrefix(f.FullName(), "(*os.Root).") || strings.HasPrefix(f.FullName(), "(*os.Process).")
	}
	_, writes := fileWriters[name]
	_, starts := processStarters[name]
	return writes || starts || fileReaders[name] || tempSources[f.FullName()] || stateWriters[name] != nil ||
		stateReaders[name] != nil || processState[name] || name == "os.OpenRoot" || name == "os.OpenInRoot"
}

// stdPure reports a standard-library function or method on the pure list.
func stdPure(f *types.Func) bool {
	path := f.Pkg().Path()
	if receiver := f.Signature().Recv(); receiver != nil && types.IsInterface(receiver.Type()) && opaqueInterfaces[path] {
		// An abstract method dispatches to whatever implements it.
		return false
	}
	if impureFunctions[f.FullName()] {
		return false
	}
	if path == "runtime" {
		return f.Signature().Recv() == nil && pureRuntime[f.Name()]
	}
	if path == "os" && f.Signature().Recv() == nil {
		return pureOS[f.Name()]
	}
	if impureFunctions[path+"."+f.Name()] || (path == "math/rand" || path == "math/rand/v2") && f.Signature().Recv() == nil {
		return false
	}
	if path == "math/rand" || path == "math/rand/v2" {
		return true
	}
	return purePackages[path]
}

// stdArguments records a standard-library call that hands a value of
// unknown effect to an interface it may call: an io, fs, net or os interface
// parameter given anything but a module type or an in-memory buffer makes
// the call unknown; fmt's Fprint and Fscan functions given an in-memory
// buffer are pure (AFP-V0-2374).
func (g *graph) stdArguments(n *node, pkg *Package, call *ast.CallExpr, callee *types.Func) {
	signature := callee.Signature()
	params := signature.Params()
	allPure := true
	for i, arg := range call.Args {
		var param types.Type
		switch {
		case signature.Variadic() && i >= params.Len()-1:
			if slice, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok && call.Ellipsis == token.NoPos {
				param = slice.Elem()
			}
		case i < params.Len():
			param = params.At(i).Type()
		}
		named, ok := types.Unalias(param).(*types.Named)
		if !ok || !types.IsInterface(param) || named.Obj().Pkg() == nil || !opaqueInterfaces[named.Obj().Pkg().Path()] {
			continue
		}
		if !g.pureArgument(pkg.Info.TypeOf(arg)) {
			allPure = false
			n.unknown = append(n.unknown, g.position(arg.Pos())+" "+callee.FullName()+" given an opaque "+named.Obj().Name())
		}
	}
	if allPure && strings.HasPrefix(callee.FullName(), "fmt.F") {
		if ident := calleeIdent(unparen(call.Fun)); ident != nil {
			g.pureUse[ident] = true
		}
	}
}

// pureArgument reports a value whose interface methods act only on itself:
// a module type, whose methods the graph follows, or an in-memory buffer.
func (g *graph) pureArgument(t types.Type) bool {
	if t == nil || types.IsInterface(t) {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	path := named.Obj().Pkg().Path()
	if pkg := g.program.byPath[path]; pkg != nil && pkg.Module {
		return true
	}
	return pureBuffers[path+"."+named.Obj().Name()]
}

// implicitReads returns the standard-library state a call into package
// path reads without naming it (AFP-V0-2374): fmt's Print* and Scan* use
// os.Stdout and os.Stdin, log prints to os.Stderr, log and log/slog use their default loggers, time uses
// time.Local.
func (g *graph) implicitReads(callee *types.Func) []*types.Var {
	path := callee.Pkg().Path()
	lookup := func(pkg, name string) *types.Var {
		if loaded := g.program.byPath[pkg]; loaded != nil {
			v, _ := loaded.Types.Scope().Lookup(name).(*types.Var)
			return v
		}
		return nil
	}
	var out []*types.Var
	add := func(vs ...*types.Var) {
		for _, v := range vs {
			if v != nil {
				out = append(out, v)
			}
		}
	}
	switch path {
	case "fmt":
		// Only Print* and Scan* use the process streams; Sprint, Errorf,
		// Fprint and Sscan act on their arguments.
		if strings.HasPrefix(callee.Name(), "Print") {
			add(lookup("os", "Stdout"))
		}
		if strings.HasPrefix(callee.Name(), "Scan") {
			add(lookup("os", "Stdin"))
		}
	case "log":
		add(lookup("os", "Stderr"), logDefault)
	case "log/slog":
		add(lookup("os", "Stderr"), logDefault, slogDefault)
	case "time":
		add(lookup("time", "Local"))
	}
	return out
}

// fileReaders are the standard-library functions that read the file or
// directory named by their first argument.
var fileReaders = map[string]bool{
	"os.ReadFile": true, "os.Open": true, "os.Stat": true, "os.Lstat": true, "os.ReadDir": true,
	"os.Readlink": true, "os.DirFS": true, "path/filepath.Walk": true, "path/filepath.WalkDir": true,
	"path/filepath.Glob": true, "path/filepath.EvalSymlinks": true, "io/fs.ReadFile": true,
	"io/ioutil.ReadFile": true, "io/ioutil.ReadDir": true,
}

// tempSources are the calls whose result names a private temporary path.
var tempSources = map[string]bool{
	"(*testing.common).TempDir": true, "os.MkdirTemp": true, "os.CreateTemp": true,
	"io/ioutil.TempDir": true, "io/ioutil.TempFile": true,
}

// stateWriters and stateReaders map standard-library functions to the
// pseudo variable they write or read.
var (
	stateWriters = map[string]*types.Var{
		"os.Setenv": environment, "os.Unsetenv": environment, "os.Clearenv": environment,
		"syscall.Setenv": environment, "syscall.Unsetenv": environment, "syscall.Clearenv": environment,
		"os/signal.Notify": signals, "os/signal.Ignore": signals, "os/signal.Reset": signals,
		"log.SetOutput": logDefault, "log.SetFlags": logDefault, "log.SetPrefix": logDefault,
		"log/slog.SetDefault": slogDefault, "os/signal.NotifyContext": signals, "log/slog.SetLogLoggerLevel": slogDefault,
	}
	stateReaders = map[string]*types.Var{
		"os.Getenv": environment, "os.LookupEnv": environment, "os.TempDir": environment,
		"os.UserHomeDir": environment, "os.UserCacheDir": environment, "os.UserConfigDir": environment,
		"os.MkdirTemp": environment, "os.CreateTemp": environment, "os.Environ": environment, "os.ExpandEnv": environment,
		"syscall.Getenv": environment, "syscall.Environ": environment,
		"os/signal.Notify": signals, "os/signal.NotifyContext": signals, "os/signal.Ignored": signals,
		"os/signal.Stop": signals, "syscall.Kill": signals, "os/signal.Ignore": signals, "os/signal.Reset": signals,
	}
)

// processStarters are the functions that start a process whose program the
// model reads from an argument (index) or cannot read at all (-1).
var processStarters = map[string]int{
	"os/exec.Command": 0, "os/exec.CommandContext": 1, "os.StartProcess": -1,
	"syscall.Exec": -1, "syscall.ForkExec": -1, "syscall.StartProcess": -1, "plugin.Open": -1,
}

// exemptStateMethods are pointer-receiver methods whose call on a package
// variable leaves no state a later test observes (AFP-V0-2374).
var exemptStateMethods = map[string]bool{}

func buildGraph(program *Program) *graph {
	g := &graph{program: program, funcs: map[*types.Func]*node{}, held: map[*types.Var]*node{}, methods: map[string][]*types.Func{}, called: map[*ast.Ident]bool{}, plainRead: map[*ast.Ident]bool{}, private: map[*types.Var]bool{}, derived: map[types.Object][]types.Object{}, pureUse: map[*ast.Ident]bool{}}
	for _, pkg := range program.Packages() {
		initialization := newNode(program.relative(pkg)+": package initialization", pkg)
		g.always = append(g.always, initialization)
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					g.funcDecl(pkg, file, decl, initialization)
				case *ast.GenDecl:
					g.derive(pkg, decl)
					if decl.Tok != token.VAR {
						continue
					}
					for _, spec := range decl.Specs {
						g.packageVar(pkg, spec.(*ast.ValueSpec), initialization)
					}
				}
			}
		}
	}
	return g
}

// derive records, for each constant, type and variable a declaration
// defines, the constants and types its type, value or type parameters name
// (AFP-V0-2373): a changed constant or type changes every constant, type
// and variable built from it, in any package.
func (g *graph) derive(pkg *Package, decl *ast.GenDecl) {
	if decl.Tok == token.IMPORT {
		return
	}
	for _, spec := range decl.Specs {
		var defined []*ast.Ident
		var parts []ast.Node
		switch typed := spec.(type) {
		case *ast.ValueSpec:
			defined = typed.Names
			if typed.Type != nil {
				parts = append(parts, typed.Type)
			}
			if decl.Tok == token.CONST {
				for _, value := range typed.Values {
					parts = append(parts, value)
				}
			}
		case *ast.TypeSpec:
			defined = []*ast.Ident{typed.Name}
			parts = append(parts, typed.Type)
			if typed.TypeParams != nil {
				parts = append(parts, typed.TypeParams)
			}
		}
		for _, part := range parts {
			ast.Inspect(part, func(x ast.Node) bool {
				ident, ok := x.(*ast.Ident)
				if !ok {
					return true
				}
				switch used := pkg.Info.Uses[ident].(type) {
				case *types.Const, *types.TypeName:
					for _, name := range defined {
						if object := pkg.Info.Defs[name]; object != nil {
							g.derived[used] = append(g.derived[used], object)
						}
					}
				}
				return true
			})
		}
	}
	// An iota group's later members repeat its earlier values: a change of
	// one member is a change of every later one.
	if decl.Tok == token.CONST {
		var previous []types.Object
		for _, spec := range decl.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				object := pkg.Info.Defs[name]
				if object == nil {
					continue
				}
				for _, earlier := range previous {
					g.derived[earlier] = append(g.derived[earlier], object)
				}
				previous = append(previous, object)
			}
		}
	}
}

func (g *graph) funcDecl(pkg *Package, file *ast.File, decl *ast.FuncDecl, initialization *node) {
	object, _ := pkg.Info.Defs[decl.Name].(*types.Func)
	label := g.position(decl.Pos()) + " " + decl.Name.Name
	if decl.Recv != nil && object != nil {
		if receiver := object.Signature().Recv(); receiver != nil {
			label = g.position(decl.Pos()) + " " + receiverName(receiver.Type()) + "." + decl.Name.Name
		}
	}
	n := newNode(label, pkg)
	switch {
	case decl.Recv == nil && decl.Name.Name == "init" && decl.Body != nil:
		// Every init runs before any test (AFP-V0-2372); what it only
		// stores into a package variable is held by that variable.
		for _, statement := range decl.Body.List {
			if targets := g.heldAssignment(pkg, statement); targets != nil {
				for i, rhs := range statement.(*ast.AssignStmt).Rhs {
					g.walkInitializer(initialization, g.holder(targets[i], pkg), pkg, rhs)
				}
				continue
			}
			g.walk(initialization, pkg, statement, signatureOf(object))
		}
		return
	case decl.Recv == nil && decl.Name.Name == "TestMain" && pkg.Tests[file] && decl.Body != nil:
		for _, statement := range decl.Body.List {
			if g.childOnly(pkg, statement) {
				// The condition runs in the parent; only the body is the
				// child's.
				g.children = append(g.children, g.position(statement.Pos()))
				g.walk(n, pkg, statement.(*ast.IfStmt).Cond, signatureOf(object))
				continue
			}
			g.walk(n, pkg, statement, signatureOf(object))
		}
		g.always = append(g.always, n)
		return
	}
	// The signature's types and constants (an array length) are part of
	// what the function does.
	g.walk(n, pkg, decl.Type, nil)
	if decl.Recv != nil {
		g.walk(n, pkg, decl.Recv, nil)
	}
	if decl.Body != nil {
		g.walk(n, pkg, decl.Body, signatureOf(object))
		scopeSignals(n)
	}
	if object == nil {
		return
	}
	g.funcs[object] = n
	if decl.Recv != nil {
		g.methods[decl.Name.Name] = append(g.methods[decl.Name.Name], object)
	} else if pkg.Tests[file] && isTestFunction(decl.Name.Name) {
		g.tests = append(g.tests, testRoot{name: decl.Name.Name, node: n})
	}
}

// packageVar splits a package variable declaration (AFP-V0-2371): the calls
// its initializer makes run at initialization, while the functions, literals
// and converted values it only stores are held by the variables, reached by
// the code that reads them.
func (g *graph) packageVar(pkg *Package, spec *ast.ValueSpec, initialization *node) {
	held := newNode(g.position(spec.Pos())+" var "+identNames(spec.Names), pkg)
	for _, name := range spec.Names {
		if object, ok := pkg.Info.Defs[name].(*types.Var); ok {
			if existing := g.held[object]; existing != nil {
				held = existing
			}
			g.held[object] = held
		}
	}
	var declared types.Type
	if spec.Type != nil {
		declared = pkg.Info.Types[spec.Type].Type
	}
	for _, value := range spec.Values {
		if declared != nil {
			g.convertTo(held, pkg, declared, value)
		}
		g.walkInitializer(initialization, held, pkg, value)
	}
}

// walkInitializer walks a package variable initializer: a call and its
// arguments run at initialization, a function literal that is not called
// is held, and everything else is held too.
func (g *graph) walkInitializer(initialization, held *node, pkg *Package, expr ast.Expr) {
	ast.Inspect(expr, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.FuncLit:
			g.walk(held, pkg, x.Body, signatureOf(pkg.Info.Types[x].Type))
			return false
		case *ast.CallExpr:
			if lit, ok := unparen(x.Fun).(*ast.FuncLit); ok {
				g.walk(initialization, pkg, lit.Body, signatureOf(pkg.Info.Types[lit].Type))
				for _, arg := range x.Args {
					g.walk(initialization, pkg, arg, nil)
				}
				return false
			}
			if tv, ok := pkg.Info.Types[unparen(x.Fun)]; ok && tv.IsType() {
				g.visitExpr(held, pkg, x)
				return true
			}
			g.walk(initialization, pkg, x, nil)
			return false
		}
		g.visitExpr(held, pkg, x)
		return true
	})
}

// scopeSignals drops the signal.Notify writes of a function that also calls
// signal.Stop: the registration ends with the function (AFP-V0-2374).
func scopeSignals(n *node) {
	stops := false
	for f := range n.calls {
		stops = stops || isFunc(f, "os/signal", "Stop")
	}
	if !stops {
		return
	}
	notifies := false
	for f := range n.calls {
		notifies = notifies || isFunc(f, "os/signal", "Notify")
	}
	if notifies && !n.resets {
		delete(n.writes, signals)
	}
}

// holder returns the node a package variable holds, creating it.
func (g *graph) holder(v *types.Var, pkg *Package) *node {
	if held := g.held[v]; held != nil {
		return held
	}
	held := newNode("var "+v.Name(), pkg)
	g.held[v] = held
	return held
}

// heldAssignment returns, for a plain assignment whose every left side is a
// package variable and whose sides pair up, those variables.
func (g *graph) heldAssignment(pkg *Package, statement ast.Stmt) []*types.Var {
	assign, ok := statement.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return nil
	}
	var targets []*types.Var
	for _, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			return nil
		}
		v, ok := pkg.Info.Uses[ident].(*types.Var)
		if !ok || !isPackageVar(v) {
			return nil
		}
		targets = append(targets, v)
	}
	return targets
}

// childOnly reports a TestMain statement that runs only in a child process
// the test binary starts as itself (AFP-V0-2372): an if statement without
// else whose condition argsSelect admits and whose body ends by calling
// os.Exit.
func (g *graph) childOnly(pkg *Package, statement ast.Stmt) bool {
	branch, ok := statement.(*ast.IfStmt)
	if !ok || branch.Else != nil || branch.Init != nil || len(branch.Body.List) == 0 {
		return false
	}
	last, ok := branch.Body.List[len(branch.Body.List)-1].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := last.X.(*ast.CallExpr)
	if !ok || !isFunc(calleeOf(pkg.Info, unparen(call.Fun)), "os", "Exit") {
		return false
	}
	return argsSelect(pkg.Info, branch.Cond)
}

// argsSelect reports a condition that holds only when os.Args[N], N >= 1,
// equals a non-empty string constant not starting with "-": `go test` passes
// the test binary only -test.* flags, so the parent never takes the branch.
// A conjunction holds when either side does.
func argsSelect(info *types.Info, cond ast.Expr) bool {
	binary, ok := unparen(cond).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	switch binary.Op {
	case token.LAND:
		return argsSelect(info, binary.X) || argsSelect(info, binary.Y)
	case token.EQL:
		return argsIndex(info, binary.X) && childWord(info, binary.Y) || argsIndex(info, binary.Y) && childWord(info, binary.X)
	}
	return false
}

func argsIndex(info *types.Info, expr ast.Expr) bool {
	index, ok := unparen(expr).(*ast.IndexExpr)
	if !ok {
		return false
	}
	selector, ok := unparen(index.X).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	v, ok := info.Uses[selector.Sel].(*types.Var)
	if !ok || v.Pkg() == nil || v.Pkg().Path() != "os" || v.Name() != "Args" {
		return false
	}
	value := info.Types[index.Index].Value
	if value == nil || value.Kind() != constant.Int {
		return false
	}
	n, exact := constant.Int64Val(value)
	return exact && n >= 1
}

func childWord(info *types.Info, expr ast.Expr) bool {
	value := info.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return false
	}
	word := constant.StringVal(value)
	return word != "" && !strings.HasPrefix(word, "-")
}

func isFunc(f *types.Func, pkg, name string) bool {
	return f != nil && f.Pkg() != nil && f.Pkg().Path() == pkg && f.Name() == name && f.Signature().Recv() == nil
}

// readOnlyReceivers are standard-library types whose pointer-receiver methods
// change no state a later caller observes, except the named ones.
var readOnlyReceivers = map[string]map[string]bool{
	"regexp.Regexp":          {"Longest": true},
	"strings.Replacer":       {},
	"text/template.Template": {"Funcs": true, "Parse": true, "New": true, "AddParseTree": true, "Delims": true, "Option": true},
	"html/template.Template": {"Funcs": true, "Parse": true, "New": true, "AddParseTree": true, "Delims": true, "Option": true, "ParseFiles": true, "ParseGlob": true, "ParseFS": true},
}

// readOnlyMethod reports a standard-library method readOnlyReceivers admits.
func readOnlyMethod(f *types.Func) bool {
	receiver := f.Signature().Recv()
	if receiver == nil {
		return false
	}
	pointer, ok := receiver.Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	excluded, ok := readOnlyReceivers[named.Obj().Pkg().Path()+"."+named.Obj().Name()]
	return ok && !excluded[f.Name()]
}

func (g *graph) merge(into, from *node) {
	for k := range from.calls {
		into.calls[k] = true
	}
	for k := range from.converted {
		into.converted[k] = true
	}
	for k := range from.reads {
		into.reads[k] = true
	}
	for k := range from.writes {
		into.writes[k] = true
	}
	for k := range from.mentions {
		into.mentions[k] = true
	}
	for k := range from.exprTypes {
		into.exprTypes[k] = true
	}
	for k := range from.paramMethods {
		into.paramMethods[k] = true
	}
	into.starts = append(into.starts, from.starts...)
	into.process = append(into.process, from.process...)
	into.unknown = append(into.unknown, from.unknown...)
}

// bodyVisitor walks one body, carrying the signature of the innermost
// function so a return statement's conversions are known.
type bodyVisitor struct {
	g         *graph
	n         *node
	pkg       *Package
	signature *types.Signature
}

func (g *graph) walk(n *node, pkg *Package, body ast.Node, signature *types.Signature) {
	g.markPrivate(pkg, body)
	ast.Walk(bodyVisitor{g: g, n: n, pkg: pkg, signature: signature}, body)
}

func (v bodyVisitor) Visit(x ast.Node) ast.Visitor {
	if x == nil {
		return nil
	}
	if lit, ok := x.(*ast.FuncLit); ok {
		v.g.visitExpr(v.n, v.pkg, lit)
		return bodyVisitor{g: v.g, n: v.n, pkg: v.pkg, signature: signatureOf(v.pkg.Info.Types[lit].Type)}
	}
	if ret, ok := x.(*ast.ReturnStmt); ok && v.signature != nil {
		results := v.signature.Results()
		switch {
		case len(ret.Results) == results.Len():
			for i, result := range ret.Results {
				v.g.convertTo(v.n, v.pkg, results.At(i).Type(), result)
			}
		case len(ret.Results) == 1:
			v.g.convertTuple(v.n, v.pkg, ret.Results[0], func(i int) types.Type {
				if i < results.Len() {
					return results.At(i).Type()
				}
				return nil
			})
		}
	}
	v.g.visitExpr(v.n, v.pkg, x)
	return v
}

// visitExpr records what one syntax node contributes to n.
func (g *graph) visitExpr(n *node, pkg *Package, x ast.Node) {
	info := pkg.Info
	if expr, ok := x.(ast.Expr); ok {
		if tv, ok := info.Types[expr]; ok && tv.Type != nil && !tv.IsType() {
			n.exprTypes[tv.Type] = true
		}
	}
	switch x := x.(type) {
	case *ast.Ident:
		if object := info.Uses[x]; object != nil && object.Pkg() != nil && object.Pkg().Path() == "os/exec" {
			// Any use of os/exec (a Cmd however made, its Start, Run,
			// Output or CombinedOutput) may start a process (AFP-V0-2373).
			n.starts = append(n.starts, g.position(x.Pos())+" uses os/exec."+object.Name())
			n.reads[environment] = true
		}
		if object := info.Uses[x]; object != nil && object.Pkg() != nil && object.Pkg().Path() == "os" && (object.Name() == "Root" || object.Name() == "OpenRoot" || object.Name() == "OpenInRoot") {
			// An os.Root names files relative to a root the model does not
			// follow: every use reads and writes the file system.
			n.reads[files] = true
			n.writes[files] = true
		}
		switch object := info.Uses[x].(type) {
		case *types.Func:
			if object.Pkg() != nil && !g.moduleFunc(object) && !g.pureUse[x] && !stdModelled(object) && !stdPure(object) {
				n.unknown = append(n.unknown, g.position(x.Pos())+" "+object.FullName())
			}
			n.calls[object.Origin()] = true
			if !g.called[x] {
				g.valueEffects(n, object, x)
			}
		case *types.Var:
			if isPackageVar(object) {
				n.reads[object] = true
				// A copy of a reference can be written through
				// (AFP-V0-2374): only a use that yields no shared
				// reference is a plain read.
				if g.moduleVar(object) && referenceType(object.Type()) && !g.plainRead[x] {
					n.writes[object] = true
				}
			} else {
				n.mentions[object.Origin()] = true
			}
		case *types.Const, *types.TypeName, *types.PkgName:
			n.mentions[object] = true
		}
		if instance, ok := info.Instances[x]; ok {
			for i := 0; i < instance.TypeArgs.Len(); i++ {
				n.converted[instance.TypeArgs.At(i)] = true
			}
		}
	case *ast.SelectorExpr:
		g.markPlainReads(pkg, x)
		if selection := info.Selections[x]; selection != nil && selection.Kind() == types.MethodVal && !g.called[x.Sel] {
			// A bound method value can be called anywhere it flows.
			if f, ok := selection.Obj().(*types.Func); ok {
				if _, pointer := f.Signature().Recv().Type().(*types.Pointer); pointer && !exemptStateMethods[f.FullName()] && !readOnlyMethod(f) {
					if root := rootVar(info, x.X); root != nil && g.moduleVar(root) {
						n.writes[root] = true
					}
				}
			}
		}
		if selection := info.Selections[x]; selection != nil && selection.Kind() != types.FieldVal {
			if _, param := types.Unalias(selection.Recv()).(*types.TypeParam); param {
				n.paramMethods[x.Sel.Name] = true
			}
		}
	case *ast.CallExpr:
		if ident := calleeIdent(unparen(x.Fun)); ident != nil {
			g.called[ident] = true
		}
		g.markPlainReads(pkg, x)
		g.call(n, pkg, x)
	case *ast.IndexExpr, *ast.RangeStmt:
		g.markPlainReads(pkg, x)
	case *ast.AssignStmt:
		for _, lhs := range x.Lhs {
			g.write(n, pkg, lhs)
		}
		switch {
		case x.Tok == token.ASSIGN && len(x.Lhs) == len(x.Rhs):
			for i, rhs := range x.Rhs {
				g.convertTo(n, pkg, info.TypeOf(x.Lhs[i]), rhs)
			}
		case x.Tok == token.ASSIGN && len(x.Rhs) == 1:
			g.convertTuple(n, pkg, x.Rhs[0], func(i int) types.Type {
				if i < len(x.Lhs) {
					return info.TypeOf(x.Lhs[i])
				}
				return nil
			})
		}
	case *ast.IncDecStmt:
		g.write(n, pkg, x.X)
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			g.write(n, pkg, x.X)
		}
	case *ast.ValueSpec:
		if x.Type != nil {
			for _, value := range x.Values {
				g.convertTo(n, pkg, info.TypeOf(x.Type), value)
			}
		}
	case *ast.SendStmt:
		if channel, ok := underlying(info.TypeOf(x.Chan)).(*types.Chan); ok {
			g.convertTo(n, pkg, channel.Elem(), x.Value)
		}
	case *ast.BinaryExpr:
		g.markPlainReads(pkg, x)
		if x.Op == token.EQL || x.Op == token.NEQ {
			g.convertTo(n, pkg, info.TypeOf(x.X), x.Y)
			g.convertTo(n, pkg, info.TypeOf(x.Y), x.X)
		}
	case *ast.CompositeLit:
		if named, ok := types.Unalias(info.TypeOf(x)).(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "os/exec" && named.Obj().Name() == "Cmd" {
			n.starts = append(n.starts, g.position(x.Pos())+" os/exec.Cmd literal")
		}
		g.composite(n, pkg, x)
	case *ast.TypeAssertExpr, *ast.TypeSwitchStmt:
	}
}

// call records a call's process starts, process-state writes, state writes
// through arguments and receivers, and argument conversions.
func (g *graph) call(n *node, pkg *Package, call *ast.CallExpr) {
	info := pkg.Info
	fun := unparen(call.Fun)
	if tv, ok := info.Types[fun]; ok && tv.IsType() {
		if len(call.Args) == 1 {
			g.convertTo(n, pkg, tv.Type, call.Args[0])
		}
		return
	}
	callee := calleeOf(info, fun)
	if callee != nil && callee.Pkg() != nil && !g.moduleFunc(callee) {
		g.stdArguments(n, pkg, call, callee)
	}
	if callee != nil && callee.Pkg() != nil {
		name := callee.Pkg().Path() + "." + callee.Name()
		if name == "log/slog.SetDefault" {
			// slog.SetDefault also redirects the log package's output.
			n.writes[logDefault] = true
		}
		if name == "os.Symlink" || name == "os.Link" {
			// A link can lead a private path anywhere.
			n.unknown = append(n.unknown, g.position(call.Pos())+" "+name)
		}
		if callee.Signature().Recv() == nil {
			switch {
			case stateWriters[name] != nil:
				n.writes[stateWriters[name]] = true
			case processState[name] && !queryOnly(info, name, call):
				n.process = append(n.process, g.position(call.Pos())+" calls "+name)
			}
			if fileReaders[name] && len(call.Args) > 0 {
				g.fileRead(n, pkg, call.Args[0])
			}
			if readOnlyOpen(info, name, call) {
				g.fileRead(n, pkg, call.Args[0])
			} else {
				for _, index := range fileWriters[name] {
					if index < len(call.Args) {
						g.fileWrite(n, pkg, call.Args[index], name)
					}
				}
			}
			if name == "os/signal.Ignore" || name == "os/signal.Reset" {
				n.resets = true
			}
			if stateReaders[name] != nil {
				n.reads[stateReaders[name]] = true
			}
			if index, ok := processStarters[name]; ok {
				// A started process inherits the environment.
				n.reads[environment] = true
				if !g.provenTool(info, call, index) {
					n.starts = append(n.starts, g.position(call.Pos())+" "+name)
				}
			}
		} else if strings.HasPrefix(callee.FullName(), "(*os.Process).") {
			n.reads[signals] = true
		} else if strings.HasPrefix(callee.FullName(), "(*os.Root).") {
			n.reads[files] = true
			n.writes[files] = true
		}
		for _, v := range g.implicitReads(callee) {
			n.reads[v] = true
		}
		if selector, ok := fun.(*ast.SelectorExpr); ok {
			if selection := info.Selections[selector]; selection != nil && selection.Kind() == types.MethodVal {
				if _, pointer := callee.Signature().Recv().Type().(*types.Pointer); pointer && !exemptStateMethods[callee.FullName()] && !readOnlyMethod(callee) {
					if root := rootVar(info, selector.X); root != nil && g.moduleVar(root) {
						n.writes[root] = true
					}
				}
			}
		}
	}
	if ident, ok := fun.(*ast.Ident); ok {
		if builtin, ok := info.Uses[ident].(*types.Builtin); ok {
			// A builtin writes at most through its first argument.
			switch builtin.Name() {
			case "append", "copy", "clear", "delete":
				if len(call.Args) > 0 {
					if root := rootVar(info, call.Args[0]); root != nil && g.moduleVar(root) && referenceType(root.Type()) {
						n.writes[root] = true
					}
				}
			}
			if builtin.Name() == "append" {
				if slice, ok := underlying(info.TypeOf(call)).(*types.Slice); ok {
					for _, arg := range call.Args[1:] {
						if call.Ellipsis == token.NoPos {
							g.convertTo(n, pkg, slice.Elem(), arg)
						}
					}
				}
			}
			return
		}
	}
	signature := signatureOf(info.TypeOf(fun))
	if signature == nil {
		return
	}
	params := signature.Params()
	paramAt := func(i int) types.Type {
		switch {
		case signature.Variadic() && i >= params.Len()-1 && call.Ellipsis == token.NoPos:
			if slice, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
				return slice.Elem()
			}
		case i < params.Len():
			return params.At(i).Type()
		}
		return nil
	}
	if len(call.Args) == 1 {
		// f(g()) with a multi-value g.
		g.convertTuple(n, pkg, call.Args[0], paramAt)
	}
	for i, arg := range call.Args {
		var param types.Type
		switch {
		case signature.Variadic() && i >= params.Len()-1 && call.Ellipsis == token.NoPos:
			if slice, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
				param = slice.Elem()
			}
		case i < params.Len():
			param = params.At(i).Type()
		}
		if param != nil {
			g.convertTo(n, pkg, param, arg)
		}
		if root := rootVar(info, arg); root != nil && g.moduleVar(root) && referenceType(root.Type()) {
			n.writes[root] = true
		}
	}
}

// fileWriters are the standard-library functions that create, change or
// remove a file, with the positions of the arguments naming it.
var fileWriters = map[string][]int{
	"os.WriteFile": {0}, "os.Create": {0}, "os.OpenFile": {0}, "os.Mkdir": {0}, "os.MkdirAll": {0},
	"os.Remove": {0}, "os.RemoveAll": {0}, "os.Rename": {0, 1}, "os.Symlink": {1}, "os.Link": {1},
	"os.Chmod": {0}, "os.Chtimes": {0}, "os.Truncate": {0}, "os.Lchown": {0}, "os.Chown": {0},
	"os.CopyFS": {0}, "io/ioutil.WriteFile": {0},
}

// readOnlyOpen reports os.OpenFile with constant flags that neither write,
// create, truncate nor append.
func readOnlyOpen(info *types.Info, name string, call *ast.CallExpr) bool {
	if name != "os.OpenFile" || len(call.Args) < 2 {
		return false
	}
	value := info.Types[call.Args[1]].Value
	if value == nil || value.Kind() != constant.Int {
		return false
	}
	flags, exact := constant.Int64Val(value)
	return exact && flags&int64(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) == 0
}

// fileWrite records a file write (AFP-V0-2374): a constant path is
// process-wide state; a path a package variable names writes that variable;
// any other path not derived in the same body from a temporary-directory
// call writes the files pseudo variable.
func (g *graph) fileWrite(n *node, pkg *Package, path ast.Expr, name string) {
	if tv := pkg.Info.Types[path]; tv.Value != nil {
		n.process = append(n.process, g.position(path.Pos())+" "+name+" of the fixed path "+tv.Value.ExactString())
		return
	}
	g.packagePathVars(n, pkg, path)
	if !g.privatePath(pkg, path) {
		n.writes[files] = true
	}
}

// fileRead records a file read of a path not provably private.
func (g *graph) fileRead(n *node, pkg *Package, path ast.Expr) {
	if !g.privatePath(pkg, path) {
		n.reads[files] = true
	}
}

func (g *graph) packagePathVars(n *node, pkg *Package, path ast.Expr) {
	ast.Inspect(path, func(x ast.Node) bool {
		if ident, ok := x.(*ast.Ident); ok {
			if v, ok := pkg.Info.Uses[ident].(*types.Var); ok && isPackageVar(v) && g.moduleVar(v) {
				n.writes[v] = true
			}
		}
		return true
	})
}

// privatePath reports a path expression that calls a temporary-directory
// source or names a local variable a body assigned from one.
func (g *graph) privatePath(pkg *Package, path ast.Expr) bool {
	switch typed := unparen(path).(type) {
	case *ast.Ident:
		v, ok := pkg.Info.Uses[typed].(*types.Var)
		return ok && g.private[v]
	case *ast.CallExpr:
		f := calleeOf(pkg.Info, unparen(typed.Fun))
		if f == nil {
			return false
		}
		if tempSources[f.FullName()] {
			return true
		}
		// A join stays inside its first element when every later element
		// is a constant that climbs nowhere.
		if (isFunc(f, "path/filepath", "Join") || isFunc(f, "path", "Join")) && len(typed.Args) > 0 && typed.Ellipsis == token.NoPos {
			if !g.privatePath(pkg, typed.Args[0]) {
				return false
			}
			for _, arg := range typed.Args[1:] {
				if !containedConstant(pkg.Info, arg, false) {
					return false
				}
			}
			return true
		}
	case *ast.BinaryExpr:
		// dir + "/name" stays inside dir.
		return typed.Op == token.ADD && g.privatePath(pkg, typed.X) && containedConstant(pkg.Info, typed.Y, true)
	}
	return false
}

// containedConstant reports a constant string path element that names no
// parent directory and, when slash is set, starts with a separator.
func containedConstant(info *types.Info, expr ast.Expr, slash bool) bool {
	value := info.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return false
	}
	element := constant.StringVal(value)
	if slash && !strings.HasPrefix(element, "/") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(element), "/") {
		if part == ".." {
			return false
		}
	}
	return !filepath.IsAbs(strings.TrimPrefix(element, "/")) && element != ""
}

// markPrivate marks the local variables a body assigns, to a fixed point,
// from a temporary-directory source or another such variable.
func (g *graph) markPrivate(pkg *Package, body ast.Node) {
	for grown := true; grown; {
		grown = false
		ast.Inspect(body, func(x ast.Node) bool {
			assign, ok := x.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != len(assign.Rhs) {
				return true
			}
			for i, lhs := range assign.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				v, _ := pkg.Info.Defs[ident].(*types.Var)
				if v == nil {
					v, _ = pkg.Info.Uses[ident].(*types.Var)
				}
				if v == nil || isPackageVar(v) || g.private[v] || g.assignedElsewhere(pkg, body, v, assign) {
					continue
				}
				if g.privatePath(pkg, assign.Rhs[i]) {
					g.private[v] = true
					grown = true
				}
			}
			return true
		})
	}
}

// assignedElsewhere reports another assignment to v in body, which could
// make the variable name a shared path.
func (g *graph) assignedElsewhere(pkg *Package, body ast.Node, v *types.Var, own *ast.AssignStmt) bool {
	other := false
	ast.Inspect(body, func(x ast.Node) bool {
		if assign, ok := x.(*ast.AssignStmt); ok && assign != own {
			for _, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && (pkg.Info.Defs[ident] == v || pkg.Info.Uses[ident] == v) {
					other = true
				}
			}
		}
		if unary, ok := x.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unparen(unary.X).(*ast.Ident); ok && pkg.Info.Uses[ident] == v {
				other = true
			}
		}
		return !other
	})
	return other
}

// valueEffects records the effects of a standard-library function taken as a
// value, which may be called anywhere the value flows (AFP-V0-2374).
func (g *graph) valueEffects(n *node, f *types.Func, at *ast.Ident) {
	if f.Pkg() == nil || f.Signature().Recv() != nil {
		return
	}
	name := f.Pkg().Path() + "." + f.Name()
	if _, ok := processStarters[name]; ok {
		n.starts = append(n.starts, g.position(at.Pos())+" "+name+" as a value")
		n.reads[environment] = true
	}
	if v := stateWriters[name]; v != nil {
		n.writes[v] = true
	}
	if v := stateReaders[name]; v != nil {
		n.reads[v] = true
	}
	for _, v := range g.implicitReads(f) {
		n.reads[v] = true
	}
	if name == "log/slog.SetDefault" {
		n.writes[logDefault] = true
	}
	if _, writes := fileWriters[name]; writes || fileReaders[name] {
		n.writes[files] = true
		n.reads[files] = true
	}
	if processState[name] {
		n.process = append(n.process, g.position(at.Pos())+" "+name+" as a value")
	}
}

// markPlainReads marks the package-variable identifiers a node reads without
// yielding a shared reference: an index, field or range whose result holds
// none, a len or cap argument, or a comparison operand.
func (g *graph) markPlainReads(pkg *Package, x ast.Node) {
	info := pkg.Info
	mark := func(expr ast.Expr) {
		switch typed := unparen(expr).(type) {
		case *ast.Ident:
			g.plainRead[typed] = true
		case *ast.SelectorExpr:
			if info.Selections[typed] == nil {
				// A qualified identifier, pkg.Var.
				g.plainRead[typed.Sel] = true
			}
		}
	}
	switch x := x.(type) {
	case *ast.IndexExpr:
		if !referenceType(info.TypeOf(x)) {
			mark(x.X)
		}
	case *ast.SelectorExpr:
		if selection := info.Selections[x]; selection != nil && selection.Kind() == types.FieldVal && !referenceType(info.TypeOf(x)) {
			mark(x.X)
		}
	case *ast.RangeStmt:
		if elem := rangeElem(info.TypeOf(x.X)); elem != nil && !referenceType(elem) {
			mark(x.X)
		}
	case *ast.CallExpr:
		if ident, ok := unparen(x.Fun).(*ast.Ident); ok && len(x.Args) == 1 {
			if builtin, ok := info.Uses[ident].(*types.Builtin); ok && (builtin.Name() == "len" || builtin.Name() == "cap") {
				mark(x.Args[0])
			}
		}
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			mark(x.X)
			mark(x.Y)
		}
	}
}

func rangeElem(t types.Type) types.Type {
	switch typed := underlying(t).(type) {
	case *types.Slice:
		return typed.Elem()
	case *types.Array:
		return typed.Elem()
	case *types.Map:
		if referenceType(typed.Key()) {
			return typed.Key()
		}
		return typed.Elem()
	case *types.Pointer:
		if array, ok := underlying(typed.Elem()).(*types.Array); ok {
			return array.Elem()
		}
	case *types.Basic:
		return typed
	}
	return nil
}

func calleeIdent(fun ast.Expr) *ast.Ident {
	switch typed := fun.(type) {
	case *ast.Ident:
		return typed
	case *ast.SelectorExpr:
		return typed.Sel
	case *ast.IndexExpr:
		return calleeIdent(typed.X)
	case *ast.IndexListExpr:
		return calleeIdent(typed.X)
	}
	return nil
}

// queryOnly reports runtime.GOMAXPROCS with a constant argument below 1,
// which reads the setting without changing it.
func queryOnly(info *types.Info, name string, call *ast.CallExpr) bool {
	if name != "runtime.GOMAXPROCS" || len(call.Args) != 1 {
		return false
	}
	value := info.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.Int {
		return false
	}
	n, exact := constant.Int64Val(value)
	return exact && n < 1
}

// provenTool reports whether a process start names, as a constant, a
// program known to run no module code.
func (g *graph) provenTool(info *types.Info, call *ast.CallExpr, index int) bool {
	if index < 0 || index >= len(call.Args) {
		return false
	}
	tv := info.Types[call.Args[index]]
	if tv.Value == nil || tv.Value.Kind() != constant.String {
		return false
	}
	return externalTools[constant.StringVal(tv.Value)]
}

// composite records the conversions of a composite literal's elements.
func (g *graph) composite(n *node, pkg *Package, lit *ast.CompositeLit) {
	info := pkg.Info
	switch typed := underlying(info.TypeOf(lit)).(type) {
	case *types.Struct:
		for i, element := range lit.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				if key, ok := pair.Key.(*ast.Ident); ok {
					if field, ok := info.Uses[key].(*types.Var); ok {
						g.convertTo(n, pkg, field.Type(), pair.Value)
					}
				}
				continue
			}
			if i < typed.NumFields() {
				g.convertTo(n, pkg, typed.Field(i).Type(), element)
			}
		}
	case *types.Slice:
		g.elements(n, pkg, nil, typed.Elem(), lit.Elts)
	case *types.Array:
		g.elements(n, pkg, nil, typed.Elem(), lit.Elts)
	case *types.Map:
		g.elements(n, pkg, typed.Key(), typed.Elem(), lit.Elts)
	}
}

func (g *graph) elements(n *node, pkg *Package, key, elem types.Type, elements []ast.Expr) {
	for _, element := range elements {
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			if key != nil {
				g.convertTo(n, pkg, key, pair.Key)
			}
			element = pair.Value
		}
		g.convertTo(n, pkg, elem, element)
	}
}

// convertTo records value's type as converted when target is an interface
// and value's own type is not.
func (g *graph) convertTo(n *node, pkg *Package, target types.Type, value ast.Expr) {
	if target == nil || !types.IsInterface(target) {
		return
	}
	if _, param := types.Unalias(target).(*types.TypeParam); param {
		return
	}
	if from := pkg.Info.TypeOf(value); from != nil && !types.IsInterface(from) {
		n.converted[from] = true
	}
}

// convertTuple records the conversions of a multi-value expression's
// results to the destination types target names by position.
func (g *graph) convertTuple(n *node, pkg *Package, value ast.Expr, target func(int) types.Type) {
	tuple, ok := pkg.Info.TypeOf(value).(*types.Tuple)
	if !ok {
		return
	}
	for i := 0; i < tuple.Len(); i++ {
		to, from := target(i), tuple.At(i).Type()
		if to != nil && types.IsInterface(to) && !types.IsInterface(from) {
			if _, param := types.Unalias(to).(*types.TypeParam); !param {
				n.converted[from] = true
			}
		}
	}
}

// write records the package variable at the root of a written expression.
func (g *graph) write(n *node, pkg *Package, expr ast.Expr) {
	if root := rootVar(pkg.Info, expr); root != nil {
		n.writes[root] = true
	}
}

func (g *graph) position(pos token.Pos) string {
	position := g.program.Fset.Position(pos)
	name := position.Filename
	if relative, err := filepath.Rel(g.program.Root, name); err == nil {
		name = filepath.ToSlash(relative)
	}
	return fmt.Sprintf("%s:%d", name, position.Line)
}

// rootVar returns the module package variable an expression is rooted at
// through selectors, indexes, slices, dereferences and parentheses.
func rootVar(info *types.Info, expr ast.Expr) *types.Var {
	for {
		switch typed := expr.(type) {
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.SelectorExpr:
			if selection := info.Selections[typed]; selection == nil {
				// A qualified identifier: pkg.Var.
				expr = typed.Sel
				continue
			}
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.SliceExpr:
			expr = typed.X
		case *ast.StarExpr:
			expr = typed.X
		case *ast.TypeAssertExpr:
			expr = typed.X
		case *ast.Ident:
			if object, ok := info.Uses[typed].(*types.Var); ok && isPackageVar(object) {
				return object
			}
			return nil
		default:
			return nil
		}
	}
}

// moduleVar reports a package variable of a module package.
func (g *graph) moduleVar(v *types.Var) bool {
	pkg := g.program.byPath[v.Pkg().Path()]
	return pkg != nil && pkg.Module
}

func isPackageVar(object *types.Var) bool {
	return object.Pkg() != nil && !object.IsField() && object.Parent() == object.Pkg().Scope()
}

// referenceType reports a type whose value shares memory with its copies.
func referenceType(t types.Type) bool {
	return holdsReference(t, map[types.Type]bool{})
}

// holdsReference reports a map, slice, pointer or channel, or a struct or
// array holding one, whose copy shares memory with the original.
func holdsReference(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch typed := underlying(t).(type) {
	case *types.Map, *types.Slice, *types.Pointer, *types.Chan, *types.Interface:
		return true
	case *types.Struct:
		for i := 0; i < typed.NumFields(); i++ {
			if holdsReference(typed.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return holdsReference(typed.Elem(), seen)
	}
	return false
}

func calleeOf(info *types.Info, fun ast.Expr) *types.Func {
	switch typed := fun.(type) {
	case *ast.Ident:
		object, _ := info.Uses[typed].(*types.Func)
		return object
	case *ast.SelectorExpr:
		object, _ := info.Uses[typed.Sel].(*types.Func)
		return object
	case *ast.IndexExpr:
		return calleeOf(info, typed.X)
	case *ast.IndexListExpr:
		return calleeOf(info, typed.X)
	}
	return nil
}

func signatureOf(t any) *types.Signature {
	switch typed := t.(type) {
	case *types.Func:
		if typed == nil {
			return nil
		}
		return typed.Signature()
	case types.Type:
		signature, _ := underlying(typed).(*types.Signature)
		return signature
	}
	return nil
}

func underlying(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return t.Underlying()
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func receiverName(t types.Type) string {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Obj().Name()
	}
	return t.String()
}

func identNames(names []*ast.Ident) string {
	var out []string
	for _, name := range names {
		out = append(out, name.Name)
	}
	return strings.Join(out, ",")
}

// isTestFunction is the `go test` rule for a top-level function name it runs.
func isTestFunction(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return rest == "" || !(rest[0] >= 'a' && rest[0] <= 'z')
		}
	}
	return false
}

func sortedLabels(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for label := range set {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}
