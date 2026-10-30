package typescript

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightUnknownDeclaredPathContradicted is the selection unknown of a dirty path the
// repository declares as documentation that an observed source names (TJAA-V0-101).
const PlaywrightUnknownDeclaredPathContradicted = "playwright:declared-path-contradicted"

// Declaration states (TJAA-V0-101).
const (
	PlaywrightDeclarationApplied = "APPLIED"
	PlaywrightDeclarationRefused = "REFUSED"
)

// PlaywrightPathDeclaration records the project-owned path declaration a plan consulted: the
// declaration's path and SHA-256, the dirty paths it matched, and whether the plan applied it or
// refused it because a source in the Playwright closure names a matched path (TJAA-V0-101,
// AFP-V0-442). OutOfClosure lists the observed sources outside that closure that name one,
// which do not contradict it; it is omitted when there are none.
type PlaywrightPathDeclaration struct {
	Matched      []string                       `json:"matched"`
	OutOfClosure []PlaywrightOutOfClosureReader `json:"outOfClosure,omitzero"`
	Path         string                         `json:"path"`
	SHA256       string                         `json:"sha256"`
	State        string                         `json:"state"`
}

// PlaywrightOutOfClosureReader is one observed source outside the Playwright closure that names
// a matched declared path: the path it names first and that site as file, line and spelling
// (AFP-V0-442).
type PlaywrightOutOfClosureReader struct {
	Path string `json:"path"`
	Site string `json:"site"`
}

// playwrightDeclaredPaths returns the record of the AFP-V0-160 declaration that classified dirty
// paths in base, or nil when it classified none, plus one selection unknown per matched path that
// a source in the Playwright closure names (TJAA-V0-101, AFP-V0-442). closure is the plan's
// Playwright closure. The declaration is read again and must still hold the entries the graph
// was built with.
func playwrightDeclaredPaths(root, configPath string, graph *affected.Graph, base affected.Plan, closure *playwrightClosure) (*PlaywrightPathDeclaration, []PlaywrightUnknown, error) {
	var matched []string
	for _, group := range base.NonSource {
		if group.Class == affected.ClassDeclaredDocumentation {
			matched = append(matched, group.Paths...)
		}
	}
	if len(matched) == 0 {
		return nil, nil, nil
	}
	digest, entries, err := affected.ReadDocumentationDeclaration(root)
	if err != nil {
		return nil, nil, err
	}
	if !slices.Equal(entries, graph.DeclaredDocumentation()) {
		return nil, nil, errors.New(affected.DocumentationPathsPath + " changed while the plan was compiled")
	}
	files, err := affected.SourceFiles(root, playwrightObservedName)
	if err != nil {
		return nil, nil, err
	}
	if !slices.Contains(files, configPath) {
		files = append(files, configPath)
		slices.Sort(files)
	}
	record := &PlaywrightPathDeclaration{Matched: matched, Path: affected.DocumentationPathsPath, SHA256: digest, State: PlaywrightDeclarationApplied}
	contradicting := make(map[string]string, len(matched))
	for _, relative := range files {
		if len(contradicting) == len(matched) {
			break
		}
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			return nil, nil, err
		}
		text := string(body)
		decoded, rawOffset := decodeStringEscapes(text)
		for _, declared := range matched {
			if _, found := contradicting[declared]; found || relative == declared {
				continue
			}
			offset, spelling := declaredPathNamed(decoded, declared)
			if offset < 0 {
				continue
			}
			site := lineSite(relative, text, rawOffset(offset)) + " `" + spelling + "`"
			why, inside := closure.member(relative)
			if !inside {
				record.OutOfClosure = append(record.OutOfClosure, PlaywrightOutOfClosureReader{Path: declared, Site: site})
				break
			}
			contradicting[declared] = site + " names it (" + why + ")"
		}
	}
	var unknown []PlaywrightUnknown
	for _, declared := range matched {
		if site, found := contradicting[declared]; found {
			record.State = PlaywrightDeclarationRefused
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownDeclaredPathContradicted,
				affected.DocumentationPathsPath+" (sha256:"+digest+") declares "+declared+" read by no build or test input, but "+site))
		}
	}
	return record, unknown, nil
}

