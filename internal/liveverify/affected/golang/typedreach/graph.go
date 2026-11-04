package typedreach

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
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
	// methods indexes every module method by name, for calls on a type
	// parameter.
	methods map[string][]*types.Func
}

type testRoot struct {
	name string
	node *node
}

// externalTools are the programs a process start may name as a constant and
// still be proven to run no module code (AFP-V0-2373).
var externalTools = map[string]bool{"git": true, "/usr/bin/git": true}

// processState are standard-library functions that change state every later
// test of the process observes (AFP-V0-2374).
var processState = map[string]bool{
	"os.Chdir":        true,
	"syscall.Chdir":   true,
	"net/http.Handle": true, "net/http.HandleFunc": true, "log.SetOutput": true, "log.SetFlags": true,
	"log.SetPrefix":      true,
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
)

// stateWriters and stateReaders map standard-library functions to the
// pseudo variable they write or read.
var (
	stateWriters = map[string]*types.Var{
		"os.Setenv": environment, "os.Unsetenv": environment, "os.Clearenv": environment,
		"syscall.Setenv": environment, "syscall.Unsetenv": environment, "syscall.Clearenv": environment,
		"os/signal.Notify": signals, "os/signal.Ignore": signals, "os/signal.Reset": signals,
	}
	stateReaders = map[string]*types.Var{
		"os.Getenv": environment, "os.LookupEnv": environment, "os.Environ": environment, "os.ExpandEnv": environment,
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
var exemptStateMethods = map[string]bool{
	"(*sync.Mutex).Lock": true, "(*sync.Mutex).Unlock": true, "(*sync.Mutex).TryLock": true,
	"(*sync.RWMutex).Lock": true, "(*sync.RWMutex).Unlock": true, "(*sync.RWMutex).RLock": true,
	"(*sync.RWMutex).RUnlock": true, "(*sync.RWMutex).TryLock": true, "(*sync.RWMutex).TryRLock": true,
	"(*sync.Once).Do": true,
}

func buildGraph(program *Program) *graph {
	g := &graph{program: program, funcs: map[*types.Func]*node{}, held: map[*types.Var]*node{}, methods: map[string][]*types.Func{}}
	for _, pkg := range program.Packages() {
		initialization := newNode(program.relative(pkg)+": package initialization", pkg)
		g.always = append(g.always, initialization)
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					g.funcDecl(pkg, file, decl, initialization)
				case *ast.GenDecl:
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
				g.children = append(g.children, g.position(statement.Pos()))
				continue
			}
			g.walk(n, pkg, statement, signatureOf(object))
		}
		g.always = append(g.always, n)
		return
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
// else whose condition reads os.Args, os.Getenv or os.LookupEnv and whose
// body ends by calling os.Exit.
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
	guarded := false
	ast.Inspect(branch.Cond, func(x ast.Node) bool {
		if ident, ok := x.(*ast.Ident); ok {
			switch object := pkg.Info.Uses[ident].(type) {
			case *types.Var:
				guarded = guarded || (object.Pkg() != nil && object.Pkg().Path() == "os" && object.Name() == "Args")
			case *types.Func:
				guarded = guarded || isFunc(object, "os", "Getenv") || isFunc(object, "os", "LookupEnv")
			}
		}
		return true
	})
	return guarded
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
	"sync.Pool":              {},
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
	if ret, ok := x.(*ast.ReturnStmt); ok && v.signature != nil && len(ret.Results) == v.signature.Results().Len() {
		for i, result := range ret.Results {
			v.g.convertTo(v.n, v.pkg, v.signature.Results().At(i).Type(), result)
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
		switch object := info.Uses[x].(type) {
		case *types.Func:
			n.calls[object.Origin()] = true
		case *types.Var:
			if isPackageVar(object) {
				n.reads[object] = true
			} else {
				n.mentions[object.Origin()] = true
			}
		case *types.Const, *types.TypeName:
			n.mentions[object] = true
		}
		if instance, ok := info.Instances[x]; ok {
			for i := 0; i < instance.TypeArgs.Len(); i++ {
				n.converted[instance.TypeArgs.At(i)] = true
			}
		}
	case *ast.SelectorExpr:
		if selection := info.Selections[x]; selection != nil && selection.Kind() != types.FieldVal {
			if _, param := types.Unalias(selection.Recv()).(*types.TypeParam); param {
				n.paramMethods[x.Sel.Name] = true
			}
		}
	case *ast.CallExpr:
		g.call(n, pkg, x)
	case *ast.AssignStmt:
		for _, lhs := range x.Lhs {
			g.write(n, pkg, lhs)
		}
		if x.Tok == token.ASSIGN && len(x.Lhs) == len(x.Rhs) {
			for i, rhs := range x.Rhs {
				g.convertTo(n, pkg, info.TypeOf(x.Lhs[i]), rhs)
			}
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
		if x.Op == token.EQL || x.Op == token.NEQ {
			g.convertTo(n, pkg, info.TypeOf(x.X), x.Y)
			g.convertTo(n, pkg, info.TypeOf(x.Y), x.X)
		}
	case *ast.CompositeLit:
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
	if callee != nil && callee.Pkg() != nil {
		name := callee.Pkg().Path() + "." + callee.Name()
		if callee.Signature().Recv() == nil {
			switch {
			case stateWriters[name] != nil:
				n.writes[stateWriters[name]] = true
			case processState[name] && !queryOnly(info, name, call):
				n.process = append(n.process, g.position(call.Pos())+" calls "+name)
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
	switch underlying(t).(type) {
	case *types.Map, *types.Slice, *types.Pointer, *types.Chan:
		return true
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
