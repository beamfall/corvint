package appmap

import (
	"regexp"
	"strings"
)

// The Ruby reader of site knowledge (SKH-V0-010, SKH-V0-020): Capybara page classes, SitePrism
// pages and the RSpec/Capybara feature scenarios that drive them. It reads lines only.

var (
	rbTest     = regexp.MustCompile(`^\s*(?:scenario|it|specify)\s*\(?\s*(['"])(.*?)['"].*\bdo\b\s*(\|[^|]*\|)?\s*$`)
	rbOpen     = regexp.MustCompile(`(?:\bdo\b\s*(\|[^|]*\|)?\s*$)|^\s*(?:if|unless|while|until|case|begin|def|class|module)\b|=\s*(?:if|unless|case|begin)\b`)
	rbEnd      = regexp.MustCompile(`^\s*end\b`)
	rbDefLine  = regexp.MustCompile(`^\s*def\s+(?:self\.)?([A-Za-z_]\w*[?!]?)\s*(?:\(([^)]*)\)|\s+(.*))?\s*$`)
	rbVisit    = regexp.MustCompile(`^\s*visit\s*\(?\s*(.+?)\s*\)?\s*$`)
	rbClick    = regexp.MustCompile(`^\s*click_(on|button|link)\s*\(?\s*(['"])(.*?)['"]`)
	rbFill     = regexp.MustCompile(`^\s*fill_in\s*\(?\s*(['"])(.*?)['"]\s*,\s*with:\s*(.+?)\s*\)?\s*$`)
	rbSelect   = regexp.MustCompile(`^\s*select\s*\(?\s*(.+?)\s*,\s*from:\s*(['"])(.*?)['"]\s*\)?\s*$`)
	rbCheck    = regexp.MustCompile(`^\s*(check|uncheck|choose)\s*\(?\s*(['"])(.*?)['"]`)
	rbAttach   = regexp.MustCompile(`^\s*attach_file\s*\(?\s*(['"])(.*?)['"]\s*,\s*(.+?)\s*\)?\s*$`)
	rbSymbol   = regexp.MustCompile(`^:[a-z_][a-z0-9_]*$`)
	rbFindAct  = regexp.MustCompile(`^\s*find\s*\(\s*(['"])(.*?)['"]\s*\)\s*\.\s*(click|set|select_option|hover|send_keys)\b\s*\(?\s*(.*?)\s*\)?\s*$`)
	rbExpect   = regexp.MustCompile(`^\s*expect\s*\(\s*(page|[a-z_]\w*)\s*\)\s*\.\s*(to|not_to|to_not)\s+(\w+)\s*\(?\s*(.*?)\s*\)?\s*$`)
	rbSetURL   = regexp.MustCompile(`^\s*set_url\s+(['"])(.*?)['"]`)
	rbElement  = regexp.MustCompile(`^\s*(element|section)\s+:(\w+)\s*,\s*(['"])(.*?)['"]`)
	rbElemAct  = regexp.MustCompile(`^\s*(?:(\w+)\.)?(\w+)\.(click|set|select|hover|send_keys)\b\s*\(?\s*(.*?)\s*\)?\s*$`)
	rbNew      = regexp.MustCompile(`^\s*(\w+)\s*=\s*([A-Z][\w:]*)\.new\b`)
	rbInstCall = regexp.MustCompile(`^\s*(\w+)\.(\w+[?!]?)\s*(?:\(\s*(.*?)\s*\)|\s+(.+?))?\s*$`)
	rbBare     = regexp.MustCompile(`^\s*([a-z_]\w*[?!]?)\s*(?:\(\s*(.*?)\s*\)|\s+([^=].*?))?\s*$`)
	rbControl  = regexp.MustCompile(`^\s*(?:if|unless|case|while|until)\b|\.each\b|\.times\b`)
	rbLoad     = regexp.MustCompile(`^\s*(?:(\w+)\.)?load\b`)
	rbCapybara = regexp.MustCompile(`^\s*(?:visit|click_on|click_button|click_link|fill_in|select|check|uncheck|choose|attach_file|find)\b`)
	rbAssign   = regexp.MustCompile(`^\s*@?\w+\s*(?:\|\|)?=\s*([A-Za-z_][\w:]*)?`)
	rbQuiet    = wordSet("end else elsif rescue ensure let let! before after subject puts p pp sleep require private protected public include extend")
)

