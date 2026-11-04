package typedreach

import (
	"go/types"
	"sort"
	"strings"
)

// Declaration names one changed top-level declaration of a module package:
// a function, variable, constant or type by its name, a method as
// Type.Method.
type Declaration struct {
	Package string
	Name    string
	// Initialization marks a change that runs at the package's
	// initialization; Name then labels it.
	Initialization bool
}

// Whole-package reasons and kept-test reasons (AFP-V0-2374, AFP-V0-2375).
const (
	WholeAlwaysRunReaches = "ALWAYS_RUN_REACHES_CHANGE"
	WholeAlwaysRunStarts  = "ALWAYS_RUN_STARTS_PROCESS"
	WholeProcessState     = "KEPT_TEST_WRITES_PROCESS_STATE"
	WholeEveryTestKept    = "EVERY_TEST_REACHES_CHANGE"
	KeptReachesChange     = "REACHES_CHANGE"
	KeptStartsProcess     = "STARTS_PROCESS"
	KeptReadsSharedState  = "READS_STATE_A_KEPT_TEST_WRITES"
)

// Result is the decision for one test package: Whole names why every test
// must run; otherwise Kept maps each test function that must run to its
// reason with a witness, and Dropped lists the rest.
type Result struct {
	Whole   string
	Kept    map[string]string
	Dropped []string
	Tests   int
}

// change is the resolved change set over the program's objects.
type change struct {
	funcs    map[*types.Func]bool
	objects  map[types.Object]bool
	types    map[*types.Named]bool
	packages map[string]bool
	// initialization labels a linked package's changed initialization.
	initialization string
}

// Analyze decides which test functions of program can reach the changed
// declarations, or the packages wholly changed (every declaration of them
// counts as changed), under the AFP-V0-2373 model.
func Analyze(program *Program, changed []Declaration, wholePackages []string) Result {
	g := buildGraph(program)
	c := resolve(program, changed, wholePackages)
	a := &analysis{g: g, c: c, structural: map[types.Type][]*types.Named{}}
	result := Result{Kept: map[string]string{}, Tests: len(g.tests)}
	if c.initialization != "" {
		result.Whole = WholeAlwaysRunReaches + ": " + c.initialization
		return result
	}
	always := a.reach(g.always)
	if always.touched != "" {
		result.Whole = WholeAlwaysRunReaches + ": " + always.touched
		return result
	}
	if len(always.starts) > 0 {
		result.Whole = WholeAlwaysRunStarts + ": " + always.starts[0]
		return result
	}
	closures := make([]closure, len(g.tests))
	writes := map[*types.Var]bool{}
	for i, test := range g.tests {
		closures[i] = a.reach([]*node{test.node})
		switch {
		case closures[i].touched != "":
			result.Kept[test.name] = KeptReachesChange + ": " + closures[i].touched
		case len(closures[i].starts) > 0:
			result.Kept[test.name] = KeptStartsProcess + ": " + closures[i].starts[0]
		default:
			continue
		}
		if len(closures[i].process) > 0 {
			result.Whole = WholeProcessState + ": " + test.name + ": " + closures[i].process[0]
			return result
		}
		for v := range closures[i].writes {
			writes[v] = true
		}
	}
	// A dropped test that reads state a kept test may write could observe
	// the change through it, whatever the order the tests run in.
	for grown := true; grown; {
		grown = false
		for i, test := range g.tests {
			if _, kept := result.Kept[test.name]; kept {
				continue
			}
			if shared := sharedVar(closures[i].reads, writes); shared != nil {
				result.Kept[test.name] = KeptReadsSharedState + ": " + shared.Pkg().Path() + "." + shared.Name()
				if len(closures[i].process) > 0 {
					result.Whole = WholeProcessState + ": " + test.name + ": " + closures[i].process[0]
					return result
				}
				for v := range closures[i].writes {
					writes[v] = true
				}
				grown = true
			}
		}
	}
	for _, test := range g.tests {
		if _, kept := result.Kept[test.name]; !kept {
			result.Dropped = append(result.Dropped, test.name)
		}
	}
	sort.Strings(result.Dropped)
	if len(result.Dropped) == 0 && len(g.tests) > 0 {
		result.Whole = WholeEveryTestKept
	}
	return result
}

func sharedVar(reads, writes map[*types.Var]bool) *types.Var {
	var found *types.Var
	for v := range reads {
		if writes[v] && (found == nil || v.Pkg().Path()+"."+v.Name() < found.Pkg().Path()+"."+found.Name()) {
			found = v
		}
	}
	return found
}

func resolve(program *Program, changed []Declaration, wholePackages []string) change {
	c := change{funcs: map[*types.Func]bool{}, objects: map[types.Object]bool{}, types: map[*types.Named]bool{}, packages: map[string]bool{}}
	for _, path := range wholePackages {
		c.packages[path] = true
	}
	for _, declaration := range changed {
		pkg := program.byPath[declaration.Package]
		if pkg == nil || !pkg.Module {
			// Not linked into this test binary: it cannot run here.
			continue
		}
		if declaration.Initialization {
			if c.initialization == "" {
				c.initialization = declaration.Name
			}
			continue
		}
		scope := pkg.Types.Scope()
		typeName, method, isMethod := strings.Cut(declaration.Name, ".")
		object := scope.Lookup(typeName)
		if object == nil {
			// A removed declaration: every remaining reference to it was
			// rewritten, but a removed method can change interface
			// satisfaction, so a name not found keeps its package changed.
			c.packages[declaration.Package] = true
			continue
		}
		if !isMethod {
			switch typed := object.(type) {
			case *types.Func:
				c.funcs[typed] = true
			case *types.TypeName:
				if named, ok := types.Unalias(typed.Type()).(*types.Named); ok {
					c.types[named.Origin()] = true
				}
				c.objects[typed] = true
			default:
				c.objects[object] = true
			}
			continue
		}
		named, ok := types.Unalias(object.Type()).(*types.Named)
		if !ok {
			c.packages[declaration.Package] = true
			continue
		}
		found := false
		for i := 0; i < named.NumMethods(); i++ {
			if named.Method(i).Name() == method {
				c.funcs[named.Method(i)] = true
				found = true
			}
		}
		if !found {
			// An added or removed method changes the type's method set.
			c.types[named.Origin()] = true
		}
	}
	return c
}

