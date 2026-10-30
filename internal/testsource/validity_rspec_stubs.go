package testsource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec readings of RB-STUB-DECIDES-ASSERTION, RB-MESSAGE-CHAIN-PREFIX-ONLY,
// RB-FLAG-CONDITIONAL and RB-FLAG-FORCED-NON-DEFAULT (TSA-V0-430..442). Like TSA-V0-120..130 they
// read one example's body and the before, let! and subject! hooks on its group chain, and never the
// code under test: a stub-decided assertion abstains instead of refusing, and a flag stub qualifies
// the result instead of changing an axis.

const (
	maxFlagLookups  = 256
	maxFlagDefaults = 1024
	maxStubText     = 160
)

var (
	// rspecNoChange is a negated change matcher; group 1 opens its block or argument list.
	rspecNoChange = regexp.MustCompile(`\.\s*(?:not_to|to_not)\s*\(?\s*change\s*([{(])`)
	// rspecObserved is a change block's single call chain: a root name and its calls.
	rspecObserved = regexp.MustCompile(`^(@{0,2}[A-Za-z_]\w*(?:::[A-Z]\w*)*)((?:\s*\.\s*[A-Za-z_]\w*[?!]?(?:\s*\([^()]*\))?)*)$`)
	rspecRootName = regexp.MustCompile(`^@{0,2}[A-Za-z_]\w*(?:::[A-Z]\w*)*$`)
	rspecCallName = regexp.MustCompile(`\.\s*([A-Za-z_]\w*[?!]?)`)
	rspecBlockArg = regexp.MustCompile(`^\|[^|]*\|`)
	// rspecChainStub is `allow(R).to receive_message_chain(`; groups: 1 allow|expect, 2 R.
	rspecChainStub = regexp.MustCompile(`\b(allow|expect)\s*\(\s*([A-Za-z_][\w:]*[?!]?)\s*\)\s*\.\s*to\s*\(?\s*receive_message_chain\s*\(`)
	rspecChainSym  = regexp.MustCompile(`^:([A-Za-z_]\w*[?!]?)$`)
	rspecChainStr  = regexp.MustCompile(`^["']([A-Za-z_]\w*[?!]?(?:\.[A-Za-z_]\w*[?!]?)*)["']$`)
	rspecChainHash = regexp.MustCompile(`^([A-Za-z_]\w*[?!]?):(?:[^:]|$)`)
	rspecDoBlock   = regexp.MustCompile(`^do\b`)
	rspecTailCall  = regexp.MustCompile(`^\s*\.\s*([A-Za-z_]\w*[?!]?)`)
	// takesArgs are the stub calls whose arguments the reader reads or must see to be complete.
	takesArgs = map[string]bool{"with": true, "and_return": true, "and_raise": true, "and_throw": true, "and_yield": true,
		"and_invoke": true, "exactly": true, "at_least": true, "at_most": true}
	// rspecRubyLiteral is one literal a flag key or forced value can be read from.
	rspecRubyLiteral = regexp.MustCompile(`^(?:true|false|nil|-?\d+(?:\.\d+)?|:[A-Za-z_]\w*[?!]?|"[^"\\#]*"|'[^'\\]*')$`)
	flagKey          = regexp.MustCompile(`^[\w.:/-]{1,128}$`)
	flagReceiver     = regexp.MustCompile(`^(?:::)?[A-Z]\w*(?:::[A-Z]\w*)*$`)
	flagMethod       = regexp.MustCompile(`^[A-Za-z_]\w{0,127}[?!]?$`)
	// rspecMutators are operations that write state; a stub of one replaces the write.
	rspecMutators = map[string]bool{"save": true, "update": true, "update_attribute": true, "update_attributes": true,
		"update_column": true, "update_columns": true, "destroy": true, "delete": true, "touch": true, "increment": true,
		"decrement": true, "toggle": true, "create": true, "insert": true, "upsert": true, "cancel": true, "commit": true,
		"transition": true, "apply": true, "perform": true, "call": true, "run": true, "execute": true, "process": true}
)

// stubTail is what a stub declares after `receive(:m)`: its `.with` and `.and_return` argument
// ranges, the number of returned values, and whether it calls through (and_call_original,
// and_wrap_original), raises (and_raise, and_throw), runs a block, or continues past what the
// reader reads (an unknown call, or a chain continued on the next line).
type stubTail struct {
	with, ret          [2]int
	hasWith, hasReturn bool
	values             int
	original, raise    bool
	block, unread      bool
}

