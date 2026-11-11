package testdiscovery

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/lineanchor"
)

var (
	rspecWordHead      = regexp.MustCompile(`^(?:RSpec\.)?\w+[?!]?`)
	rspecHookHead      = regexp.MustCompile(`^(\w+)\s*\(?\s*(?::(\w+))?`)
	rspecHookCondition = regexp.MustCompile(`\b(?:if|unless):|:(?:if|unless)\s*=>`)
	rspecMatcherAfter  = regexp.MustCompile(`^\s*\.\s*(to|not_to|to_not)\b\s*\(?\s*([A-Za-z_]\w*[?!]?|[=!<>~]=?~?)`)
	rspecBlockClosing  = regexp.MustCompile(`^(?:\}|end\b)\s*\.\s*(to|not_to|to_not)\b\s*\(?\s*([A-Za-z_]\w*[?!]?)`)
	rspecExpectHead    = regexp.MustCompile(`\b(expect_any_instance_of|expect)\s*([({]|do\b)|\bis_expected\b|(\.)?\bshould(_not)?\b|\b((?:assert|refute)(?:_\w+)?)\b[?!]?`)
	rspecShouldMatcher = regexp.MustCompile(`^\s*\(?\s*([A-Za-z_]\w*[?!]?|[=!<>~]=?~?)`)
	rspecVisit         = regexp.MustCompile(`(?:^|[^.\w:@$])(visit)\b`)
	rspecRequest       = regexp.MustCompile(`^(get|post|put|patch|delete|head)\b(?:\s*\(|\s+\S)`)
	rspecSkipCall      = regexp.MustCompile(`^(skip|pending)\b`)
	rspecRequire       = regexp.MustCompile(`^(require_relative|require)\b`)
	rspecSymbolArg     = regexp.MustCompile(`^:(\w+[?!]?)$`)
	rspecPairArg       = regexp.MustCompile(`^(?:(\w+[?!]?):|:(\w+[?!]?)\s*=>)\s*(\S.*)$`)
	rspecLiteralValue  = regexp.MustCompile(`^(?::\w+[?!]?|true|false|nil|-?\d+(?:\.\d+)?|'[^'\\]*'|"[^"\\#]*")$`)
	// Declarators whose arguments after the description are metadata.
	rspecSkipWords  = map[string]bool{"xit": true, "xspecify": true, "xexample": true, "xscenario": true, "skip": true, "xdescribe": true, "xcontext": true}
	rspecFocusWords = map[string]bool{"fit": true, "fspecify": true, "fexample": true, "fscenario": true, "focus": true, "fdescribe": true, "fcontext": true}
)

type rspecDiscovery struct {
	f        *discoveryFile
	r        *RSpecReader
	ids      map[int]string // example index to scoped id
	idReason map[int]string
	requires []int // indexes of require and require_relative lines
}

// discoverRSpec reads one RSpec file and returns "" or the reader's uncertainty reason.
func (f *discoveryFile) discoverRSpec() string {
	r := NewRSpecReader(f.path, f.text)
	if reason := r.Read(); reason != "" {
		f.unknown("", reason, "the RSpec reader could not balance or bound the file's blocks, so no example is read", 0, 0)
		return reason
	}
	d := &rspecDiscovery{f: f, r: r, ids: map[int]string{}, idReason: map[int]string{}}
	d.assignIDs()
	for i, l := range r.Lines {
		if rspecRequire.MatchString(l.Text) {
			d.requires = append(d.requires, i)
		}
	}
	for k := range r.Examples {
		d.example(k)
	}
	return ""
}

// physicalEnd is the last physical line of logical line i: the line before the next logical
// line, with trailing blank and comment lines dropped.
func (d *rspecDiscovery) physicalEnd(i int) int {
	end := d.f.index.Lines()
	if i+1 < len(d.r.Lines) {
		end = d.r.Lines[i+1].Line - 1
	}
	start := d.r.Lines[i].Line
	for end > start {
		t := strings.TrimSpace(d.f.index.Line(end))
		if t != "" && !strings.HasPrefix(t, "#") {
			break
		}
		end--
	}
	return max(start, end)
}

// assignIDs gives each example its RSpec scoped id where the source fixes it (DCX-V1-371).
func (d *rspecDiscovery) assignIDs() {
	r := d.r
	n, blocked := 0, ""
	for _, item := range r.Children(0) {
		if item.Group < 0 && (item.Include < 0 || !r.Includes[item.Include].Nests()) {
			continue
		}
		if item.Loop && blocked == "" {
			blocked = DiscoverReasonScopedLoop
		}
		n++
		d.assignItem(item, strconv.Itoa(n), blocked)
	}
}

