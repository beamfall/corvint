package testsource

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// LintSchema names the `corvint tests lint` report (TSA-V0-016).
const LintSchema = "corvint-tests-lint/0"

// Rule identifiers are stable; the set is closed (TSA-V0-013).
const (
	RuleNoAssertion       = "no-assertion"
	RuleWeakAssertionOnly = "weak-assertion-only"
	RuleConditional       = "conditional-assertion"
	RuleSwallowedWait     = "swallowed-wait-error"
	RuleFixedWait         = "fixed-wait"
	RuleTautological      = "tautological-assertion"
	RuleBrittleSelector   = "brittle-selector"
	RuleSkipOnly          = "skip-only-without-reason"
	// The two advisory rules (TSA-V0-029, TSA-V0-031): their findings never change the exit status.
	RuleOneShotUIState   = "one-shot-ui-state-assertion"
	RuleExactTextVariant = "exact-text-cross-variant"
)

// Rules lists every rule identifier in report order.
var Rules = []string{RuleNoAssertion, RuleWeakAssertionOnly, RuleConditional, RuleSwallowedWait, RuleFixedWait,
	RuleTautological, RuleBrittleSelector, RuleSkipOnly, RuleOneShotUIState, RuleExactTextVariant}

// Finding is one rule violation. Line is where it is reported and where a suppression must point;
// EndLine closes the span whose changed lines bring a test-level finding into scope. Advisory marks
// a finding of an advisory rule; Resolution is UNKNOWN when a helper could not be followed.
type Finding struct {
	Rule       string `json:"rule"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	EndLine    int    `json:"endLine"`
	Message    string `json:"message"`
	Advisory   bool   `json:"advisory,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// Suppression is one `corvint-lint-allow` annotation (TSA-V0-015). Valid ones name a known rule
// and give a reason; Used reports that it suppressed a finding.
type Suppression struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Target  int    `json:"target"`
	Rule    string `json:"rule"`
	Reason  string `json:"reason"`
	Valid   bool   `json:"valid"`
	Problem string `json:"problem,omitempty"`
	Used    bool   `json:"used"`
}

// LintReport is the deterministic `corvint tests lint` result (TSA-V0-016).
type LintReport struct {
	Schema       string        `json:"schema"`
	Base         string        `json:"base"`
	Revision     string        `json:"revision"`
	Scope        string        `json:"scope"`
	Rules        []string      `json:"rules"`
	Files        []string      `json:"files"`
	Findings     []Finding     `json:"findings"`
	Suppressed   []Finding     `json:"suppressed"`
	Suppressions []Suppression `json:"suppressions"`
	Uncertain    []string      `json:"uncertain"`
	CrossVariant CrossVariant  `json:"crossVariant"`
}

// Blocking counts the findings that are not advisory; only they make the exit status 1
// (TSA-V0-016, TSA-V0-029).
func (r LintReport) Blocking() int {
	n := 0
	for _, f := range r.Findings {
		if !f.Advisory {
			n++
		}
	}
	return n
}

// LintOptions select the revisions and the line scope. Runs is the Playwright evidence the
// cross-variant rule joins; nil reports it NOT_PRODUCED (TSA-V0-031, TSA-V0-032).
type LintOptions struct {
	Base, Revision string
	AllLines       bool
	Runs           []RunEvidence
}

// isLintTarget reports a test or page-object file (TSA-V0-012).
func isLintTarget(name string) bool { return IsTestFile(name) || IsPageObjectFile(name) }

// Lint runs the closed rule set over the test and page-object files changed from base to revision,
// reading committed objects only (TSA-V0-012..016).
func Lint(r Repo, options LintOptions) (LintReport, error) { return lint(r, options, newReadBudget()) }

