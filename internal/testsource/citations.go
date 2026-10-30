package testsource

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
)

// Revision-pinned source citations (TSA-V0-250..256; V1-1934 / GitHub #1195). A review document
// cites committed source as `path:start-end@rev` (or in a configured syntax, TSA-V0-380..387); the
// check reads the cited path, range and symbol from the object store at the cited revision. It
// never checks out, writes or uses the network. Documents are read in bounded chunks and cited
// files through a bounded cache, so memory does not grow with the corpus (TSA-V0-384).

// CitationsSchema is the citation report's closed profile (TSA-V0-386).
const CitationsSchema = "corvint-source-citations/1"

// Citation finding reasons (TSA-V0-253).
const (
	CitationMissingAtRevision  = "missing-at-revision"
	CitationRangeOutOfBounds   = "range-out-of-bounds"
	CitationSymbolNotInRange   = "symbol-not-in-range"
	CitationRevisionUnresolved = "revision-unresolved"
	CitationNotAFile           = "not-a-file"
	CitationFileTooLarge       = "file-too-large"
)

const (
	// MaxCitationDocuments bounds the documents one report reads (TSA-V0-384).
	MaxCitationDocuments = 65536
	// MaxCitationsPerReport bounds the citation candidates one report reads (TSA-V0-384).
	MaxCitationsPerReport = 262144
	// citationChunkDocs and citationChunkBytes bound the documents held at once.
	citationChunkDocs  = 256
	citationChunkBytes = 16 << 20
	// citationCacheBytes bounds the cited file contents held at once; citationLoadBytes one load.
	citationCacheBytes = 64 << 20
	citationLoadBytes  = 16 << 20
	// citationCacheEntries bounds the revision and path metadata memos.
	citationCacheEntries = 65536
	// citationSuggestionLookups bounds the history lookups that suggest a nearest revision.
	citationSuggestionLookups = 256
	// citationPathBatch bounds the paths one `ls-tree` names.
	citationPathBatch = 512
)

// CitationOptions names the documents to check, read at Revision (default HEAD), and the syntax
// their citations are written in (TSA-V0-380..383).
type CitationOptions struct {
	Revision string
	// Docs are named documents; Manifests are committed files listing documents; Globs match
	// committed paths.
	Docs, Manifests, Globs []string
	// Syntax names a profile (inline-code, html-sub); Pattern is a caller pattern with named
	// groups and Marker an optional candidate pattern for it.
	Syntax, Pattern, Marker string
	// Repository is the name a citation's repo group must carry to be checked.
	Repository string
}

// CitationReport is one `corvint tests citations` document.
type CitationReport struct {
	Schema      string                  `json:"schema"`
	Revision    string                  `json:"revision"`
	Syntax      string                  `json:"syntax"`
	Documents   int                     `json:"documents"`
	Checked     int                     `json:"checked"`
	Findings    []CitationFinding       `json:"findings"`
	Abstained   []CitationFinding       `json:"abstained"`
	Unparseable []CitationFinding       `json:"unparseable"`
	Totals      []CitationDocumentTotal `json:"totals"`
}

// CitationDocumentTotal counts one document's citations and outcomes (TSA-V0-385).
type CitationDocumentTotal struct {
	Doc         string `json:"doc"`
	Citations   int    `json:"citations"`
	Findings    int    `json:"findings"`
	Abstained   int    `json:"abstained"`
	Unparseable int    `json:"unparseable"`
}

// CitationFinding is one citation the object store does not support, one the check could not
// decide (Abstained), or one the syntax could not read (Unparseable).
type CitationFinding struct {
	Doc        string `json:"doc"`
	Line       int    `json:"line"`
	Citation   string `json:"citation"`
	Revision   string `json:"revision"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion,omitempty"`
}

