package testdiscovery

import (
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

var (
	// jsRunMembers are the declaration members the reader reads (TSA-V0-073).
	jsRunMembers = map[string]bool{"only": true, "skip": true, "todo": true, "each": true, "concurrent": true,
		"sequential": true, "fixme": true, "serial": true, "parallel": true}
	// jsUnsupportedMembers make an example or describe a shape the reader does not decide.
	jsUnsupportedMembers = map[string]bool{"failing": true, "fails": true, "fail": true, "skipIf": true, "runIf": true}
	JSModuleMocks        = map[string]bool{"mock": true, "doMock": true, "unstable_mockModule": true}
	JSTimerMocks         = map[string]bool{"useFakeTimers": true, "setSystemTime": true}
	JSBoundaryModules    = map[string]bool{"axios": true, "node-fetch": true, "cross-fetch": true, "got": true, "superagent": true,
		"undici": true, "ky": true, "http": true, "https": true}
	JSBoundaryGlobals = map[string]bool{"global": true, "globalThis": true, "window": true}
	JSBoundarySpied   = map[string]bool{"fetch": true, "setTimeout": true, "setInterval": true}
)

// JSDecl is one example or describe the reader placed.
type JSDecl struct {
	Kind        string // "test" or "describe"
	Root        string
	At, Line    int
	EndLine     int // the line of the declaring call's closing parenthesis
	Members     map[string]bool
	unsupported bool
	Title       string
	// literalTitle marks a title that is one exact string literal on a declaration with no
	// `.each` table, so it names the example or group as the runner does (LPCV-V0-147).
	literalTitle bool
	EachTable    [2]int // the `.each(...)` argument range, or the template token as [i, i+1]
	EachKind     string // "call", "template" or ""
	HasBody      bool
	Body         [2]int
	Concise      bool // the body is an arrow function's expression, which the example returns
	NoParams     bool // the callback declares no parameter, so it takes no `done` (TSA-V0-389)
}

// jsAnnotation is one runtime `test.skip()` or `test.fixme()` call with no title.
type jsAnnotation struct {
	At, Line      int
	Unconditional bool
}

// jsBinding is one imported identifier and the specifier it comes from.
type jsBinding struct {
	Spec string
	line int
	// imported is the exported name an `import { X as Y }` binding takes (X), or the local name.
	imported string
}

type JSReader struct {
	S           *Source
	Name        string
	Decls       []JSDecl
	Annotations []jsAnnotation
	Bindings    map[string]jsBinding
	// SubjectSpecs and subjectNames are the TSA-V0-079 subject modules and identifiers.
	SubjectSpecs, SubjectNames map[string]bool
	// FlowOwner maps tokens to their example for TSA-V0-120..130, built on first use.
	FlowOwner []int
	// FlowSubjects caches each describe's subject locals for TSA-V0-120..130.
	FlowSubjects map[int]map[string]bool
}

// Read places every example, describe and runtime annotation, and the file's import bindings.
func (r *JSReader) Read() {
	s := r.S
	for i := range s.Tokens {
		if !s.Ident(i, "") || s.MemberAccess(i) || s.Ident(i-1, "function") {
			continue
		}
		root := s.Tokens[i].Text
		if !testRoots[root] && !describeRoots[root] {
			continue
		}
		d := JSDecl{Kind: "test", Root: root, At: i, Line: s.Tokens[i].Line, Members: map[string]bool{}}
		if describeRoots[root] {
			d.Kind = "describe"
		}
		j, ignore := i+1, false
		for !ignore && s.Punct(j, ".") && s.Ident(j+1, "") {
			member := s.Tokens[j+1].Text
			j += 2
			switch {
			case member == "describe" && testRoots[root]:
				d.Kind = "describe"
			case jsRunMembers[member]:
				d.Members[member] = true
			case jsUnsupportedMembers[member]:
				d.unsupported = true
				// skipIf(condition)(title, fn): step over the condition call.
				if s.Punct(j, "(") && s.Match[j] > j && s.Punct(s.Match[j]+1, "(") {
					j = s.Match[j] + 1
				}
			default:
				ignore = true
			}
			if member != "each" || ignore {
				continue
			}
			switch {
			case s.Punct(j, "(") && s.Match[j] > j:
				d.EachTable, d.EachKind = [2]int{j + 1, s.Match[j]}, "call"
				j = s.Match[j] + 1
			case j < len(s.Tokens) && s.Tokens[j].Kind == appmap.JSTemplate:
				d.EachTable, d.EachKind = [2]int{j, j + 1}, "template"
				j++
			default:
				ignore = true
			}
		}
		if ignore || !s.Punct(j, "(") || s.Match[j] < 0 {
			continue
		}
		d.EndLine = s.Tokens[s.Match[j]].Line
		args := s.SplitArgs(j, s.Match[j])
		titled := len(args) > 0 && s.LoneString(args[0])
		if titled {
			d.Title = titleText(s.Tokens[args[0][0]])
			d.literalTitle = s.Tokens[args[0][0]].Literal && d.EachKind == ""
		}
		for k := 1; k < len(args); k++ {
			// functionBody also takes a typed arrow, `(): void => {…}`, which callback does not.
			body, concise, ok := s.FunctionBody(args[k][0])
			ok = ok && body[1] <= args[k][1]
			if !ok && !s.Callback(args[k]) {
				continue
			}
			if ok {
				d.Body, d.HasBody, d.Concise, d.NoParams = body, true, concise, s.noParams(args[k][0])
			}
			break
		}
		if d.Kind == "test" && !titled && !d.HasBody && (d.Members["skip"] || d.Members["fixme"]) {
			unconditional := len(args) == 0 || args[0][1]-args[0][0] == 1 && s.Ident(args[0][0], "true")
			r.Annotations = append(r.Annotations, jsAnnotation{At: i, Line: d.Line, Unconditional: unconditional})
			continue
		}
		if d.Kind == "describe" && !d.HasBody {
			continue
		}
		r.Decls = append(r.Decls, d)
	}
	r.Bindings = s.importBindings()
	r.SubjectSpecs, r.SubjectNames = r.subjects()
}

// Abstention returns why the reader does not decide example e, or "".
func (r *JSReader) Abstention(e JSDecl, sameLine int) string {
	if sameLine > 1 {
		return testvalidity.HygieneReasonJSLineAmbiguous
	}
	if e.unsupported {
		return testvalidity.HygieneReasonJSShapeUnsupported
	}
	for _, g := range r.Ancestors(e.At) {
		if r.Decls[g].unsupported {
			return testvalidity.HygieneReasonJSShapeUnsupported
		}
	}
	if !e.Members["todo"] && !e.HasBody {
		return testvalidity.HygieneReasonJSBodyNotLiteral
	}
	return ""
}

// TitlePath returns example e's literal title path, outermost describe first and e's own title
// last (LPCV-V0-147), or nil when any title on it is not one literal string.
func (r *JSReader) TitlePath(e JSDecl) []string {
	if !e.literalTitle {
		return nil
	}
	path := []string{e.Title}
	for _, g := range r.Ancestors(e.At) {
		if !r.Decls[g].literalTitle {
			return nil
		}
		path = append(path, r.Decls[g].Title)
	}
	slices.Reverse(path)
	return path
}

// Ancestors returns the indexes of the describes whose bodies hold token i, innermost first.
func (r *JSReader) Ancestors(i int) []int {
	out := []int{}
	for k, d := range r.Decls {
		if d.Kind == "describe" && d.At < i && Within(i, d.Body) {
			out = append(out, k)
		}
	}
	slices.Reverse(out)
	return out
}

// InExampleBody reports token i inside some example's body other than the one at token except.
func (r *JSReader) InExampleBody(i, except int) bool {
	for _, d := range r.Decls {
		if d.Kind == "test" && d.At != except && d.HasBody && Within(i, d.Body) {
			return true
		}
	}
	return false
}

// JSAssertion reports an assertion at token k: a TSA-V0-013 assertion call, an `assert.` or
// `expect.` member call, or a `.should` member (TSA-V0-077).
func (s *Source) JSAssertion(k int) bool {
	if s.AssertionCount(k) {
		return false
	}
	if s.IsAssertion(k) {
		return true
	}
	if !s.Ident(k, "") {
		return false
	}
	switch s.Tokens[k].Text {
	case "assert", "expect":
		return !s.MemberAccess(k) && s.Punct(k+1, ".") && s.Ident(k+2, "") && s.Punct(k+3, "(")
	case "should":
		return s.MemberAccess(k)
	}
	return false
}

// AssertionCount reports `expect.assertions(` or `expect.hasAssertions(` at token k, either at
// `expect` or at the member: a declared count, not an assertion (TSA-V0-077).
func (s *Source) AssertionCount(k int) bool {
	counted := func(i int) bool {
		return s.Ident(i, "") && (s.Tokens[i].Text == "assertions" || s.Tokens[i].Text == "hasAssertions") && s.Punct(i+1, "(")
	}
	if s.Ident(k, "expect") && !s.MemberAccess(k) && s.Punct(k+1, ".") && counted(k+2) {
		return true
	}
	return s.MemberAccess(k) && s.Ident(k-2, "expect") && !s.MemberAccess(k-2) && counted(k)
}

func IsRelativeSpec(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// importBindings maps each identifier an `import` declaration or a top-level `require` binds to
// its specifier.
func (s *Source) importBindings() map[string]jsBinding {
	out := map[string]jsBinding{}
	literal := func(k int) bool {
		return k < len(s.Tokens) && s.Tokens[k].Kind == appmap.JSString && s.Tokens[k].Literal
	}
	for i := range s.Tokens {
		switch {
		case s.Ident(i, "import") && !s.MemberAccess(i) && !s.Punct(i+1, "(") && !s.Punct(i+1, "."):
			names := []int{}
			for k := i + 1; k < len(s.Tokens) && k < i+256; k++ {
				if s.Ident(k, "from") && literal(k+1) {
					for _, n := range names {
						imported := s.Tokens[n].Text
						if n >= 2 && s.Ident(n-1, "as") && s.Ident(n-2, "") {
							imported = s.Tokens[n-2].Text
						}
						out[s.Tokens[n].Text] = jsBinding{Spec: s.Tokens[k+1].Text, line: s.Tokens[i].Line, imported: imported}
					}
					break
				}
				if s.Tokens[k].Kind == appmap.JSString || s.Punct(k, ";") {
					break
				}
				if s.Ident(k, "") && !s.Ident(k, "type") && !s.Ident(k, "as") && !s.Ident(k+1, "as") {
					names = append(names, k)
				}
			}
		case (s.Ident(i, "const") || s.Ident(i, "let") || s.Ident(i, "var")) && !s.MemberAccess(i):
			names, j := []int{}, i+1
			switch {
			case s.Ident(j, ""):
				names, j = append(names, j), j+1
			case s.Punct(j, "{") && s.Match[j] > j:
				for k := j + 1; k < s.Match[j]; k++ {
					if s.Ident(k, "") && !s.Punct(k+1, ":") {
						names = append(names, k)
					}
				}
				j = s.Match[j] + 1
			}
			if s.Punct(j, "=") && s.Ident(j+1, "require") && s.Punct(j+2, "(") && literal(j+3) && s.Punct(j+4, ")") {
				for _, n := range names {
					imported := s.Tokens[n].Text
					if s.Punct(n-1, ":") && s.Ident(n-2, "") {
						// `const { X: Y } = require(...)` binds Y to the exported X.
						imported = s.Tokens[n-2].Text
					}
					out[s.Tokens[n].Text] = jsBinding{Spec: s.Tokens[j+3].Text, line: s.Tokens[i].Line, imported: imported}
				}
			}
		}
	}
	return out
}

// SpecStem is a module specifier's base name without its extension, or its directory's name for
// an index module.
func SpecStem(spec string) string {
	base := path.Base(spec)
	if ext := path.Ext(base); WebExtensions[ext] {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "index" {
		return path.Base(path.Dir(spec))
	}
	return base
}

func BoundaryModule(spec string) bool {
	return JSBoundaryModules[strings.TrimPrefix(spec, "node:")]
}

// subjectWord folds a stem or identifier for the TSA-V0-079 (c) comparison: lower case, without
// `-`, `_` or `.`, so `user-service` and `UserService` agree.
func subjectWord(name string) string {
	return strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(name))
}

// subjects returns the subject module specifiers and the identifiers that name the subject
// (TSA-V0-079).
func (r *JSReader) subjects() (map[string]bool, map[string]bool) {
	stem := strings.TrimSuffix(path.Base(r.Name), path.Ext(r.Name))
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".spec"), ".test")
	specs, names := map[string]bool{}, map[string]bool{}
	for _, b := range r.Bindings {
		switch {
		case BoundaryModule(b.Spec):
		case SpecStem(b.Spec) == stem:
			specs[b.Spec] = true
		case IsRelativeSpec(b.Spec) && b.imported != "" && subjectWord(b.imported) == subjectWord(stem):
			// (c) a relative module, such as a barrel, that exports the stem's name (V1-1862).
			specs[b.Spec] = true
		}
	}
	locals := make([]string, 0, len(r.Bindings))
	for local := range r.Bindings {
		locals = append(locals, local)
	}
	slices.Sort(locals)
	// importedAs finds the first local, in name order, whose exported name is title, for
	// `import { X as Y }`.
	importedAs := func(title string) (string, jsBinding, bool) {
		for _, local := range locals {
			if b := r.Bindings[local]; b.imported == title {
				return local, b, true
			}
		}
		return "", jsBinding{}, false
	}
	s := r.S
	for _, d := range r.Decls {
		if d.Kind != "describe" || len(r.Ancestors(d.At)) != 0 {
			continue
		}
		name := d.Title
		if first := d.At + 1; name == "" {
			for !s.Punct(first, "(") && first < len(s.Tokens) {
				first++
			}
			if s.Ident(first+1, "") && (s.Punct(first+2, ",") || s.Punct(first+2, ")")) {
				name = s.Tokens[first+1].Text
			}
		}
		b, ok := r.Bindings[name]
		// Only a string title can name an exported X; an identifier argument is a local binding.
		if !ok && d.Title != "" {
			var local string
			if local, b, ok = importedAs(name); ok {
				names[local] = true
			}
		}
		if ok && !BoundaryModule(b.Spec) {
			specs[b.Spec] = true
			names[name] = true
		}
	}
	for n, b := range r.Bindings {
		if specs[b.Spec] {
			names[n] = true
		}
	}
	return specs, names
}