// lint is Lint drawing every read, the linted files and the helper modules they import, from one
// budget (TSA-V0-012, TSA-V0-030).
func lint(r Repo, options LintOptions, budget *readBudget) (LintReport, error) {
	revision, err := r.Resolve(options.Revision)
	if err != nil {
		return LintReport{}, err
	}
	// --all-lines reads the whole tree, so it needs no base; a base given with it is still
	// resolved and recorded (TSA-V0-650).
	base, changed := "", map[string]*lineChanges{}
	if options.Base != "" || !options.AllLines {
		if base, err = r.Resolve(options.Base); err != nil {
			return LintReport{}, err
		}
		if changed, err = changedLines(r, base, revision, isLintTarget); err != nil {
			return LintReport{}, err
		}
	}
	// --all-lines reads every test and page-object file at the revision, changed or not (TSA-V0-012).
	keep := func(name string) bool { _, ok := changed[name]; return ok }
	if options.AllLines {
		keep = isLintTarget
	}
	texts, _, err := readFilesWithin(r, revision, keep, budget)
	if err != nil {
		return LintReport{}, err
	}
	report := LintReport{Schema: LintSchema, Base: base, Revision: revision, Scope: "changed-lines", Rules: Rules,
		Files: []string{}, Findings: []Finding{}, Suppressed: []Finding{}, Suppressions: []Suppression{}, Uncertain: []string{}}
	if options.AllLines {
		report.Scope = "all-lines"
	}
	for name := range texts {
		report.Files = append(report.Files, name)
	}
	sort.Strings(report.Files)
	sources := map[string]*source{}
	for _, name := range report.Files {
		sources[name] = parseSource(name, texts[name])
	}
	c := newLintContext(sources)
	if err := c.readImports(r, revision, budget); err != nil {
		return LintReport{}, err
	}
	report.CrossVariant = c.joinRuns(options.Runs, revision, report.Files)
	for _, name := range report.Files {
		inScope := func(start, end int) bool {
			return options.AllLines || changed[name].touches(start, end)
		}
		lintFile(&report, c, sources[name], inScope)
	}
	// A helper module the lexer could not fully place may hide or misplace a helper the advisory
	// rules follow, so it is uncertain too (TSA-V0-010).
	for name, s := range c.sources {
		if _, linted := texts[name]; !linted && s.Uncertain {
			report.Uncertain = append(report.Uncertain, name)
		}
	}
	sort.Strings(report.Uncertain)
	c.collectSplits(&report.CrossVariant)
	return report, nil
}

// LintText lints one source with every line in scope; it serves fixtures and callers that already
// hold the text.
// Helpers resolve within the text only.
func LintText(name, text string) LintReport { return lintText(name, text, nil, "") }

func lintText(name, text string, runs []RunEvidence, revision string) LintReport {
	report := LintReport{Schema: LintSchema, Revision: revision, Scope: "all-lines", Rules: Rules, Files: []string{name}, Findings: []Finding{},
		Suppressed: []Finding{}, Suppressions: []Suppression{}, Uncertain: []string{}}
	s := parseSource(name, text)
	c := newLintContext(map[string]*source{name: s})
	report.CrossVariant = c.joinRuns(runs, revision, report.Files)
	lintFile(&report, c, s, func(int, int) bool { return true })
	c.collectSplits(&report.CrossVariant)
	return report
}

func lintFile(report *LintReport, c *lintContext, s *source, inScope func(start, end int) bool) {
	if s.Uncertain {
		report.Uncertain = append(report.Uncertain, s.Path)
	}
	findings := sourceLint(s, c)
	suppressions := suppressions(s)
	for _, f := range findings {
		if !inScope(f.Line, f.EndLine) {
			continue
		}
		suppressed := false
		for i := range suppressions {
			if suppressions[i].Valid && suppressions[i].Rule == f.Rule && suppressions[i].Target == f.Line {
				suppressions[i].Used, suppressed = true, true
			}
		}
		if suppressed {
			report.Suppressed = append(report.Suppressed, f)
		} else {
			report.Findings = append(report.Findings, f)
		}
	}
	for _, sup := range suppressions {
		if sup.Used || inScope(sup.Line, sup.Line) || inScope(sup.Target, sup.Target) {
			report.Suppressions = append(report.Suppressions, sup)
		}
	}
}