// playwrightClosureBound caps the units outside the edge closure whose reach is walked to settle
// membership; past it, membership is unknown and every naming file refuses (AFP-V0-442).
const playwrightClosureBound = 2048

// playwrightClosure is the set of TypeScript graph units the Playwright run may load: the config's
// unit, every bound Playwright test's unit, every unit those reach by any dependency edge, and,
// to a fixpoint, every unit a change of which reaches a member, such as a script a spec names as
// an argument and the modules that script imports (AFP-V0-442). It is built on first use, since
// a plan whose declared paths no file names needs none. bound is non-empty when membership itself
// is unknown because the walk exceeded playwrightClosureBound; unknown is non-empty when the set
// may be incomplete, which bound implies. frontier holds only the reasons that can change the
// Playwright answer (TJAA-V0-108), so an informational reason never makes the set incomplete.
type playwrightClosure struct {
	configPath string
	graph      *affected.Graph
	frontier   []string
	sites      frontierSet
	// seeds, when non-nil, replace every bound test unit as the units the walk starts from
	// beside the config, giving one test's closure (TJAA-V0-140).
	seeds    []string
	membered bool
	built    bool
	units    map[string]string
	bound    string
	unknown  string
}

func newPlaywrightClosure(graph *affected.Graph, configPath string, frontier []string, sites frontierSet) *playwrightClosure {
	return &playwrightClosure{configPath: configPath, graph: graph, frontier: frontier, sites: sites}
}

// members computes the closure's membership once, without the completeness checks build adds,
// so frontier relevance can be judged against it before the relevant frontier is known
// (TJAA-V0-108).
func (closure *playwrightClosure) members() {
	if closure.membered {
		return
	}
	closure.membered = true
	graph := closure.graph
	closure.units = map[string]string{}
	add := func(id, why string) {
		if _, exists := graph.Unit(id); !exists || closure.units[id] != "" {
			return
		}
		closure.units[id] = why
		for queue := []string{id}; len(queue) != 0; queue = queue[1:] {
			unit, _ := graph.Unit(queue[0])
			for _, edges := range [][]string{unit.Imports, unit.TestImports, unit.AdvisoryImports, unit.RouteDispatchImports, unit.Execs} {
				for _, next := range edges {
					if _, exists := graph.Unit(next); exists && closure.units[next] == "" {
						closure.units[next] = next + " is in the Playwright closure"
						queue = append(queue, next)
					}
				}
			}
		}
	}
	if id, owned := graph.OwnerOf(closure.configPath); owned {
		add(id, id+" is in the Playwright closure")
	}
	for _, id := range graph.UnitIDs() {
		if unit, _ := graph.Unit(id); closure.seeds == nil && len(unit.Tests) != 0 || slices.Contains(closure.seeds, id) {
			add(id, id+" is in the Playwright closure")
		}
	}
	var outside []string
	for _, id := range graph.UnitIDs() {
		if unit, _ := graph.Unit(id); closure.units[id] == "" && len(unit.Sources)+len(unit.Tests) != 0 {
			outside = append(outside, id)
		}
	}
	if len(outside) > playwrightClosureBound {
		closure.bound = strconv.Itoa(len(outside)) + " units outside the edge closure exceed the " + strconv.Itoa(playwrightClosureBound) + "-unit membership bound"
		return
	}
	// One reach of every outside file settles the common case: when it touches no member, no
	// outside unit joins and the per-unit walk below is skipped (TJAA-V0-108).
	var files []string
	for _, id := range outside {
		unit, _ := graph.Unit(id)
		files = append(append(files, unit.Sources...), unit.Tests...)
	}
	if !slices.ContainsFunc(graph.ReachedUnitIDs(files), func(id string) bool { return closure.units[id] != "" }) {
		return
	}
	reached := make(map[string][]string, len(outside))
	for _, id := range outside {
		unit, _ := graph.Unit(id)
		reached[id] = graph.ReachedUnitIDs(append(slices.Clone(unit.Sources), unit.Tests...))
	}
	for changed := true; changed; {
		changed = false
		for _, id := range outside {
			if closure.units[id] != "" {
				continue
			}
			for _, next := range reached[id] {
				if next != id && closure.units[next] != "" {
					add(id, id+" reaches Playwright closure unit "+next)
					changed = true
					break
				}
			}
		}
	}
}