func readStubTail(t rubyText, from int) stubTail {
	out := stubTail{}
	m := t.masked
	for at := from; ; {
		trimmed := strings.TrimLeft(m[at:], " \t")
		switch {
		case strings.HasPrefix(trimmed, "{") || rspecDoBlock.MatchString(trimmed):
			out.block = true
			return out
		case !strings.HasPrefix(trimmed, "."):
			return out
		}
		loc := rspecTailCall.FindStringSubmatchIndex(m[at:])
		if loc == nil {
			out.unread = true
			return out
		}
		name := m[at+loc[2] : at+loc[3]]
		at += loc[1]
		args, has := [2]int{}, false
		if rest := strings.TrimLeft(m[at:], " \t"); strings.HasPrefix(rest, "(") {
			open := len(m) - len(rest)
			close := balanced(m, open)
			if close < 0 {
				out.unread = true
				return out
			}
			args, has, at = [2]int{open + 1, close}, true, close+1
		} else if takesArgs[name] && rest != "" && !strings.ContainsRune(".)}", rune(rest[0])) {
			// `and_return true`: arguments without parentheses are not read.
			out.unread = true
			return out
		}
		switch name {
		case "with":
			out.with, out.hasWith = args, has
			out.unread = out.unread || !has
		case "and_return":
			out.ret, out.hasReturn, out.values = args, has, len(topLevelArgs(m, args))
		case "and_call_original", "and_wrap_original":
			out.original = true
		case "and_raise", "and_throw":
			out.raise = true
		case "and_yield", "and_invoke":
			out.block = true
		case "once", "twice", "thrice", "exactly", "at_least", "at_most", "times", "ordered", "never":
		default:
			out.unread = true
			return out
		}
	}
}

// topLevelArgs splits the argument range span of masked at its top-level commas, each trimmed.
func topLevelArgs(masked string, span [2]int) [][2]int {
	out := [][2]int{}
	if strings.TrimSpace(masked[span[0]:span[1]]) == "" {
		return out
	}
	depth, start := 0, span[0]
	for i := span[0]; i < span[1]; i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, trimSpan(masked, start, i))
				start = i + 1
			}
		}
	}
	return append(out, trimSpan(masked, start, span[1]))
}

// stubSite is one stub on an example's path: in a hook (rank is its group's depth from the
// outermost group) or in the body (rank is the chain's length), so a higher rank runs later.
type stubSite struct {
	s    rubyStub
	tail stubTail
	rank int
	body bool
}

// stubSites returns the stubs of e's hooks, outermost group first, then of its body.
func (c *flowContext) stubSites() []stubSite {
	r, e := c.r, c.e
	chain := []int{}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		chain = append(chain, g)
	}
	slices.Reverse(chain)
	out := []stubSite{}
	add := func(lines []rubyLine, rank int, body bool) {
		for i, l := range lines {
			continued := i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1].text), ".")
			for _, s := range rubyStubs([]rubyLine{l}) {
				tail := readStubTail(s.t, s.end)
				tail.unread = tail.unread || continued
				out = append(out, stubSite{s: s, tail: tail, rank: rank, body: body})
			}
		}
	}
	for rank, g := range chain {
		for _, h := range r.groups[g].hooks {
			add(h.body, rank, false)
		}
	}
	add(e.body, len(chain), true)
	return out
}

// flagOf returns the flag lookup stub s is, classified by its constant receiver or the class of
// the verified double or new object it is bound to (TSA-V0-435).
func (c *flowContext) flagOf(s rubyStub, rc receiver) (testvalidity.FlagLookup, bool) {
	class := rc.class
	switch {
	case constant(s.recv):
		class = s.recv
	case !rc.fresh:
		return testvalidity.FlagLookup{}, false
	}
	if class == "" {
		return testvalidity.FlagLookup{}, false
	}
	return c.r.flags.Match(class, s.method)
}

