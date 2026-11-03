// Package v8 converts JavaScript coverage into the line-granular profile
// lines the linecov index reads (LPCV-V0-965): V8 precise coverage, as
// NODE_V8_COVERAGE or the inspector's Profiler.takePreciseCoverage writes it,
// mapped through source maps to original sources, and the Istanbul and LCOV
// reports of runners that transform code before V8 sees it.
//
// Each source line becomes one block spanning the line, `L.1,L+1.1`, so a
// record says per test which lines ran. A line no report instruments has no
// block.
package v8

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Lines is what one process tree executed: per repository-relative,
// slash-separated path, each instrumented line and whether it ran.
type Lines map[string]map[int]bool

// Add marks line of path instrumented, and executed when ran.
func (l Lines) Add(path string, line int, ran bool) {
	if line < 1 {
		return
	}
	m := l[path]
	if m == nil {
		m = map[int]bool{}
		l[path] = m
	}
	m[line] = m[line] || ran
}

// Merge adds every line of o.
func (l Lines) Merge(o Lines) {
	for p, m := range o {
		for line, ran := range m {
			l.Add(p, line, ran)
		}
	}
}

// Profile renders l as linecov profile lines under module, sorted, one block
// per line: `module/path:L.1,L+1.1 1 count`.
func (l Lines) Profile(module string) []byte {
	paths := make([]string, 0, len(l))
	for p := range l {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b bytes.Buffer
	b.WriteString("mode: set\n")
	for _, p := range paths {
		lines := make([]int, 0, len(l[p]))
		for line := range l[p] {
			lines = append(lines, line)
		}
		sort.Ints(lines)
		for _, line := range lines {
			n := 0
			if l[p][line] {
				n = 1
			}
			fmt.Fprintf(&b, "%s/%s:%d.1,%d.1 1 %d\n", module, p, line, line+1, n)
		}
	}
	return b.Bytes()
}

// Scope decides which files a report may name: Rel maps an absolute path to
// its repository-relative form and reports whether it is kept. A path outside
// the repository, under node_modules, or untracked at the index commit is
// dropped, since its lines cannot be bound to immutable content.
type Scope func(abs string) (rel string, ok bool)

// RepoScope keeps the files under root that keep admits.
func RepoScope(root string, keep func(rel string) bool) Scope {
	root = filepath.Clean(root)
	return func(abs string) (string, bool) {
		rel, err := filepath.Rel(root, filepath.Clean(abs))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", false
		}
		rel = filepath.ToSlash(rel)
		if rel == "node_modules" || strings.HasPrefix(rel, "node_modules/") || strings.Contains(rel, "/node_modules/") {
			return "", false
		}
		if keep != nil && !keep(rel) {
			return "", false
		}
		return rel, true
	}
}

// Report is one V8 coverage file as NODE_V8_COVERAGE writes it.
type Report struct {
	Result         []Script             `json:"result"`
	SourceMapCache map[string]CachedMap `json:"source-map-cache"`
}

// Script is one script's function coverage.
type Script struct {
	URL       string     `json:"url"`
	Functions []Function `json:"functions"`
}

// Function is one function's ranges: the first spans the function, later ones
// nest inside it (block coverage).
type Function struct {
	Ranges []Range `json:"ranges"`
}

// Range is a half-open UTF-16 offset span and how often it ran.
type Range struct {
	Start int `json:"startOffset"`
	End   int `json:"endOffset"`
	Count int `json:"count"`
}

// CachedMap is Node's source-map-cache entry: the generated script's line
// lengths in UTF-16 units and its source map.
type CachedMap struct {
	LineLengths []int     `json:"lineLengths"`
	Data        SourceMap `json:"data"`
}

// SourceMap is a version 3 source map.
type SourceMap struct {
	Version    int      `json:"version"`
	Sources    []string `json:"sources"`
	SourceRoot string   `json:"sourceRoot"`
	Mappings   string   `json:"mappings"`
}

// Options says how to read the scripts a report names.
type Options struct {
	Scope Scope
	// Read returns a generated script's text; nil reads the file.
	Read func(abs string) ([]byte, error)
	// SourceMap returns the source map of a script that has none in the
	// report's cache (an inline or external sourceMappingURL); nil reads the
	// comment from the script itself.
	SourceMap func(abs string, src []byte) (*SourceMap, string, error)
}