// build adds to members the completeness checks behind unknown: a membership bound, a closure
// unit with unit frontier or bounded path tokens, or a relevant frontier reason with no located
// site or a site in the closure or in no unit (AFP-V0-442).
func (closure *playwrightClosure) build() {
	closure.built = true
	closure.members()
	graph := closure.graph
	if closure.bound != "" {
		closure.unknown = closure.bound
		return
	}
	for _, id := range sortedKeys(closure.units) {
		unit, _ := graph.Unit(id)
		switch {
		case len(unit.Frontier) != 0:
			closure.unknown = "closure unit " + id + " has frontier " + unit.Frontier[0]
		case unit.PathTokensBounded:
			closure.unknown = "closure unit " + id + " keeps no path tokens over the bound"
		}
		if closure.unknown != "" {
			return
		}
	}
	for _, reason := range closure.frontier {
		located := orderedSites(closure.sites[reason])
		if len(located) == 0 {
			closure.unknown = "graph frontier " + reason + " has no located site"
			return
		}
		for _, site := range located {
			id, owned := graph.OwnerOf(playwrightSiteFile(site))
			if !owned || closure.units[id] != "" {
				closure.unknown = "graph frontier " + reason + " at " + site + " may add a closure edge"
				return
			}
		}
	}
}

// playwrightSiteFile is the file of a frontier site, `file` or `file:line`.
func playwrightSiteFile(site string) string {
	if index := strings.LastIndexByte(site, ':'); index >= 0 {
		if _, err := strconv.Atoi(site[index+1:]); err == nil {
			return site[:index]
		}
	}
	return site
}

// member reports whether relative may be loaded by the Playwright run, and why: it is the
// config, is not a graph unit, the closure is incomplete, or its unit is in the closure
// (AFP-V0-442).
func (closure *playwrightClosure) member(relative string) (string, bool) {
	if relative == closure.configPath {
		return "the Playwright config", true
	}
	id, owned := closure.graph.OwnerOf(relative)
	if !owned {
		return "not a TypeScript graph unit, so its Playwright closure membership is unknown", true
	}
	if !closure.built {
		closure.build()
	}
	if closure.unknown != "" {
		return "Playwright closure membership is unknown: " + closure.unknown, true
	}
	if why := closure.units[id]; why != "" {
		return why, true
	}
	return id + " is outside the Playwright closure", false
}

// declaredPathNamed returns the offset and spelling of the first place text names declared
// as a string: the whole path or a trailing part of it (file name, or a path relative to an
// ancestor) closed by a quote, or the path or one of its ancestor directories opened by a quote,
// `./`, `../` or a template substitution's `}/` and closed by a quote or by `/` before a quote,
// substitution or glob character. It returns -1 when no spelling matches (TJAA-V0-101).
func declaredPathNamed(text, declared string) (int, string) {
	segments := strings.Split(declared, "/")
	first, spelling := -1, ""
	consider := func(candidate string, rooted bool) {
		for from := 0; ; {
			index := strings.Index(text[from:], candidate)
			if index < 0 {
				return
			}
			index += from
			from = index + 1
			if first >= 0 && index >= first {
				return
			}
			if declaredSpellingBounded(text, index, candidate, rooted) {
				first, spelling = index, candidate
				return
			}
		}
	}
	for start := range segments {
		consider(strings.Join(segments[start:], "/"), start == 0)
	}
	for end := 1; end < len(segments); end++ {
		consider(strings.Join(segments[:end], "/"), true)
	}
	return first, spelling
}