func (d *rspecDiscovery) assignItem(item RSpecItem, id, blocked string) {
	r := d.r
	switch {
	case item.Example >= 0:
		if blocked != "" {
			d.idReason[item.Example] = blocked
		} else {
			d.ids[item.Example] = id
		}
	case item.Group >= 0:
		d.assignGroup(item.Group, id, blocked)
	case item.Include >= 0 && r.Includes[item.Include].Nested >= 0:
		// The customization block's examples follow the shared body's, whose size is not read.
		if blocked == "" {
			blocked = discoverReasonScopedShared
		}
		d.assignGroup(r.Includes[item.Include].Nested, id, blocked)
	}
}

func (d *rspecDiscovery) assignGroup(g int, prefix, blocked string) {
	r := d.r
	n := 0
	for _, kid := range r.Children(g) {
		if kid.Loop && blocked == "" {
			blocked = DiscoverReasonScopedLoop
		}
		if kid.Include >= 0 && !r.Includes[kid.Include].Nests() {
			slots, reason := d.sharedSlots(r.Includes[kid.Include], map[int]bool{})
			if reason != "" {
				if blocked == "" {
					blocked = reason
				}
				continue
			}
			n += slots
			continue
		}
		n++
		d.assignItem(kid, prefix+":"+strconv.Itoa(n), blocked)
	}
}

// sharedSlots is how many scoped-id slots an inline inclusion adds to its group: one per example,
// group or nesting inclusion in the shared body, and an inline inclusion's own slots, counted
// through the same rule. A loop-generated child, or an inclusion whose definition is not one
// same-file group (or includes itself), leaves the count unknown, with its reason.
func (d *rspecDiscovery) sharedSlots(inc RSpecInclude, active map[int]bool) (int, string) {
	r := d.r
	def, ok := d.definition(inc)
	if !ok || active[def] {
		return 0, DiscoverReasonScopedInline
	}
	active[def] = true
	defer delete(active, def)
	n := 0
	for _, kid := range r.Children(def) {
		if kid.Loop {
			return 0, DiscoverReasonScopedLoop
		}
		if kid.Include >= 0 && !r.Includes[kid.Include].Nests() {
			slots, reason := d.sharedSlots(r.Includes[kid.Include], active)
			if reason != "" {
				return 0, reason
			}
			n += slots
			continue
		}
		n++
	}
	return n, ""
}

// headArgs is a declaring line's text after its declarator word.
func headArgs(head string) string {
	return strings.TrimSpace(head[len(rspecWordHead.FindString(head)):])
}

// definition is the one same-file shared group definition inclusion inc names.
func (d *rspecDiscovery) definition(inc RSpecInclude) (int, bool) {
	if !inc.Literal {
		return 0, false
	}
	found := -1
	for k, g := range d.r.Groups {
		if k == 0 || !SharedDefinitionWords[Declarator(g.Head)] {
			continue
		}
		if name, ok := SharedName(headArgs(g.Head)); ok && name == inc.Name {
			if found >= 0 {
				return 0, false
			}
			found = k
		}
	}
	return found, found >= 0
}

// chain is example group g and its enclosing groups, innermost first, the file root excluded.
func (d *rspecDiscovery) chain(g int) []int {
	out := []int{}
	for ; g > 0; g = d.r.Groups[g].Parent {
		out = append(out, g)
	}
	return out
}

