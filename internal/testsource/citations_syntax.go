package testsource

import (
	"bytes"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Configurable citation syntax and document selection for `corvint tests citations`
// (TSA-V0-380..387; V1-1972 / GitHub #1224). A syntax is a named profile or a caller pattern with
// named capture groups; documents are named, listed in committed manifests or matched by globs.

// Citation syntax profile names (TSA-V0-380).
const (
	CitationSyntaxInlineCode = "inline-code"
	CitationSyntaxHTMLSub    = "html-sub"
	CitationSyntaxPattern    = "pattern"
)

// Unparseable is the reason of a citation candidate the syntax cannot read (TSA-V0-382).
const CitationUnparseable = "unparseable"

// CitationRepositoryNotChecked abstains a citation that names another repository (TSA-V0-381).
const CitationRepositoryNotChecked = "repository-not-checked"

const (
	// maxCitationPatternBytes bounds a caller's --pattern or --marker (TSA-V0-381).
	maxCitationPatternBytes = 4096
	// maxCitationManifestBytes bounds one committed manifest (TSA-V0-383).
	maxCitationManifestBytes = 16 << 20
	// maxCitationGlobs bounds the --glob and --manifest options of one run (TSA-V0-383).
	maxCitationGlobs = 64
)

// citationGroups are the capture-group names a caller pattern may use (TSA-V0-381).
var citationGroups = map[string]bool{"repo": true, "path": true, "start": true, "end": true, "symbol": true, "rev": true, "body": true}

// citationSyntax finds citation candidates in a line and reads each one.
type citationSyntax struct {
	name string
	// marker finds candidates; its `body` group, else the whole match, is the candidate text.
	marker *regexp.Regexp
	// citation reads a whole candidate through its named groups.
	citation *regexp.Regexp
	// attempt, when set, admits a candidate citation does not read as unparseable; a candidate it
	// does not match is not a citation at all (inline-code only).
	attempt *regexp.Regexp
	// anchored keeps the TSA-V0-250 rule: a bare path counts only under a default revision.
	anchored bool
}

const (
	citationPathText  = `[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)+`
	citationRevText   = `[A-Za-z0-9_.\-/^~]+`
	citationIdentText = `[A-Za-z_$][A-Za-z0-9_$]*`
)

var (
	citationRevision   = regexp.MustCompile(`^` + citationRevText + `$`)
	citationIdentifier = regexp.MustCompile(`^` + citationIdentText + `$`)
)

// inlineCodeSyntax is the TSA-V0-250 grammar: an inline-code span `path[:START[-END]][#Symbol][@REV]`.
// A span that opens with such a path and `:DIGIT` or `@` but does not read is unparseable.
var inlineCodeSyntax = citationSyntax{
	name:     CitationSyntaxInlineCode,
	marker:   regexp.MustCompile("`(?P<body>[^`\n]+)`"),
	citation: regexp.MustCompile(`^(?P<path>` + citationPathText + `)(?::(?P<start>[0-9]+)(?:-(?P<end>[0-9]+))?)?(?:#(?P<symbol>` + citationIdentText + `))?(?:@(?P<rev>` + citationRevText + `))?$`),
	attempt:  regexp.MustCompile(`^` + citationPathText + `(?::[0-9]|@)`),
	anchored: true,
}

// htmlSubSyntax reads HTML `<sub>` markers that carry a path and `line N` (TSA-V0-380): an optional
// `source:`, `path:`, `file:` or `see:` label, the path (optionally in backticks or <code>), an
// optional comma or colon, `line N` or `lines N-M` (`-`, en dash, em dash or `to`), an optional
// `#Symbol` and an optional `@REV`. Every `<sub>` naming `line` that does not read is unparseable.
var htmlSubSyntax = citationSyntax{
	name:   CitationSyntaxHTMLSub,
	marker: regexp.MustCompile(`(?i)<sub(?:\s[^>]*)?>(?P<body>.*?)</sub>`),
	citation: regexp.MustCompile(`(?i)^\s*(?:(?:source|path|file|see)\s*:\s*)?(?:<code>|` + "`" + `)?(?P<path>[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)*)(?:</code>|` + "`" + `)?\s*[,:]?\s*lines?\s+(?P<start>[0-9]+)(?:\s*(?:-|–|—|to)\s*(?P<end>[0-9]+))?` +
		`(?:\s*#(?P<symbol>` + citationIdentText + `))?(?:\s*@\s*(?P<rev>` + citationRevText + `))?\s*$`),
	attempt: regexp.MustCompile(`(?i)\blines?\b`),
}

// compileCitationSyntax selects the run's syntax: a named profile (default inline-code), or a
// caller pattern with an optional marker (TSA-V0-380, TSA-V0-381).
func compileCitationSyntax(options CitationOptions) (citationSyntax, error) {
	if options.Pattern == "" {
		if options.Marker != "" {
			return citationSyntax{}, refuse("tests-invalid-arguments", "--marker needs --pattern")
		}
		switch options.Syntax {
		case "", CitationSyntaxInlineCode:
			return inlineCodeSyntax, nil
		case CitationSyntaxHTMLSub:
			return htmlSubSyntax, nil
		}
		return citationSyntax{}, refuse("tests-invalid-arguments", "unknown citation syntax %q (inline-code or html-sub)", options.Syntax)
	}
	if options.Syntax != "" && options.Syntax != CitationSyntaxPattern {
		return citationSyntax{}, refuse("tests-invalid-arguments", "--syntax and --pattern are exclusive")
	}
	citation, err := compileCitationPattern(options.Pattern, "--pattern")
	if err != nil {
		return citationSyntax{}, err
	}
	if citation.SubexpIndex("path") < 0 {
		return citationSyntax{}, refuse("tests-invalid-arguments", "--pattern must name a path group")
	}
	syntax := citationSyntax{name: CitationSyntaxPattern, marker: citation, citation: regexp.MustCompile(`^(?:` + citation.String() + `)$`)}
	if options.Marker != "" {
		if syntax.marker, err = compileCitationPattern(options.Marker, "--marker"); err != nil {
			return citationSyntax{}, err
		}
	}
	return syntax, nil
}

// compileCitationPattern compiles one bounded RE2 pattern whose named groups are all known.
func compileCitationPattern(text, option string) (*regexp.Regexp, error) {
	if len(text) > maxCitationPatternBytes || strings.ContainsAny(text, "\x00") {
		return nil, refuse("tests-invalid-arguments", "%s is over %d bytes or holds NUL", option, maxCitationPatternBytes)
	}
	re, err := regexp.Compile(text)
	if err != nil {
		return nil, refuse("tests-invalid-arguments", "%s does not compile: %v", option, err)
	}
	for _, name := range re.SubexpNames() {
		if name != "" && !citationGroups[name] {
			return nil, refuse("tests-invalid-arguments", "%s names unknown group %q (repo, path, start, end, symbol, rev, body)", option, name)
		}
	}
	return re, nil
}

// candidate is one marker match on one line: either a parsed citation or an unparseable text.
type candidate struct {
	citation
	repo        string
	unparseable bool
}

// parse returns a document's citation candidates in line order (TSA-V0-250, TSA-V0-251,
// TSA-V0-382). Fenced code is skipped for every syntax.
func (s citationSyntax) parse(text string) []candidate {
	out := []candidate{}
	lines := strings.Split(text, "\n")
	inFence, def := fencedLines(lines)
	body := s.marker.SubexpIndex("body")
	for i, line := range lines {
		if inFence[i] {
			continue
		}
		for _, m := range s.marker.FindAllStringSubmatchIndex(line, -1) {
			raw := line[m[0]:m[1]]
			if body >= 0 && m[2*body] >= 0 {
				raw = line[m[2*body]:m[2*body+1]]
			}
			// A clone keeps a finding from pinning its whole document in memory (TSA-V0-384).
			raw = strings.Clone(raw)
			c, ok, counted := s.read(raw, def)
			switch {
			case ok && counted:
				c.line = i + 1
				out = append(out, c)
			case !ok && (s.attempt == nil || s.attempt.MatchString(raw)):
				out = append(out, candidate{citation: citation{line: i + 1, text: raw}, unparseable: true})
			}
		}
	}
	return out
}

// read parses one candidate. ok is false when it does not read; counted is false for an
// inline-code bare path without a default revision (TSA-V0-250).
func (s citationSyntax) read(raw, def string) (candidate, bool, bool) {
	m := s.citation.FindStringSubmatch(raw)
	if m == nil {
		return candidate{}, false, false
	}
	group := func(name string) string {
		if i := s.citation.SubexpIndex(name); i >= 0 {
			return strings.TrimSpace(m[i])
		}
		return ""
	}
	c := candidate{citation: citation{text: raw, path: group("path"), symbol: group("symbol"), revision: group("rev")}, repo: group("repo")}
	startText, endText := group("start"), group("end")
	if s.anchored && startText == "" && c.symbol == "" && c.revision == "" && def == "" {
		return candidate{}, true, false
	}
	if !citationPathValid(c.path) || c.revision != "" && (!citationRevision.MatchString(c.revision) || strings.HasPrefix(c.revision, "-")) ||
		c.symbol != "" && !citationIdentifier.MatchString(c.symbol) || startText == "" && endText != "" {
		return candidate{}, false, false
	}
	if startText != "" {
		start, err := strconv.Atoi(startText)
		if err != nil || start > 1<<30 {
			return candidate{}, false, false
		}
		c.ranged, c.start, c.end = true, start, start
		if endText != "" {
			if c.end, err = strconv.Atoi(endText); err != nil || c.end > 1<<30 {
				return candidate{}, false, false
			}
		}
	}
	if c.revision == "" {
		c.revision = def
	}
	return c, true, true
}

// citationPathValid accepts a repository-relative path outside .git.
func citationPathValid(name string) bool {
	return fs.ValidPath(name) && name != "." && name != ".git" && !strings.HasPrefix(name, ".git/") && !strings.ContainsAny(name, "\x00\n\r\\")
}

// fencedLines marks the lines inside fenced code and returns the document's default citation
// revision, read outside fences (TSA-V0-251).
func fencedLines(lines []string) ([]bool, string) {
	inFence := make([]bool, len(lines))
	fence, def := "", ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); fence == "" && marker != "" {
			fence, inFence[i] = marker, true
			continue
		} else if fence != "" {
			inFence[i] = true
			if marker != "" && marker[0] == fence[0] && len(marker) >= len(fence) && strings.TrimLeft(trimmed, marker[:1]) == "" {
				fence = ""
			}
			continue
		}
		if m := citationDefault.FindStringSubmatch(line); m != nil && def == "" {
			def = m[1]
		}
	}
	return inFence, def
}