// readSKRuby reads one Ruby source.
func readSKRuby(s skSource, m *SKModel) *skFile {
	rb := parseRubyStructure(s.entry.path, s.data)
	f := &skFile{path: s.entry.path, entry: s.entry, data: s.data, ruby: true, rb: rb, raw: rb.raw, classes: map[string]*skClass{},
		funcs: map[string]*skUnit{}, imports: map[string]skImport{}}
	if rb.broken > 0 {
		m.Unknowns = append(m.Unknowns, SKUnknown{Path: f.path, Line: rb.broken, Form: FormCapybaraClass, Construct: "nesting",
			Knowledge: "page-objects", Reason: "parse-unsupported"})
	}
	for _, sc := range rb.scopes {
		short := sc.name[strings.LastIndex(sc.name, ":")+1:]
		c := &skClass{name: short, start: sc.line, end: sc.end, props: map[string]SKBinding{}, roots: map[string]bool{},
			methods: map[string]*skUnit{}, file: f}
		if i := strings.LastIndex(sc.super, ":"); i >= 0 {
			c.extends = sc.super[i+1:]
		} else {
			c.extends = sc.super
		}
		for _, inc := range sc.includes {
			if inc.name != "" {
				c.includes = append(c.includes, inc.name[strings.LastIndex(inc.name, ":")+1:])
			}
		}
		capy := false
		for _, d := range sc.defs {
			if !d.instance {
				continue
			}
			u := &skUnit{file: f, class: c, name: d.name, from: d.line + 1, to: d.end - 1, start: d.line, end: d.end,
				pageArgs: map[string]bool{}}
			if mm := rbDefLine.FindStringSubmatch(f.raw[d.line-1]); mm != nil {
				u.params = rbParams(mm[2] + mm[3])
			}
			c.addMethod(u)
			for l := u.from; l <= u.to && l <= len(f.raw); l++ {
				if rbCapybara.MatchString(f.raw[l-1]) {
					capy = true
				}
			}
		}
		for l := sc.line; l <= sc.end && l <= len(f.raw); l++ {
			line := f.raw[l-1]
			if mm := rbSetURL.FindStringSubmatch(line); mm != nil {
				c.url = mm[2]
			}
			if mm := rbElement.FindStringSubmatch(line); mm != nil {
				sel := newSelector("css", mm[4], "", strengthWeak, l)
				if t := testIDAttribute.FindStringSubmatch(mm[4]); t != nil {
					sel = newSelector("test-id", t[1], "", strengthStrong, l)
				}
				c.props[mm[2]] = SKBinding{Name: mm[2], Locator: sel, Anchor: spanOf(f.entry, f.data, l, l)}
			}
		}
		switch {
		case strings.HasSuffix(sc.super, "SitePrism::Page") || strings.HasSuffix(sc.super, "SitePrism::Section"):
			c.form = FormSitePrismPage
		case sc.kind == "class" && capy:
			c.form = FormCapybaraClass
		}
		if f.classes[short] == nil {
			f.classes[short] = c
			if c.form != "" {
				f.corder = append(f.corder, short)
			}
		}
	}
	f.readRubyTests()
	return f
}