// stubFindings returns example e's TSA-V0-431, TSA-V0-433, TSA-V0-436 and TSA-V0-438 findings, in
// that rule order and each in line order, and its flag conditions (TSA-V0-437).
func (c *flowContext) stubFindings(test string) ([]testvalidity.SourceFinding, []testvalidity.FlagCondition) {
	sites := c.stubSites()
	slices.SortStableFunc(sites, func(a, b stubSite) int { return a.s.t.line - b.s.t.line })
	plain, flags := []stubSite{}, []stubSite{}
	lookups := map[int]testvalidity.FlagLookup{}
	for _, site := range sites {
		if lookup, ok := c.flagOf(site.s, c.receiver(site.s)); ok {
			lookups[len(flags)] = lookup
			flags = append(flags, site)
			continue
		}
		plain = append(plain, site)
	}
	findings := c.stubDecides(test, plain)
	findings = append(findings, c.chainPrefixes(test)...)
	conditions, flagged := c.flagConditions(test, flags, lookups)
	return append(findings, flagged...), conditions
}

func (c *flowContext) finding(rule, severity, test string, location, cause int, message string) testvalidity.SourceFinding {
	return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: c.e.title,
		Location: c.at(location), Cause: c.at(cause), Message: message}
}

// lineText is a line's trimmed source text, cut at maxStubText runes.
func lineText(t rubyText) string {
	text := strings.TrimSpace(t.raw)
	if utf8.RuneCountInString(text) > maxStubText {
		text = string([]rune(text)[:maxStubText]) + "..."
	}
	return text
}

// observed is one negated change matcher: the line, its text, the observed root and its calls.
type observed struct {
	t     rubyText
	text  string
	root  string
	calls []string
}

// exampleLines is the example body after its declaring line, or the declaring line itself for a
// one-line example (`it { ... }`).
func (c *flowContext) exampleLines() []rubyLine {
	if len(c.e.body) == 1 {
		return c.e.body
	}
	return c.e.body[1:]
}

// noChanges returns the example body's negated change matchers whose observed value is one call
// chain on a root (`change { R.a.b }`) or `change(R, :attr)`.
func (c *flowContext) noChanges() []observed {
	out := []observed{}
	for _, l := range c.exampleLines() {
		t := maskRuby(l)
		for _, loc := range rspecNoChange.FindAllStringSubmatchIndex(t.masked, -1) {
			open := loc[2]
			close := balanced(t.masked, open)
			if close < 0 {
				continue
			}
			o := observed{t: t, text: lineText(t)}
			inner := t.masked[open+1 : close]
			if t.masked[open] == '{' {
				inner = strings.TrimSpace(rspecBlockArg.ReplaceAllString(strings.TrimSpace(inner), ""))
				m := rspecObserved.FindStringSubmatch(inner)
				if m == nil {
					continue
				}
				o.root = m[1]
				for _, call := range rspecCallName.FindAllStringSubmatch(m[2], -1) {
					o.calls = append(o.calls, call[1])
				}
			} else {
				args := topLevelArgs(t.masked, [2]int{open + 1, close})
				if len(args) != 2 {
					continue
				}
				root := t.masked[args[0][0]:args[0][1]]
				attr := rspecChainSym.FindStringSubmatch(t.masked[args[1][0]:args[1][1]])
				if !rspecRootName.MatchString(root) || attr == nil {
					continue
				}
				o.root, o.calls = root, []string{attr[1]}
			}
			out = append(out, o)
		}
	}
	return out
}

// stubDecides reports each stub that decides a negated change matcher on its own receiver
// (TSA-V0-431, TSA-V0-432): it fixes the observed call's value (the first call, after a leading
// reload) or replaces an operation that would change the observed state.
func (c *flowContext) stubDecides(test string, sites []stubSite) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	matchers := c.noChanges()
	if len(matchers) == 0 {
		return out
	}
	for _, site := range sites {
		s, tail := site.s, site.tail
		if tail.original || tail.block || tail.unread {
			continue
		}
		for _, o := range matchers {
			if o.root != s.recv || site.body && s.t.line >= o.t.line || s.anyInstance && !constant(o.root) {
				continue
			}
			read := o.calls
			if len(read) > 0 && read[0] == "reload" {
				read = read[1:]
			}
			how := ""
			switch {
			case !s.anyInstance && len(read) > 0 && read[0] == s.method && !tail.raise && tail.values <= 1:
				how = fmt.Sprintf("fixes the value of %s.%s that the matcher reads", s.recv, s.method)
			case !strings.HasSuffix(s.method, "?") && !slices.Contains(o.calls, s.method) && s.method != "reload" && replaces(s):
				name := s.recv + "." + s.method
				if s.anyInstance {
					name = s.recv + "#" + s.method
				}
				how = fmt.Sprintf("replaces the operation %s that would change it", name)
			default:
				continue
			}
			out = append(out, c.finding(testvalidity.RuleRBStubDecidesAssertion, testvalidity.SeverityAbstained, test, o.t.line, s.t.line, fmt.Sprintf(
				"the assertion at %s (%s) observes %s, and the stub at %s (%s) %s, so the stub, not the code under test, decides the assertion and it passes whatever that code does; hint: inject the failure below the operation, or assert state the stub cannot decide; whether the code under test reaches the stub is not read, so the example abstains instead of being refused",
				c.at(o.t.line), o.text, o.root, c.at(s.t.line), lineText(s.t), how)))
			break
		}
	}
	return out
}

