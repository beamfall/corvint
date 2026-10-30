package testsource

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// BLOCK-NOT-CALLED (TSA-V0-053..055): a spec helper that takes a block and either hands an
// assertion a literal block that only references it, or never calls it, makes every example that
// passes it a block vacuous. The reading is line-based and conservative: any use of the block it
// cannot classify counts as a call, so an unsure helper is never reported.

var (
	// rspecDefHead matches an instance method's def head; group 1 is the name, group 2 or 3 the
	// parameter list. A `def self.x` is a group-level method an example cannot call.
	rspecDefHead = regexp.MustCompile(`^def\s+([A-Za-z_]\w*[?!]?)\s*(?:\((.*)\)|\s+(.*))?$`)
	// rspecNamedBlockParam is a named block parameter, which Ruby requires to be the last one.
	rspecNamedBlockParam = regexp.MustCompile(`(?:^|,)\s*&([A-Za-z_]\w*)\s*$`)
	// rspecAnonBlockParam is an anonymous block parameter (`&`), whose forwarding is not read.
	rspecAnonBlockParam = regexp.MustCompile(`(?:^|,)\s*&\s*$`)
	// rspecAssertionHead names a call that asserts over the block it is given.
	rspecAssertionHead = regexp.MustCompile(`(?:^|[^.\w:@$])((?:assert|refute)(?:_\w+)?|expect|perform_enqueued_jobs)\b[?!]?`)
	rspecDoOpener      = regexp.MustCompile(`^do(?:\s*\|[^|]*\|)?$`)
	rspecBraceOpener   = regexp.MustCompile(`\{\s*(?:\|[^|]*\|)?$`)
	rspecBraceParams   = regexp.MustCompile(`^\s*\|[^|]*\|`)
	rspecTrailingPipes = regexp.MustCompile(`\|[^|]*\|\s*$`)
	rspecParenlessDo   = regexp.MustCompile(`^([^{}]*?)\s+do(?:\s*\|[^|]*\|)?$`)
	rspecDefLine       = regexp.MustCompile(`^def\b`)
	rspecBlockArgument = regexp.MustCompile(`(?:^|,)\s*&`)
	// rspecHiddenText is a string interpolation or a heredoc, whose code the line reading hides.
	rspecHiddenText = regexp.MustCompile(`#\{|<<[~-][A-Za-z_'"]`)
	// rspecBlockRefusal makes a block_given? check something other than a forgotten yield.
	rspecBlockRefusal = map[string]bool{"raise": true, "fail": true, "enum_for": true, "to_enum": true}
)

// rspecHelper is one `def` read directly in an example group or at file level: its group and
// its logical lines, the head and the closing `end` included.
type rspecHelper struct {
	group int
	path  string
	body  []rubyLine
}

// blockDefect is a BLOCK-NOT-CALLED helper: its name, block parameter (empty for a helper that
// only checks block_given?), the line of the finding, and whether that line is an assertion whose
// literal block only references the parameter (vacuous) or the def line of a helper that never
// calls its block.
type blockDefect struct {
	name     string
	param    string
	line     int
	vacuous  bool
	reported bool
}

// blockNotCalled reports each defective helper at its definition and each example in its scope
// that calls it with a block (TSA-V0-054).
func (r *rspecReader) blockNotCalled() []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for h, helper := range r.helpers {
		d := helperDefect(helper.body)
		if !d.reported {
			continue
		}
		path := helper.path
		def := fmt.Sprintf("%s:%d", path, helper.body[0].line)
		message := fmt.Sprintf("%s takes a block but never calls it on any path, so the block each example passes never runs and its expectations prove nothing; call it (block.call or yield) or pass it on with &%s", d.name, d.param)
		if d.param == "" {
			message = fmt.Sprintf("%s checks block_given? but never yields, so the block each example passes never runs and its expectations prove nothing; yield to the block", d.name)
		}
		if d.vacuous {
			message = fmt.Sprintf("%s hands the assertion at %s:%d a block that only references %s instead of calling it, so the assertion runs nothing and passes for every example that calls %s; call it inside the block (%s.call or yield) or pass &%s to the assertion", d.name, path, d.line, d.param, d.name, d.param, d.param)
		}
		out = append(out, testvalidity.SourceFinding{
			Rule: testvalidity.RuleBlockNotCalled, Severity: testvalidity.SeverityInvalid,
			Test: def, Title: d.name, Location: fmt.Sprintf("%s:%d", path, d.line), Cause: def, Message: message,
		})
		for _, e := range r.examples {
			out = append(out, r.blockNotCalledAt(e, h, d)...)
		}
	}
	return out
}

