package testsource

import (
	"bytes"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

// Revision-pinned source citations (TSA-V0-220..226; V1-1934 / GitHub #1195). A review document
// cites committed source as `path:start-end@rev`; the check reads the cited path, range and symbol
// from the object store at the cited revision. It never checks out, writes or uses the network.

// CitationsSchema is the citation report's closed profile.
const CitationsSchema = "corvint-source-citations/0"

// Citation finding reasons (TSA-V0-223).
const (
	CitationMissingAtRevision  = "missing-at-revision"
	CitationRangeOutOfBounds   = "range-out-of-bounds"
	CitationSymbolNotInRange   = "symbol-not-in-range"
	CitationRevisionUnresolved = "revision-unresolved"
	CitationNotAFile           = "not-a-file"
	CitationFileTooLarge       = "file-too-large"
)

// MaxCitationsPerReport bounds the citations one report checks (TSA-V0-226).
const MaxCitationsPerReport = 4096

// CitationOptions names the documents to check, read at Revision (default HEAD).
type CitationOptions struct {
	Revision string
	Docs     []string
}

// CitationReport is one `corvint tests citations` document.
type CitationReport struct {
	Schema    string            `json:"schema"`
	Revision  string            `json:"revision"`
	Checked   int               `json:"checked"`
	Findings  []CitationFinding `json:"findings"`
	Abstained []CitationFinding `json:"abstained"`
}

// CitationFinding is one citation the object store does not support, or one the check could not
// decide (Abstained).
type CitationFinding struct {
	Doc        string `json:"doc"`
	Line       int    `json:"line"`
	Citation   string `json:"citation"`
	Revision   string `json:"revision"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion,omitempty"`
}

// citationSpan matches one inline-code citation: a repository path holding a `/`, then a line or
// range, an optional `#Symbol` and an optional `@rev`; or a path with `@rev` alone (TSA-V0-220).
var citationSpan = regexp.MustCompile(`^([A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)+)(?::([0-9]+)(?:-([0-9]+))?)?(?:#([A-Za-z_$][A-Za-z0-9_$]*))?(?:@([A-Za-z0-9_.\-/^~]+))?$`)

// citationDefault is a document's default citation revision (TSA-V0-221).
var citationDefault = regexp.MustCompile(`^\s*<!--\s*corvint-cite-revision:\s*([A-Za-z0-9_.\-/^~]+)\s*-->\s*$`)

var inlineCode = regexp.MustCompile("`([^`\n]+)`")

// citation is one parsed citation.
type citation struct {
	line         int
	text         string
	path, symbol string
	start, end   int
	ranged       bool
	revision     string
}

// parseCitations returns a document's citations in line order (TSA-V0-220, TSA-V0-221). A span
// counts only when it carries a line, a symbol or a revision, or the document declares a default
// revision. Fenced code blocks are skipped.
func parseCitations(text string) []citation {
	out := []citation{}
	def := ""
	lines := strings.Split(text, "\n")
	inFence := make([]bool, len(lines))
	fence := ""
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
	for i, line := range lines {
		if inFence[i] {
			continue
		}
		for _, span := range inlineCode.FindAllStringSubmatch(line, -1) {
			m := citationSpan.FindStringSubmatch(span[1])
			if m == nil || m[2] == "" && m[4] == "" && m[5] == "" && def == "" {
				continue
			}
			c := citation{line: i + 1, text: span[1], path: m[1], symbol: m[4], revision: m[5]}
			if c.revision == "" {
				c.revision = def
			}

			if m[2] != "" {
				c.ranged = true
				c.start, _ = strconv.Atoi(m[2])
				c.end = c.start
				if m[3] != "" {
					c.end, _ = strconv.Atoi(m[3])
				}
			}
			out = append(out, c)
		}
	}
	return out
}

// Citations checks every citation of the named committed documents against the object store
// (TSA-V0-222..226). Documents are read at options.Revision; each citation at its own revision.
func Citations(r Repo, options CitationOptions) (CitationReport, error) {
	docs := []string{}
	seen := map[string]bool{}
	for _, name := range options.Docs {
		if !fs.ValidPath(name) || name == "." || name == ".git" || strings.HasPrefix(name, ".git/") || strings.ContainsAny(name, "\x00\n\r\\") {
			return CitationReport{}, refuse("tests-invalid-arguments", "%q is not a repository-relative file path", name)
		}
		if !seen[name] {
			seen[name] = true
			docs = append(docs, name)
		}
	}
	if len(docs) == 0 {
		return CitationReport{}, refuse("tests-invalid-arguments", "no document was named")
	}
	if len(docs) > MaxValidityFiles {
		return CitationReport{}, refuse("tests-bound-exceeded", "more than %d documents", MaxValidityFiles)
	}
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return CitationReport{}, err
	}
	report := CitationReport{Schema: CitationsSchema, Revision: commit, Findings: []CitationFinding{}, Abstained: []CitationFinding{}}
	resolved := map[string]string{}
	files := map[[2]string]citedFile{}
	for _, doc := range docs {
		data, err := r.committedFile(commit, doc)
		if err != nil {
			return CitationReport{}, err
		}
		for _, c := range parseCitations(string(data)) {
			report.Checked++
			if report.Checked > MaxCitationsPerReport {
				return CitationReport{}, refuse("tests-bound-exceeded", "more than %d citations", MaxCitationsPerReport)
			}
			finding := CitationFinding{Doc: doc, Line: c.line, Citation: c.text, Revision: c.revision}
			rev := c.revision
			if rev == "" {
				rev, finding.Revision = commit, commit
			}
			at, ok := resolved[rev]
			if !ok {
				at, err = r.Resolve(rev)
				if err != nil {
					at = ""
				}
				resolved[rev] = at
			}
			if at == "" {
				finding.Reason = CitationRevisionUnresolved
				report.Abstained = append(report.Abstained, finding)
				continue
			}
			finding.Revision = at
			key := [2]string{at, c.path}
			file, ok := files[key]
			if !ok {
				if file, err = r.citedFile(at, c.path); err != nil {
					return CitationReport{}, err
				}
				files[key] = file
			}
			switch {
			case !file.exists:
				finding.Reason = CitationMissingAtRevision
				finding.Suggestion = r.nearestRevision(at, commit, c.path)
			case !c.ranged && c.symbol == "":
				continue
			case file.tooLarge:
				finding.Reason = CitationFileTooLarge
				report.Abstained = append(report.Abstained, finding)
				continue
			case !file.blob:
				finding.Reason = CitationNotAFile
			case !c.ranged:
				if !containsSymbol(file.lines, c.symbol) {
					finding.Reason = CitationSymbolNotInRange
				}
			case c.start < 1 || c.end < c.start || c.end > len(file.lines):
				finding.Reason = CitationRangeOutOfBounds
			case c.symbol != "" && !containsSymbol(file.lines[c.start-1:c.end], c.symbol):
				finding.Reason = CitationSymbolNotInRange
			}
			if finding.Reason != "" {
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return report, nil
}

// fenceMarker returns the run of three or more backticks or tildes a line opens with, or "".
func fenceMarker(trimmed string) string {
	for _, c := range []string{"`", "~"} {
		n := len(trimmed) - len(strings.TrimLeft(trimmed, c))
		if n >= 3 {
			return trimmed[:n]
		}
	}
	return ""
}

// citedFile is one cited path at one commit.
type citedFile struct {
	exists, blob, tooLarge bool
	lines                  []string
}

// citedFile reads path at commit from the object store: whether it exists, whether it is a regular
// file, and its lines.
func (r Repo) citedFile(commit, path string) (citedFile, error) {
	out, err := r.git(1<<16, nil, "ls-tree", "-z", "-l", "--full-tree", commit, "--", path)
	if err != nil {
		return citedFile{}, err
	}
	meta, listed, ok := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	if !ok || listed != path {
		return citedFile{}, nil
	}
	fields := strings.Fields(meta)
	if len(fields) != 4 || fields[1] != "blob" || fields[0] == "120000" || fields[0] == "160000" {
		return citedFile{exists: true}, nil
	}
	size, err := strconv.Atoi(fields[3])
	if err != nil || size > MaxFileBytes {
		return citedFile{exists: true, blob: true, tooLarge: true}, nil
	}
	data, _, err := r.blobs([]entry{{path: path, oid: fields[2], size: size}}, maxTotalBytes)
	if err != nil {
		return citedFile{}, err
	}
	text := string(bytes.TrimSuffix(data[fields[2]], []byte("\n")))
	lines := []string{}
	if text != "" || size > 0 {
		lines = strings.Split(text, "\n")
	}
	return citedFile{exists: true, blob: true, lines: lines}, nil
}

// nearestRevision names the closest commit where path exists: the first commit after at (up to
// head) that adds it, or else the last commit before at that still had it. "" when unknown
// (TSA-V0-224).
func (r Repo) nearestRevision(at, head, path string) string {
	if at != head {
		if out, err := r.git(1<<20, nil, "log", "--reverse", "--format=%H", "--diff-filter=A", "--max-count=64", at+".."+head, "--", path); err == nil {
			if first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n"); first != "" {
				return first
			}
		}
	}
	out, err := r.git(4096, nil, "log", "-1", "--format=%H", "--diff-filter=D", at, "--", path)
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return ""
	}
	parent, err := r.Resolve(strings.TrimSpace(string(out)) + "^")
	if err != nil {
		return ""
	}
	return parent
}

// containsSymbol reports an identifier occurrence of symbol in lines.
func containsSymbol(lines []string, symbol string) bool {
	word := regexp.MustCompile(`(^|[^A-Za-z0-9_$])` + regexp.QuoteMeta(symbol) + `($|[^A-Za-z0-9_$])`)
	for _, line := range lines {
		if word.MatchString(line) {
			return true
		}
	}
	return false
}