// lint applies every rule and returns findings ordered by line, then rule.
func sourceLint(s *source, c *lintContext) []Finding {
	out := []Finding{}
	add := func(rule string, line, end int, message string) {
		if end < line {
			end = line
		}
		out = append(out, Finding{Rule: rule, Path: s.Path, Line: line, EndLine: end, Message: message})
	}
	emit := func(f Finding) { out = append(out, f) }
	for _, d := range s.Decls {
		lintDeclaration(s, d, add)
		if d.Kind == "test" {
			lintExactVariant(s, c, d, emit)
		}
	}
	retrying := toPassRanges(s)
	for i, t := range s.Tokens {
		lintToken(s, i, t, add)
		if s.Ident(i, "expect") && !s.MemberAccess(i) {
			lintOneShot(s, c, i, retrying, emit)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Line != out[b].Line {
			return out[a].Line < out[b].Line
		}
		return out[a].Rule < out[b].Rule
	})
	// One finding per rule and line.
	unique := out[:0]
	for i, f := range out {
		if i > 0 && f.Rule == out[i-1].Rule && f.Line == out[i-1].Line {
			continue
		}
		unique = append(unique, f)
	}
	return unique
}

var weakMatchers = map[string]bool{"toBeVisible": true, "toBeHidden": true, "toHaveURL": true, "toBeAttached": true, "toBeInViewport": true}

func lintDeclaration(s *source, d decl, add func(string, int, int, string)) {
	switch {
	case d.Modifiers["only"]:
		add(RuleSkipOnly, d.Line, d.Line, "a focused (only) "+d.Kind+" runs alone and silently drops the rest of the suite")
	case (d.Modifiers["skip"] || d.Modifiers["fixme"]) && !hasReason(s, d):
		add(RuleSkipOnly, d.Line, d.Line, "a skipped "+d.Kind+" gives no reason")
	}
	if d.Kind != "test" || d.Modifiers["skip"] || d.Modifiers["fixme"] || !s.Callback(d.Body) {
		return
	}
	expects, helpers, weak := 0, 0, 0
	for i := d.Body[0]; i < d.Body[1]; i++ {
		if !s.IsAssertion(i) {
			continue
		}
		chain, ok := s.ReadExpect(i)
		if !ok {
			helpers++
			continue
		}
		expects++
		if weakMatchers[chain.Matcher] {
			weak++
		}
	}
	switch {
	case expects+helpers == 0:
		add(RuleNoAssertion, d.Line, d.EndLine, "the test makes no assertion")
	case helpers == 0 && weak == expects:
		add(RuleWeakAssertionOnly, d.Line, d.EndLine, "the test asserts only visibility or the URL")
	}
}

// hasReason reports a skip or fixme with a reason: a runtime call with a non-empty string
// argument, or a declaration whose details carry an annotation description.
func hasReason(s *source, d decl) bool {
	args := d.Args
	if d.Kind != "annotation" {
		if len(args) < 3 {
			return false
		}
		for i := args[1][0]; i < args[len(args)-2][1]; i++ {
			if s.Ident(i, "description") && s.Punct(i+1, ":") && s.LoneString([2]int{i + 2, i + 3}) && strings.TrimSpace(s.Tokens[i+2].Text) != "" {
				return true
			}
		}
		return false
	}
	for _, arg := range args {
		if s.LoneString(arg) && strings.TrimSpace(s.Tokens[arg[0]].Text) != "" {
			return true
		}
	}
	return false
}

var selectorMethods = map[string]bool{"locator": true, "$": true, "$$": true, "$eval": true, "$$eval": true, "click": true,
	"dblclick": true, "fill": true, "type": true, "press": true, "check": true, "uncheck": true, "hover": true, "tap": true,
	"focus": true, "waitForSelector": true, "querySelector": true, "querySelectorAll": true, "get": true,
	"textContent": true, "innerText": true, "innerHTML": true, "inputValue": true, "isVisible": true, "isHidden": true,
	"getAttribute": true, "selectOption": true, "setInputFiles": true, "dispatchEvent": true}

// valueFirst lists the selector methods whose one-argument locator form takes a value, not a
// selector: only their two-argument page form is checked.
var valueFirst = map[string]bool{"fill": true, "type": true, "press": true, "getAttribute": true, "selectOption": true,
	"setInputFiles": true, "dispatchEvent": true}

var (
	positional = regexp.MustCompile(`:nth-child\(|:nth-last-child\(|:nth-of-type\(|:nth-last-of-type\(|:first-child|:last-child|:eq\(|(^|\s|>>)nth=`)
	cssClass   = regexp.MustCompile(`(^|[\s>+~,(a-zA-Z0-9_\]*)-])\.[A-Za-z_-][A-Za-z0-9_-]*`)
	bracketed  = regexp.MustCompile(`\[[^\]]*\]|"[^"]*"|'[^']*'`)
)