// blockNotCalledFor returns the BLOCK-NOT-CALLED findings of one example only (TSA-V0-054).
func (r *rspecReader) blockNotCalledFor(e rspecExample) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for h, helper := range r.helpers {
		if d := helperDefect(helper.body); d.reported {
			out = append(out, r.blockNotCalledAt(e, h, d)...)
		}
	}
	return out
}

// blockNotCalledAt is the finding of example e when it passes defective helper h a block.
func (r *rspecReader) blockNotCalledAt(e rspecExample, h int, d blockDefect) []testvalidity.SourceFinding {
	call := helperCall(e.body, d.name)
	if call == 0 {
		return nil
	}
	// The support-aware resolution decides; one that abstains is reported by vacuousFindings.
	res := r.resolveAcross(e.group, d.name)
	if !res.found || res.helper.path != r.helpers[h].path || res.helper.body[0].line != r.helpers[h].body[0].line {
		return nil
	}
	def := fmt.Sprintf("%s:%d", r.helpers[h].path, r.helpers[h].body[0].line)
	return []testvalidity.SourceFinding{{
		Rule: testvalidity.RuleBlockNotCalled, Severity: testvalidity.SeverityInvalid,
		Test: fmt.Sprintf("%s:%d", e.path, e.line), Title: e.title, Location: fmt.Sprintf("%s:%d", e.path, call), Cause: def,
		Message: fmt.Sprintf("the block this example passes to %s never runs, because the helper at %s does not call it, so the example passes whatever the code under test does; fix the helper", d.name, def),
	}}
}

func helperName(body []rubyLine) string {
	head := stripRubyStrings(body[0].text)
	if rubyOneLine.MatchString(head) {
		head, _, _ = strings.Cut(head, ";") // a one-line def (TSA-V0-084)
	}
	if m := rspecDefHead.FindStringSubmatch(strings.TrimSpace(head)); m != nil {
		return m[1]
	}
	return ""
}

// helperDefect reads one def (TSA-V0-053..055). It is reported when the def has a block
// parameter, or checks block_given?, and either passes an assertion a literal block whose body
// only references the parameter, or never calls the block. A call is `yield`, `super` (which
// forwards the block), `binding`, or any use of the parameter other than a bare reference
// (optionally `.itself`) that is a literal block's whole body. A bare reference at the def's own
// level may be its return value, so it also counts as a call. A def with an anonymous `&` or a
// `...` parameter, a nested def, a string interpolation or a heredoc is not read, nor is a
// block_given? helper that raises or builds an enumerator.
func helperDefect(body []rubyLine) blockDefect {
	if len(body) < 2 {
		return blockDefect{}
	}
	m := rspecDefHead.FindStringSubmatch(stripRubyStrings(body[0].text))
	if m == nil {
		return blockDefect{}
	}
	params := m[2] + m[3]
	if strings.Contains(params, "...") || rspecAnonBlockParam.MatchString(params) {
		return blockDefect{}
	}
	d := blockDefect{name: m[1]}
	if p := rspecNamedBlockParam.FindStringSubmatch(params); p != nil {
		d.param = p[1]
	}
	inner := body[1 : len(body)-1]
	texts := make([]string, len(inner))
	for k, l := range inner {
		texts[k] = strings.TrimSpace(stripRubyStrings(l.text))
		if rspecDefLine.MatchString(texts[k]) || rspecHiddenText.MatchString(l.text) {
			return blockDefect{}
		}
	}
	called, checked, refuses := false, false, false
	literal := literalBlockDepths(texts)
	for k, t := range texts {
		for _, loc := range rspecIdentifier.FindAllStringIndex(t, -1) {
			word := t[loc[0]:loc[1]]
			if loc[0] > 0 && strings.ContainsRune(".:@$", rune(t[loc[0]-1])) {
				continue
			}
			if rest := t[loc[1]:]; strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "::") {
				continue // a hash key; a ternary's `block : x` is a use
			}
			refuses = refuses || rspecBlockRefusal[word]
			switch {
			case word == "yield" || word == "super" || word == "binding":
				called = true
			case word == "block_given?":
				checked = true
			case d.param != "" && word == d.param && !referenceOnly(t, loc, literal[k]):
				called = true
			}
		}
	}
	if d.param == "" && (!checked || refuses) {
		return blockDefect{}
	}
	if d.param != "" {
		if line := vacuousAssertion(inner, texts, d.param); line > 0 {
			d.line, d.vacuous, d.reported = line, true, true
			return d
		}
	}
	if !called {
		d.line, d.reported = body[0].line, true
	}
	return d
}

// literalBlockDepths returns, for each line, whether it sits inside a literal do or brace block
// opened within the def body; an if/unless/while/case/begin frame is not a literal block.
func literalBlockDepths(texts []string) []bool {
	out := make([]bool, len(texts))
	stack := []bool{}
	for k, t := range texts {
		if (rubyEnd.MatchString(t) || strings.HasPrefix(t, "}")) && len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
		for _, lit := range stack {
			out[k] = out[k] || lit
		}
		switch {
		case rubyDo.MatchString(t) || rspecBraceOpener.MatchString(t):
			stack = append(stack, true)
		case rubyOpener.MatchString(t):
			stack = append(stack, false)
		}
	}
	return out
}

