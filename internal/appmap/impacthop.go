package appmap

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Bounds of the template/component reference hop (ANL-V0-032). A file over the per-file bound,
// or one that would carry the read set past the total bound, is not loaded but scanned for its
// names; a walk whose names it may hold reads unknown `reference-graph-bound` naming that file and
// bound (ANL-V0-069). Exceeding the file count or the walked nodes leaves the path unknown with the
// bound named. No bound ever narrows the verdict.
const (
	hopMaxFiles   = 20000
	hopMaxFile    = 1 << 20
	hopMaxTotal   = 32 << 20
	hopMaxNodes   = 64
	hopMaxLiteral = 512
	// An unread file is scanned for its distinct names within these bounds; past them it may
	// mention any path.
	hopMaxScan      = 64 << 20
	hopMaxScanTotal = 256 << 20
	hopMaxRuns      = 1 << 18
	hopMaxRun       = 256
)

var hopScriptExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte"}

// hopRegistrations are the AngularJS module methods whose first argument registers a DI name
// (ANL-V0-071): every name-taking registration of the AngularJS module loader.
var hopRegistrations = []string{"factory", "service", "provider", "value", "constant", "decorator", "animation", "filter", "component", "directive", "controller"}

// hopDI is what one walked script registers: tokens whose every bounded occurrence references it,
// and directive or component names that any markup spelling normalizing to them references.
type hopDI struct {
	tokens []string
	ng     []string
}

// hopTarget is one terminal a reference walk reached: screens selected through a router
// anchor, or a test file of the map.
type hopTarget struct {
	screens []string
	specs   []string
	via     string
}

// referenceGraph holds the tracked UI-extension files at the evaluated revision: the files read
// within the bounds (names), and the files over a bound (skips) with only their distinct names.
type referenceGraph struct {
	files  map[string][]byte
	blobs  map[string]string
	names  []string
	unread map[string]*hopUnread
	skips  []string
	lexed  map[string]hopLex
}

// hopUnread is a file over a bound: detail names it and the bound, and runs holds its distinct
// names (maximal runs of name bytes), or nil when it may mention anything.
type hopUnread struct {
	detail string
	runs   map[string]bool
}

// hopLex caches hopLexed for one read file, with the argument spans of its require( and import(
// calls in lexed code.
type hopLex struct {
	code  string
	lexed bool
	loads [][2]int
}

// loadReferenceGraph reads the tracked UI-extension files at head in path order. A file over
// hopMaxFile, or one that would carry the read set past hopMaxTotal, is not loaded but scanned for
// its names. It returns nil and the bound's detail only when the tree cannot be listed or read.
func loadReferenceGraph(r repo, head string) (*referenceGraph, string) {
	entries, err := r.tree(head, []string{"."}, hopMaxFiles)
	if err != nil {
		var ke *gokernel.Error
		if errors.As(err, &ke) && ke.Code == "appmap-bound-exceeded" {
			return nil, fmt.Sprintf("more than %d tracked files", hopMaxFiles)
		}
		return nil, "the tracked file tree is unreadable"
	}
	ui := []blobEntry{}
	for _, e := range entries {
		if impactUIExtensions[strings.ToLower(path.Ext(e.path))] {
			ui = append(ui, e)
		}
	}
	sort.Slice(ui, func(i, j int) bool { return ui[i].path < ui[j].path })
	g := &referenceGraph{files: map[string][]byte{}, blobs: map[string]string{}, unread: map[string]*hopUnread{}, lexed: map[string]hopLex{}}
	held, over, sum, counted := []blobEntry{}, []blobEntry{}, 0, map[string]bool{}
	for _, e := range ui {
		switch {
		case e.size > hopMaxFile:
			g.unread[e.path] = &hopUnread{detail: fmt.Sprintf("%s exceeds %d bytes", e.path, hopMaxFile)}
			over = append(over, e)
		case !counted[e.oid] && sum+e.size+128 > hopMaxTotal:
			g.unread[e.path] = &hopUnread{detail: fmt.Sprintf("%s exceeds the %d-byte reference graph total", e.path, hopMaxTotal)}
			over = append(over, e)
		default:
			if !counted[e.oid] {
				counted[e.oid], sum = true, sum+e.size+128
			}
			held = append(held, e)
		}
	}
	data, err := r.blobs(held, hopMaxFile, hopMaxTotal)
	if err != nil {
		return nil, "the tracked files are unreadable"
	}
	for _, e := range held {
		g.files[e.path] = data[e.oid]
		g.blobs[e.path] = e.oid
		g.names = append(g.names, e.path)
	}
	scanned := 0
	for _, e := range over {
		g.skips = append(g.skips, e.path)
		if e.size > hopMaxScan || scanned+e.size > hopMaxScanTotal {
			continue // never scanned, so it may mention anything
		}
		scanned += e.size
		if one, err := r.blobs([]blobEntry{e}, e.size, e.size+128); err == nil {
			g.unread[e.path].runs = hopRuns(one[e.oid])
		}
	}
	return g, ""
}