// rbParams reads a def parameter list; keyword and splat parameters keep their names.
func rbParams(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		p = strings.TrimLeft(p, "*&")
		if i := strings.IndexAny(p, ":= "); i >= 0 {
			p = p[:i]
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (f *skFile) readRubyTests() {
	code := f.rb.code
	for l := 1; l <= len(f.raw); l++ {
		mm := rbTest.FindStringSubmatch(f.raw[l-1])
		if mm == nil {
			continue
		}
		depth, end := 0, len(f.raw)
		for k := l; k <= len(code); k++ {
			if rbOpen.MatchString(code[k-1]) {
				depth++
			}
			if rbEnd.MatchString(code[k-1]) {
				depth--
				if depth == 0 {
					end = k
					break
				}
			}
		}
		f.tests = append(f.tests, &skUnit{file: f, name: "scenario", title: mm[2], from: l + 1, to: end - 1, start: l, end: end,
			pageArgs: map[string]bool{}})
		l = end
	}
}

// rbValue reads one Ruby argument: a quoted literal, a parameter slot, or dynamic.
func rbValue(s string, params map[string]bool) SKArg {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] && !strings.ContainsAny(s[1:len(s)-1], `'"\`) &&
		!strings.Contains(s, "#{") {
		return SKArg{Literal: s[1 : len(s)-1]}
	}
	if params[s] {
		return SKArg{Slot: s}
	}
	if rbSymbol.MatchString(s) {
		return SKArg{Literal: s[1:]}
	}
	if s != "" && strings.Trim(s, "0123456789") == "" {
		return SKArg{Literal: s}
	}
	return SKArg{Dynamic: true}
}

func rbArgs(s string, params map[string]bool) []SKArg {
	out := []SKArg{}
	if strings.TrimSpace(s) == "" {
		return out
	}
	for _, p := range strings.Split(s, ",") {
		out = append(out, rbValue(p, params))
	}
	return out
}

// rubySteps reads a Ruby unit's body lines into ordered steps (SKH-V0-020).
func (m *SKModel) rubySteps(u *skUnit) []SKStep {
	f := u.file
	x := &skCtx{m: m, f: f, c: u.class, u: u}
	params := map[string]bool{}
	for _, p := range u.params {
		params[p] = true
	}
	inst := map[string]string{}
	for l := u.from; l <= u.to && l <= len(f.raw); l++ {
		raw := f.raw[l-1]
		code := strings.TrimSpace(f.rb.code[l-1])
		if code == "" {
			continue
		}
		at := spanOf(f.entry, f.data, l, l)
		val := func(kind string, sel *Selector, a SKArg) {
			var loc *skLoc
			if sel != nil {
				loc = &skLoc{sel: *sel}
			}
			x.add(x.valued(kind, loc, a, at))
		}
		if rbControl.MatchString(code) {
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Anchor: at})
			continue
		}
		if mm := rbNew.FindStringSubmatch(raw); mm != nil {
			inst[mm[1]] = mm[2][strings.LastIndex(mm[2], ":")+1:]
			continue
		}
		if mm := rbClick.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("text", mm[3], "", strengthWeak, l)
			switch mm[1] {
			case "button":
				sel = newSelector("role", "button", mm[3], strengthMedium, l)
			case "link":
				sel = newSelector("role", "link", mm[3], strengthMedium, l)
			}
			x.add(SKStep{Kind: StepClick, Locator: &sel, Anchor: at})
			continue
		}
		if mm := rbFill.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("label", mm[2], "", strengthMedium, l)
			val(StepFill, &sel, rbValue(mm[3], params))
			continue
		}
		if mm := rbSelect.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("label", mm[3], "", strengthMedium, l)
			val(StepSelect, &sel, rbValue(mm[1], params))
			continue
		}
		if mm := rbCheck.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("label", mm[3], "", strengthMedium, l)
			st := SKStep{Kind: StepCheck, Locator: &sel, Anchor: at}
			if mm[1] == "uncheck" {
				st.Value = "false"
			}
			x.add(st)
			continue
		}
		if mm := rbAttach.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("label", mm[2], "", strengthMedium, l)
			val(StepUpload, &sel, rbValue(mm[3], params))
			continue
		}
		if mm := rbFindAct.FindStringSubmatch(raw); mm != nil {
			sel := newSelector("css", mm[2], "", strengthWeak, l)
			switch mm[3] {
			case "click":
				x.add(SKStep{Kind: StepClick, Locator: &sel, Anchor: at})
			case "hover":
				x.add(SKStep{Kind: StepHover, Locator: &sel, Anchor: at})
			case "send_keys":
				val(StepPress, &sel, rbValue(mm[4], params))
			case "set":
				val(StepFill, &sel, rbValue(mm[4], params))
			default:
				val(StepSelect, &sel, rbValue(mm[4], params))
			}
			continue
		}
		if mm := rbVisit.FindStringSubmatch(raw); mm != nil {
			val(StepVisit, nil, rbValue(mm[1], params))
			continue
		}
		if mm := rbExpect.FindStringSubmatch(raw); mm != nil {
			matcher := mm[3]
			if mm[2] != "to" {
				matcher = "not." + matcher
			}
			st := SKStep{Kind: StepAssert, Matcher: matcher, Anchor: at}
			if strings.TrimSpace(mm[4]) == "" {
				x.add(st)
			} else {
				x.add(x.withExpected(st, rbValue(mm[4], params)))
			}
			continue
		}
		if mm := rbLoad.FindStringSubmatch(raw); mm != nil {
			c := u.class
			if mm[1] != "" {
				c, _ = m.findClass(f, inst[mm[1]])
			}
			if c != nil && c.url != "" {
				x.add(SKStep{Kind: StepVisit, Target: c.url, Anchor: at})
			} else {
				x.add(SKStep{Kind: StepOpaque, Reason: OpaqueUnresolvedCall, Call: "load", Anchor: at})
			}
			continue
		}
		if mm := rbElemAct.FindStringSubmatch(raw); mm != nil {
			c := u.class
			if mm[1] != "" {
				c, _ = m.findClass(f, inst[mm[1]])
			}
			if c != nil {
				if b, ok := c.props[mm[2]]; ok {
					sel := b.Locator
					switch mm[3] {
					case "click":
						x.add(SKStep{Kind: StepClick, Locator: &sel, Anchor: at})
					case "hover":
						x.add(SKStep{Kind: StepHover, Locator: &sel, Anchor: at})
					case "send_keys":
						val(StepPress, &sel, rbValue(mm[4], params))
					case "set":
						val(StepFill, &sel, rbValue(mm[4], params))
					default:
						val(StepSelect, &sel, rbValue(mm[4], params))
					}
					continue
				}
			}
		}
		if mm := rbAssign.FindStringSubmatch(raw); mm != nil && !strings.Contains(raw, "==") {
			if mm[1] != "" {
				u.setup = append(u.setup, mm[1])
			}
			continue
		}
		if mm := rbInstCall.FindStringSubmatch(raw); mm != nil && inst[mm[1]] != "" {
			x.add(SKStep{Kind: skCall, Call: inst[mm[1]] + "." + mm[2], Args: rbArgs(mm[3]+mm[4], params), Anchor: at})
			continue
		}
		if mm := rbBare.FindStringSubmatch(raw); mm != nil && !rbQuiet[mm[1]] {
			if u.class != nil {
				if t, _, _ := m.classMethod(f, u.class, mm[1], 0); t != nil {
					x.add(SKStep{Kind: skCall, Call: "this." + mm[1], Args: rbArgs(mm[2]+mm[3], params), Anchor: at})
					continue
				}
			}
			if params[mm[1]] || inst[mm[1]] != "" {
				continue
			}
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueUnresolvedCall, Call: mm[1], Anchor: at})
			continue
		}
		first := strings.Fields(code)[0]
		if rbQuiet[first] || rbEnd.MatchString(code) {
			continue
		}
		x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Anchor: at})
	}
	return x.steps
}