// AddV8 adds the lines one V8 coverage report executed. A script with a
// source map counts against its original sources; one without counts against
// its own lines, which suits TypeScript Node runs by type stripping, since
// that keeps every offset. A generated script outside the scope, such as an
// untracked build output, still counts through its map.
func (l Lines) AddV8(data []byte, o Options) error {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("v8: %w", err)
	}
	read := o.Read
	if read == nil {
		read = os.ReadFile
	}
	for _, s := range r.Result {
		abs, ok := filePath(s.URL)
		if !ok || strings.Contains(filepath.ToSlash(abs), "/node_modules/") {
			continue // node: internals, data: URLs, eval, dependencies
		}
		var sm *SourceMap
		var starts []int // UTF-16 offset of each generated line
		var src []byte
		mapURL := s.URL
		if cached, ok := r.SourceMapCache[s.URL]; ok {
			sm = &cached.Data
			if cached.LineLengths != nil {
				starts = lineStartsFromLengths(cached.LineLengths)
			}
		}
		if starts == nil {
			b, err := read(abs)
			if err != nil {
				continue // a script gone since the run: nothing to bind
			}
			src, starts = b, lineStarts(b)
		}
		if sm == nil {
			var err error
			if sm, mapURL, err = o.sourceMap(abs, src); err != nil {
				return err
			}
		}
		ran := paint(s.Functions, starts)
		if sm != nil {
			if err := l.addMapped(sm, mapURL, starts, ran, o.Scope); err != nil {
				return fmt.Errorf("v8: %s: %w", s.URL, err)
			}
			continue
		}
		rel, keep := o.Scope(abs)
		if !keep {
			continue // neither in scope nor mapped into it
		}
		for i, empty := range blankLines(src) {
			if !empty {
				l.Add(rel, i+1, ran.line(i))
			}
		}
	}
	return nil
}

func (o Options) sourceMap(abs string, src []byte) (*SourceMap, string, error) {
	if o.SourceMap != nil {
		return o.SourceMap(abs, src)
	}
	return ReadSourceMap(abs, src)
}

// ReadSourceMap reads the map a script's last sourceMappingURL comment names:
// an inline base64 data URL or a file beside the script. A script without one
// has a nil map.
func ReadSourceMap(abs string, src []byte) (*SourceMap, string, error) {
	i := bytes.LastIndex(src, []byte("sourceMappingURL="))
	if i < 0 {
		return nil, "", nil
	}
	ref := string(src[i+len("sourceMappingURL="):])
	if j := strings.IndexAny(ref, " \t\r\n*"); j >= 0 {
		ref = ref[:j]
	}
	var raw []byte
	mapURL := "file://" + filepath.ToSlash(abs)
	if rest, ok := strings.CutPrefix(ref, "data:"); ok {
		comma := strings.IndexByte(rest, ',')
		if comma < 0 || !strings.HasSuffix(rest[:comma], ";base64") {
			return nil, "", nil
		}
		b, err := decodeBase64(rest[comma+1:])
		if err != nil {
			return nil, "", nil
		}
		raw = b
	} else {
		u, err := url.PathUnescape(ref)
		if err != nil || strings.Contains(u, "://") {
			return nil, "", nil
		}
		file := filepath.Join(filepath.Dir(abs), filepath.FromSlash(u))
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, "", nil // a dangling comment: count the script itself
		}
		raw, mapURL = b, "file://"+filepath.ToSlash(file)
	}
	var sm SourceMap
	if err := json.Unmarshal(raw, &sm); err != nil {
		return nil, "", fmt.Errorf("v8: source map of %s: %w", abs, err)
	}
	return &sm, mapURL, nil
}

// addMapped adds each original line a generated segment maps to.
func (l Lines) addMapped(sm *SourceMap, mapURL string, starts []int, ran executed, scope Scope) error {
	sources := make([]string, len(sm.Sources))
	kept := make([]bool, len(sm.Sources))
	for i, s := range sm.Sources {
		abs, ok := resolveSource(mapURL, sm.SourceRoot, s)
		if !ok {
			continue
		}
		sources[i], kept[i] = scope(abs)
	}
	return decodeMappings(sm.Mappings, func(genLine, genCol, src, srcLine int) {
		if src < 0 || src >= len(sources) || !kept[src] || genLine >= len(starts) {
			return
		}
		l.Add(sources[src], srcLine+1, ran.at(starts[genLine]+genCol))
	})
}