// replaces reports a stub that stands in for a state-changing operation: a bang method or a known
// write operation. Whatever it returns or raises, the real operation never runs (TSA-V0-431).
func replaces(s rubyStub) bool {
	return strings.HasSuffix(s.method, "!") || rspecMutators[s.method]
}

// chainStub is one `allow(R).to receive_message_chain(...)` and its message names.
type chainStub struct {
	t     rubyText
	recv  string
	chain []string
}

func readChainStubs(lines []rubyLine) []chainStub {
	out := []chainStub{}
	for _, l := range lines {
		t := maskRuby(l)
		for _, loc := range rspecChainStub.FindAllStringSubmatchIndex(t.masked, -1) {
			if t.masked[loc[2]:loc[3]] != "allow" {
				continue // expect(R).to receive_message_chain verifies the whole chain
			}
			open := loc[1] - 1
			close := balanced(t.masked, open)
			if close < 0 {
				continue
			}
			args := topLevelArgs(t.masked, [2]int{open + 1, close})
			chain := []string{}
			for i, a := range args {
				masked, raw := t.masked[a[0]:a[1]], t.raw[a[0]:a[1]]
				if m := rspecChainSym.FindStringSubmatch(masked); m != nil {
					chain = append(chain, m[1])
				} else if m := rspecChainStr.FindStringSubmatch(raw); m != nil {
					chain = append(chain, strings.Split(m[1], ".")...)
				} else if m := rspecChainHash.FindStringSubmatch(masked); m != nil && i == len(args)-1 {
					chain = append(chain, m[1])
				} else {
					chain = nil
					break
				}
			}
			if len(chain) >= 2 {
				out = append(out, chainStub{t: t, recv: t.masked[loc[4]:loc[5]], chain: chain})
			}
		}
	}
	return out
}

// chainPrefixes reports each message-chain stub whose example asserts have_received of a strict
// prefix message on its receiver and no expectation names its final message (TSA-V0-433,
// TSA-V0-434).
func (c *flowContext) chainPrefixes(test string) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	lines := []rubyLine{}
	for g := c.e.group; g >= 0; g = c.r.groups[g].parent {
		for _, h := range c.r.groups[g].hooks {
			lines = append(lines, h.body...)
		}
	}
	stubs := readChainStubs(append(lines, c.e.body...))
	slices.SortStableFunc(stubs, func(a, b chainStub) int { return a.t.line - b.t.line })
	for _, s := range stubs {
		received := regexp.MustCompile(`\bexpect\s*\(\s*` + regexp.QuoteMeta(s.recv) + `\s*\)\s*\.\s*to\s*\(?\s*have_received\s*\(\s*:([A-Za-z_]\w*[?!]?)`)
		final := s.chain[len(s.chain)-1]
		named := regexp.MustCompile(`(?:^|[^\w])` + regexp.QuoteMeta(final) + `(?:[^\w?!]|$)`)
		prefix, at, covered := "", 0, false
		for _, l := range c.exampleLines() {
			if l.line == s.t.line {
				continue
			}
			t := maskRuby(l)
			if !rspecExpectation.MatchString(t.masked) {
				continue
			}
			if named.MatchString(t.raw) {
				covered = true
			}
			for _, m := range received.FindAllStringSubmatch(t.masked, -1) {
				if prefix == "" && slices.Contains(s.chain[:len(s.chain)-1], m[1]) {
					prefix, at = m[1], l.line
				}
			}
		}
		if prefix == "" || covered {
			continue
		}
		out = append(out, c.finding(testvalidity.RuleRBMessageChainPrefixOnly, testvalidity.SeverityAbstained, test, at, s.t.line, fmt.Sprintf(
			"%s stubs the message chain %s at %s, and the assertion at %s verifies only have_received(:%s), a prefix of it; no expectation names the final message %s, so removing or replacing it still passes; hint: use expect(%s).to receive_message_chain(...) for the whole chain, or return a verifying double from %s and assert have_received(:%s) on it",
			s.recv, strings.Join(s.chain, "."), c.at(s.t.line), c.at(at), prefix, final, s.recv, s.chain[len(s.chain)-2], final)))
	}
	return out
}