func (d *rspecDiscovery) example(k int) {
	f, r := d.f, d.r
	e := r.Examples[k]
	last := e.Index + len(e.Body) - 1
	out, ok := f.newExample(e.Line, d.physicalEnd(last))
	if !ok {
		return
	}
	chain := d.chain(e.Group)
	shared := false
	for _, g := range chain {
		shared = shared || SharedDefinitionWords[Declarator(r.Groups[g].Head)]
	}
	out.Identity = ExampleIdentity{Kind: ExampleIdentityRSpec, File: f.path}
	if id, ok := d.ids[k]; ok {
		out.Identity.Example = f.path + "[" + id + "]"
	} else {
		reason := d.idReason[k]
		if reason == "" {
			reason = discoverReasonScopedUnplaced
			if shared {
				reason = DiscoverReasonSharedDefinition
			}
		}
		out.Identity.Unknown = reason
		f.unknown(out.ID, reason, "the source does not fix the example's RSpec scoped id", e.Line, e.Line)
	}
	if titles := r.TitlePath(e); shared {
		// The including group's description prefixes the title, so it is not fixed here.
		out.TitleUnknown = DiscoverReasonSharedDefinition
	} else if titles == nil {
		out.TitleUnknown = DiscoverReasonTitleNotLiteral
		f.unknown(out.ID, DiscoverReasonTitleNotLiteral, "a description on the example's path is not one literal string or constant", e.Line, e.Line)
	} else {
		out.Identity.FullName = strings.Join(titles, " ")
		lines := []int{}
		for _, g := range chain {
			lines = append(lines, r.Groups[g].Line)
		}
		slices.Reverse(lines)
		lines = append(lines, e.Line)
		for i, title := range titles {
			a, _ := f.anchor(lines[i], lines[i], "static test discovery title")
			out.TitlePath = append(out.TitlePath, DiscoveredTitle{Title: title, Evidence: a})
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		g := r.Groups[chain[i]]
		word := Declarator(g.Head)
		if SharedDefinitionWords[word] || word == "it_behaves_like" || word == "it_should_behave_like" {
			continue
		}
		d.declFacts(&out, g.Index)
	}
	d.declFacts(&out, e.Index)
	d.skipStatements(&out, e.Index, e.Body)
	chainSet := map[int]bool{0: true}
	for _, g := range chain {
		chainSet[g] = true
	}
	for _, h := range r.hookSites {
		if !chainSet[h.group] {
			continue
		}
		d.hook(&out, h)
	}
	d.expectations(&out, e.Index, e.Body)
	d.navigations(&out, e.Index, e.Body, "example")
	d.reaches(&out, chainSet)
	f.capFacts(&out)
	f.out.Examples = append(f.out.Examples, out)
}

// declFacts adds the metadata tags and annotations of the declaring line at index i.
func (d *rspecDiscovery) declFacts(out *DiscoveredExample, i int) {
	f, r := d.f, d.r
	text := r.Lines[i].Text
	masked := MaskRubyStrings(text)
	where, _ := f.anchor(r.Lines[i].Line, r.Lines[i].Line, "static test discovery declaration")
	word := Declarator(text)
	switch {
	case rspecSkipWords[word]:
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "skip", Evidence: where})
	case word == "pending":
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "pending", Evidence: where})
	case rspecFocusWords[word]:
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "focus", Evidence: where})
	}
	args := rubyArgList(masked, len(rspecWordHead.FindString(masked)))
	for n, a := range args {
		m, o := strings.TrimSpace(masked[a[0]:a[1]]), strings.TrimSpace(text[a[0]:a[1]])
		if n == 0 && !rspecPairArg.MatchString(m) {
			continue // the description
		}
		pairs := [][2]string{{m, o}}
		if strings.HasPrefix(m, "{") && strings.HasSuffix(m, "}") {
			pairs = nil
			inner := m[1 : len(m)-1]
			off := strings.Index(text[a[0]:a[1]], "{") + a[0] + 1
			for _, p := range rubyArgList("("+inner+")", 0) {
				pairs = append(pairs, [2]string{strings.TrimSpace(inner[p[0]-1 : p[1]-1]), strings.TrimSpace(text[off+p[0]-1 : off+p[1]-1])})
			}
		}
		for _, p := range pairs {
			d.metadataArg(out, p[0], p[1], where, r.Lines[i].Line)
		}
	}
}

// metadataArg adds one metadata argument: a :symbol tag, a key-value tag, or an unknown.
func (d *rspecDiscovery) metadataArg(out *DiscoveredExample, masked, text string, where lineanchor.Anchor, line int) {
	if m := rspecSymbolArg.FindStringSubmatch(masked); m != nil {
		out.Tags = append(out.Tags, DiscoveredTag{Name: m[1], Value: "true", Evidence: where})
		d.metadataAnnotation(out, m[1], "true", where)
		return
	}
	m := rspecPairArg.FindStringSubmatch(masked)
	if m == nil {
		d.f.unknown(out.ID, DiscoverReasonMetadata, "metadata argument "+text, line, line)
		return
	}
	key := m[1] + m[2]
	value := strings.TrimSpace(text[len(text)-len(m[3]):])
	if !rspecLiteralValue.MatchString(value) {
		out.Tags = append(out.Tags, DiscoveredTag{Name: key, Evidence: where})
		d.f.unknown(out.ID, DiscoverReasonMetadata, "metadata "+key+" has the value "+value, line, line)
		return
	}
	if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') {
		value = value[1 : len(value)-1]
	}
	out.Tags = append(out.Tags, DiscoveredTag{Name: key, Value: value, Evidence: where})
	d.metadataAnnotation(out, key, value, where)
}