// brittle names why a selector literal is brittle, or returns "".
func brittle(selector string) string {
	trimmed := strings.TrimSpace(selector)
	switch {
	case strings.HasPrefix(trimmed, "//"), strings.HasPrefix(trimmed, "(//"), strings.HasPrefix(trimmed, "xpath="), strings.HasPrefix(trimmed, "./"):
		return "an xpath selector"
	case positional.MatchString(trimmed):
		return "a positional selector"
	}
	for _, engine := range []string{"text=", "role=", "data-testid=", "id=", "internal:", "label=", "placeholder=", "alt=", "title="} {
		if strings.HasPrefix(trimmed, engine) {
			return ""
		}
	}
	if cssClass.MatchString(bracketed.ReplaceAllString(trimmed, "")) {
		return "a CSS-class selector"
	}
	return ""
}

func lintToken(s *source, i int, t appmap.JSToken, add func(string, int, int, string)) {
	switch {
	case t.Kind == appmap.JSString && t.Literal && t.Text == "networkidle":
		add(RuleFixedWait, t.Line, t.Line, "a networkidle wait depends on background traffic, not on the state under test")
	case t.Kind != appmap.JSIdent:
		return
	}
	switch name := t.Text; {
	case name == "waitForTimeout" && s.Punct(i+1, "("):
		add(RuleFixedWait, t.Line, t.Line, "a fixed sleep (waitForTimeout)")
	case name == "setTimeout" && !s.MemberAccess(i) && s.Punct(i+1, "(") && numericArg(s, i+1):
		add(RuleFixedWait, t.Line, t.Line, "a fixed sleep (setTimeout)")
	case (name == "sleep" || name == "delay" || name == "pause" || name == "wait") && s.Punct(i+1, "(") &&
		s.Tokens[min(i+2, len(s.Tokens)-1)].Kind == appmap.JSNumber && s.Punct(i+3, ")"):
		add(RuleFixedWait, t.Line, t.Line, "a fixed sleep ("+name+")")
	case name == "nth" && s.MemberAccess(i) && s.Punct(i+1, "(") && i+2 < len(s.Tokens) && s.Tokens[i+2].Kind == appmap.JSNumber:
		add(RuleBrittleSelector, t.Line, t.Line, "a positional selector (nth)")
	case selectorMethods[name] && (s.MemberAccess(i) || name == "$" || name == "$$") && s.Punct(i+1, "(") && s.Match[i+1] > 0:
		if name == "get" && !s.Ident(i-2, "cy") || valueFirst[name] && len(s.SplitArgs(i+1, s.Match[i+1])) < 2 {
			return
		}
		if arg := s.Tokens[i+2]; arg.Kind == appmap.JSString && arg.Literal {
			if why := brittle(arg.Text); why != "" {
				add(RuleBrittleSelector, arg.Line, arg.Line, why+": "+arg.Text)
			}
		}
	case name == "expect":
		lintExpect(s, i, add)
	case name == "catch" && s.MemberAccess(i) && s.Punct(i+1, "("):
		lintCatchCall(s, i, add)
	case name == "try" && s.Punct(i+1, "{"):
		lintTryCatch(s, i, add)
	}
	if s.IsAssertion(i) {
		if guard := guard(s, i); guard != "" {
			add(RuleConditional, t.Line, t.Line, "an assertion inside "+guard+" may never run")
		}
	}
}

// numericArg reports a call at open whose arguments include a top-level number.
func numericArg(s *source, open int) bool {
	if s.Match[open] < 0 {
		return false
	}
	for _, arg := range s.SplitArgs(open, s.Match[open]) {
		if arg[1]-arg[0] == 1 && s.Tokens[arg[0]].Kind == appmap.JSNumber {
			return true
		}
	}
	return false
}