// referenceOnly reports whether the parameter at loc in line t is only referenced: the whole of a
// one-line literal block (`{ block }`, `{ block.itself }`), or the whole line inside a multi-line
// literal block.
func referenceOnly(t string, loc []int, insideLiteral bool) bool {
	after := strings.TrimPrefix(t[loc[1]:], ".itself")
	if insideLiteral && loc[0] == 0 && strings.TrimSpace(after) == "" {
		return true
	}
	before := strings.TrimRight(rspecTrailingPipes.ReplaceAllString(t[:loc[0]], ""), " \t")
	return strings.HasSuffix(before, "{") && strings.HasPrefix(strings.TrimLeft(after, " \t"), "}")
}

// vacuousAssertion returns the line of the first assertion call whose literal block's whole body
// is a bare reference to param, or 0.
func vacuousAssertion(inner []rubyLine, texts []string, param string) int {
	ref := regexp.MustCompile(`^` + regexp.QuoteMeta(param) + `(?:\.itself)?$`)
	for k, t := range texts {
		for _, m := range rspecAssertionHead.FindAllStringIndex(t, -1) {
			after, ok := afterArguments(t[m[1]:])
			if !ok {
				continue
			}
			switch {
			case strings.HasPrefix(after, "{"):
				if end := closingBrace(after); end > 0 {
					if ref.MatchString(strings.TrimSpace(rspecBraceParams.ReplaceAllString(after[1:end], ""))) {
						return inner[k].line
					}
				} else if rspecBraceOpener.MatchString(after) && k+2 < len(texts) && ref.MatchString(texts[k+1]) && strings.HasPrefix(texts[k+2], "}") {
					return inner[k].line
				}
			case rspecDoOpener.MatchString(after):
				end := rubyBlockEnd(inner, k)
				if end >= len(inner) || end-k < 2 {
					continue
				}
				only := true
				for _, b := range texts[k+1 : end] {
					only = only && ref.MatchString(b)
				}
				if only {
					return inner[k].line
				}
			}
		}
	}
	return 0
}

// afterArguments skips an assertion's arguments, parenthesised or not, and returns the trimmed
// text that follows them. ok is false when what follows cannot be a block of that call.
func afterArguments(rest string) (string, bool) {
	trimmed := strings.TrimLeft(rest, " \t")
	switch {
	case strings.HasPrefix(trimmed, "("):
		args, ok := parenArguments(trimmed)
		if !ok {
			return "", false
		}
		return strings.TrimLeft(trimmed[len(args)+2:], " \t"), true
	case strings.HasPrefix(trimmed, "{") || rspecDoOpener.MatchString(trimmed):
		return trimmed, true
	case trimmed != "" && len(trimmed) < len(rest):
		// Arguments without parentheses: only a do block binds to this call.
		if m := rspecParenlessDo.FindStringSubmatch(trimmed); m != nil && !strings.HasPrefix(m[1], ".") {
			return strings.TrimLeft(trimmed[len(m[1]):], " \t"), true
		}
	}
	return "", false
}

// closingBrace returns the index of the `}` that closes the `{` at text[0], or -1.
func closingBrace(text string) int {
	depth := 0
	for i, c := range text {
		switch c {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

// helperCall returns the line of the first call in an example's lines that passes name a block
// (`name { }`, `name(args) { }`, `name args do`, `name(&blk)`), or 0.
func helperCall(body []rubyLine, name string) int {
	for _, l := range body {
		t := stripRubyStrings(l.text)
		for _, loc := range rspecIdentifier.FindAllStringIndex(t, -1) {
			if t[loc[0]:loc[1]] != name || loc[0] > 0 && strings.ContainsRune(".:@$", rune(t[loc[0]-1])) {
				continue
			}
			rest := t[loc[1]:]
			if args, ok := parenArguments(strings.TrimLeft(rest, " \t")); ok && rspecBlockArgument.MatchString(args) {
				return l.line
			}
			if after, ok := afterArguments(rest); ok && (strings.HasPrefix(after, "{") || rspecDoOpener.MatchString(after)) {
				return l.line
			}
		}
	}
	return 0
}

// parenArguments returns the text inside the parentheses that open text; ok is false when text
// does not start with a balanced parenthesis.
func parenArguments(text string) (string, bool) {
	if !strings.HasPrefix(text, "(") {
		return "", false
	}
	depth := 0
	for i, c := range text {
		switch c {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return text[1:i], true
			}
		}
	}
	return "", false
}