type analysis struct {
	g          *graph
	c          change
	structural map[types.Type][]*types.Named
}

// closure is what one set of roots reaches.
type closure struct {
	touched string
	starts  []string
	process []string
	reads   map[*types.Var]bool
	writes  map[*types.Var]bool
}

func (a *analysis) reach(roots []*node) closure {
	out := closure{reads: map[*types.Var]bool{}, writes: map[*types.Var]bool{}}
	seen := map[*node]bool{}
	queue := append([]*node(nil), roots...)
	for _, root := range roots {
		seen[root] = true
	}
	visitFunc := func(from *node, f *types.Func) {
		if out.touched == "" && a.changedFunc(f) {
			out.touched = from.label + " -> " + f.FullName()
		}
		if next := a.g.funcs[f]; next != nil && !seen[next] {
			seen[next] = true
			queue = append(queue, next)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if out.touched == "" {
			if why := a.touches(n); why != "" {
				out.touched = n.label + " -> " + why
			}
		}
		out.starts = append(out.starts, n.starts...)
		out.process = append(out.process, n.process...)
		for v := range n.reads {
			out.reads[v] = true
			if held := a.g.held[v]; held != nil && !seen[held] {
				seen[held] = true
				queue = append(queue, held)
			}
		}
		for v := range n.writes {
			out.writes[v] = true
		}
		for f := range n.calls {
			visitFunc(n, f)
		}
		for t := range n.converted {
			for _, named := range a.namedIn(t) {
				for i := 0; i < named.NumMethods(); i++ {
					visitFunc(n, named.Method(i))
				}
			}
		}
		for name := range n.paramMethods {
			for _, f := range a.g.methods[name] {
				visitFunc(n, f)
			}
		}
	}
	sort.Strings(out.starts)
	sort.Strings(out.process)
	return out
}

func (a *analysis) changedFunc(f *types.Func) bool {
	return a.c.funcs[f] || (f.Pkg() != nil && a.c.packages[f.Pkg().Path()])
}

// touches names what in n's own code is changed, other than a call, which
// reach checks on the edge.
func (a *analysis) touches(n *node) string {
	for object := range n.mentions {
		if a.c.objects[object] || (object.Pkg() != nil && a.c.packages[object.Pkg().Path()]) {
			return object.Pkg().Path() + "." + object.Name()
		}
	}
	for v := range n.reads {
		if a.c.objects[v] || a.c.packages[v.Pkg().Path()] {
			return v.Pkg().Path() + "." + v.Name()
		}
	}
	if len(a.c.types) == 0 && len(a.c.packages) == 0 {
		return ""
	}
	for t := range n.exprTypes {
		for _, named := range a.namedIn(t) {
			if a.c.types[named] || a.c.packages[named.Obj().Pkg().Path()] {
				return "a value of " + named.Obj().Pkg().Path() + "." + named.Obj().Name()
			}
		}
	}
	return ""
}

// namedIn lists the module named types a value of t holds or points to,
// through every composite type, origin types for instances: the types whose
// methods a conversion of t to an interface lets other code call, reflection
// over nested values included.
func (a *analysis) namedIn(t types.Type) []*types.Named {
	if cached, ok := a.structural[t]; ok {
		return cached
	}
	a.structural[t] = nil
	seen := map[types.Type]bool{}
	var out []*types.Named
	var visit func(types.Type)
	visit = func(t types.Type) {
		t = types.Unalias(t)
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch typed := t.(type) {
		case *types.Named:
			origin := typed.Origin()
			if origin.Obj().Pkg() != nil && a.g.program.byPath[origin.Obj().Pkg().Path()] != nil && a.g.program.byPath[origin.Obj().Pkg().Path()].Module {
				out = append(out, origin)
			}
			if args := typed.TypeArgs(); args != nil {
				for i := 0; i < args.Len(); i++ {
					visit(args.At(i))
				}
			}
			visit(typed.Underlying())
		case *types.Pointer:
			visit(typed.Elem())
		case *types.Slice:
			visit(typed.Elem())
		case *types.Array:
			visit(typed.Elem())
		case *types.Map:
			visit(typed.Key())
			visit(typed.Elem())
		case *types.Chan:
			visit(typed.Elem())
		case *types.Struct:
			for i := 0; i < typed.NumFields(); i++ {
				visit(typed.Field(i).Type())
			}
		case *types.Tuple:
			for i := 0; i < typed.Len(); i++ {
				visit(typed.At(i).Type())
			}
		case *types.TypeParam:
			// A type parameter's values come from the instantiations, whose
			// type arguments the instantiating code records as converted.
		}
	}
	visit(t)
	a.structural[t] = out
	return out
}
