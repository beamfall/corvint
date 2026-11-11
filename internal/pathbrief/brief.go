package pathbrief

import (
	"cmp"
	"context"
	"errors"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Profile names the path brief document.
const Profile = "corvint-path-brief/0"

// SectionLimit is the most items one section lists for one path (TBC-V1-010).
const SectionLimit = 64

// Section states, as the change report uses them (CRI-V1-002).
const (
	StateObserved    = "OBSERVED"
	StateIncomplete  = "INCOMPLETE"
	StateNotObserved = "NOT_OBSERVED"
)

// Names of the optional sources a reason can report as missing (TBC-V1-011).
const (
	SourceOwners       = "CODEOWNERS"
	SourceFlows        = ".corvint/flows.json"
	SourcePlan         = "affected-plan"
	SourceEvidence     = "test-evidence"
	SourceTasks        = TasksExecutable
	SourceTicketCommit = "completed-ticket-commits"
)

// Notice is the standing statement of TBC-V1-007.
const Notice = "non-blocking; nothing here was shown to help an agent; a section lists links, it does not rank or judge them"

// ErrOutsideRepository refuses a path that does not lie inside the repository (TBC-V1-009).
var ErrOutsideRepository = errors.New("path-outside-repository")

// Brief is the document `corvint context --path` prints.
type Brief struct {
	Profile  string      `json:"profile"`
	Revision string      `json:"revision"`
	Blocking bool        `json:"blocking"`
	Notice   string      `json:"notice"`
	Paths    []PathBrief `json:"paths"`
}

// PathBrief holds the six sections of one `--path`.
type PathBrief struct {
	Path string `json:"path"`
	// Kind is file, directory or absent, read at the brief's revision.
	Kind string `json:"kind"`
	// Files is the number of tracked files the path stands for.
	Files    int       `json:"files"`
	Sections []Section `json:"sections"`
}

// Section is one section of a path's brief.
type Section struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	// Total is the number of items before the limit.
	Total     int  `json:"total"`
	Truncated bool `json:"truncated"`
	// MissingSources repeats the sources the reason names.
	MissingSources []string `json:"missingSources,omitempty"`
	// Scope is the affected plan's scope verdict (tests only).
	Scope string `json:"scope,omitempty"`
	// SharedReaders counts tests the plan selects for any change because they read the tree
	// without a bound. They are counted, not listed (tests only).
	SharedReaders int    `json:"sharedReaders,omitempty"`
	Items         []Item `json:"items"`
}

// Item is one listed link. Link names what produced it; the other fields depend on the section.
type Item struct {
	Link string `json:"link"`
	// Path is the instruction file, the CODEOWNERS file, or the linked file of a requirement.
	Path string `json:"path,omitempty"`

	Line    int      `json:"line,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
	Owners  []string `json:"owners,omitempty"`
	// Files is the number of the path's files the owners rule is the last match for.
	Files int `json:"files,omitempty"`

	Test string `json:"test,omitempty"`
	// Reason is the plan's reason for a test.
	Reason string `json:"reason,omitempty"`
	// Component and RecordCommit come from a retained test-evidence record.
	Component    string `json:"component,omitempty"`
	RecordCommit string `json:"recordCommit,omitempty"`

	Requirement string    `json:"requirement,omitempty"`
	Evidence    *Evidence `json:"evidence,omitempty"`
	// Entries is the number of link-file entries behind a requirement item.
	Entries int `json:"entries,omitempty"`
	// Commit is the record commit of the reported entry.
	Commit string `json:"commit,omitempty"`
	// ChangedSince says whether the linked file differs at the brief's revision from the file at
	// Commit.
	ChangedSince *bool `json:"changedSince,omitempty"`

	Ticket string `json:"ticket,omitempty"`
	Title  string `json:"title,omitempty"`
	// Field and Entry name the ticket field and the value in it that covers the path.
	Field string `json:"field,omitempty"`
	Entry string `json:"entry,omitempty"`
}

// Evidence is the pinned span a requirement link points at.
type Evidence struct {
	File    string `json:"file"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	BlobOid string `json:"blobOid"`
}

// PlannedTest is one test the affected plan ties to a path.
type PlannedTest struct {
	Test, Reason string
}

