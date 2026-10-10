package obligation

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Preflight finding kinds (TOL-V0-025..027). The plan check adds the
// TOL-V0-020 kinds unchanged.
const (
	FindingUnnamed         = "UNNAMED"
	FindingMixed           = "MIXED_EXPECTED_FAIL"
	FindingSplitTests      = "SPLIT_TESTS"
	FindingPlanMissing     = "PLAN_MISSING"
	FindingDeepCheckFailed = "DEEP_CHECK_FAILED"
	// FindingUnreadableSpecPath is a spec file whose path holds a line
	// break, which preflight cannot ask Git for (TOL-V0-025).
	FindingUnreadableSpecPath = "UNREADABLE_SPEC_PATH"
)

// Remedies of the source findings.
const (
	UnnamedRemedy    = "name the obligation in a test title or test.step title, or as <contract>:<id> in a contract annotation, and commit it"
	SplitTestsRemedy = "consolidate these one-obligation tests into one test with a test.step per obligation, or annotate a test that needs its own state with type 'isolated'"
)

// Finding is one preflight problem. Line is 0 and Path "" when the finding
// has no source position.
type Finding struct {
	Kind, ID, Path string
	Line           int
	Detail, Remedy string
}

// SortFindings orders findings by kind, id, path and line.
func SortFindings(f []Finding) {
	sort.Slice(f, func(i, j int) bool {
		a, b := f[i], f[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
}

var specFile = regexp.MustCompile(`\.(spec|test)\.(ts|tsx|js|jsx|mjs|cjs|mts|cts)$`)

// SpecFile reports whether path names a Playwright-style spec file
// (`*.spec.*` or `*.test.*` with a JavaScript or TypeScript extension).
func SpecFile(path string) bool { return specFile.MatchString(path) }

// lexeme kinds.
const (
	lexIdent  = 'i'
	lexString = 's'
	lexPunct  = 'p'
	lexOther  = 'n'
)

type lexeme struct {
	kind byte
	text string
	line int
}

// regexKeywords may precede a regular expression literal.
var regexKeywords = map[string]bool{"return": true, "typeof": true, "case": true, "do": true, "else": true, "in": true,
	"of": true, "new": true, "delete": true, "void": true, "throw": true, "yield": true, "await": true, "instanceof": true}

// lex splits JavaScript or TypeScript source into identifiers, string and
// template literals (their text, interpolations dropped) and punctuation,
// skipping comments and regular expression literals. It is a heuristic
// tokenizer, not a parser: malformed source still yields tokens.
func lex(src string) []lexeme {
	var out []lexeme
	line := 1
	n := len(src)
	prevAllowsRegex := func() bool {
		if len(out) == 0 {
			return true
		}
		p := out[len(out)-1]
		switch p.kind {
		case lexPunct:
			return p.text != ")" && p.text != "]" && p.text != "}"
		case lexIdent:
			return regexKeywords[p.text]
		}
		return false
	}
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '/' && prevAllowsRegex():
			i++
			class := false
			for i < n && src[i] != '\n' {
				if src[i] == '\\' {
					i += 2
					continue
				}
				if src[i] == '[' {
					class = true
				} else if src[i] == ']' {
					class = false
				} else if src[i] == '/' && !class {
					i++
					break
				}
				i++
			}
			for i < n && isIdentByte(src[i]) {
				i++
			}
			out = append(out, lexeme{kind: lexOther, text: "/re/", line: line})
		case c == '\'' || c == '"':
			start := line
			var b strings.Builder
			i++
			for i < n && src[i] != c && src[i] != '\n' {
				if src[i] == '\\' && i+1 < n {
					if src[i+1] == '\n' {
						line++
					}
					b.WriteByte(src[i+1])
					i += 2
					continue
				}
				b.WriteByte(src[i])
				i++
			}
			i++
			out = append(out, lexeme{kind: lexString, text: b.String(), line: start})
		case c == '`':
			start := line
			var b strings.Builder
			i++
			for i < n && src[i] != '`' {
				switch {
				case src[i] == '\\' && i+1 < n:
					if src[i+1] == '\n' {
						line++
					}
					b.WriteByte(src[i+1])
					i += 2
				case src[i] == '$' && i+1 < n && src[i+1] == '{':
					depth := 0
					for i < n {
						if src[i] == '{' {
							depth++
						} else if src[i] == '}' {
							depth--
							if depth == 0 {
								i++
								break
							}
						} else if src[i] == '\n' {
							line++
						}
						i++
					}
					b.WriteByte(' ')
				default:
					if src[i] == '\n' {
						line++
					}
					b.WriteByte(src[i])
					i++
				}
			}
			i++
			out = append(out, lexeme{kind: lexString, text: b.String(), line: start})
		case isIdentStart(c):
			j := i
			for j < n && isIdentByte(src[j]) {
				j++
			}
			out = append(out, lexeme{kind: lexIdent, text: src[i:j], line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (isIdentByte(src[j]) || src[j] == '.') {
				j++
			}
			out = append(out, lexeme{kind: lexOther, text: src[i:j], line: line})
			i = j
		case c < 0x80:
			out = append(out, lexeme{kind: lexPunct, text: string(c), line: line})
			i++
		default:
			i++
		}
	}
	return out
}

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
}

func isIdentByte(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

// node kinds.
const (
	nodeDescribe = iota
	nodeTest
	nodeStep
	nodeFail
)

// node is one recognized call: its kind, title, declaration line and the
// token range of its parentheses.
type node struct {
	kind        int
	title       string
	line        int
	open, close int
	fail        bool
	failDesc    string
	isolated    bool
	parentTest  int
	parentDescr int
}

// testModifiers may sit between `test` and the call of a declaration.
var testModifiers = map[string]bool{"only": true, "skip": true, "fixme": true, "fail": true}

// testRoot reports whether a call chain's root is a Playwright test object:
// `test`, `it`, or an extended fixture named `*Test`.
func testRoot(name string) bool {
	return name == "test" || name == "it" || strings.HasSuffix(name, "Test")
}

// parseNodes recognizes describe blocks, test declarations, test.step calls
// and test.fail(...) annotation calls in toks.
func parseNodes(toks []lexeme) []node {
	var nodes []node
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind != lexIdent || (i > 0 && toks[i-1].kind == lexPunct && toks[i-1].text == ".") || !testRoot(t.text) {
			continue
		}
		segs := []string{t.text}
		j := i + 1
		for j+1 < len(toks) && toks[j].kind == lexPunct && toks[j].text == "." && toks[j+1].kind == lexIdent {
			segs = append(segs, toks[j+1].text)
			j += 2
		}
		if j >= len(toks) || toks[j].kind != lexPunct || toks[j].text != "(" {
			continue
		}
		closeAt, args := callArgs(toks, j)
		title, hasTitle := "", false
		if len(args) > 0 && args[0][1]-args[0][0] == 1 && toks[args[0][0]].kind == lexString {
			title, hasTitle = toks[args[0][0]].text, true
		}
		n := node{title: title, line: t.line, open: j, close: closeAt, parentTest: -1, parentDescr: -1}
		rest := segs[1:]
		switch {
		case contains(rest, "describe"):
			if !hasTitle {
				continue
			}
			n.kind = nodeDescribe
			n.isolated = isolatedArgs(toks, args, title)
		case len(rest) > 0 && rest[len(rest)-1] == "step":
			if !hasTitle {
				continue
			}
			n.kind = nodeStep
		case allModifiers(rest) && hasTitle && len(args) >= 2:
			n.kind = nodeTest
			n.fail = contains(rest, "fail")
			n.isolated = isolatedArgs(toks, args, title)
		case len(rest) == 1 && rest[0] == "fail":
			n.kind = nodeFail
			for _, a := range args {
				if a[1]-a[0] == 1 && toks[a[0]].kind == lexString {
					n.failDesc = toks[a[0]].text
					break
				}
			}
		default:
			continue
		}
		nodes = append(nodes, n)
	}
	// Nodes are in order of their opening parenthesis, so the innermost
	// enclosing test and describe are found with one stack.
	var stack []int
	for k := range nodes {
		for len(stack) > 0 && nodes[stack[len(stack)-1]].close < nodes[k].open {
			stack = stack[:len(stack)-1]
		}
		for s := len(stack) - 1; s >= 0; s-- {
			p := nodes[stack[s]]
			if p.kind == nodeTest && nodes[k].parentTest < 0 {
				nodes[k].parentTest = stack[s]
			}
			if p.kind == nodeDescribe && nodes[k].parentDescr < 0 {
				nodes[k].parentDescr = stack[s]
			}
		}
		if nodes[k].kind == nodeTest || nodes[k].kind == nodeDescribe {
			stack = append(stack, k)
		}
	}
	return nodes
}

// callArgs returns the index of the parenthesis closing the one at open and
// the token ranges [start, end) of its top-level arguments.
func callArgs(toks []lexeme, open int) (int, [][2]int) {
	var args [][2]int
	depth, start := 0, open+1
	for k := open; k < len(toks); k++ {
		if toks[k].kind != lexPunct {
			continue
		}
		switch toks[k].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				if k > start {
					args = append(args, [2]int{start, k})
				}
				return k, args
			}
		case ",":
			if depth == 1 {
				args = append(args, [2]int{start, k})
				start = k + 1
			}
		}
	}
	if len(toks) > start {
		args = append(args, [2]int{start, len(toks)})
	}
	return len(toks), args
}