func (d *rspecDiscovery) metadataAnnotation(out *DiscoveredExample, key, value string, where lineanchor.Anchor) {
	if key != "skip" && key != "pending" && key != "focus" || value == "false" || value == "nil" {
		return
	}
	description := ""
	if value != "true" && !strings.HasPrefix(value, ":") {
		description = value
	}
	out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: key, Description: description, Evidence: where})
}

// skipStatements adds the skip and pending statements of an example body.
func (d *rspecDiscovery) skipStatements(out *DiscoveredExample, index int, body []RubyLine) {
	depth := 0
	for k := 1; k < len(body); k++ {
		text := body[k].Text
		if RubyEnd.MatchString(text) {
			depth--
			continue
		}
		if m := rspecSkipCall.FindStringSubmatch(text); m != nil {
			masked := MaskRubyStrings(text)
			conditional := depth > 0 || regexp.MustCompile(`\s(?:if|unless)\s`).MatchString(masked)
			description := ""
			if args := rubyArgList(masked, len(m[1])); len(args) > 0 {
				v := strings.TrimSpace(text[args[0][0]:args[0][1]])
				if rspecLiteralValue.MatchString(v) && len(v) >= 2 && (v[0] == '\'' || v[0] == '"') {
					description = v[1 : len(v)-1]
				}
			}
			where, _ := d.f.anchor(body[k].Line, d.physicalEnd(index+k), "static test discovery runtime annotation")
			out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: m[1], Description: description, Conditional: conditional, Evidence: where})
		}
		if RubyOpens(text) {
			depth++
		}
	}
}

// hook adds one applicable hook and its navigations.
func (d *rspecDiscovery) hook(out *DiscoveredExample, h rspecHookSite) {
	head := h.body[0].Text
	m := rspecHookHead.FindStringSubmatch(head)
	scope := m[2]
	switch scope {
	case "", "example":
		scope = "each"
	case "context":
		scope = "all"
	}
	asserts := false
	for _, l := range h.body {
		masked := MaskRubyStrings(l.Text)
		if RSpecExpectation.MatchString(masked) || rspecExpectHead.MatchString(masked) && strings.Contains(masked, "assert") {
			asserts = true
		}
	}
	conditional := h.loop || rspecHookCondition.MatchString(MaskRubyStrings(head))
	where, _ := d.f.anchor(h.body[0].Line, d.physicalEnd(h.index+len(h.body)-1), "static test discovery hook")
	out.Hooks = append(out.Hooks, DiscoveredHook{Kind: m[1], Scope: scope, Asserts: asserts, Conditional: conditional, Evidence: where})
	d.navigations(out, h.index, h.body, "hook")
}