// citationDefault is a document's default citation revision (TSA-V0-251).
var citationDefault = regexp.MustCompile(`^\s*<!--\s*corvint-cite-revision:\s*([A-Za-z0-9_.\-/^~]+)\s*-->\s*$`)

// citation is one parsed citation.
type citation struct {
	line         int
	text         string
	path, symbol string
	start, end   int
	ranged       bool
	revision     string
}

// Citations checks every citation of the selected committed documents against the object store
// (TSA-V0-252..256, TSA-V0-380..387). Documents are read at options.Revision; each citation at its
// own revision.
func Citations(r Repo, options CitationOptions) (CitationReport, error) {
	syntax, err := compileCitationSyntax(options)
	if err != nil {
		return CitationReport{}, err
	}
	for _, name := range append(append([]string{}, options.Docs...), options.Manifests...) {
		if !citationPathValid(name) {
			return CitationReport{}, refuse("tests-invalid-arguments", "%q is not a repository-relative file path", name)
		}
	}
	if len(options.Manifests)+len(options.Globs) > maxCitationGlobs {
		return CitationReport{}, refuse("tests-bound-exceeded", "more than %d manifests and globs", maxCitationGlobs)
	}
	globs := []citationGlob{}
	for _, pattern := range options.Globs {
		g, err := compileCitationGlob(pattern)
		if err != nil {
			return CitationReport{}, err
		}
		globs = append(globs, g)
	}
	if len(options.Docs)+len(options.Manifests)+len(globs) == 0 {
		return CitationReport{}, refuse("tests-invalid-arguments", "no document was named")
	}
	if strings.ContainsAny(options.Repository, "\x00\n\r") || len(options.Repository) > maxCitationPatternBytes {
		return CitationReport{}, refuse("tests-invalid-arguments", "the repository name is invalid")
	}
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return CitationReport{}, err
	}
	docs, err := r.citationDocuments(commit, options.Docs, options.Manifests, globs)
	if err != nil {
		return CitationReport{}, err
	}
	if len(docs) == 0 {
		return CitationReport{}, refuse("tests-invalid-arguments", "no document was named or matched")
	}
	report := CitationReport{Schema: CitationsSchema, Revision: commit, Syntax: syntax.name, Documents: len(docs), Findings: []CitationFinding{},
		Abstained: []CitationFinding{}, Unparseable: []CitationFinding{}, Totals: make([]CitationDocumentTotal, 0, len(docs))}
	check := newCitationCheck(r, commit, options.Repository)
	read := 0
	for start := 0; start < len(docs); {
		end, size := start, 0
		for end < len(docs) && end-start < citationChunkDocs && (end == start || size+docs[end].size <= citationChunkBytes) {
			size += docs[end].size
			end++
		}
		data, _, err := r.blobs(docs[start:end], maxTotalBytes)
		if err != nil {
			return CitationReport{}, err
		}
		type pending struct {
			total int
			c     candidate
		}
		chunk := []pending{}
		for _, doc := range docs[start:end] {
			report.Totals = append(report.Totals, CitationDocumentTotal{Doc: doc.path})
			for _, c := range syntax.parse(string(data[doc.oid])) {
				if read++; read > MaxCitationsPerReport {
					return CitationReport{}, refuse("tests-bound-exceeded", "more than %d citations", MaxCitationsPerReport)
				}
				chunk = append(chunk, pending{len(report.Totals) - 1, c})
			}
		}
		cands := make([]candidate, 0, len(chunk))
		for _, p := range chunk {
			cands = append(cands, p.c)
		}
		if err := check.prepare(cands); err != nil {
			return CitationReport{}, err
		}
		for _, p := range chunk {
			total := &report.Totals[p.total]
			if p.c.unparseable {
				total.Unparseable++
				report.Unparseable = append(report.Unparseable, CitationFinding{Doc: total.Doc, Line: p.c.line, Citation: p.c.text, Reason: CitationUnparseable})
				continue
			}
			total.Citations++
			report.Checked++
			finding, abstained, err := check.evaluate(total.Doc, p.c)
			switch {
			case err != nil:
				return CitationReport{}, err
			case finding.Reason == "":
			case abstained:
				total.Abstained++
				report.Abstained = append(report.Abstained, finding)
			default:
				total.Findings++
				report.Findings = append(report.Findings, finding)
			}
		}
		start = end
	}
	return report, nil
}