// EvidencedTest is one test the retained test evidence ties to a path.
type EvidencedTest struct {
	Test, Component, RecordCommit string
}

// TestsResult is what the two test sources say about the files of one path.
type TestsResult struct {
	// PlanMissing is empty when the affected plan was read, else why it was not.
	PlanMissing   string
	Scope         string
	Planned       []PlannedTest
	SharedReaders int
	// EvidenceMissing is empty when the retained test evidence was read, else why it was not.
	EvidenceMissing string
	Evidenced       []EvidencedTest
}

// Sources are the optional sources that live outside this package. A nil field is a missing
// source.
type Sources struct {
	// Tests answers for each group of files at the brief's revision, in order. It is called once.
	Tests func(ctx context.Context, revision string, groups [][]string) []TestsResult
	// Tickets returns every open ticket. It is called at most once.
	Tickets func(ctx context.Context) ([]Ticket, error)
}

// NormalizePath turns one `--path` value into a slash-separated path relative to root, "" for
// the repository itself. It refuses a path outside the repository.
func NormalizePath(root, value string) (string, error) {
	if value == "" || strings.ContainsAny(value, "\x00\n\r") {
		return "", ErrOutsideRepository
	}
	if filepath.IsAbs(value) {
		relative, err := filepath.Rel(root, value)
		if err != nil || escapes(relative) {
			resolved, resolveErr := filepath.EvalSymlinks(root)
			if resolveErr != nil {
				return "", ErrOutsideRepository
			}
			// The root or the value may be spelled through a link, as /tmp is on macOS.
			if relative, err = filepath.Rel(resolved, resolveExisting(value)); err != nil {
				return "", ErrOutsideRepository
			}
		}
		value = relative
	}
	cleaned := path.Clean(filepath.ToSlash(value))
	if escapes(cleaned) || path.IsAbs(cleaned) {
		return "", ErrOutsideRepository
	}
	if cleaned == "." {
		return "", nil
	}
	return cleaned, nil
}

// resolveExisting resolves the links of the longest part of an absolute path that exists and
// keeps the rest as written, so a path that is absent on disk still normalizes.
func resolveExisting(value string) string {
	rest := ""
	for current := filepath.Clean(value); ; {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return value
		}
		rest, current = filepath.Join(filepath.Base(current), rest), parent
	}
}

func escapes(relative string) bool {
	relative = filepath.ToSlash(relative)
	return relative == ".." || strings.HasPrefix(relative, "../")
}

// Build reads the committed tree at HEAD of the repository at root and returns the brief of the
// given paths, which NormalizePath produced. It writes nothing. Only a failing Git read is an
// error; every other missing source is reported in its section (TBC-V1-011).
func Build(ctx context.Context, root string, paths []string, sources Sources) (Brief, error) {
	r := repo{ctx: ctx, root: root}
	head, err := r.resolve("HEAD")
	if err != nil {
		return Brief{}, err
	}
	tree, err := r.tree(head)
	if err != nil {
		return Brief{}, err
	}
	b := builder{repo: r, tree: tree, sources: sources}
	b.names = make([]string, 0, len(tree))
	for name := range tree {
		b.names = append(b.names, name)
	}
	slices.Sort(b.names)
	b.readOwners()
	b.readLinks()

	brief := Brief{Profile: Profile, Revision: head, Notice: Notice, Paths: []PathBrief{}}
	subjects := make([]subject, len(paths))
	groups := make([][]string, len(paths))
	for index, name := range paths {
		subjects[index] = b.subject(name)
		groups[index] = subjects[index].files
	}
	var tests []TestsResult
	if sources.Tests != nil {
		// The ticket read and the tests source are independent and both slow, so they run together.
		ticketsRead := make(chan struct{})
		go func() {
			defer close(ticketsRead)
			if len(paths) != 0 {
				b.openTickets()
			}
		}()
		tests = sources.Tests(ctx, head, groups)
		<-ticketsRead
	}
	for index, s := range subjects {
		result := TestsResult{PlanMissing: "not-available", EvidenceMissing: "not-available"}
		if index < len(tests) {
			result = tests[index]
		}
		brief.Paths = append(brief.Paths, PathBrief{Path: s.display(), Kind: s.kind, Files: len(s.files), Sections: []Section{
			b.instructions(s), b.owners(s), b.tests(s, result), b.flows(s), b.requirements(s), b.tickets(s),
		}})
	}
	return brief, nil
}

