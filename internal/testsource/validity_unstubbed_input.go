package testsource

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// UNSTUBBED-OUTBOUND-INPUT reads (TSA-V0-169..171): the file's stub table is its literal request
// matchers (Playwright `X.route(glob, ...)` and MSW `http|rest.<method>(url, ...)`); an integration
// is a Playwright matcher path with a query parameter, and its stubbed values are the parameter's
// values across the matchers. MSW ignores the query when matching, so an MSW handler covers every
// value at its path. A test input naming that parameter with a value no matcher covers would reach
// the real outbound integration.

// uoStub is one stubbed value of one integration parameter, or a matcher without a query
// constraint (param empty). anyQuery marks an MSW handler. For an input, full is the request it
// makes: the literal URL, or the integration path with the one parameter.
type uoStub struct {
	path, param, value, full string
	line                     int
	anyQuery                 bool
}

var uoMSWMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "all": true, "head": true, "options": true}

// uoFindings reports every test input that names a stubbed integration parameter with a value no
// stub in the file matches (TSA-V0-169, TSA-V0-170). A file with any matcher that is not a literal
// string is not read, since that matcher may cover the value (TSA-V0-171).
func (s *source) uoFindings() []testvalidity.SourceFinding {
	stubs, calls, ok := s.uoStubTable()
	out := []testvalidity.SourceFinding{}
	if !ok {
		return out
	}
	sibling, crossFile := s.xfStubImport()
	if len(stubs) == 0 {
		if crossFile {
			s.uoAbstainUnread(sibling)
		}
		return out
	}
	for _, d := range s.decls {
		if d.kind != "test" {
			continue
		}
		for _, in := range s.uoInputs(d.body, calls, stubs) {
			if uoCovered(stubs, in) {
				continue
			}
			near := uoNearest(stubs, in)
			where := fmt.Sprintf("%s:%d", s.path, in.line)
			cause := fmt.Sprintf("%s:%d", s.path, near.line)
			test := fmt.Sprintf("%s:%d", s.path, d.line)
			message := fmt.Sprintf("input %s=%q reaches the stubbed integration %s, but no stub in this file matches that value, so the request goes to the real outbound integration and can fail only at run time (for example as a 500); the nearest stubbed value is %q at %s; use a stubbed value or add a stub for %q",
				in.param, in.value, in.path, near.value, cause, in.value)
			if crossFile {
				// A sibling stub module may declare a stub covering the value (TSA-V0-301).
				module, at := s.xfWhere(sibling, true, where)
				s.xfAbstain(testvalidity.RuleUnstubbedOutboundInput, testvalidity.AbstainStubTableCrossFile, test, where, at,
					message+fmt.Sprintf("; withheld: this file imports %s, which may declare a stub covering the value, and this reading does not follow imports", module))
				continue
			}
			out = append(out, testvalidity.SourceFinding{
				Rule: testvalidity.RuleUnstubbedOutboundInput, Severity: testvalidity.SeverityAdvisory,
				Test: test, Title: d.title, Location: where, Cause: cause, Message: message,
			})
		}
	}
	return out
}

// uoAbstainUnread records, for a file with tests but no stub of its own that imports a sibling
// module that may declare a stub table, one file-level abstention at that import: its stub table may be declared there, which
// this reading does not follow. The file is still not read for the rule (TSA-V0-171, TSA-V0-302).
func (s *source) uoAbstainUnread(sibling xfImport) {
	for _, d := range s.decls {
		if d.kind != "test" {
			continue
		}
		module, at := s.xfWhere(sibling, true, "")
		s.xfAbstain(testvalidity.RuleUnstubbedOutboundInput, testvalidity.AbstainStubTableCrossFile, at, at, at,
			fmt.Sprintf("this file declares no stub and imports %s, which may declare its stub table; this reading does not follow imports, so UNSTUBBED-OUTBOUND-INPUT is not read for this file", module))
		return
	}
}

// uoStubTable reads every literal matcher of the file and the token ranges of the stub calls. ok
// is false when some matcher is not a literal string.
func (s *source) uoStubTable() ([]uoStub, [][2]int, bool) {
	stubs, calls := []uoStub{}, [][2]int{}
	for i := range s.tokens {
		open := -1
		switch {
		case s.ident(i, "route") && s.memberAccess(i) && s.punct(i+1, "("):
			open = i + 1
		case (s.ident(i, "http") || s.ident(i, "rest")) && !s.memberAccess(i) && s.punct(i+1, ".") && s.ident(i+2, "") && uoMSWMethods[s.tokens[i+2].Text] && s.punct(i+3, "("):
			open = i + 3
		}
		if open < 0 || s.match[open] < 0 {
			continue
		}
		args := s.splitArgs(open, s.match[open])
		if len(args) == 0 {
			continue
		}
		if !s.loneString(args[0]) || !s.tokens[args[0][0]].Literal {
			return nil, nil, false
		}
		calls = append(calls, [2]int{open, s.match[open]})
		pattern, line := s.tokens[args[0][0]].Text, s.tokens[args[0][0]].Line
		base, query, _ := strings.Cut(pattern, "?")
		if !s.ident(i, "route") {
			stubs = append(stubs, uoStub{path: base, line: line, anyQuery: true})
			continue
		}
		values, err := url.ParseQuery(query)
		if err != nil {
			return nil, nil, false
		}
		for param, vs := range values {
			for _, v := range vs {
				stubs = append(stubs, uoStub{path: base, param: param, value: v, line: line})
			}
		}
		if query == "" {
			stubs = append(stubs, uoStub{path: base, line: line})
		}
	}
	return stubs, calls, true
}