// citationGlob is one validated --glob: `*`, `?` and `[...]` within a segment, `**` for zero or
// more whole segments (TSA-V0-383).
type citationGlob struct {
	segments []string
	prefix   string
}

func compileCitationGlob(pattern string) (citationGlob, error) {
	segments := strings.Split(pattern, "/")
	if pattern == "" || len(pattern) > maxCitationPatternBytes || strings.ContainsAny(pattern, "\x00\n\r\\") || len(segments) > 64 {
		return citationGlob{}, refuse("tests-invalid-arguments", "%q is not a repository glob", pattern)
	}
	literal := true
	prefix := []string{}
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || segment == ".git" {
			return citationGlob{}, refuse("tests-invalid-arguments", "%q is not a repository glob", pattern)
		}
		if _, err := path.Match(segment, ""); err != nil {
			return citationGlob{}, refuse("tests-invalid-arguments", "%q is not a repository glob", pattern)
		}
		if literal && !strings.ContainsAny(segment, "*?[") {
			prefix = append(prefix, segment)
		} else {
			literal = false
		}
	}
	g := citationGlob{segments: segments}
	if len(prefix) == len(segments) {
		prefix = prefix[:len(prefix)-1]
	}
	if len(prefix) > 0 {
		g.prefix = strings.Join(prefix, "/") + "/"
	}
	return g, nil
}