type builder struct {
	repo    repo
	tree    map[string]string
	names   []string
	sources Sources

	ownersFile  string
	ownerRules  []ownerRule
	links       *LinkFile
	ticketsRead bool
	ticketList  []Ticket
	ticketsErr  error
}

// subject is one `--path` resolved against the tree.
type subject struct {
	path  string
	kind  string
	files []string
}

func (s subject) display() string {
	if s.path == "" {
		return "."
	}
	return s.path
}

func (b *builder) subject(name string) subject {
	if _, tracked := b.tree[name]; tracked {
		return subject{path: name, kind: "file", files: []string{name}}
	}
	prefix := name + "/"
	if name == "" {
		prefix = ""
	}
	from := sort.SearchStrings(b.names, prefix)
	to := from
	for to < len(b.names) && strings.HasPrefix(b.names[to], prefix) {
		to++
	}
	if to == from {
		return subject{path: name, kind: "absent"}
	}
	return subject{path: name, kind: "directory", files: b.names[from:to]}
}

func (b *builder) readOwners() {
	for _, name := range ownersFiles {
		oid, tracked := b.tree[name]
		if !tracked {
			continue
		}
		if data, ok := b.repo.blob(oid); ok {
			b.ownersFile, b.ownerRules = name, parseOwners(string(data))
		}
		return
	}
}

func (b *builder) readLinks() {
	oid, tracked := b.tree[LinksPath]
	if !tracked {
		return
	}
	if data, ok := b.repo.blob(oid); ok {
		b.links, _ = ParseLinks(data)
	}
}

func (b *builder) openTickets() ([]Ticket, error) {
	if !b.ticketsRead {
		b.ticketsRead, b.ticketsErr = true, errTasks
		if b.sources.Tickets != nil {
			b.ticketList, b.ticketsErr = b.sources.Tickets(b.repo.ctx)
		}
	}
	return b.ticketList, b.ticketsErr
}

// section applies the limit and writes the reason: the code, then each missing source.
func section(name, state, code string, missing []string, items []Item) Section {
	out := Section{Name: name, State: state, Reason: code, Total: len(items), Items: items, MissingSources: missing}
	if len(missing) != 0 {
		out.Reason = code + "; missing-source: " + strings.Join(missing, ", ")
	}
	if len(items) > SectionLimit {
		out.Items, out.Truncated = items[:SectionLimit], true
	}
	if out.Items == nil {
		out.Items = []Item{}
	}
	return out
}

// partial is the state of a section that lacks a source or a bound: what it has is listed as
// incomplete, and nothing is never listed as incomplete (TBC-V1-011).
func partial(items []Item) string {
	if len(items) == 0 {
		return StateNotObserved
	}
	return StateIncomplete
}

// ancestors lists the directory and each parent, the repository top ("") included.
func ancestors(directory string) []string {
	out := []string{""}
	for directory != "" && directory != "." {
		out = append(out, directory)
		directory = path.Dir(directory)
	}
	return out
}

func parent(name string) string {
	if directory := path.Dir(name); directory != "." {
		return directory
	}
	return ""
}

func (b *builder) instructions(s subject) Section {
	directories := map[string]bool{}
	switch s.kind {
	case "absent":
		for _, directory := range ancestors(parent(s.path)) {
			directories[directory] = true
		}
	default:
		for _, file := range s.files {
			for directory := parent(file); !directories[directory]; directory = parent(directory) {
				directories[directory] = true
				if directory == "" {
					break
				}
			}
		}
		directories[""] = true
	}
	ordered := make([]string, 0, len(directories))
	for directory := range directories {
		ordered = append(ordered, directory)
	}
	slices.Sort(ordered)
	items := []Item{}
	for _, directory := range ordered {
		for _, base := range []string{"AGENTS.md", "CLAUDE.md"} {
			name := path.Join(directory, base)
			if _, tracked := b.tree[name]; tracked {
				items = append(items, Item{Link: "directory-ancestry", Path: name})
			}
		}
	}
	if len(items) == 0 {
		return section("instructions", StateObserved, "no-instruction-file-on-ancestry", nil, items)
	}
	return section("instructions", StateObserved, "tree-at-revision", nil, items)
}