// lintExpect flags a literal subject, or a subject equal to the toBe/toEqual argument.
func lintExpect(s *source, i int, add func(string, int, int, string)) {
	chain, ok := s.ReadExpect(i)
	if !ok {
		return
	}
	line := s.Tokens[i].Line
	subject := chain.Subject
	if subject[1]-subject[0] == 1 {
		t := s.Tokens[subject[0]]
		literalWord := t.Kind == appmap.JSIdent && (t.Text == "true" || t.Text == "false" || t.Text == "null" || t.Text == "undefined")
		if t.Kind == appmap.JSNumber || (t.Kind == appmap.JSString && t.Literal) || literalWord {
			add(RuleTautological, line, line, "the assertion's subject is a literal")
			return
		}
	}
	switch chain.Matcher {
	case "toBe", "toEqual", "toStrictEqual":
		if subject[1] > subject[0] && s.Render(subject[0], subject[1]) == s.Render(chain.Margs[0], chain.Margs[1]) {
			add(RuleTautological, line, line, "the assertion compares a value with itself")
		}
	}
}

// waits reports a wait or assertion call inside [start, end).
func waits(s *source, start, end int) bool {
	for i := start; i < end; i++ {
		if s.Ident(i, "") && (strings.HasPrefix(s.Tokens[i].Text, "waitFor") || s.IsAssertion(i)) {
			return true
		}
	}
	return false
}

// swallows reports a handler range that neither rethrows, rejects nor asserts.
func swallows(s *source, start, end int) bool {
	for i := start; i < end; i++ {
		if s.Ident(i, "throw") || s.Ident(i, "reject") || s.IsAssertion(i) {
			return false
		}
	}
	return true
}

// lintCatchCall flags `<wait>.catch(handler)` whose handler swallows the failure.
func lintCatchCall(s *source, i int, add func(string, int, int, string)) {
	open := i + 1
	if s.Match[open] < 0 {
		return
	}
	args := s.SplitArgs(open, s.Match[open])
	if len(args) != 1 || !s.Callback(args[0]) || !swallows(s, args[0][0], args[0][1]) {
		return
	}
	if start := receiverStart(s, i-1); waits(s, start, i-1) {
		add(RuleSwallowedWait, s.Tokens[i].Line, s.Tokens[i].Line, "a catch handler swallows a failed wait or assertion")
	}
}

// receiverStart walks back from the `.` at dot over the member chain it is called on.
func receiverStart(s *source, dot int) int {
	j := dot - 1
	for j >= 0 {
		t := s.Tokens[j]
		if t.Kind == appmap.JSPunct && (t.Text == ")" || t.Text == "]") && s.Match[j] >= 0 {
			j = s.Match[j] - 1
			continue
		}
		if t.Kind == appmap.JSPunct {
			break
		}
		j--
		if s.Punct(j, ".") {
			j--
			continue
		}
		break
	}
	return j + 1
}

// lintTryCatch flags `try { <wait> } catch [(e)] { <swallow> }`.
func lintTryCatch(s *source, i int, add func(string, int, int, string)) {
	block := i + 1
	end := s.Match[block]
	if end < 0 || !s.Ident(end+1, "catch") {
		return
	}
	handler := end + 2
	if s.Punct(handler, "(") && s.Match[handler] > 0 {
		handler = s.Match[handler] + 1
	}
	if !s.Punct(handler, "{") || s.Match[handler] < 0 {
		return
	}
	if waits(s, block, end) && swallows(s, handler, s.Match[handler]) {
		line := s.Tokens[end+1].Line
		add(RuleSwallowedWait, line, line, "a catch block swallows a failed wait or assertion")
	}
}

