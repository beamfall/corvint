package console

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// maxCitingSpecs bounds how many spec files one code page reads to find the
// requirements whose Traceability rows cite its path.
const maxCitingSpecs = 64

// maxTraceabilityCitations bounds the requirement-to-path pairs one spec's
// Traceability tables may expand to. A `PREFIX-NNN..MMM` range multiplies
// with every path its row cites, so without a bound a few megabytes of
// committed text expand to gigabytes (LAC-V0-030).
const maxTraceabilityCitations = 100000

var errTraceabilityTooLarge = errors.New("its Traceability tables expand to more than " +
	strconv.Itoa(maxTraceabilityCitations) + " requirement citations, so no link is read from them")

// CodeLink is one path a requirement's Traceability row cites. ObjectID is
// the blob the path names at the pinned commit. Gap says why no link is
// rendered; a gap is never drawn as a link (LAC-V0-030).
type CodeLink struct {
	Path     string
	ObjectID string
	Gap      string
}

// RequirementLinks is the requirement page: the clause, and the code its
// owning spec's Traceability table cites, resolved at one commit.
type RequirementLinks struct {
	Clause *Clause
	Commit string
	Spec   *Blob
	Code   []CodeLink
	Gap    string
}

// RequirementLink is one requirement whose Traceability row cites a path.
type RequirementLink struct {
	ID   string
	Spec string
	Gap  string
}

// CodeBacklinks is the code page's list of citing requirements at one commit.
type CodeBacklinks struct {
	Commit       string
	Requirements []RequirementLink
	Gap          string
	Truncated    bool
}

