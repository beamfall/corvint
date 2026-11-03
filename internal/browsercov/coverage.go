// SPDX-License-Identifier: Apache-2.0

// Package browsercov is the browser end-to-end line-coverage collector of
// docs/specs/live-proof-carrying-verification-v0.md (LPCV-V0-970..979): it runs
// each browser test alone with precise V8 coverage in the browser (Chrome
// DevTools Protocol) and in every Node process under test (NODE_V8_COVERAGE),
// joins both onto the repository's tracked files at one commit, and emits
// internal/linecov records, so one index serves the same selector.
package browsercov

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

// Script is one script's precise coverage as V8 reports it, from the browser
// or from a Node process.
type Script struct {
	URL       string     `json:"url"`
	Source    string     `json:"source"`
	Functions []Function `json:"functions"`
	// SourceMap is the external map the hook fetched while the server ran.
	SourceMap *fetchedMap `json:"sourceMap,omitempty"`
}

type fetchedMap struct {
	URL   string `json:"url"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// Function is one V8 FunctionCoverage; Ranges nest, outer first.
type Function struct {
	FunctionName string  `json:"functionName"`
	Ranges       []Range `json:"ranges"`
}

// Range offsets are UTF-16 code units into the script source; End is
// exclusive.
type Range struct {
	StartOffset int `json:"startOffset"`
	EndOffset   int `json:"endOffset"`
	Count       int `json:"count"`
}

// Repo is the tracked content one collection run attributes coverage to.
type Repo struct {
	Root  string
	OIDs  map[string]string   // path -> blob OID at the index commit
	paths map[string][]string // blob OID -> paths
	sha   int                 // hex length of the repository's object IDs
	// Generated lists absolute paths inside Root that the collector itself
	// wrote (its hook and Cypress wrapper), which run but are not code under
	// test.
	Generated []string
}

// NewRepo indexes oids by blob, so a served script is matched by content.
func NewRepo(root string, oids map[string]string) *Repo {
	r := &Repo{Root: root, OIDs: oids, paths: map[string][]string{}}
	for p, oid := range oids {
		r.paths[oid] = append(r.paths[oid], p)
		r.sha = len(oid)
	}
	for _, ps := range r.paths {
		sort.Strings(ps)
	}
	return r
}

// blobOID is the Git object ID of content as a blob in this repository's
// object format.
func (r *Repo) blobOID(content []byte) string {
	head := fmt.Appendf(nil, "blob %d\x00", len(content))
	if r.sha == 64 {
		h := sha256.New()
		h.Write(head)
		h.Write(content)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New()
	h.Write(head)
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// Observation is what one test's run says about one tracked file. A direct
// observation holds a state per UTF-16 unit of the file text (V8 offsets
// apply to the file unchanged); a mapped one, reached through a source map,
// holds only line states.
type Observation struct {
	Path  string
	Text  string
	Units []int8       // direct: -1 outside every range, 0 unexecuted, 1 executed
	Lines map[int]bool // mapped: 0-based original line -> executed
}

// paint returns the per-unit execution state of a script of n UTF-16 units.
// Inner ranges follow their enclosing range, so painting in start order,
// outer before inner, leaves each unit with its innermost count.
func paint(n int, fns []Function) []int8 {
	units := make([]int8, n)
	for i := range units {
		units[i] = -1
	}
	var rs []Range
	for _, f := range fns {
		rs = append(rs, f.Ranges...)
	}
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].StartOffset != rs[j].StartOffset {
			return rs[i].StartOffset < rs[j].StartOffset
		}
		return rs[i].EndOffset > rs[j].EndOffset
	})
	for _, r := range rs {
		st := int8(0)
		if r.Count > 0 {
			st = 1
		}
		for i := max(r.StartOffset, 0); i < min(r.EndOffset, n); i++ {
			units[i] = st
		}
	}
	return units
}

// Attribute maps one script's coverage onto tracked files. A script it
// cannot bind to tracked content while it may be repository code (served
// from a loopback origin, or a file: URL inside the repository) returns a
// reason that makes the record incomplete (LPCV-V0-954, LPCV-V0-972).
func (r *Repo) Attribute(s Script) ([]Observation, string) {
	if s.URL == "" || ignoredURL(s.URL) {
		return nil, ""
	}
	if strings.HasPrefix(s.URL, "file:") {
		return r.attributeFile(s)
	}
	src := []byte(s.Source)
	if ps := r.paths[r.blobOID(src)]; len(ps) > 0 {
		units := paint(len(utf16.Encode([]rune(s.Source))), s.Functions)
		// Identical copies all receive the coverage: over-attribution only
		// selects more tests.
		var out []Observation
		for _, p := range ps {
			out = append(out, Observation{Path: p, Text: s.Source, Units: units})
		}
		return out, ""
	}
	if obs, reason, ok := r.attributeMapped(s); ok {
		return obs, reason
	}
	if obs := r.attributeInline(s); obs != nil {
		return obs, ""
	}
	if cypressInjected(s.Source) {
		return nil, "" // the runner's own script, injected into the page under test
	}
	if local(s.URL) {
		return nil, "script " + s.URL + " matches no tracked file and has no source map"
	}
	return nil, ""
}

// ignoredURL names scripts that are never repository code under test: the
// runners' own pages and injected scripts, extensions, and Node internals.
func ignoredURL(u string) bool {
	for _, p := range []string{"node:", "internal/", "chrome-extension:", "extensions::", "chrome:", "devtools:", "about:", "__playwright", "pptr:", "debugger:", "evalmachine.", "<embedded>"} {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	if pu, err := url.Parse(u); err == nil && (strings.HasPrefix(pu.Path, "/__cypress") || strings.HasPrefix(pu.Path, "/__/")) {
		return true // Cypress's runner, prompt driver and spec iframes are test code
	}
	return false
}

// cypressInjected reports Cypress's inline script that its proxy prepends
// to every HTML page under test; it binds the page to the runner's frame.
func cypressInjected(source string) bool {
	return strings.Contains(source, "window.Cypress=parent.Cypress") || strings.Contains(source, "window.Cypress = parent.Cypress")
}

func local(u string) bool {
	pu, err := url.Parse(u)
	if err != nil {
		return true
	}
	h := pu.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0" || strings.HasSuffix(h, ".localhost") || h == ""
}

// attributeFile binds a Node script by path: the file must be tracked and its
// working-tree content must be the indexed blob, or the run did not execute
// the indexed content.
func (r *Repo) attributeFile(s Script) ([]Observation, string) {
	pu, err := url.Parse(s.URL)
	if err != nil {
		return nil, "unparsable script URL " + s.URL
	}
	rel, err := filepath.Rel(r.Root, filepath.FromSlash(pu.Path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, ""
	}
	rel = filepath.ToSlash(rel)
	oid, tracked := r.OIDs[rel]
	if !tracked {
		if r.untrackedCode(rel) {
			// A generated or uncommitted file runs code no index entry can
			// name; a build output's map is not read (LPCV-V0-972).
			return nil, rel + " runs but is not tracked at the index commit"
		}
		return nil, "" // a dependency, or the collector's own file
	}
	body, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err.Error()
	}
	bom := 0
	if strings.HasPrefix(string(body), "\uFEFF") {
		bom = 1 // Node compiles the text after a byte-order mark
	}
	if r.blobOID(body) != oid {
		return nil, rel + " differs from its blob at the index commit"
	}
	switch filepath.Ext(rel) {
	case ".js", ".cjs", ".mjs":
	default:
		// A transpiled module's V8 offsets are not offsets into the file.
		return nil, rel + " runs transpiled without a recorded source map"
	}
	text := string(body)
	units := paint(len(utf16.Encode([]rune(text)))-bom, s.Functions)
	if bom == 1 {
		units = append([]int8{-1}, units...)
	}
	return []Observation{{Path: rel, Text: text, Units: units}}, ""
}

// code returns the root-relative path of a file: script URL that is
// repository code: tracked, or untracked outside dependencies and the
// collector's own files.
func (r *Repo) code(u string) (string, bool) {
	pu, err := url.Parse(u)
	if err != nil || pu.Scheme != "file" {
		return "", false
	}
	rel, err := filepath.Rel(r.Root, filepath.FromSlash(pu.Path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if _, tracked := r.OIDs[rel]; tracked && !strings.Contains("/"+rel, "/node_modules/") {
		return rel, true
	}
	return rel, r.untrackedCode(rel)
}

// untrackedCode reports whether an untracked root-relative path is code that
// ran from the repository: not a dependency and not the collector's own file.
func (r *Repo) untrackedCode(rel string) bool {
	if strings.Contains("/"+rel, "/node_modules/") {
		return false
	}
	full := filepath.Join(r.Root, filepath.FromSlash(rel))
	for _, g := range r.Generated {
		if full == g || strings.HasPrefix(full, g+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

// attributeMapped applies the script's source map. ok is false when the
// script names no map. Each original source binds to a tracked file whose
// blob matches its sourcesContent, or else whose path uniquely ends with the
// source's path; a source outside the repository (a dependency) is skipped.
func (r *Repo) attributeMapped(s Script) ([]Observation, string, bool) {
	text, ok, err := inlineMap(s.Source)
	switch {
	case err != nil:
		return nil, "script " + s.URL + ": " + err.Error(), true
	case !ok && s.SourceMap == nil:
		return nil, "", false
	case !ok && s.SourceMap.Error != "":
		return nil, "script " + s.URL + ": source map " + s.SourceMap.URL + ": " + s.SourceMap.Error, true
	case !ok:
		text = []byte(s.SourceMap.Text)
	}
	m, err := parseSourceMap(text)
	if err != nil {
		return nil, "script " + s.URL + ": " + err.Error(), true
	}
	units := paint(len(utf16.Encode([]rune(s.Source))), s.Functions)
	blank := blankUnits(s.Source)
	lineStart := utf16LineStarts(s.Source)
	bound := make([]string, len(m.Sources))
	for i, src := range m.Sources {
		var content *string
		if i < len(m.SourcesContent) {
			content = m.SourcesContent[i]
		}
		p, reason := r.bindSource(joinSourceRoot(m.SourceRoot, src), content)
		if reason != "" {
			return nil, "script " + s.URL + ": " + reason, true
		}
		bound[i] = p
	}
	lines := map[string]map[int]bool{}
	for gl, segs := range m.Lines {
		if gl >= len(lineStart) {
			break
		}
		end := len(units)
		if gl+1 < len(lineStart) {
			end = lineStart[gl+1]
		}
		for k, seg := range segs {
			if seg.Src < 0 || bound[seg.Src] == "" {
				continue
			}
			from := lineStart[gl] + seg.Col
			to := end
			if k+1 < len(segs) {
				to = lineStart[gl] + segs[k+1].Col
			}
			state := int8(-1)
			for u := from; u < min(to, len(units)); u++ {
				if !blank[u] {
					state = max(state, units[u])
				}
			}
			if state < 0 {
				continue
			}
			p := bound[seg.Src]
			if lines[p] == nil {
				lines[p] = map[int]bool{}
			}
			lines[p][seg.OrigLine] = lines[p][seg.OrigLine] || state == 1
		}
	}
	var out []Observation
	for p, ls := range lines {
		body, _ := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(p)))
		out = append(out, Observation{Path: p, Text: string(body), Lines: ls})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, "", true
}

// bindSource resolves one source-map source to a tracked path, "" for a
// source that is not repository content.
func (r *Repo) bindSource(src string, content *string) (string, string) {
	if content != nil {
		if ps := r.paths[r.blobOID([]byte(*content))]; len(ps) > 0 {
			return pickSuffix(ps, src), ""
		}
	}
	var hits []string
	if abs, ok := absSource(src); ok {
		// An absolute source names its file: inside the repository it must be
		// tracked, outside it is a dependency.
		rel, err := filepath.Rel(r.Root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", ""
		}
		rel = filepath.ToSlash(rel)
		if _, tracked := r.OIDs[rel]; !tracked {
			if r.untrackedCode(rel) {
				return "", "source " + src + " is not tracked at the index commit"
			}
			return "", ""
		}
		hits = []string{rel}
	} else {
		clean := sourcePath(src)
		if clean == "" || strings.Contains("/"+clean, "/node_modules/") {
			return "", ""
		}
		for p := range r.OIDs {
			if p == clean || strings.HasSuffix(p, "/"+clean) {
				hits = append(hits, p)
			}
		}
	}
	switch {
	case len(hits) == 0:
		return "", ""
	case len(hits) > 1:
		sort.Strings(hits)
		return "", "source " + src + " matches several tracked files: " + strings.Join(hits, ", ")
	case content != nil:
		// The map embeds content that is not the indexed blob: the served
		// bundle was built from other content.
		return "", "source " + src + " differs from " + hits[0] + " at the index commit"
	}
	body, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(hits[0])))
	if err != nil || r.blobOID(body) != r.OIDs[hits[0]] {
		return "", "source " + src + " differs from " + hits[0] + " at the index commit"
	}
	return hits[0], ""
}

// sourcePath strips a source-map source of its scheme, bundler namespace and
// relative prefix.
func sourcePath(src string) string {
	if i := strings.Index(src, "://"); i >= 0 {
		src = src[i+3:]
		if j := strings.IndexByte(src, '/'); j >= 0 && !strings.HasPrefix(src, "/") {
			src = src[j:] // webpack://<namespace>/path
		}
	}
	src = strings.TrimPrefix(src, "file:")
	src = filepath.ToSlash(filepath.Clean("/" + src))
	return strings.TrimPrefix(src, "/")
}

// joinSourceRoot prefixes a relative source with the map's sourceRoot.
func joinSourceRoot(root, src string) string {
	switch {
	case root == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "/"):
		return src
	case strings.HasSuffix(root, "/"):
		return root + src
	}
	return root + "/" + src
}

// absSource returns the filesystem path of an absolute source: a file: URL
// or a rooted path that exists on disk. A webpack:// or other bundler source
// is relative to its namespace.
func absSource(src string) (string, bool) {
	if strings.HasPrefix(src, "file://") {
		u, err := url.Parse(src)
		if err != nil {
			return "", false
		}
		return filepath.FromSlash(u.Path), true
	}
	if filepath.IsAbs(filepath.FromSlash(src)) {
		if _, err := os.Stat(filepath.FromSlash(src)); err == nil {
			return filepath.FromSlash(src), true
		}
	}
	return "", false
}

func pickSuffix(ps []string, src string) string {
	clean := sourcePath(src)
	for _, p := range ps {
		if p == clean || strings.HasSuffix(p, "/"+clean) || strings.HasSuffix(clean, "/"+p) {
			return p
		}
	}
	return ps[0]
}

// attributeInline binds an inline <script> by finding its text in exactly one
// tracked HTML file.
func (r *Repo) attributeInline(s Script) []Observation {
	if strings.TrimSpace(s.Source) == "" {
		return nil
	}
	var found []Observation
	for p := range r.OIDs {
		if ext := strings.ToLower(filepath.Ext(p)); ext != ".html" && ext != ".htm" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(p)))
		if err != nil || r.blobOID(body) != r.OIDs[p] {
			continue
		}
		text := string(body)
		i := strings.Index(text, s.Source)
		if i < 0 || strings.Contains(text[i+1:], s.Source) {
			continue
		}
		shift := len(utf16.Encode([]rune(text[:i])))
		inner := paint(len(utf16.Encode([]rune(s.Source))), s.Functions)
		units := make([]int8, len(utf16.Encode([]rune(text))))
		for k := range units {
			units[k] = -1
		}
		copy(units[shift:], inner)
		found = append(found, Observation{Path: p, Text: text, Units: units})
	}
	if len(found) != 1 {
		return nil
	}
	return found
}

// blankUnits marks the UTF-16 units of s that are whitespace.
func blankUnits(s string) []bool {
	var out []bool
	for _, c := range s {
		b := c == ' ' || c == '\t' || c == '\r' || c == '\n'
		for range utf16.RuneLen(c) {
			out = append(out, b)
		}
	}
	return out
}

// utf16LineStarts returns the UTF-16 offset of each line's first unit.
func utf16LineStarts(s string) []int {
	starts := []int{0}
	n := 0
	for _, c := range s {
		n += utf16.RuneLen(c)
		if c == '\n' {
			starts = append(starts, n)
		}
	}
	return starts
}

// decode reads one hook or CDP dump of browser script coverage.
func decodeBrowser(b []byte) ([]Script, error) {
	var out []Script
	err := json.Unmarshal(b, &out)
	return out, err
}

// nodeDump is one NODE_V8_COVERAGE file.
type nodeDump struct {
	Result []struct {
		URL       string     `json:"url"`
		Functions []Function `json:"functions"`
	} `json:"result"`
	SourceMapCache map[string]json.RawMessage `json:"source-map-cache"`
}