// declaredSpellingBounded reports whether candidate at index is a whole string spelling: a
// rooted spelling (the full path or an ancestor) must open at a quote, `./` or `}/`; a trailing
// part may also follow any `/`. Both close at the quote that opened them (for a `/` opening, the
// enclosing quote), and an ancestor also at `/` that such a quote, substitution or glob
// character follows.
func declaredSpellingBounded(text string, index int, candidate string, rooted bool) bool {
	before := byte(0)
	if index > 0 {
		before = text[index-1]
	}
	opened, opener := isQuote(before), before
	if before == '/' {
		opener = enclosingQuote(text, index)
		opened = !rooted || (index >= 2 && (text[index-2] == '.' || text[index-2] == '}'))
	}
	if !opened {
		return false
	}
	rest := text[index+len(candidate):]
	if rest == "" {
		return false
	}
	closes := func(value byte) bool { return isQuote(value) && (opener == 0 || value == opener) }
	if closes(rest[0]) {
		return true
	}
	return rooted && rest[0] == '/' && len(rest) > 1 && (closes(rest[1]) || strings.IndexByte("${*?[", rest[1]) >= 0)
}

// escapedOpaque stands, in decoded text, for an escape that can be no path byte or string
// delimiter: an escaped quote or backslash, or a control escape such as \n.
const escapedOpaque = 0x01

// decodeStringEscapes returns text with JavaScript string escapes decoded, so `\u002e`, `\x2e`,
// `\u{2e}` and `\.` match as `.`, plus a function mapping a decoded offset to the offset of the
// raw byte or escape it came from. An escaped quote decodes to an opaque byte, so it neither
// opens nor closes a spelling; a line continuation decodes to nothing (TJAA-V0-101).
func decodeStringEscapes(text string) (string, func(int) int) {
	if !strings.Contains(text, "\\") {
		return text, func(offset int) int { return offset }
	}
	var decoded strings.Builder
	raw := make([]int, 0, len(text)+1)
	emit := func(value string, at int) {
		decoded.WriteString(value)
		for range len(value) {
			raw = append(raw, at)
		}
	}
	for index := 0; index < len(text); {
		if text[index] != '\\' || index+1 == len(text) {
			emit(text[index:index+1], index)
			index++
			continue
		}
		next := text[index+1]
		switch {
		case next == '\n':
			index += 2
		case next == '\r':
			index += 2
			if index < len(text) && text[index] == '\n' {
				index++
			}
		case next == 'x' && index+4 <= len(text):
			if value, err := strconv.ParseUint(text[index+2:index+4], 16, 8); err == nil {
				emit(string(rune(value)), index)
				index += 4
			} else {
				emit(text[index+1:index+2], index)
				index += 2
			}
		case next == 'u':
			value, width := decodeUnicodeEscape(text[index+2:])
			if width == 0 {
				emit("u", index)
				index += 2
				continue
			}
			emit(string(value), index)
			index += 2 + width
		case isQuote(next) || next == '\\' || strings.IndexByte("bfnrtv0", next) >= 0:
			emit(string(rune(escapedOpaque)), index)
			index += 2
		default:
			emit(text[index+1:index+2], index)
			index += 2
		}
	}
	raw = append(raw, len(text))
	return decoded.String(), func(offset int) int { return raw[offset] }
}

// decodeUnicodeEscape decodes the XXXX or {X...} after `\u` and returns the rune and the
// number of bytes it spans, or width 0 when no valid escape follows.
func decodeUnicodeEscape(text string) (rune, int) {
	digits, width := "", 0
	if strings.HasPrefix(text, "{") {
		end := strings.IndexByte(text, '}')
		if end < 2 || end > 7 {
			return 0, 0
		}
		digits, width = text[1:end], end+1
	} else if len(text) >= 4 {
		digits, width = text[:4], 4
	} else {
		return 0, 0
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil || !utf8.ValidRune(rune(value)) {
		return 0, 0
	}
	return rune(value), width
}

// enclosingQuote returns the nearest quote before index on the same line, the delimiter a
// spelling that opens at `/` sits inside, or 0 (any quote closes) when the line holds none or a
// template substitution's `}` lies between them.
func enclosingQuote(text string, index int) byte {
	for at := index - 1; at >= 0; at-- {
		switch value := text[at]; {
		case value == '\n' || value == '}':
			return 0
		case isQuote(value):
			return value
		}
	}
	return 0
}

func isQuote(value byte) bool { return value == '"' || value == '\'' || value == '`' }