// hopRuns collects the distinct maximal runs of name bytes in b, or nil past hopMaxRuns. A run
// longer than hopMaxRun is not kept; mayMention counts a token part that long as present.
func hopRuns(b []byte) map[string]bool {
	runs := map[string]bool{}
	for i := 0; i < len(b); {
		if !hopNameByte(b[i]) {
			i++
			continue
		}
		j := i
		for j < len(b) && hopNameByte(b[j]) {
			j++
		}
		if j-i <= hopMaxRun && !runs[string(b[i:j])] {
			if len(runs) >= hopMaxRuns {
				return nil
			}
			runs[string(b[i:j])] = true
		}
		i = j
	}
	return runs
}

// mayMention reports whether the unread file may hold a bounded occurrence of one of tokens. Such
// an occurrence needs every name-byte part of the token as a whole run of the file, so a file
// lacking one part provably holds no mention.
func (u *hopUnread) mayMention(tokens []string) bool {
	if u.runs == nil {
		return true
	}
	for _, t := range tokens {
		all := true
		for _, part := range strings.FieldsFunc(t, func(c rune) bool { return c > 0x7f || !hopNameByte(byte(c)) }) {
			if len(part) <= hopMaxRun && !u.runs[part] {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// mayMentionNg reports whether the unread file may spell one of the directive or component names
// in any normalized markup form: every lower-cased word of the name lies inside some run of the
// file, whatever its case, prefix or separators.
func (u *hopUnread) mayMentionNg(names []string) bool {
	if len(names) == 0 {
		return false
	}
	if u.runs == nil {
		return true
	}
	lower := make([]string, 0, len(u.runs))
	for r := range u.runs {
		lower = append(lower, strings.ToLower(r))
	}
	for _, n := range names {
		all := true
		for _, w := range hopWords(n) {
			found := false
			for _, r := range lower {
				if strings.Contains(r, w) {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// hopWords splits a camelCase name into its lower-cased words (`myDir` is `my`, `dir`).
func hopWords(n string) []string {
	words, start := []string{}, 0
	for k := 1; k <= len(n); k++ {
		if k == len(n) || n[k] >= 'A' && n[k] <= 'Z' {
			if k > start {
				words = append(words, strings.ToLower(n[start:k]))
			}
			start = k
		}
	}
	return words
}

// mentionTokens are the texts a reference to target must contain one of: the stem of a script
// (imports omit the extension), and for an index file both its directory name and `index` (always
// the second token); the base name of anything else.
func mentionTokens(target string) []string {
	base := path.Base(target)
	if hopScript(target) {
		stem := strings.TrimSuffix(base, path.Ext(base))
		if stem == "index" {
			return []string{path.Base(path.Dir(target)), stem}
		}
		return []string{stem}
	}
	return []string{base}
}

// hopScript reports whether name carries one of the script extensions.
func hopScript(name string) bool {
	ext := path.Ext(name)
	for _, s := range hopScriptExtensions {
		if strings.EqualFold(ext, s) {
			return true
		}
	}
	return false
}

func hopNameByte(c byte) bool {
	return c == '_' || c == '$' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// hopOccurrences returns the offsets of every occurrence of token in text that is not part of a
// longer name (a property `x.token` is an occurrence like any other).
func hopOccurrences(text, token string) []int {
	out := []int{}
	if token == "" {
		return out
	}
	for from := 0; ; {
		i := strings.Index(text[from:], token)
		if i < 0 {
			return out
		}
		i += from
		from = i + len(token)
		if i > 0 && hopNameByte(text[i-1]) || from < len(text) && hopNameByte(text[from]) {
			continue
		}
		out = append(out, i)
	}
}

func hopLine(text string, i int) int { return strings.Count(text[:i], "\n") + 1 }

// resolvesTo reports whether literal, written in referrer, names target: relative to the
// referrer's directory when it starts with ./ or ../, repository-relative otherwise, with the
// script extensions and /index.* tried as module resolution does.
func resolvesTo(literal, referrer, target string) bool {
	if literal == "" || len(literal) > hopMaxLiteral || strings.Contains(literal, "${") || strings.ContainsAny(literal, "\\?#*") {
		return false
	}
	var p string
	if strings.HasPrefix(literal, "./") || strings.HasPrefix(literal, "../") {
		p = path.Join(path.Dir(referrer), literal)
	} else {
		p = path.Clean(strings.TrimPrefix(literal, "/"))
	}
	if p == target {
		return true
	}
	for _, s := range hopScriptExtensions {
		if p+s == target || p+"/index"+s == target {
			return true
		}
	}
	return false
}

// lex returns hopLexed for the read file at name and its load spans, lexing it once.
func (g *referenceGraph) lex(name string) hopLex {
	if l, ok := g.lexed[name]; ok {
		return l
	}
	l := hopLex{}
	l.code, l.lexed = hopLexed(name, string(g.files[name]))
	if l.code != "" {
		l.loads = hopLoadSpans(l.code)
	}
	g.lexed[name] = l
	return l
}

// Classes of one occurrence of a path token (hopClassify).
const (
	hopNoMention = iota
	hopReference
	hopRefuse
)

// references finds the mentions of target in the read file at name. It returns the sorted 1-based
// lines that reference target and the 1-based line of the first mention the hop cannot prove, or
// 0 when there is none. A path-token occurrence references target inside one lexer-delimited string
// literal that resolves to it; a plain identifier, property or key in confidently lexed code, or a
// non-path-like occurrence in markup, is no mention (ANL-V0-070); anything else (a computed load,
// a comment, an unresolved or unlexed literal, a doubted file) refuses. Every occurrence of one of
// the DI tokens target registers, and every run that normalizes to a directive or component name
// it registers, references it (ANL-V0-071).
func (g *referenceGraph) references(name, target string, di hopDI) ([]int, int) {
	text := string(g.files[name])
	set := map[int]bool{}
	for k, token := range mentionTokens(target) {
		occ := hopOccurrences(text, token)
		if len(occ) == 0 {
			continue
		}
		l := g.lex(name)
		for _, i := range occ {
			switch hopClassify(text, l, name, target, k, i, i+len(token)) {
			case hopRefuse:
				return nil, hopLine(text, i)
			case hopReference:
				set[hopLine(text, i)] = true
			}
		}
	}
	for _, token := range di.tokens {
		for _, i := range hopOccurrences(text, token) {
			set[hopLine(text, i)] = true
		}
	}
	if len(di.ng) > 0 {
		ng := map[string]bool{}
		for _, n := range di.ng {
			ng[n] = true
		}
		for i := 0; i < len(text); {
			if !hopNgByte(text[i]) {
				i++
				continue
			}
			j := i
			for j < len(text) && hopNgByte(text[j]) {
				j++
			}
			if f := hopNgForms(text[i:j]); ng[f[0]] || ng[f[1]] {
				set[hopLine(text, i)] = true
			}
			i = j
		}
	}
	lines := []int{}
	for n := range set {
		lines = append(lines, n)
	}
	sort.Ints(lines)
	return lines, 0
}

// hopClassify classes the occurrence text[i:from] of the k-th path token of target in the file
// at name, whose lexing is l (code "" when the lexer doubts it; lexed false for a file the
// JavaScript lexer does not read).
func hopClassify(text string, l hopLex, name, target string, k, i, from int) int {
	code, lexed := l.code, l.lexed
	if lexed && code != "" && code[i:from] == text[i:from] {
		switch {
		case k == 1:
			// An index file's `index` token is a path segment, never the binding an import
			// names (ANL-V0-047, V1-1216).
			return hopNoMention
		case hopInSpans(l.loads, i):
			return hopRefuse // a computed require(...) or import(...) may load the target
		}
		return hopNoMention // an identifier, property or key never loads a file (ANL-V0-070)
	}
	ls := strings.LastIndexByte(text[:i], '\n') + 1
	le := strings.IndexByte(text[i:], '\n')
	if le < 0 {
		le = len(text)
	} else {
		le += i
	}
	q, end := hopLiteralAround(text[ls:le], i-ls)
	if q >= 0 && resolvesTo(text[ls+q+1:ls+end], name, target) && lexed && code != "" && hopLexedLiteral(text, code, ls+q, ls+end) {
		return hopReference
	}
	if !lexed && k == 0 && !hopPathLike(text, i, from, target) && !hopLineLoads(text[ls:le]) {
		return hopNoMention // markup: an expression or prose word, not a path (ANL-V0-070)
	}
	return hopRefuse
}

// hopLoadSpans returns the argument spans (from the opening to the closing parenthesis) of every
// require( and import( call in lexed code. A call left open runs to the end of the file, so an
// occurrence anywhere after it may be loaded.
func hopLoadSpans(code string) [][2]int {
	spans := [][2]int{}
	for _, f := range []string{"require", "import"} {
		for _, i := range hopOccurrences(code, f) {
			open := i + len(f)
			for open < len(code) && strings.IndexByte(" \t\r\n", code[open]) >= 0 {
				open++
			}
			if open == len(code) || code[open] != '(' {
				continue
			}
			end, depth := len(code), 0
			for j := open; j < len(code); j++ {
				switch code[j] {
				case '(', '[', '{':
					depth++
				case ')', ']', '}':
					depth--
				}
				if depth == 0 {
					end = j
					break
				}
			}
			spans = append(spans, [2]int{open, end})
		}
	}
	return spans
}

// hopInSpans reports whether offset i lies inside one of spans.
func hopInSpans(spans [][2]int, i int) bool {
	for _, s := range spans {
		if s[0] < i && i <= s[1] {
			return true
		}
	}
	return false
}

// hopPathLike reports whether a markup occurrence of a path token of target may name a path: any
// occurrence for a target that is not a script (its token carries the extension), and for a script
// one beside a path separator, followed by a script extension, or filling a whole quoted value.
func hopPathLike(text string, i, from int, target string) bool {
	if !hopScript(target) {
		return true
	}
	var before, after byte
	if i > 0 {
		before = text[i-1]
	}
	if from < len(text) {
		after = text[from]
	}
	switch {
	case before == '/' || before == '\\' || after == '/' || after == '\\':
		return true
	case (before == '\'' || before == '"' || before == '`') && after == before:
		return true
	case after == '.':
		j := from + 1
		for j < len(text) && hopNameByte(text[j]) {
			j++
		}
		return hopScript("x." + text[from+1:j])
	}
	return false
}

// hopLineLoads reports whether a markup line may load a module (a script block's require or import).
func hopLineLoads(line string) bool {
	return strings.Contains(line, "require(") || strings.Contains(line, "import")
}

// hopLexed lexes a plain script file (not .vue or .svelte, which mix markup) with the JavaScript
// lexer. It returns the source with comment, regex and literal bodies blanked, or "" when the lexer
// cannot be sure it placed every literal; lexed is false for a file the lexer does not read, whose
// mentions (markup, styles, single-file components) therefore never prove a reference.
func hopLexed(name, text string) (string, bool) {
	switch strings.ToLower(path.Ext(name)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
	default:
		return "", false
	}
	toks, code := lexJS(text)
	for _, t := range toks {
		if t.unsure || t.inexact {
			return "", true
		}
	}
	return code, true
}

// hopLexedLiteral reports whether text[open] and text[end] are the lexer's own delimiters of one
// quoted string literal: both survive blanking and everything between them was blanked.
func hopLexedLiteral(text, code string, open, end int) bool {
	if text[open] != '\'' && text[open] != '"' || code[open] != text[open] || code[end] != text[end] {
		return false
	}
	return strings.TrimSpace(code[open+1:end]) == ""
}

// hopLiteralAround scans one line for quoted literals, honouring backslash escapes, and returns the
// opening and closing quote offsets of the literal holding offset at, or -1 when none does
// (including a literal left open at the end of the line). The lexer then confirms the pair.
func hopLiteralAround(line string, at int) (int, int) {
	open := -1
	for k := 0; k < len(line); k++ {
		c := line[k]
		switch {
		case open >= 0 && c == '\\':
			k++
		case open >= 0 && c == line[open]:
			if open < at && at < k {
				return open, k
			}
			open = -1
		case open < 0 && (c == '\'' || c == '"' || c == '`'):
			open = k
		}
	}
	return -1, -1
}

// registrations returns what the read script at name registers through a `.method(` call in code
// position for each hopRegistrations method, and the 1-based line of a registration whose first
// argument is not exactly one lexer-delimited string literal (`'A' + 'B'`, a variable, an object
// map), or of any registration call in a file the lexer doubts or does not read (0 when there is
// none). A call whose first argument is a function or arrow function (`rows.filter(fn)`-style
// array methods) registers nothing, since a registered name is never a function.
func (g *referenceGraph) registrations(name string) (hopDI, int) {
	text := string(g.files[name])
	di := hopDI{}
	for _, method := range hopRegistrations {
		for _, i := range hopOccurrences(text, method) {
			from := i + len(method)
			if !strings.HasSuffix(strings.TrimRight(text[:i], " \t\r\n"), ".") {
				continue
			}
			args := strings.TrimLeft(text[from:], " \t\r\n")
			if !strings.HasPrefix(args, "(") {
				continue
			}
			l := g.lex(name)
			if !l.lexed || l.code == "" {
				return hopDI{}, hopLine(text, i)
			}
			if l.code[i:from] != method {
				continue // inside a comment, string or regular expression
			}
			a, b := hopFirstArg(l.code, len(text)-len(args))
			if hopFunctionArg(l.code[a:b]) {
				continue
			}
			n, ok := hopLiteralAt(text, l.code, a)
			if !ok || a+len(n)+2 != b {
				return hopDI{}, hopLine(text, i)
			}
			tokens, ng := hopDINames(method, n)
			di.tokens = append(di.tokens, tokens...)
			if ng != "" {
				di.ng = append(di.ng, ng)
			}
		}
	}
	return di, 0
}

// hopFirstArg returns the trimmed bounds of the first argument of the call whose `(` is at
// code[open]: up to the first top-level `,` or the closing `)` (the end of the file when the call
// is left open).
func hopFirstArg(code string, open int) (int, int) {
	end, depth := len(code), 0
	for j := open + 1; j < len(code); j++ {
		c := code[j]
		if strings.IndexByte("([{", c) >= 0 {
			depth++
		} else if strings.IndexByte(")]}", c) >= 0 {
			if depth == 0 {
				end = j
				break
			}
			depth--
		} else if c == ',' && depth == 0 {
			end = j
			break
		}
	}
	a, b := open+1, end
	for a < b && strings.IndexByte(" \t\r\n", code[a]) >= 0 {
		a++
	}
	for b > a && strings.IndexByte(" \t\r\n", code[b-1]) >= 0 {
		b--
	}
	return a, b
}

// hopFunctionArg reports whether one lexed argument is a function expression: it opens with the
// `function` or `async function` keyword, or holds a top-level `=>`.
func hopFunctionArg(arg string) bool {
	for _, kw := range []string{"function", "async function"} {
		if strings.HasPrefix(arg, kw) && (len(arg) == len(kw) || !hopNameByte(arg[len(kw)])) {
			return true
		}
	}
	depth := 0
	for j := 0; j+1 < len(arg); j++ {
		switch {
		case strings.IndexByte("([{", arg[j]) >= 0:
			depth++
		case strings.IndexByte(")]}", arg[j]) >= 0:
			depth--
		case depth == 0 && arg[j] == '=' && arg[j+1] == '>':
			return true
		}
	}
	return false
}

// hopLiteralAt returns the body of the single-line, lexer-delimited quoted string literal opening
// at text[j].
func hopLiteralAt(text, code string, j int) (string, bool) {
	if j >= len(text) || text[j] != '\'' && text[j] != '"' {
		return "", false
	}
	end := strings.IndexByte(code[j+1:], text[j])
	if end < 0 {
		return "", false
	}
	end += j + 1
	n := text[j+1 : end]
	if n == "" || len(n) > hopMaxLiteral || strings.ContainsAny(n, "\\\n") || !hopLexedLiteral(text, code, j, end) {
		return "", false
	}
	return n, true
}

// hopDINames are the texts that inject or use a registered name: the name itself, a provider's
// `NameProvider`, a filter's `NameFilter`, and an animation's class without its leading dot. For a
// directive or component it also returns the name, whose markup spellings hopNgForms matches.
func hopDINames(method, n string) ([]string, string) {
	out := []string{n}
	switch method {
	case "provider":
		out = append(out, n+"Provider")
	case "filter":
		out = append(out, n+"Filter")
	case "animation":
		if c := strings.TrimLeft(n, "."); c != "" && c != n {
			out = append(out, c)
		}
	case "directive", "component":
		return out, n
	}
	return out, ""
}

// hopNgByte reports whether c may appear in a markup spelling of a directive or component name.
func hopNgByte(c byte) bool { return c == ':' || hopNameByte(c) }

// hopNgForms returns the names AngularJS's directive normalization may give a markup run: the run
// lower-cased, without one leading `x` or `data` prefix and its `:`, `-` or `_` separator, then
// camel-cased across every run of those separators; and, over-approximating, the same without the
// prefix stripped.
func hopNgForms(run string) [2]string {
	s := strings.ToLower(run)
	full := hopNgCamel(s)
	for _, p := range []string{"x", "data"} {
		if len(s) > len(p) && strings.HasPrefix(s, p) && strings.IndexByte(":-_", s[len(p)]) >= 0 {
			return [2]string{hopNgCamel(s[len(p)+1:]), full}
		}
	}
	return [2]string{full, full}
}

// hopNgCamel removes each run of `:`, `-` and `_` and upper-cases the byte after it (a leading run
// keeps that byte's case; a trailing run stays).
func hopNgCamel(s string) string {
	var b strings.Builder
	for k := 0; k < len(s); k++ {
		if strings.IndexByte(":-_", s[k]) < 0 {
			b.WriteByte(s[k])
			continue
		}
		j := k
		for j < len(s) && strings.IndexByte(":-_", s[j]) >= 0 {
			j++
		}
		if j == len(s) {
			b.WriteString(s[k:])
			break
		}
		c := s[j]
		if k > 0 && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b.WriteByte(c)
		k = j
	}
	return b.String()
}

// templateHop walks the referrers of an unplaced UI path up to router anchors and map test files.
// Every walk must end at such a terminal: a referrer with an unprovable mention, an unread file
// that may mention a walked file, a file nothing references, or a bound reached leaves the path
// unknown with the returned reason and a detail naming the file and line or the bound
// (ANL-V0-069, ANL-V0-072).
func templateHop(m *Map, g *referenceGraph, start string) ([]hopTarget, string, string) {
	anchored, tests := map[string][]*Screen{}, map[string][]TestFile{}
	for i := range m.Screens {
		s := &m.Screens[i]
		if s.Anchor.Repo == "" {
			anchored[s.Anchor.Path] = append(anchored[s.Anchor.Path], s)
		}
	}
	for _, f := range m.Files {
		if f.Anchor.Repo == "" {
			tests[f.Path] = append(tests[f.Path], f)
		}
	}
	targets, seen, queue := []hopTarget{}, map[string]bool{start: true}, []string{start}
	reaches, up := map[string]bool{}, map[string][]string{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		var di hopDI
		if hopScript(cur) {
			if u := g.unread[cur]; u != nil {
				return nil, "reference-graph-bound", u.detail // its registrations cannot be read
			}
			names, bad := g.registrations(cur)
			if bad > 0 {
				return nil, "unresolved-reference", fmt.Sprintf("%s:%d", cur, bad)
			}
			di = names
		}
		all := append(mentionTokens(cur), di.tokens...)
		for _, name := range g.skips {
			if name != cur && (g.unread[name].mayMention(all) || g.unread[name].mayMentionNg(di.ng)) {
				return nil, "reference-graph-bound", g.unread[name].detail
			}
		}
		referrers := 0
		for _, name := range g.names {
			if name == cur {
				continue
			}
			lines, bad := g.references(name, cur, di)
			if bad > 0 {
				return nil, "unresolved-reference", fmt.Sprintf("%s:%d", name, bad)
			}
			if len(lines) == 0 {
				continue
			}
			referrers++
			if ss := anchored[name]; len(ss) > 0 {
				targets = append(targets, hopTarget{screens: spanScreens(ss, lines, g.blobs[name]), via: name})
				reaches[cur] = true
				continue
			}
			t := hopTarget{via: name}
			for _, f := range tests[name] {
				if f.Screen != "" {
					t.screens = append(t.screens, f.Screen)
				}
				if f.Role == roleSpec {
					t.specs = append(t.specs, f.Path)
				}
			}
			if len(t.screens)+len(t.specs) > 0 {
				targets = append(targets, t)
				reaches[cur] = true
				continue
			} // a map test file selecting nothing (a helper) is walked through like any other file
			up[cur] = append(up[cur], name)
			if !seen[name] {
				if len(seen) >= hopMaxNodes {
					return nil, "reference-graph-bound", fmt.Sprintf("more than %d walked files (next %s)", hopMaxNodes, name)
				}
				seen[name] = true
				queue = append(queue, name)
			}
		}
		if referrers == 0 {
			return nil, "unmapped-path", "nothing references " + cur
		}
	}
	// A file whose referrers only reference each other (an import cycle) never reaches a terminal;
	// every walked file must reach one through its referrers or the path stays unknown.
	walked := []string{}
	for n := range seen {
		walked = append(walked, n)
	}
	sort.Strings(walked)
	for changed := true; changed; {
		changed = false
		for _, n := range walked {
			if reaches[n] {
				continue
			}
			for _, r := range up[n] {
				if reaches[r] {
					reaches[n], changed = true, true
					break
				}
			}
		}
	}
	for _, n := range walked {
		if !reaches[n] {
			return nil, "unmapped-path", n + " reaches no router anchor or test file"
		}
	}
	return targets, "", ""
}

// spanScreens selects the screens whose anchor span holds a reference line. When the router file
// at head differs from an anchored blob, or any reference line lies outside every span (a
// top-level import used by name), every screen of the file is selected: the hop may widen, never
// narrow.
func spanScreens(ss []*Screen, lines []int, blob string) []string {
	all, out := []string{}, map[string]bool{}
	stale := false
	for _, s := range ss {
		all = append(all, s.ID)
		stale = stale || s.Anchor.Blob != blob
	}
	if stale {
		return all
	}
	for _, n := range lines {
		held := false
		for _, s := range ss {
			if n >= s.Anchor.Start && n <= s.Anchor.End {
				out[s.ID], held = true, true
			}
		}
		if !held {
			return all
		}
	}
	ids := []string{}
	for id := range out {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
