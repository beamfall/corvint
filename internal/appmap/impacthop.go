package appmap

import (
	"path"
	"sort"
	"strings"
)

// Bounds of the template/component reference hop (ANL-V0-032). Exceeding one leaves every
// unplaced path unknown; it never narrows the verdict.
const (
	hopMaxFiles   = 20000
	hopMaxFile    = 1 << 20
	hopMaxTotal   = 32 << 20
	hopMaxNodes   = 64
	hopMaxLiteral = 512
)

var hopScriptExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte"}

// hopTarget is one terminal a reference walk reached: screens selected through a router
// anchor, or a test file of the map.
type hopTarget struct {
	screens []string
	specs   []string
	via     string
}

// referenceGraph holds the tracked UI-extension files at the evaluated revision.
type referenceGraph struct {
	files map[string][]byte
	blobs map[string]string
	names []string
}

func loadReferenceGraph(r repo, head string) (*referenceGraph, string) {
	entries, err := r.tree(head, []string{"."}, hopMaxFiles)
	if err != nil {
		return nil, "reference-graph-bound"
	}
	ui := []blobEntry{}
	for _, e := range entries {
		if impactUIExtensions[strings.ToLower(path.Ext(e.path))] {
			ui = append(ui, e)
		}
	}
	data, err := r.blobs(ui, hopMaxFile, hopMaxTotal)
	if err != nil {
		return nil, "reference-graph-bound"
	}
	g := &referenceGraph{files: map[string][]byte{}, blobs: map[string]string{}}
	for _, e := range ui {
		g.files[e.path] = data[e.oid]
		g.blobs[e.path] = e.oid
		g.names = append(g.names, e.path)
	}
	sort.Strings(g.names)
	return g, ""
}

// mentionTokens are the texts a reference to target must contain one of: the stem of a script
// (imports omit the extension), and for an index file both its directory name and `index`; the
// base name of anything else.
func mentionTokens(target string) []string {
	base := path.Base(target)
	ext := path.Ext(base)
	for _, s := range hopScriptExtensions {
		if strings.EqualFold(ext, s) {
			stem := strings.TrimSuffix(base, ext)
			if stem == "index" {
				return []string{path.Base(path.Dir(target)), stem}
			}
			return []string{stem}
		}
	}
	return []string{base}
}

func hopNameByte(c byte) bool {
	return c == '_' || c == '$' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

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

// references finds every mention of a token of target in the file at name. It returns the sorted
// 1-based lines of mentions that sit inside one string literal of a lexed script file resolving to
// target, and false when any mention does not (a computed, aliased, commented, markup or ambiguous
// reference the hop cannot prove).
func (g *referenceGraph) references(name, target string) ([]int, bool) {
	text := string(g.files[name])
	code, lexed := hopLexed(name, text)
	set := map[int]bool{}
	for _, token := range mentionTokens(target) {
		for from := 0; ; {
			i := strings.Index(text[from:], token)
			if i < 0 {
				break
			}
			i += from
			from = i + len(token)
			if i > 0 && hopNameByte(text[i-1]) || from < len(text) && hopNameByte(text[from]) {
				continue // part of a longer name; a property `x.token` is a mention like any other
			}
			ls := strings.LastIndexByte(text[:i], '\n') + 1
			le := strings.IndexByte(text[i:], '\n')
			if le < 0 {
				le = len(text)
			} else {
				le += i
			}
			q, end := hopLiteralAround(text[ls:le], i-ls)
			if q < 0 || !resolvesTo(text[ls+q+1:ls+end], name, target) {
				return nil, false
			}
			if !lexed || code == "" || !hopLexedLiteral(text, code, ls+q, ls+end) {
				return nil, false // only the JavaScript lexer can confirm a literal; markup is not read
			}
			set[strings.Count(text[:i], "\n")+1] = true
		}
	}
	lines := []int{}
	for n := range set {
		lines = append(lines, n)
	}
	sort.Ints(lines)
	return lines, true
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

// templateHop walks the referrers of an unplaced UI path up to router anchors and map test files.
// Every walk must end at such a terminal: a referrer with an unprovable mention, a file nothing
// references, or a bound reached leaves the path unknown with the returned reason.
func templateHop(m *Map, g *referenceGraph, start string) ([]hopTarget, string) {
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
		referrers := 0
		for _, name := range g.names {
			if name == cur {
				continue
			}
			lines, ok := g.references(name, cur)
			if !ok {
				return nil, "unresolved-reference"
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
					return nil, "reference-graph-bound"
				}
				seen[name] = true
				queue = append(queue, name)
			}
		}
		if referrers == 0 {
			return nil, "unmapped-path"
		}
	}
	// A file whose referrers only reference each other (an import cycle) never reaches a terminal;
	// every walked file must reach one through its referrers or the path stays unknown.
	for changed := true; changed; {
		changed = false
		for n := range seen {
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
	for n := range seen {
		if !reaches[n] {
			return nil, "unmapped-path"
		}
	}
	return targets, ""
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