// resolveSource resolves one source-map source against the map's own URL.
func resolveSource(mapURL, root, source string) (string, bool) {
	if root != "" && !strings.HasSuffix(root, "/") {
		root += "/"
	}
	ref, err := url.Parse(root + source)
	if err != nil {
		return "", false
	}
	base, err := url.Parse(mapURL)
	if err != nil {
		return "", false
	}
	return filePath(base.ResolveReference(ref).String())
}

// filePath is the local path of a file: URL or an absolute path.
func filePath(u string) (string, bool) {
	if filepath.IsAbs(u) {
		return filepath.Clean(u), true
	}
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "file" {
		return "", false
	}
	return filepath.Clean(filepath.FromSlash(p.Path)), true
}

// executed answers, per UTF-16 offset, whether the innermost range holding it
// ran.
type executed struct {
	ran    []bool
	starts []int
}

func (e executed) at(off int) bool {
	return off >= 0 && off < len(e.ran) && e.ran[off]
}

// line reports whether any non-space unit of generated line i ran.
func (e executed) line(i int) bool {
	end := len(e.ran)
	if i+1 < len(e.starts) {
		end = e.starts[i+1]
	}
	for off := e.starts[i]; off < end && off < len(e.ran); off++ {
		if e.ran[off] {
			return true
		}
	}
	return false
}

// paint lays every range from widest to narrowest, so each offset carries
// its innermost range's count, as V8's block coverage means it.
func paint(fns []Function, starts []int) executed {
	var all []Range
	size := 0
	if len(starts) > 0 {
		size = starts[len(starts)-1] + 1
	}
	for _, f := range fns {
		for _, r := range f.Ranges {
			all = append(all, r)
			if r.End > size {
				size = r.End
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].End-all[i].Start > all[j].End-all[j].Start })
	ran := make([]bool, size)
	for _, r := range all {
		for off := max(r.Start, 0); off < r.End && off < size; off++ {
			ran[off] = r.Count > 0
		}
	}
	return executed{ran, starts}
}

// lineStarts is the UTF-16 offset of each line of src.
func lineStarts(src []byte) []int {
	starts := []int{0}
	units := 0
	for _, r := range string(src) {
		units += utf16.RuneLen(r)
		if r == '\n' {
			starts = append(starts, units)
		}
	}
	return starts
}

func lineStartsFromLengths(lengths []int) []int {
	starts := make([]int, len(lengths))
	off := 0
	for i, n := range lengths {
		starts[i] = off
		off += n + 1
	}
	return starts
}

// blankLines reports, per line of src, whether it holds only white space.
func blankLines(src []byte) []bool {
	lines := bytes.Split(src, []byte("\n"))
	out := make([]bool, len(lines))
	for i, line := range lines {
		out[i] = len(bytes.TrimSpace(line)) == 0
	}
	return out
}

const vlqChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// decodeMappings calls seg for each mapped segment of a v3 mappings string,
// with every value zero-based.
func decodeMappings(m string, seg func(genLine, genCol, src, srcLine int)) error {
	var genLine, genCol, src, srcLine int
	var fields [5]int // the column and name fields are read and unused
	n := 0
	value, shift := 0, 0
	flush := func() error {
		if n == 0 {
			return nil
		}
		if n != 1 && n != 4 && n != 5 {
			return fmt.Errorf("source map segment of %d fields", n)
		}
		genCol += fields[0]
		if n >= 4 {
			src += fields[1]
			srcLine += fields[2]
			seg(genLine, genCol, src, srcLine)
		}
		n = 0
		return nil
	}
	for i := 0; i < len(m); i++ {
		c := m[i]
		switch c {
		case ';':
			if err := flush(); err != nil {
				return err
			}
			genLine++
			genCol = 0
			continue
		case ',':
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		d := strings.IndexByte(vlqChars, c)
		if d < 0 {
			return fmt.Errorf("source map mapping byte %q", c)
		}
		value += (d & 31) << shift
		if d&32 != 0 {
			shift += 5
			if shift > 30 {
				return fmt.Errorf("source map value overflow")
			}
			continue
		}
		v := value >> 1
		if value&1 != 0 {
			v = -v
		}
		if n == len(fields) {
			return fmt.Errorf("source map segment of more than 5 fields")
		}
		fields[n] = v
		n++
		value, shift = 0, 0
	}
	return flush()
}

func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	out := make([]byte, 0, len(s)*3/4)
	var acc, bits uint
	for i := 0; i < len(s); i++ {
		d := strings.IndexByte(vlqChars, s[i])
		if d < 0 {
			return nil, fmt.Errorf("base64 byte %q", s[i])
		}
		acc = acc<<6 | uint(d)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>bits))
			acc &= 1<<bits - 1
		}
	}
	return out, nil
}