// expectations adds every expectation of an example body (DCX-V1-372).
func (d *rspecDiscovery) expectations(out *DiscoveredExample, index int, body []RubyLine) {
	f := d.f
	for k, l := range body {
		text, masked := l.Text, MaskRubyStrings(l.Text)
		for _, loc := range rspecExpectHead.FindAllStringSubmatchIndex(masked, -1) {
			x := DiscoveredExpectation{}
			endIndex := index + k
			group := func(n int) string {
				if loc[2*n] < 0 {
					return ""
				}
				return masked[loc[2*n]:loc[2*n+1]]
			}
			rest := masked[loc[1]:]
			switch {
			case group(1) != "" && group(2) == "(":
				open := loc[4]
				close := matchingClose(masked, open)
				if close < 0 {
					x.Unknown = DiscoverReasonMatcherUnread
					break
				}
				x.Receiver = boundText(strings.TrimSpace(text[open+1 : close]))
				d.matcher(&x, masked[close+1:])
			case group(1) != "":
				open := loc[4]
				close := -1
				if group(2) == "{" {
					close = matchingClose(masked, open)
				}
				if close >= 0 {
					x.Receiver = boundText("{ " + strings.TrimSpace(text[open+1:close]) + " }")
					d.matcher(&x, masked[close+1:])
					break
				}
				// A block expectation spanning lines: its matcher follows the `}` or `end` that
				// closes this block, not one closing a block nested inside it.
				x.Receiver = "block"
				x.Unknown = DiscoverReasonMatcherUnread
				if j := blockClose(body, k); j >= 0 {
					if c := rspecBlockClosing.FindStringSubmatch(MaskRubyStrings(body[j].Text)); c != nil {
						x.Matcher, x.Negated, x.Unknown = c[2], c[1] != "to", ""
						endIndex = index + j
					}
				}
			case strings.HasPrefix(masked[loc[0]:], "is_expected"):
				x.Receiver = "subject"
				d.matcher(&x, rest)
			case loc[6] >= 0 || strings.Contains(masked[loc[0]:loc[1]], "should"):
				x.Receiver = "subject"
				if group(3) == "." {
					x.Receiver = boundText(strings.TrimSpace(text[:loc[0]]))
				}
				x.Negated = group(4) != ""
				if s := rspecShouldMatcher.FindStringSubmatch(rest); s != nil {
					x.Matcher = s[1]
				} else {
					x.Unknown = DiscoverReasonMatcherUnread
				}
			default:
				x.Matcher = group(5)
				x.Negated = strings.HasPrefix(x.Matcher, "refute")
				if args := rubyArgList(masked, loc[1]); len(args) > 0 {
					x.Receiver = boundText(strings.TrimSpace(text[args[0][0]:args[0][1]]))
				}
			}
			x.Evidence, _ = f.anchor(l.Line, d.physicalEnd(endIndex), "static test discovery expectation")
			out.Expectations = append(out.Expectations, x)
		}
	}
}

// blockClose is the index of the body line that closes the block opened on line k, counting the
// `do`/`end` and brace blocks opened and closed in between, or -1 when no line closes it.
func blockClose(body []RubyLine, k int) int {
	depth := 1
	for j := k + 1; j < len(body); j++ {
		masked := MaskRubyStrings(body[j].Text)
		if strings.HasPrefix(masked, "}") || RubyEnd.MatchString(masked) {
			if depth--; depth == 0 {
				return j
			}
			continue
		}
		if RubyOpens(masked) || strings.Count(masked, "{") > strings.Count(masked, "}") {
			depth++
		}
	}
	return -1
}

// matcher reads `.to MATCHER`, `.not_to MATCHER` or `.to_not MATCHER` after a receiver.
func (d *rspecDiscovery) matcher(x *DiscoveredExpectation, rest string) {
	if m := rspecMatcherAfter.FindStringSubmatch(rest); m != nil {
		x.Matcher, x.Negated = m[2], m[1] != "to"
		return
	}
	x.Unknown = DiscoverReasonMatcherUnread
}

// navigations adds every visit and request statement of a body (DCX-V1-373).
func (d *rspecDiscovery) navigations(out *DiscoveredExample, index int, body []RubyLine, source string) {
	for k, l := range body {
		text, masked := l.Text, MaskRubyStrings(l.Text)
		add := func(kind, method string, from, at int) {
			n := DiscoveredNavigation{Kind: kind, Method: method, Source: source}
			args := rubyArgList(masked, from)
			end := len(text)
			if len(args) > 0 {
				end = args[len(args)-1][1]
				if end < len(masked) && masked[end] == ')' {
					end++
				}
			}
			n.Expression = boundText(strings.TrimSpace(text[at:end]))
			if len(args) > 0 {
				v := strings.TrimSpace(text[args[0][0]:args[0][1]])
				if len(v) >= 2 && rspecLiteralValue.MatchString(v) && (v[0] == '\'' || v[0] == '"') {
					n.Target = v[1 : len(v)-1]
				}
			}
			if n.Target == "" {
				n.Unknown = DiscoverReasonTargetNotLiteral
			}
			n.Evidence, _ = d.f.anchor(l.Line, d.physicalEnd(index+k), "static test discovery navigation")
			out.Navigations = append(out.Navigations, n)
		}
		for _, loc := range rspecVisit.FindAllStringSubmatchIndex(masked, -1) {
			add("visit", "", loc[3], loc[2])
		}
		if k > 0 {
			if m := rspecRequest.FindStringSubmatch(masked); m != nil {
				add("request", strings.ToUpper(m[1]), len(m[1]), 0)
			}
		}
	}
}