// citationDocuments lists the selected documents at commit, each once in selection order: named
// documents, then manifest entries, then glob matches in tree order (TSA-V0-383). A named or listed
// document must be a committed regular file; a glob keeps the regular files it matches.
func (r Repo) citationDocuments(commit string, docs, manifests []string, globs []citationGlob) ([]entry, error) {
	names, seen := []string{}, map[string]bool{}
	add := func(name string) error {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
			if len(names) > MaxCitationDocuments {
				return refuse("tests-bound-exceeded", "more than %d documents", MaxCitationDocuments)
			}
		}
		return nil
	}
	for _, name := range docs {
		if err := add(name); err != nil {
			return nil, err
		}
	}
	for _, manifest := range manifests {
		data, err := r.committedText(commit, manifest, maxCitationManifestBytes)
		if err != nil {
			return nil, err
		}
		listed, err := manifestPaths(manifest, data)
		if err != nil {
			return nil, err
		}
		for _, name := range listed {
			if err := add(name); err != nil {
				return nil, err
			}
		}
	}
	out := make([]entry, 0, len(names))
	for start := 0; start < len(names); start += citationPathBatch {
		batch := names[start:min(start+citationPathBatch, len(names))]
		listed, err := r.listPaths(commit, batch)
		if err != nil {
			return nil, err
		}
		for _, name := range batch {
			meta, ok := listed[name]
			if !ok {
				return nil, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
			}
			if !meta.regular {
				return nil, refuse("tests-source-unavailable", "%s is not a regular committed file", name)
			}
			out = append(out, entry{path: name, oid: meta.oid, size: meta.size})
		}
	}
	for _, g := range globs {
		args := []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "-l", "--full-tree", commit}
		if g.prefix != "" {
			args = append(args, "--", g.prefix)
		}
		listing, err := r.git(maxDiffBytes, nil, args...)
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
			meta, ok := parseTreeLine(string(line))
			if !ok || !meta.regular || seen[meta.path] || !g.match(meta.path) {
				continue
			}
			seen[meta.path] = true
			out = append(out, entry{path: meta.path, oid: meta.oid, size: meta.size})
			if len(out) > MaxCitationDocuments {
				return nil, refuse("tests-bound-exceeded", "more than %d documents", MaxCitationDocuments)
			}
		}
	}
	return out, nil
}

// treeLine is one `ls-tree -l` entry.
type treeLine struct {
	path, mode, kind, oid string
	size                  int
	regular               bool
}

func parseTreeLine(line string) (treeLine, bool) {
	meta, name, ok := strings.Cut(line, "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 4 || name == "" {
		return treeLine{}, false
	}
	t := treeLine{path: name, mode: fields[0], kind: fields[1], oid: fields[2]}
	t.size, _ = strconv.Atoi(fields[3])
	t.regular = t.kind == "blob" && (t.mode == "100644" || t.mode == "100755")
	return t, true
}

// listPaths reads the tree entries of the named paths at commit through one literal `ls-tree`.
func (r Repo) listPaths(commit string, names []string) (map[string]treeLine, error) {
	limit := 4096
	for _, name := range names {
		limit += len(name) + 128
	}
	out, err := r.git(limit, nil, append([]string{"--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", commit, "--"}, names...)...)
	if err != nil {
		return nil, err
	}
	listed := map[string]treeLine{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if t, ok := parseTreeLine(string(line)); ok {
			listed[t.path] = t
		}
	}
	return listed, nil
}