// uoInputs reads the inputs of one test body outside every stub call: an object property
// `param: 'value'` naming a stubbed parameter, and a literal URL whose path a matcher names with
// a query value for a stubbed parameter.
func (s *source) uoInputs(body [2]int, calls [][2]int, stubs []uoStub) []uoStub {
	params := map[string]string{}
	for _, st := range stubs {
		if st.param != "" {
			params[st.param] = st.path
		}
	}
	out := []uoStub{}
	for j := body[0]; j < body[1]; j++ {
		if uoInside(j, calls) {
			continue
		}
		t := s.tokens[j]
		if (s.ident(j, "") || s.loneString([2]int{j, j + 1})) && s.punct(j+1, ":") && s.loneString([2]int{j + 2, j + 3}) && s.tokens[j+2].Literal && (s.punct(j+3, ",") || s.punct(j+3, "}")) {
			if p, ok := params[t.Text]; ok {
				v := s.tokens[j+2].Text
				out = append(out, uoStub{path: p, param: t.Text, value: v, full: p + "?" + url.Values{t.Text: {v}}.Encode(), line: t.Line})
			}
			continue
		}
		if (t.Kind != appmap.JSString && t.Kind != appmap.JSTemplate) || !t.Literal || s.punct(j+1, ":") {
			continue
		}
		base, query, found := strings.Cut(t.Text, "?")
		values, err := url.ParseQuery(query)
		if !found || err != nil {
			continue
		}
		for param, vs := range values {
			for _, st := range stubs {
				if st.param == param && uoPathMatches(st.path, base) {
					for _, v := range vs {
						out = append(out, uoStub{path: st.path, param: param, value: v, full: t.Text, line: t.Line})
					}
					break
				}
			}
		}
	}
	return out
}

func uoInside(j int, ranges [][2]int) bool {
	for _, r := range ranges {
		if j >= r[0] && j <= r[1] {
			return true
		}
	}
	return false
}

// uoPathMatches reports a literal request path that a matcher path names: equal, or a suffix of
// the request after a leading `**`.
func uoPathMatches(matcher, request string) bool {
	if matcher == request {
		return true
	}
	suffix, ok := strings.CutPrefix(matcher, "**")
	return ok && suffix != "" && strings.HasSuffix(request, suffix)
}

// uoCovered reports an input some matcher covers: a matcher of the same path whose value glob
// matches it, an MSW handler at its path, or a Playwright matcher without a query constraint whose
// glob matches the request.
func uoCovered(stubs []uoStub, in uoStub) bool {
	for _, st := range stubs {
		switch {
		case st.param != "":
			if st.path == in.path && st.param == in.param && uoGlobMatch(st.value, in.value) {
				return true
			}
		case st.anyQuery:
			if uoPathMatches(st.path, in.path) || uoGlobMatch(st.path, strings.SplitN(in.full, "?", 2)[0]) {
				return true
			}
		case uoGlobMatch(st.path, in.full):
			return true
		}
	}
	return false
}

// uoGlobMatch reports text matching a Playwright URL glob: `**` matches any characters, `*` any
// characters but `/`, and `{a,b}` either alternative. A glob it cannot read covers the text, so
// the rule stays silent rather than warn on a request a matcher may cover.
func uoGlobMatch(glob, text string) bool {
	var b strings.Builder
	b.WriteString("^")
	braces := 0
	for i := 0; i < len(glob); i++ {
		switch c := glob[i]; {
		case c == '*' && i+1 < len(glob) && glob[i+1] == '*':
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '{':
			braces++
			b.WriteString("(?:")
		case c == '}' && braces > 0:
			braces--
			b.WriteString(")")
		case c == ',' && braces > 0:
			b.WriteString("|")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	return err != nil || braces != 0 || re.MatchString(text)
}

// uoNearest returns the stubbed value of the input's parameter closest to its value by edit
// distance, the first on a tie.
func uoNearest(stubs []uoStub, in uoStub) uoStub {
	best, distance := uoStub{}, -1
	for _, st := range stubs {
		if st.path != in.path || st.param != in.param {
			continue
		}
		if d := uoDistance(st.value, in.value); distance < 0 || d < distance {
			best, distance = st, d
		}
	}
	return best
}

// uoDistance is the Levenshtein distance between a and b over bytes.
func uoDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