// guard names the if/else/try/catch/switch region or ternary that governs token i inside its
// nearest enclosing function, or "" (TSA-V0-013).
func guard(s *source, i int) string {
	// The same statement: a ternary, a brace-less if or a brace-less else.
	for j := i - 1; j >= 0; j-- {
		if s.Punct(j, ";") || s.Punct(j, "{") || s.Punct(j, "}") {
			break
		}
		if s.Punct(j, "?") && !s.Punct(j+1, ".") && !s.Punct(j+1, "?") && !s.Punct(j-1, "?") && !s.Punct(j+1, ":") {
			return "a conditional expression"
		}
		if s.Ident(j, "else") {
			return "an else"
		}
		if (s.Punct(j, ")") || s.Punct(j, "]")) && s.Match[j] >= 0 {
			if s.Ident(s.Match[j]-1, "if") {
				return "an if"
			}
			j = s.Match[j]
		}
	}
	// Enclosing blocks, up to the nearest function body.
	for open := i - 1; open >= 0; open-- {
		if (s.Punct(open, "}") || s.Punct(open, ")") || s.Punct(open, "]")) && s.Match[open] >= 0 {
			open = s.Match[open]
			continue
		}
		if !s.Punct(open, "{") {
			continue
		}
		before := open - 1
		if s.Punct(before, ">") && s.Arrow(before-1) {
			return ""
		}
		if s.Punct(before, ")") && s.Match[before] >= 0 {
			switch head := s.Match[before] - 1; {
			case s.Ident(head, "if"):
				return "an if"
			case s.Ident(head, "switch"):
				return "a switch"
			case s.Ident(head, "catch"):
				return "a catch"
			case s.Ident(head, "for"), s.Ident(head, "while"), s.Ident(head, "with"):
			default:
				return "" // a function or method body
			}
			continue
		}
		switch {
		case s.Ident(before, "else"):
			return "an else"
		case s.Ident(before, "try") && tryCatches(s, open):
			return "a try"
		case s.Ident(before, "catch"):
			return "a catch"
		}
	}
	return ""
}

// tryCatches reports that the try block opening at open has a catch that may swallow its
// failure. A finally-only try catches nothing, and a catch whose own block always rethrows (a
// top-level `throw` with no top-level return, branch or loop control before it) lets a failed
// assertion still fail the test, so neither makes an assertion conditional (TSA-V0-652).
func tryCatches(s *source, open int) bool {
	end := s.Match[open]
	if end < 0 || !s.Ident(end+1, "catch") {
		return false
	}
	handler := end + 2
	if s.Punct(handler, "(") && s.Match[handler] > 0 {
		handler = s.Match[handler] + 1
	}
	if !s.Punct(handler, "{") || s.Match[handler] < 0 {
		return true
	}
	for i := handler + 1; i < s.Match[handler]; i++ {
		t := s.Tokens[i]
		switch {
		case t.Kind == appmap.JSPunct && (t.Text == "(" || t.Text == "[" || t.Text == "{") && s.Match[i] > i:
			if t.Text == "{" {
				return true // a nested block may hold a branch
			}
			i = s.Match[i]
		case t.Kind == appmap.JSPunct && (t.Text == "?" || t.Text == "&" || t.Text == "|"):
			return true
		case t.Kind == appmap.JSIdent && catchBranches[t.Text]:
			return true
		case t.Kind == appmap.JSIdent && t.Text == "throw":
			return false
		}
	}
	return true
}

// catchBranches are the statements before which a catch's rethrow may not run.
var catchBranches = map[string]bool{"if": true, "else": true, "return": true, "switch": true, "break": true,
	"continue": true, "for": true, "while": true, "do": true, "try": true}

// suppressions reads every `corvint-lint-allow <rule>: <reason>` comment (TSA-V0-015). A trailing
// comment targets its own line; a comment alone on its line targets the next line holding code.
func suppressions(s *source) []Suppression {
	out := []Suppression{}
	codeLines := map[int]bool{}
	for _, t := range s.Tokens {
		codeLines[t.Line] = true
	}
	known := map[string]bool{}
	for _, rule := range Rules {
		known[rule] = true
	}
	for _, c := range s.Comments {
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"), "*/"))
		rest, ok := strings.CutPrefix(body, "corvint-lint-allow")
		if !ok || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rule, reason, _ := strings.Cut(strings.TrimSpace(rest), ":")
		sup := Suppression{Path: s.Path, Line: c.Line, Rule: strings.TrimSpace(rule), Reason: strings.TrimSpace(reason)}
		sup.Target = c.Line
		if !codeLines[c.Line] {
			last := c.Line + strings.Count(c.Text, "\n")
			for line := last + 1; line <= last+1000 && !codeLines[sup.Target]; line++ {
				if codeLines[line] {
					sup.Target = line
				}
			}
		}
		switch {
		case !known[sup.Rule]:
			sup.Problem = "unknown-rule"
		case sup.Reason == "":
			sup.Problem = "missing-reason"
		default:
			sup.Valid = true
		}
		out = append(out, sup)
	}
	return out
}