// Istanbul is a coverage-final.json report: per file, statements and
// functions with their hit counts. Jest and Vitest write it from V8 coverage
// already mapped to original sources.
type Istanbul map[string]struct {
	Path         string                  `json:"path"`
	StatementMap map[string]istanbulSpan `json:"statementMap"`
	S            map[string]int          `json:"s"`
	FnMap        map[string]struct {
		Loc istanbulSpan `json:"loc"`
	} `json:"fnMap"`
	F map[string]int `json:"f"`
}

type istanbulSpan struct {
	Start struct{ Line int } `json:"start"`
	End   struct{ Line int } `json:"end"`
}

// AddIstanbul adds a coverage-final.json report. Spans are laid from widest
// to narrowest, so a line inside a function that never ran is unexecuted even
// though the statement declaring the function ran.
func (l Lines) AddIstanbul(data []byte, scope Scope) error {
	var r Istanbul
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("istanbul: %w", err)
	}
	type span struct{ lo, hi, n int }
	for key, f := range r {
		file := f.Path
		if file == "" {
			file = key
		}
		rel, ok := scope(file)
		if !ok {
			continue
		}
		var spans []span
		for id, s := range f.StatementMap {
			spans = append(spans, span{s.Start.Line, s.End.Line, f.S[id]})
		}
		for id, fn := range f.FnMap {
			spans = append(spans, span{fn.Loc.Start.Line, fn.Loc.End.Line, f.F[id]})
		}
		sort.SliceStable(spans, func(i, j int) bool { return spans[i].hi-spans[i].lo > spans[j].hi-spans[j].lo })
		lines := map[int]bool{}
		for _, s := range spans {
			for line := s.lo; line <= s.hi; line++ {
				lines[line] = s.n > 0
			}
		}
		for line, ran := range lines {
			l.Add(rel, line, ran)
		}
	}
	return nil
}

// AddLCOV adds an LCOV tracefile's DA records, as bun and `deno coverage
// --lcov` write them. A relative SF path is relative to base. A generated file
// whose sourceMappingURL names a map counts against its original sources, as
// in AddV8, since bun reports the script it ran, not its source.
func (l Lines) AddLCOV(data []byte, base string, scope Scope) error {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	file := ""
	da := map[int]bool{} // generated line -> ran, for the current SF
	flush := func() error {
		defer func() { file, da = "", map[int]bool{} }()
		if file == "" {
			return nil
		}
		if src, err := os.ReadFile(file); err == nil {
			sm, mapURL, err := ReadSourceMap(file, src)
			if err != nil {
				return err
			}
			if sm != nil {
				return l.addMappedLines(sm, mapURL, da, scope)
			}
		}
		rel, keep := scope(file)
		if !keep {
			return nil
		}
		for n, ran := range da {
			l.Add(rel, n, ran)
		}
		return nil
	}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			if err := flush(); err != nil {
				return err
			}
			file = strings.TrimPrefix(line, "SF:")
			if p, ok := filePath(file); ok {
				file = p
			} else if !filepath.IsAbs(file) {
				file = filepath.Join(base, filepath.FromSlash(file))
			}
			if strings.Contains(filepath.ToSlash(file), "/node_modules/") {
				file = ""
			}
		case strings.HasPrefix(line, "DA:") && file != "":
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) < 2 {
				return fmt.Errorf("lcov: malformed %q", line)
			}
			n, err1 := strconv.Atoi(parts[0])
			c, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("lcov: malformed %q", line)
			}
			da[n] = da[n] || c > 0
		case line == "end_of_record":
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}

// addMappedLines adds each original line a segment of a reported generated
// line maps to, executed when that generated line ran.
func (l Lines) addMappedLines(sm *SourceMap, mapURL string, da map[int]bool, scope Scope) error {
	sources := make([]string, len(sm.Sources))
	kept := make([]bool, len(sm.Sources))
	for i, s := range sm.Sources {
		if abs, ok := resolveSource(mapURL, sm.SourceRoot, s); ok {
			sources[i], kept[i] = scope(abs)
		}
	}
	return decodeMappings(sm.Mappings, func(genLine, _, src, srcLine int) {
		ran, reported := da[genLine+1]
		if src < 0 || src >= len(sources) || !kept[src] || !reported {
			return
		}
		l.Add(sources[src], srcLine+1, ran)
	})
}