// isolatedArgs reports whether a declaration opts out of the split check:
// its title carries the tag @isolated, or a string literal "isolated" or
// "@isolated" (an annotation type or tag) sits in its arguments before the
// body.
func isolatedArgs(toks []lexeme, args [][2]int, title string) bool {
	for _, f := range strings.Fields(title) {
		if f == "@isolated" {
			return true
		}
	}
	for _, a := range args[:max(len(args)-1, 0)] {
		for k := a[0]; k < a[1]; k++ {
			if toks[k].kind == lexString && (toks[k].text == "isolated" || toks[k].text == "@isolated") {
				return true
			}
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func allModifiers(rest []string) bool {
	for _, s := range rest {
		if !testModifiers[s] {
			return false
		}
	}
	return true
}

// contractPattern matches `<contract>:<id>` for ids of prefix: a contract
// name of 1..64 bytes `[A-Za-z][A-Za-z0-9_.-]*` that starts a token.
func contractPattern(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9_.\-])[A-Za-z][A-Za-z0-9_.\-]{0,63}:(` + regexp.QuoteMeta(prefix) + `-[A-Za-z0-9\-]*)`)
}

// contractIDs returns the ids that s names as `<contract>:<id>`.
func contractIDs(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if _, err := wire.ParseObligationID("id", m[1]); err == nil {
			out = append(out, m[1])
		}
	}
	return out
}

// sourceTest is one test declaration with what it names: as in runtime
// witnessing, its title path is its describe titles (describes, outermost
// first) and its own title.
type sourceTest struct {
	path, title string
	line        int
	describe    int
	fail        bool
	failDesc    string
	isolated    bool
	describes   []string
	titleIDs    []string
	steps       []node
}

// CheckSources runs the TOL-V0-025 source checks of l's OPEN, DEFECT and
// BLOCKED obligations over the spec files in files (path to content):
// UNNAMED, MIXED_EXPECTED_FAIL and SPLIT_TESTS findings, sorted.
func CheckSources(l *ticket.ObligationLedger, files map[string][]byte) []Finding {
	open := map[string]string{}
	for _, e := range l.Entries {
		if e.State == ticket.ObligationOpen || e.State == ticket.ObligationDefect || e.State == ticket.ObligationBlocked {
			open[e.ID] = e.State
		}
	}
	named := map[string]bool{}
	var findings []Finding
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	contract := contractPattern(l.Prefix)
	for _, path := range paths {
		toks := lex(string(files[path]))
		for _, t := range toks {
			if t.kind == lexString {
				for _, id := range contractIDs(contract, t.text) {
					named[id] = true
				}
			}
		}
		nodes := parseNodes(toks)
		tests := fileTests(path, nodes, l.Prefix)
		for _, n := range nodes {
			if n.kind != nodeFail {
				for _, id := range FindIDs(n.title, l.Prefix) {
					named[id] = true
				}
			}
		}
		findings = append(findings, mixedFindings(tests, open, l.Prefix)...)
		findings = append(findings, splitFindings(tests, nodes, open, l.Prefix)...)
	}
	for id := range open {
		if !named[id] {
			findings = append(findings, Finding{Kind: FindingUnnamed, ID: id,
				Detail: "no test title, test.step title or contract annotation in a spec file at the commit names " + id, Remedy: UnnamedRemedy})
		}
	}
	SortFindings(findings)
	return findings
}

// fileTests gathers each test declaration with its steps and the test.fail
// annotation that covers it: its own declaration or call, else the
// innermost describe's, else a file-level call.
func fileTests(path string, nodes []node, prefix string) []sourceTest {
	fileFail, fileDesc := false, ""
	describeFail := map[int]string{}
	testFail := map[int]string{}
	for _, n := range nodes {
		if n.kind != nodeFail {
			continue
		}
		switch {
		case n.parentTest >= 0:
			testFail[n.parentTest] = testFail[n.parentTest] + " " + n.failDesc
		case n.parentDescr >= 0:
			describeFail[n.parentDescr] = describeFail[n.parentDescr] + " " + n.failDesc
		default:
			fileFail, fileDesc = true, fileDesc+" "+n.failDesc
		}
	}
	var tests []sourceTest
	index := map[int]int{}
	for k, n := range nodes {
		if n.kind != nodeTest {
			continue
		}
		st := sourceTest{path: path, title: n.title, line: n.line, describe: n.parentDescr, fail: n.fail || fileFail,
			failDesc: fileDesc, isolated: n.isolated, titleIDs: FindIDs(n.title, prefix)}
		if d, ok := testFail[k]; ok {
			st.fail, st.failDesc = true, st.failDesc+d
		}
		for d := n.parentDescr; d >= 0; d = nodes[d].parentDescr {
			if desc, ok := describeFail[d]; ok {
				st.fail, st.failDesc = true, st.failDesc+desc
			}
			if nodes[d].isolated {
				st.isolated = true
			}
			st.describes = append([]string{nodes[d].title}, st.describes...)
		}
		index[k] = len(tests)
		tests = append(tests, st)
	}
	for _, n := range nodes {
		if n.kind == nodeStep && n.parentTest >= 0 {
			if t, ok := index[n.parentTest]; ok {
				tests[t].steps = append(tests[t].steps, n)
			}
		}
	}
	return tests
}

// mixedFindings reports each open obligation that a test.fail test names
// without marking it expected-fail (TOL-V0-025, the static form of
// TOL-V0-023). An id is expected-fail there when its ledger state is DEFECT,
// the title naming it says expected-fail, the fail description names it, or,
// for a title-path id (a describe or the test title), the fail description
// or that title names a defect id.
func mixedFindings(tests []sourceTest, open map[string]string, prefix string) []Finding {
	var out []Finding
	for _, t := range tests {
		if !t.fail {
			continue
		}
		described := map[string]bool{}
		for _, id := range FindIDs(t.failDesc, prefix) {
			described[id] = true
		}
		describedDefect := len(DefectIDs(t.failDesc, prefix)) > 0
		seen := map[string]bool{}
		check := func(id, title string, titleID bool) {
			state, ok := open[id]
			if !ok || seen[id] {
				return
			}
			defect := titleID && (describedDefect || len(DefectIDs(title, prefix)) > 0)
			if state == ticket.ObligationDefect || SaysExpectedFail(title) || described[id] || defect {
				return
			}
			seen[id] = true
			out = append(out, Finding{Kind: FindingMixed, ID: id, Path: t.path, Line: t.line,
				Detail: "test.fail test " + quote(t.title) + " also names " + id + ", which is not marked expected-fail", Remedy: MixedRemedy})
		}
		for _, title := range t.describes {
			for _, id := range FindIDs(title, prefix) {
				check(id, title, true)
			}
		}
		for _, id := range t.titleIDs {
			check(id, t.title, true)
		}
		for _, s := range t.steps {
			for _, id := range FindIDs(s.title, prefix) {
				check(id, s.title, false)
			}
		}
	}
	return out
}

// splitFindings reports tests in one file and describe scope that each name
// exactly one open obligation in their title path and steps, when two or
// more distinct obligations are split that way; test.fail and isolated
// tests are exempt (TOL-V0-025).
func splitFindings(tests []sourceTest, nodes []node, open map[string]string, prefix string) []Finding {
	type single struct {
		t  sourceTest
		id string
	}
	groups := map[int][]single{}
	var order []int
	for _, t := range tests {
		if t.fail || t.isolated {
			continue
		}
		ids := map[string]bool{}
		for _, title := range t.describes {
			for _, id := range FindIDs(title, prefix) {
				if _, ok := open[id]; ok {
					ids[id] = true
				}
			}
		}
		for _, id := range t.titleIDs {
			if _, ok := open[id]; ok {
				ids[id] = true
			}
		}
		for _, s := range t.steps {
			for _, id := range FindIDs(s.title, prefix) {
				if _, ok := open[id]; ok {
					ids[id] = true
				}
			}
		}
		if len(ids) != 1 {
			continue
		}
		for id := range ids {
			if _, ok := groups[t.describe]; !ok {
				order = append(order, t.describe)
			}
			groups[t.describe] = append(groups[t.describe], single{t, id})
		}
	}
	var out []Finding
	for _, d := range order {
		g := groups[d]
		distinct := map[string]bool{}
		for _, s := range g {
			distinct[s.id] = true
		}
		if len(distinct) < 2 {
			continue
		}
		scope := "file scope"
		if d >= 0 {
			scope = "describe " + quote(nodes[d].title)
		}
		for _, s := range g {
			out = append(out, Finding{Kind: FindingSplitTests, ID: s.id, Path: s.t.path, Line: s.t.line,
				Detail: "test " + quote(s.t.title) + " is one of " + itoa(len(g)) + " tests in " + scope + " that each name one obligation", Remedy: SplitTestsRemedy})
		}
	}
	return out
}

// quote renders an untrusted title as a bounded, screened excerpt.
func quote(s string) string { return "'" + Excerpt(s) + "'" }

func itoa(n int) string { return string(wire.CountOf(int64(n))) }
