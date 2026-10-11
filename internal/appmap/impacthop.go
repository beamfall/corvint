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

// mentionToken is the text any reference to target must contain: the stem of a script (imports
// omit the extension), the directory of an index file, and the base name of anything else.
func mentionToken(target string) string {
	base := path.Base(target)
	ext := path.Ext(base)
	for _, s := range hopScriptExtensions {
		if strings.EqualFold(ext, s) {
			stem := strings.TrimSuffix(base, ext)
			if stem == "index" {
				return path.Base(path.Dir(target))
			}
			return stem
		}
	}
	return base
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

// references finds every mention of target's token in the file at name. It returns the 1-based
// lines of mentions that sit inside one string literal resolving to target, and false when any
// mention does not (a computed, aliased or ambiguous reference the hop cannot prove).
func (g *referenceGraph) references(name, target string) ([]int, bool) {
	body, token := g.files[name], mentionToken(target)
	text := string(body)
	lines, from := []int{}, 0
	for {
		i := strings.Index(text[from:], token)
		if i < 0 {
			return lines, true
		}
		i += from
		from = i + len(token)
		if i > 0 && (hopNameByte(text[i-1]) || text[i-1] == '.') || from < len(text) && hopNameByte(text[from]) {
			continue // part of a longer name
		}
		ls := strings.LastIndexByte(text[:i], '\n') + 1
		le := strings.IndexByte(text[i:], '\n')
		if le < 0 {
			le = len(text)
		} else {
			le += i
		}
		q := strings.LastIndexAny(text[ls:i], "'\"`")
		if q < 0 {
			return nil, false
		}
		q += ls
		end := strings.IndexByte(text[i:le], text[q])
		if end < 0 || !resolvesTo(text[q+1:i+end], name, target) {
			return nil, false
		}
		lines = append(lines, strings.Count(text[:i], "\n")+1)
	}
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
				continue
			}
			if fs := tests[name]; len(fs) > 0 {
				t := hopTarget{via: name}
				for _, f := range fs {
					if f.Screen != "" {
						t.screens = append(t.screens, f.Screen)
					}
					if f.Role == roleSpec {
						t.specs = append(t.specs, f.Path)
					}
				}
				targets = append(targets, t)
				continue
			}
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