func (b *builder) owners(s subject) Section {
	if b.ownersFile == "" {
		return section("owners", StateNotObserved, "source-absent", []string{SourceOwners}, nil)
	}
	counts := map[int]int{}
	if s.kind == "absent" {
		if rule := ownerOf(b.ownerRules, parent(s.path)+"/"); rule >= 0 {
			counts[rule] = 0
		}
	}
	for _, file := range s.files {
		if rule := ownerOf(b.ownerRules, file); rule >= 0 {
			counts[rule]++
		}
	}
	items := []Item{}
	for index, rule := range b.ownerRules {
		if count, matched := counts[index]; matched {
			items = append(items, Item{Link: "codeowners-rule", Path: b.ownersFile, Line: rule.line, Pattern: rule.pattern, Owners: rule.owners, Files: count})
		}
	}
	if len(items) == 0 {
		return section("owners", StateObserved, "no-codeowners-rule-matches", nil, items)
	}
	return section("owners", StateObserved, "last-matching-rule", nil, items)
}

func (b *builder) tests(s subject, result TestsResult) Section {
	if s.kind == "absent" {
		return section("tests", StateNotObserved, "path-not-at-head", nil, nil)
	}
	var missing []string
	items := []Item{}
	if result.EvidenceMissing != "" {
		missing = append(missing, SourceEvidence+" ("+result.EvidenceMissing+")")
	}
	if result.PlanMissing != "" {
		missing = append(missing, SourcePlan+" ("+result.PlanMissing+")")
	}
	for _, test := range result.Planned {
		items = append(items, Item{Link: "affected-plan", Test: test.Test, Reason: test.Reason})
	}
	for _, test := range result.Evidenced {
		items = append(items, Item{Link: "test-evidence", Test: test.Test, Component: test.Component, RecordCommit: test.RecordCommit})
	}
	slices.SortFunc(items, func(a, b Item) int {
		return cmp.Or(cmp.Compare(a.Test, b.Test), cmp.Compare(a.Link, b.Link), cmp.Compare(a.RecordCommit, b.RecordCommit),
			cmp.Compare(a.Component, b.Component), cmp.Compare(a.Reason, b.Reason))
	})
	items = slices.CompactFunc(items, func(a, b Item) bool {
		return a.Test == b.Test && a.Link == b.Link && a.RecordCommit == b.RecordCommit && a.Component == b.Component && a.Reason == b.Reason
	})
	var out Section
	switch {
	case len(missing) == 2:
		out = section("tests", StateNotObserved, "source-absent", missing, nil)
	case result.PlanMissing == "" && result.Scope == "UNKNOWN":
		out = section("tests", partial(items), "plan-scope-unknown", missing, items)
	case len(missing) != 0:
		out = section("tests", partial(items), "one-source-read", missing, items)
	default:
		out = section("tests", StateObserved, "plan-and-evidence-read", nil, items)
	}
	if result.PlanMissing == "" {
		out.Scope, out.SharedReaders = result.Scope, result.SharedReaders
	}
	return out
}

// flows reports the flows file as a source. This build has no reader for it, and says so rather
// than showing an empty list (CRI-V1-002).
func (b *builder) flows(s subject) Section {
	if s.kind == "absent" {
		return section("flows", StateNotObserved, "path-not-at-head", nil, nil)
	}
	if _, tracked := b.tree[SourceFlows]; !tracked {
		return section("flows", StateNotObserved, "source-absent", []string{SourceFlows}, nil)
	}
	return section("flows", StateNotObserved, "flows-reader-not-built", nil, nil)
}

// linkGroup is the entries behind one requirement item and the entry it reports.
type linkGroup struct {
	requirement string
	entries     int
	best        Link
}

// newer reports whether a is reported in place of b: the later committer date, then the lower
// SHA, then a fixed order over the rest of the entry (TBC-V1-012).
func newer(a, b Link) bool {
	return cmp.Or(
		cmp.Compare(b.CommitterTime, a.CommitterTime),
		cmp.Compare(a.Commit, b.Commit),
		cmp.Compare(a.Path, b.Path),
		compareLinkEvidence(a.Evidence, b.Evidence),
	) < 0
}