// flagConditions returns one condition per forced flag (lookup and key) and its RB-FLAG-CONDITIONAL
// and RB-FLAG-FORCED-NON-DEFAULT findings (TSA-V0-436..438). The effective stub is the last to run:
// the body over hooks, an inner group's hook over an outer one; stubs of one rank that force
// different values leave the value unresolved. An effective stub that only calls through forces
// nothing, so its key has no condition.
func (c *flowContext) flagConditions(test string, sites []stubSite, lookups map[int]testvalidity.FlagLookup) ([]testvalidity.FlagCondition, []testvalidity.SourceFinding) {
	type group struct {
		lookup, key, keyText string
		sites                []stubSite
	}
	groups := map[string]*group{}
	for i, site := range sites {
		lookup := lookups[i].String()
		key, keyText := testvalidity.FlagAnyKey, ""
		if site.tail.hasWith {
			key = ""
			if args := topLevelArgs(site.s.t.masked, site.tail.with); len(args) > 0 {
				keyText = site.s.t.squash(args[0][0], args[0][1])
				if v := rubyLiteralValue(keyText); flagKey.MatchString(v) {
					key = v
				}
			}
		}
		id := lookup + "\x00" + key
		if key == "" {
			id += "\x00" + keyText // distinct unread key expressions stay distinct
		}
		if groups[id] == nil {
			groups[id] = &group{lookup: lookup, key: key, keyText: keyText}
		}
		groups[id].sites = append(groups[id].sites, site)
	}
	conditions := []testvalidity.FlagCondition{}
	for _, id := range slices.Sorted(maps.Keys(groups)) {
		g := groups[id]
		top := slices.MaxFunc(g.sites, func(a, b stubSite) int { return a.rank - b.rank }).rank
		forced, location, first, atTop, through := "", 0, true, 0, 0
		for _, site := range g.sites {
			if site.rank != top {
				continue
			}
			atTop++
			if site.tail.original {
				through++
			}
			value := forcedValue(site)
			if first {
				forced, first = value, false
			} else if value != forced {
				forced = ""
			}
			location = max(location, site.s.t.line)
		}
		if through == atTop {
			continue // the effective stub calls through, so it forces nothing
		}
		condition := testvalidity.FlagCondition{Lookup: g.lookup, Key: g.key, Forced: forced, Location: c.at(location),
			Qualifier: testvalidity.FlagQualifier(g.lookup, g.key, forced)}
		if value, ok := c.r.flags.Defaults[g.key]; ok && g.key != "" && g.key != testvalidity.FlagAnyKey {
			condition.Default, condition.DefaultFrom = value, c.r.flags.Source
		}
		conditions = append(conditions, condition)
	}
	slices.SortStableFunc(conditions, func(a, b testvalidity.FlagCondition) int {
		return strings.Compare(lineKey(a.Location), lineKey(b.Location))
	})
	conditional, nonDefault := []testvalidity.SourceFinding{}, []testvalidity.SourceFinding{}
	for _, condition := range conditions {
		line := 0
		fmt.Sscanf(condition.Location[strings.LastIndex(condition.Location, ":")+1:], "%d", &line)
		forced := condition.Forced
		if forced == "" {
			forced = "a value the reader cannot read as one literal"
		}
		fallback := "no code default is declared for this key, so whether the forced value is the default is not known"
		if condition.Default != "" {
			fallback = fmt.Sprintf("the declared code default is %s (%s)", condition.Default, condition.DefaultFrom)
		}
		conditional = append(conditional, c.finding(testvalidity.RuleRBFlagConditional, testvalidity.SeverityAdvisory, test, line, line, fmt.Sprintf(
			"the stub at %s forces the flag lookup %s to %s, so what the example proves is %s; %s",
			condition.Location, condition.Lookup, forced, condition.Qualifier, fallback)))
		if condition.Default != "" && condition.Forced != "" && condition.Forced != condition.Default {
			nonDefault = append(nonDefault, c.finding(testvalidity.RuleRBFlagForcedNonDefault, testvalidity.SeverityAdvisory, test, line, line, fmt.Sprintf(
				"the stub at %s forces flag %s to %s, which differs from its declared default %s (%s), so the example never runs the default path; cite a second example for the default or record the condition",
				condition.Location, condition.Key, condition.Forced, condition.Default, condition.DefaultFrom)))
		}
	}
	return conditions, append(conditional, nonDefault...)
}