// committedText reads one committed regular file of at most limit bytes.
func (r Repo) committedText(commit, name string, limit int) ([]byte, error) {
	listed, err := r.listPaths(commit, []string{name})
	if err != nil {
		return nil, err
	}
	t, ok := listed[name]
	if !ok || !t.regular {
		return nil, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
	}
	if t.size > limit {
		return nil, refuse("tests-bound-exceeded", "%s exceeds %d bytes", name, limit)
	}
	return r.git(t.size+1024, nil, "cat-file", "blob", t.oid)
}

// citedKey is one cited path at one commit.
type citedKey struct{ commit, path string }

// citationCheck resolves revisions and reads cited files for a run, holding bounded memos.
type citationCheck struct {
	r                  Repo
	commit, repository string
	resolved           map[string]string
	meta               map[citedKey]treeLine
	present            map[citedKey]bool
	lines              map[citedKey][]string
	cached             int
	order              []citedKey
	position           map[citedKey]int
	suggestions        map[citedKey]string
}

func newCitationCheck(r Repo, commit, repository string) *citationCheck {
	return &citationCheck{r: r, commit: commit, repository: repository, resolved: map[string]string{commit: commit},
		meta: map[citedKey]treeLine{}, present: map[citedKey]bool{}, lines: map[citedKey][]string{}, suggestions: map[citedKey]string{}}
}

// prepare resolves a chunk's revisions through one `cat-file --batch-check` and lists its cited
// paths through one literal `ls-tree` per commit and batch.
func (k *citationCheck) prepare(cands []candidate) error {
	if len(k.resolved) > citationCacheEntries {
		k.resolved = map[string]string{k.commit: k.commit}
	}
	if len(k.present) > citationCacheEntries {
		k.meta, k.present = map[citedKey]treeLine{}, map[citedKey]bool{}
	}
	revs, request := []string{}, &bytes.Buffer{}
	for _, c := range cands {
		if _, ok := k.resolved[c.revision]; !ok && !c.unparseable && c.revision != "" {
			k.resolved[c.revision] = ""
			revs = append(revs, c.revision)
			request.WriteString(c.revision + "^{commit}\n")
		}
	}
	if len(revs) > 0 {
		out, err := k.r.git(len(revs)*128+request.Len()+4096, request.Bytes(), "cat-file", "--batch-check")
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		for i, rev := range revs {
			if i < len(lines) {
				if fields := strings.Fields(lines[i]); len(fields) == 3 && fields[1] == "commit" {
					k.resolved[rev] = fields[0]
				}
			}
		}
	}
	k.order, k.position = []citedKey{}, map[citedKey]int{}
	byCommit := map[string][]string{}
	commits := []string{}
	for _, c := range cands {
		rev := c.revision
		if rev == "" {
			rev = k.commit
		}
		at := k.resolved[rev]
		if c.unparseable || at == "" || c.repo != "" && c.repo != k.repository {
			continue
		}
		key := citedKey{at, c.path}
		if _, ok := k.position[key]; !ok {
			k.position[key] = len(k.order)
			k.order = append(k.order, key)
		}
		if _, ok := k.present[key]; !ok {
			k.present[key] = false
			if len(byCommit[at]) == 0 {
				commits = append(commits, at)
			}
			byCommit[at] = append(byCommit[at], c.path)
		}
	}
	for _, at := range commits {
		paths := byCommit[at]
		for start := 0; start < len(paths); start += citationPathBatch {
			listed, err := k.r.listPaths(at, paths[start:min(start+citationPathBatch, len(paths))])
			if err != nil {
				return err
			}
			for name, t := range listed {
				key := citedKey{at, name}
				if _, asked := k.present[key]; asked {
					k.meta[key], k.present[key] = t, true
				}
			}
		}
	}
	return nil
}