func compareLinkEvidence(a, b LinkEvidence) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End), cmp.Compare(a.BlobOid, b.BlobOid))
}

// requirements lists the kept links of the path's files. It is never OBSERVED: the links cover
// only changes that were sealed (TBC-V1-010).
func (b *builder) requirements(s subject) Section {
	if s.kind == "absent" {
		return section("requirements", StateNotObserved, "path-not-at-head", nil, nil)
	}
	var missing []string
	if b.links == nil {
		missing = append(missing, LinksPath)
	}
	// Completed tickets are the second source. Tasks does not record their commits, so the
	// source is named as missing even when the store can be read.
	if _, err := b.openTickets(); err != nil {
		missing = append(missing, SourceTasks)
	}
	missing = append(missing, SourceTicketCommit)
	type spanKey struct {
		file       string
		start, end int
	}
	byRequirement := map[string]*linkGroup{}
	bySpan := map[spanKey]*linkGroup{}
	add := func(group *linkGroup, link Link) {
		if group.entries == 0 || newer(link, group.best) {
			group.best = link
		}
		group.entries++
	}
	if b.links != nil {
		for _, file := range s.files {
			for _, link := range b.links.ForPath(file) {
				if len(link.Evidence.Requirements) == 0 {
					key := spanKey{link.Evidence.File, link.Evidence.Start, link.Evidence.End}
					if bySpan[key] == nil {
						bySpan[key] = &linkGroup{}
					}
					add(bySpan[key], link)
					continue
				}
				for _, requirement := range link.Evidence.Requirements {
					if byRequirement[requirement] == nil {
						byRequirement[requirement] = &linkGroup{requirement: requirement}
					}
					add(byRequirement[requirement], link)
				}
			}
		}
	}
	groups := make([]*linkGroup, 0, len(byRequirement)+len(bySpan))
	for _, group := range byRequirement {
		groups = append(groups, group)
	}
	for _, group := range bySpan {
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b *linkGroup) int {
		if (a.requirement == "") != (b.requirement == "") {
			if a.requirement == "" {
				return 1
			}
			return -1
		}
		return cmp.Or(cmp.Compare(a.requirement, b.requirement), compareLinkEvidence(a.best.Evidence, b.best.Evidence))
	})
	items := make([]Item, 0, len(groups))
	for _, group := range groups {
		changed := b.tree[group.best.Path] != group.best.PathBlob
		items = append(items, Item{
			Link: "requirement-link", Requirement: group.requirement, Path: group.best.Path,
			Evidence: &Evidence{File: group.best.Evidence.File, Start: group.best.Evidence.Start, End: group.best.Evidence.End, BlobOid: group.best.Evidence.BlobOid},
			Entries:  group.entries, Commit: group.best.Commit, ChangedSince: &changed,
		})
	}
	if len(items) == 0 {
		return section("requirements", StateNotObserved, "no-link-for-path", missing, items)
	}
	return section("requirements", StateIncomplete, "links-not-exhaustive", missing, items)
}

// tickets lists the open tickets whose declared scope or touch paths cover the path.
func (b *builder) tickets(s subject) Section {
	tickets, err := b.openTickets()
	if err != nil {
		return section("tickets", StateNotObserved, "source-absent", []string{SourceTasks}, nil)
	}
	items := []Item{}
	for _, ticket := range tickets {
		field, entry := "declaredScope", covering(ticket.DeclaredScope, s)
		if entry == "" {
			field, entry = "record.effects.touchPaths", covering(ticket.TouchPaths, s)
		}
		if entry != "" {
			items = append(items, Item{Link: "ticket-scope", Ticket: ticket.ID, Title: ticket.Title, Field: field, Entry: entry})
		}
	}
	slices.SortFunc(items, func(a, b Item) int { return cmp.Compare(a.Ticket, b.Ticket) })
	if len(items) == 0 {
		return section("tickets", StateObserved, "no-open-ticket-covers-path", nil, items)
	}
	return section("tickets", StateObserved, "open-ticket-scope", nil, items)
}

// covering returns the first entry, in ascending order, that covers the subject, or "".
func covering(entries []string, s subject) string {
	found := ""
	for _, entry := range entries {
		if scopeCovers(entry, s.path, s.kind == "directory") && (found == "" || entry < found) {
			found = entry
		}
	}
	return found
}