// reaches adds the shared groups the example's groups include and the file's requires
// (DCX-V1-374).
func (d *rspecDiscovery) reaches(out *DiscoveredExample, chainSet map[int]bool) {
	f, r := d.f, d.r
	for _, inc := range r.Includes {
		applies := !inc.Nests() && chainSet[inc.Group] || inc.Nested >= 0 && chainSet[inc.Nested]
		if !applies {
			continue
		}
		where, _ := f.anchor(inc.Line, inc.Line, "static test discovery inclusion")
		reach := DiscoveredReach{Kind: "shared-example", Name: inc.Name, Evidence: where}
		if inc.Kind == "include_context" {
			reach.Kind = "shared-context"
		}
		switch def, ok := d.definition(inc); {
		case !inc.Literal:
			reach.State, reach.Reason = "unresolved", DiscoverReasonComputedLookup
			reach.Specifier = boundText(headArgs(r.Lines[inc.Index].Text))
		case ok:
			reach.State, reach.Target = "repository", f.path
			decl, _ := f.anchor(r.Groups[def].Line, r.Groups[def].Line, "static test discovery shared group definition")
			reach.Declaration = &decl
		default:
			reach.State, reach.Reason = "unresolved", DiscoverReasonRegistry
		}
		out.Reaches = append(out.Reaches, reach)
	}
	for _, i := range d.requires {
		text := r.Lines[i].Text
		word := rspecRequire.FindString(text)
		where, _ := f.anchor(r.Lines[i].Line, r.Lines[i].Line, "static test discovery require")
		reach := DiscoveredReach{Kind: "import", Evidence: where}
		args := rubyArgList(MaskRubyStrings(text), len(word))
		v := ""
		if len(args) > 0 {
			v = strings.TrimSpace(text[args[0][0]:args[0][1]])
		}
		switch {
		case len(args) != 1 || len(v) < 2 || !rspecLiteralValue.MatchString(v) || v[0] != '\'' && v[0] != '"':
			reach.Specifier = boundText(headArgs(text))
			reach.State, reach.Reason = "unresolved", DiscoverReasonComputedLookup
		case word == "require_relative":
			reach.Specifier = v[1 : len(v)-1]
			reach.Target, reach.State, reach.Reason = f.resolver.ruby(f.path, reach.Specifier)
		default:
			reach.Specifier = v[1 : len(v)-1]
			reach.State, reach.Reason = "unresolved", DiscoverReasonLoadPath
		}
		out.Reaches = append(out.Reaches, reach)
	}
}

// matchingClose returns the index of the bracket closing the one at open in masked, or -1.
func matchingClose(masked string, open int) int {
	depth := 0
	for i := open; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func rubyWordAt(s string, i int, w string) bool {
	if !strings.HasPrefix(s[i:], w) {
		return false
	}
	word := func(c byte) bool {
		return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	return (i == 0 || !word(s[i-1])) && (i+len(w) == len(s) || !word(s[i+len(w)]))
}

// rubyArgList splits the arguments of a call whose name ends at from in masked: a parenthesized
// list, or a bare list up to a block opener (`do`, or a `{` that does not open a hash argument), a
// modifier `if` or `unless`, or the end of the line. It returns each argument's [start, end).
func rubyArgList(masked string, from int) [][2]int {
	i := from
	for i < len(masked) && masked[i] == ' ' {
		i++
	}
	paren := i < len(masked) && masked[i] == '('
	if paren {
		i++
	}
	args, start, depth := [][2]int{}, i, 0
scan:
	for ; i < len(masked); i++ {
		c := masked[i]
		switch {
		case c == '(' || c == '[':
			depth++
		case c == '{':
			if depth == 0 && !paren {
				// A brace after a value or at the start of the arguments opens a block; after a
				// separator, a key or an operator it opens a hash argument.
				before := strings.TrimRight(masked[from:i], " ")
				if before == "" || !strings.ContainsAny(before[len(before)-1:], ",(:>=[|") {
					break scan
				}
			}
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				break scan
			}
			depth--
		case c == ',' && depth == 0:
			args = append(args, [2]int{start, i})
			start = i + 1
		case depth == 0 && !paren && (rubyWordAt(masked, i, "do") || rubyWordAt(masked, i, "if") || rubyWordAt(masked, i, "unless")):
			break scan
		}
	}
	if strings.TrimSpace(masked[start:i]) != "" {
		args = append(args, [2]int{start, i})
	}
	for k := range args {
		for args[k][0] < args[k][1] && masked[args[k][0]] == ' ' {
			args[k][0]++
		}
		for args[k][1] > args[k][0] && masked[args[k][1]-1] == ' ' {
			args[k][1]--
		}
	}
	return args
}