// forcedValue is the literal a flag stub returns: "nil" without and_return, "" when it raises,
// calls through, runs a block, returns several or non-literal values, or continues past what the
// reader reads.
func forcedValue(site stubSite) string {
	switch {
	case site.tail.block || site.tail.unread || site.tail.raise || site.tail.original:
		return ""
	case !site.tail.hasReturn:
		return "nil"
	case site.tail.values != 1:
		return ""
	}
	return rubyLiteralValue(site.s.t.squash(site.tail.ret[0], site.tail.ret[1]))
}

// rubyLiteralValue is a literal's value: a symbol or string's text, or the literal itself; "" when
// text is not one literal.
func rubyLiteralValue(text string) string {
	if !rspecRubyLiteral.MatchString(text) {
		return ""
	}
	switch text[0] {
	case ':':
		return text[1:]
	case '"', '\'':
		return text[1 : len(text)-1]
	}
	return text
}

// DecodeFlagLookups decodes and validates a committed corvint-flag-lookups/0 declaration
// (TSA-V0-439); every defect refuses tests-declaration-invalid.
func DecodeFlagLookups(data []byte) (testvalidity.FlagLookups, error) {
	invalid := func(why string) (testvalidity.FlagLookups, error) {
		return testvalidity.FlagLookups{}, refuse("tests-declaration-invalid", "%s: %s", testvalidity.FlagLookupsPath, why)
	}
	var d testvalidity.FlagLookups
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return invalid("not a closed " + testvalidity.FlagLookupsSchema + " document")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return invalid("trailing data after the document")
	}
	switch {
	case d.Schema != testvalidity.FlagLookupsSchema:
		return invalid("schema must be " + testvalidity.FlagLookupsSchema)
	case len(d.Lookups) == 0 || len(d.Lookups) > maxFlagLookups:
		return invalid("lookups must hold 1 to 256 entries")
	case len(d.Defaults) > maxFlagDefaults:
		return invalid("defaults must hold at most 1024 keys")
	}
	seen := map[testvalidity.FlagLookup]bool{}
	for _, l := range d.Lookups {
		if !flagReceiver.MatchString(l.Receiver) || !flagMethod.MatchString(l.Method) || seen[l] {
			return invalid("each lookup must be a unique constant receiver path and method name")
		}
		seen[l] = true
	}
	for key, value := range d.Defaults {
		if !flagKey.MatchString(key) || value == "" || len(value) > 256 || strings.ContainsFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return invalid("default keys must match [\\w.:/-]{1,128} and values hold 1 to 256 bytes without control characters")
		}
	}
	for i := range d.Lookups {
		d.Lookups[i].Receiver = strings.TrimPrefix(d.Lookups[i].Receiver, "::")
	}
	d.Source = testvalidity.FlagLookupsPath
	return d, nil
}

// flagLookups reads the committed flag lookup declaration at commit, or the built-in list when none
// is committed (TSA-V0-435, TSA-V0-439).
func (r Repo) flagLookups(commit string) (testvalidity.FlagLookups, error) {
	data, found, err := r.readOne(commit, testvalidity.FlagLookupsPath, MaxFileBytes)
	if err != nil {
		return testvalidity.FlagLookups{}, err
	}
	if !found {
		return testvalidity.BuiltinFlagLookups(), nil
	}
	return DecodeFlagLookups(data)
}