// fileLines returns a cited regular file's lines, loading it with the not-yet-loaded files cited
// after it in this chunk, up to citationLoadBytes, within citationCacheBytes (TSA-V0-384).
func (k *citationCheck) fileLines(key citedKey) ([]string, error) {
	if lines, ok := k.lines[key]; ok {
		return lines, nil
	}
	batch, size := []entry{}, 0
	keys := []citedKey{}
	for i := k.position[key]; i < len(k.order); i++ {
		next := k.order[i]
		t := k.meta[next]
		if _, loaded := k.lines[next]; loaded || !k.present[next] || !t.regular || t.size > MaxFileBytes {
			continue
		}
		if len(batch) > 0 && size+t.size > citationLoadBytes {
			break
		}
		batch, keys, size = append(batch, entry{path: next.path, oid: t.oid, size: t.size}), append(keys, next), size+t.size
	}
	if k.cached+size > citationCacheBytes {
		k.lines, k.cached = map[citedKey][]string{}, 0
	}
	data, _, err := k.r.blobs(batch, maxTotalBytes)
	if err != nil {
		return nil, err
	}
	for i, next := range keys {
		text := string(bytes.TrimSuffix(data[batch[i].oid], []byte("\n")))
		lines := []string{}
		if text != "" || batch[i].size > 0 {
			lines = strings.Split(text, "\n")
		}
		k.lines[next] = lines
		k.cached += batch[i].size
	}
	return k.lines[key], nil
}

// evaluate checks one parsed citation. It returns a finding with no reason when the citation
// holds, and whether a reason is an abstention (TSA-V0-253..255, TSA-V0-381).
func (k *citationCheck) evaluate(doc string, c candidate) (CitationFinding, bool, error) {
	finding := CitationFinding{Doc: doc, Line: c.line, Citation: c.text, Revision: c.revision}
	if c.repo != "" && c.repo != k.repository {
		finding.Reason = CitationRepositoryNotChecked
		return finding, true, nil
	}
	rev := c.revision
	if rev == "" {
		rev, finding.Revision = k.commit, k.commit
	}
	at := k.resolved[rev]
	if at == "" {
		finding.Reason = CitationRevisionUnresolved
		return finding, true, nil
	}
	finding.Revision = at
	key := citedKey{at, c.path}
	t := k.meta[key]
	blob := t.kind == "blob" && t.mode != "120000" && t.mode != "160000"
	switch {
	case !k.present[key]:
		finding.Reason = CitationMissingAtRevision
		finding.Suggestion = k.suggest(key)
	case !c.ranged && c.symbol == "":
	case blob && t.size > MaxFileBytes:
		finding.Reason = CitationFileTooLarge
		return finding, true, nil
	case !blob:
		finding.Reason = CitationNotAFile
	default:
		lines, err := k.fileLines(key)
		if err != nil {
			return CitationFinding{}, false, err
		}
		switch {
		case !c.ranged:
			if !containsSymbol(lines, c.symbol) {
				finding.Reason = CitationSymbolNotInRange
			}
		case c.start < 1 || c.end < c.start || c.end > len(lines):
			finding.Reason = CitationRangeOutOfBounds
		case c.symbol != "" && !containsSymbol(lines[c.start-1:c.end], c.symbol):
			finding.Reason = CitationSymbolNotInRange
		}
	}
	return finding, false, nil
}

// suggest names the nearest revision for a missing path, memoized, for at most
// citationSuggestionLookups distinct paths per run (TSA-V0-254, TSA-V0-384).
func (k *citationCheck) suggest(key citedKey) string {
	if s, ok := k.suggestions[key]; ok {
		return s
	}
	if len(k.suggestions) >= citationSuggestionLookups {
		return ""
	}
	s := k.r.nearestRevision(key.commit, k.commit, key.path)
	k.suggestions[key] = s
	return s
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

// nearestRevision names the closest commit where path exists: the first commit after at (up to
// head) that adds it, or else the last commit before at that still had it. "" when unknown
// (TSA-V0-254).
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