// match reports a path the glob names, in O(segments x path segments).
func (g citationGlob) match(name string) bool {
	parts := strings.Split(name, "/")
	// reach[j]: the pattern prefix read so far matches parts[:j].
	reach := make([]bool, len(parts)+1)
	reach[0] = true
	for _, segment := range g.segments {
		next := make([]bool, len(parts)+1)
		for j := 0; j <= len(parts); j++ {
			if !reach[j] {
				continue
			}
			if segment == "**" {
				for k := j; k <= len(parts); k++ {
					next[k] = true
				}
				break
			}
			if j < len(parts) {
				if ok, _ := path.Match(segment, parts[j]); ok {
					next[j+1] = true
				}
			}
		}
		reach = next
	}
	return reach[len(parts)]
}

// manifestPaths reads a committed manifest: one repository-relative path per line; blank lines
// and lines opening with `#` are ignored (TSA-V0-383).
func manifestPaths(name string, data []byte) ([]string, error) {
	out := []string{}
	for n, line := range bytes.Split(data, []byte{'\n'}) {
		text := strings.TrimSpace(strings.TrimSuffix(string(line), "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if !citationPathValid(text) {
			return nil, refuse("tests-invalid-arguments", "%s line %d: %q is not a repository-relative file path", name, n+1, text)
		}
		out = append(out, text)
	}
	return out, nil
}