var (
	requirementCitation = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*)-(\d{3})(?:\.\.(\d{3}))?\b`)
	backtickSpan        = regexp.MustCompile("`([^`\\s]+)`")
	headingLine         = regexp.MustCompile(`^(#{1,6})\s+(.+)`)
)

// traceabilityCitations reads every Traceability table in a spec. For each
// requirement id named in a row's first cell (a `PREFIX-NNN..MMM` range
// expands), it returns the path spans of that table's Implementation column.
// A table without an Implementation column cites no code. Tables expanding
// past maxTraceabilityCitations are an error, never a partial reading.
func traceabilityCitations(text string) (map[string][]string, error) {
	citations := map[string][]string{}
	active, level, column, used := false, 0, -1, 0
	for _, line := range strings.Split(text, "\n") {
		if match := headingLine.FindStringSubmatch(line); match != nil {
			active, level = nextTraceabilityState(active, level, len(match[1]), match[2])
			column = -1
			continue
		}
		cells, isRow := tableCells(line)
		if !active || !isRow {
			column = -1
			continue
		}
		if header := implementationColumn(cells); header >= 0 {
			column = header
			continue
		}
		if column < 0 || column >= len(cells) {
			continue
		}
		paths := citedPaths(cells[column])
		weight := max(1, len(paths))
		ids, ok := requirementIDs(cells[0], (maxTraceabilityCitations-used)/weight)
		if !ok {
			return nil, errTraceabilityTooLarge
		}
		used += len(ids) * weight
		for _, id := range ids {
			citations[id] = append(citations[id], paths...)
		}
	}
	return citations, nil
}

// nextTraceabilityState applies the same section rule as
// script/check-traceability-tests.sh: a heading naming traceability opens a
// section, and a heading at the same or a higher level closes it.
func nextTraceabilityState(active bool, level, nextLevel int, title string) (bool, int) {
	if active && nextLevel <= level {
		active = false
	}
	if strings.Contains(strings.ToLower(title), "traceability") {
		return true, nextLevel
	}
	return active, level
}

func tableCells(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil, false
	}
	parts := strings.Split(strings.Trim(trimmed, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, true
}

func implementationColumn(cells []string) int {
	for i, cell := range cells {
		if strings.EqualFold(cell, "implementation") {
			return i
		}
	}
	return -1
}

// requirementIDs expands a row's first cell, reporting false rather than
// expanding past limit ids.
func requirementIDs(cell string, limit int) ([]string, bool) {
	var ids []string
	for _, match := range requirementCitation.FindAllStringSubmatch(cell, -1) {
		first, _ := strconv.Atoi(match[2])
		last := first
		if match[3] != "" {
			last, _ = strconv.Atoi(match[3])
		}
		for number := first; number <= last && number-first < 1000; number++ {
			if len(ids) >= limit {
				return nil, false
			}
			ids = append(ids, match[1]+"-"+leftPad3(number))
		}
	}
	return ids, true
}

func leftPad3(number int) string {
	text := strconv.Itoa(number)
	return strings.Repeat("0", max(0, 3-len(text))) + text
}

// citedPaths takes every whitespace-free backtick span of an Implementation
// cell as a cited path. A span that names nothing at the commit, such as an
// identifier, resolves to a gap rather than being dropped.
func citedPaths(cell string) []string {
	var paths []string
	for _, match := range backtickSpan.FindAllStringSubmatch(cell, -1) {
		paths = append(paths, strings.TrimSuffix(match[1], "/"))
	}
	return paths
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// RequirementLinks resolves one requirement's cited code at commit. The
// owning spec is read at that commit's object, so the links are what that
// commit's Traceability table states, not what the worktree says now.
func (w Worktree) RequirementLinks(ctx context.Context, lookup SpecLookup, commit, id string) *RequirementLinks {
	links := &RequirementLinks{Commit: commit, Clause: lookup.Resolve(id)}
	if links.Clause.Err != "" {
		links.Gap = "the requirement could not be resolved, so no code is linked"
		return links
	}
	if commit == "" {
		links.Gap = "the repository has no commit to pin links to"
		return links
	}
	links.Spec = w.Read(ctx, commit, links.Clause.Requirement.File)
	if links.Spec.Err != "" {
		links.Gap = "the owning spec is not readable at commit " + commit + ": " + links.Spec.Err
		return links
	}
	citations, err := traceabilityCitations(links.Spec.Text)
	if err != nil {
		links.Gap = links.Clause.Requirement.File + " at commit " + commit + " is not read for links: " + err.Error()
		return links
	}
	paths := sortedUnique(citations[id])
	if len(paths) == 0 {
		links.Gap = "no Traceability row in " + links.Clause.Requirement.File + " at commit " + commit +
			" cites code for " + id + "; that is an absent link, not a passing one"
		return links
	}
	links.Code = w.resolvePaths(ctx, commit, paths)
	return links
}

// resolvePaths names each cited path's blob at commit with one `git ls-tree`.
// A path absent at that commit, a directory, or a path escaping the root is a
// gap rather than a link.
func (w Worktree) resolvePaths(ctx context.Context, commit string, paths []string) []CodeLink {
	links := make([]CodeLink, 0, len(paths))
	var valid []string
	for _, path := range paths {
		clean, err := cleanRelative(path)
		if err != nil || clean != path {
			links = append(links, CodeLink{Path: path, Gap: "not a contained repository path"})
			continue
		}
		valid = append(valid, path)
	}
	if len(valid) == 0 {
		return links
	}
	out, err := w.git(ctx, append([]string{"ls-tree", "-z", commit, "--"}, valid...)...)
	entries := map[string][]string{}
	for _, record := range strings.Split(out, "\x00") {
		head, path, found := strings.Cut(record, "\t")
		if fields := strings.Fields(head); found && len(fields) == 3 {
			entries[path] = fields
		}
	}
	for _, path := range valid {
		links = append(links, codeLinkOf(path, commit, entries[path], err))
	}
	sort.Slice(links, func(i, j int) bool { return links[i].Path < links[j].Path })
	return links
}

func codeLinkOf(path, commit string, entry []string, readErr error) CodeLink {
	switch {
	case readErr != nil:
		return CodeLink{Path: path, Gap: "the tree at commit " + commit + " could not be read: " + readErr.Error()}
	case entry == nil:
		return CodeLink{Path: path, Gap: "not present at commit " + commit}
	case entry[1] != "blob":
		return CodeLink{Path: path, Gap: "names a " + entry[1] + " at commit " + commit + ", not a file the code page can render"}
	}
	return CodeLink{Path: path, ObjectID: entry[2]}
}

// CodeBacklinks finds the requirements whose Traceability rows cite path at
// commit. Specs are located with one `git grep` over the commit's tree and
// then parsed; a mention outside a Traceability Implementation cell is not a
// citation. An id no longer in REQUIREMENTS.tsv is a gap, never a link to a
// page that could not resolve it.
func (w Worktree) CodeBacklinks(ctx context.Context, lookup SpecLookup, commit, path string) *CodeBacklinks {
	backlinks := &CodeBacklinks{Commit: commit}
	if commit == "" {
		backlinks.Gap = "the repository has no commit to pin links to"
		return backlinks
	}
	specs, err := w.citingSpecs(ctx, commit, path)
	if err != nil {
		backlinks.Gap = err.Error()
		return backlinks
	}
	if len(specs) > maxCitingSpecs {
		specs, backlinks.Truncated = specs[:maxCitingSpecs], true
	}
	for _, spec := range specs {
		backlinks.Requirements = append(backlinks.Requirements, w.specCitations(ctx, commit, spec, path)...)
	}
	if len(backlinks.Requirements) == 0 {
		backlinks.Gap = "no Traceability row at commit " + commit + " cites " + path + "; that is an absent link, not a passing one"
		return backlinks
	}
	markUnresolvable(backlinks.Requirements, lookup)
	sort.Slice(backlinks.Requirements, func(i, j int) bool {
		a, b := backlinks.Requirements[i], backlinks.Requirements[j]
		return a.ID < b.ID || (a.ID == b.ID && a.Spec < b.Spec)
	})
	return backlinks
}

func (w Worktree) citingSpecs(ctx context.Context, commit, path string) ([]string, error) {
	out, err := w.git(ctx, "grep", "-l", "-z", "-F", "-e", "`"+path+"`", commit, "--", "docs/specs")
	var exit *ExitError
	if errors.As(err, &exit) && exit.Status == 1 {
		return nil, errors.New("no spec at commit " + commit + " cites " + path + "; that is an absent link, not a passing one")
	}
	if err != nil {
		return nil, errors.New("the specs at commit " + commit + " could not be searched: " + err.Error())
	}
	var specs []string
	for _, name := range strings.Split(out, "\x00") {
		if spec := strings.TrimPrefix(name, commit+":"); spec != "" && spec != name {
			specs = append(specs, spec)
		}
	}
	sort.Strings(specs)
	return specs, nil
}

func (w Worktree) specCitations(ctx context.Context, commit, spec, path string) []RequirementLink {
	blob := w.Read(ctx, commit, spec)
	if blob.Err != "" {
		return []RequirementLink{{Spec: spec, Gap: "not readable at commit " + commit + ": " + blob.Err}}
	}
	citations, err := traceabilityCitations(blob.Text)
	if err != nil {
		return []RequirementLink{{Spec: spec, Gap: "not read at commit " + commit + ": " + err.Error()}}
	}
	var links []RequirementLink
	for id, paths := range citations {
		if containsString(paths, path) {
			links = append(links, RequirementLink{ID: id, Spec: spec})
		}
	}
	return links
}

func markUnresolvable(links []RequirementLink, lookup SpecLookup) {
	known, err := lookup.requirements()
	for i := range links {
		switch {
		case links[i].Gap != "":
		case err != nil:
			links[i].Gap = "docs/specs/REQUIREMENTS.tsv is not readable: " + err.Error()
		case known[links[i].ID].ID == "":
			links[i].Gap = "cited by a Traceability row but absent from docs/specs/REQUIREMENTS.tsv, so there is no page to link"
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
