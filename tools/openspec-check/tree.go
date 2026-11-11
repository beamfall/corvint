// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	// requirementID is a requirement ID token; ticketID tokens (V1-NNNN) are excluded separately.
	requirementID = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]{3,}\b`)
	ticketID      = regexp.MustCompile(`^V[0-9]+-[0-9]+$`)
	idRange       = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*)-([0-9]{3,})\s+to\s+(?:([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*)-)?([0-9]{3,})\b`)
	// prefixRange is the reservation form `PREFIX` NNN..NNN (also `NNN to NNN`, `NNN-NNN`, in
	// parentheses), which names every ID of the range.
	prefixRange  = regexp.MustCompile("`?\\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)`?\\s+\\(?([0-9]{3,})(?:\\.\\.|\\s+to\\s+|-)([0-9]{3,})\\b")
	allIDs       = regexp.MustCompile(`(?i)\bRequirements:\s*all\b`)
	proofMarker  = regexp.MustCompile(`(?i)\bproof:`)
	commitName   = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	taskLine     = regexp.MustCompile(`^- \[([ xX])\] ([0-9]+(?:\.[0-9]+)*[a-z]?)\s+\S`)
	changeName   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	archivedName = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})-([a-z0-9]+(?:-[a-z0-9]+)*)$`)
	externalRepo = regexp.MustCompile("(?i)`?beamfall/([a-z0-9][a-z0-9._-]*)`?")
	headerLine   = regexp.MustCompile(`^[A-Z][A-Za-z0-9 ()-]*: `)
	schemaLine   = regexp.MustCompile(`^schema:\s*['"]?([A-Za-z0-9_-]+)['"]?\s*(?:#.*)?$`)
	retiredWord  = regexp.MustCompile(`(?i)\bretired\b`)
	reasonWord   = regexp.MustCompile(`(?i)\b(reason|retired)\b`)
)

var (
	proposalHeadings = []string{"Why", "What Changes", "Impact", "Non-goals"}
	designHeadings   = []string{"Decisions", "Failure modes", "Acceptance evidence", "Rollback"}
	healthClasses    = map[string]bool{"KEEP": true, "MERGE": true, "RETIRE": true, "OUT-OF-1.0": true}
)

type requirementRow struct {
	file       string
	title      string
	acceptance string
}

type task struct {
	line    int
	number  string
	ticked  bool
	text    string
	ids     []string
	all     bool
	proof   bool
	commits []string
}

type deltaRequirement struct {
	id       string
	kind     string // ADDED, MODIFIED or REMOVED
	file     string
	line     int
	scenario bool
	reason   bool
}

type change struct {
	name       string
	dir        string // repository-relative directory
	archived   bool
	date       string // archive date, archived folders only
	files      map[string]bool
	headings   map[string]map[string]bool // file -> headings
	intent     string
	delivery   string
	tickets    []string
	external   string // the other repository a proposal's owning spec lives in, or ""
	tasks      []task
	malformed  []string
	deltas     []deltaRequirement
	deltaFiles []string
	badDeltas  []string
}

type repo struct {
	root         string
	requirements map[string]requirementRow
	specDelivery map[string]string
	active       []*change
	archived     []*change
	badNames     []finding
	acceptance   map[string]string // change name -> mode, from the acceptance log
	now          time.Time
}

func loadRepo(root string) (*repo, error) {
	r := &repo{root: root, acceptance: map[string]string{}, specDelivery: map[string]string{}, now: time.Now().UTC()}
	rows, err := readRequirements(filepath.Join(root, requirementsTSV))
	if err != nil {
		return nil, err
	}
	r.requirements = rows
	if specs, err := filepath.Glob(filepath.Join(root, "docs/specs/*.md")); err == nil {
		for _, path := range specs {
			header, _ := specHeader(path)
			rel := "docs/specs/" + filepath.Base(path)
			r.specDelivery[rel] = header["Delivery status"]
		}
	}
	if log, err := os.ReadFile(filepath.Join(root, acceptanceLog)); err == nil {
		for i, line := range strings.Split(string(log), "\n") {
			fields := strings.Split(line, "\t")
			if i == 0 || len(fields) < 3 || fields[1] != "change" {
				continue
			}
			r.acceptance[fields[0]] = fields[2]
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, changesDir))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "archive" {
			continue
		}
		if !changeName.MatchString(name) {
			r.badNames = append(r.badNames, finding{"OSC-V1-006", subjectChange(name),
				"a change name is lowercase words joined by hyphens"})
		}
		c, err := loadChange(root, changesDir+"/"+name, name, false)
		if err != nil {
			return nil, err
		}
		r.active = append(r.active, c)
	}
	archive, err := os.ReadDir(filepath.Join(root, changesDir, "archive"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range archive {
		if !entry.IsDir() {
			continue
		}
		m := archivedName.FindStringSubmatch(entry.Name())
		if m == nil {
			r.badNames = append(r.badNames, finding{"OSC-V1-006", subjectChange(entry.Name()),
				"an archived folder is named archive/YYYY-MM-DD-<name>/, as openspec archive writes it"})
			continue
		}
		c, err := loadChange(root, changesDir+"/archive/"+entry.Name(), m[2], true)
		if err != nil {
			return nil, err
		}
		c.date = m[1]
		r.archived = append(r.archived, c)
	}
	return r, nil
}

func readRequirements(path string) (map[string]requirementRow, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]requirementRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseRequirements(string(data)), nil
}

// parseRequirements reads the id, file, title and acceptance columns of REQUIREMENTS.tsv.
func parseRequirements(text string) map[string]requirementRow {
	rows := map[string]requirementRow{}
	for i, line := range strings.Split(text, "\n") {
		fields := strings.Split(line, "\t")
		if i == 0 || len(fields) < 2 || fields[0] == "" {
			continue
		}
		row := requirementRow{file: fields[1]}
		if len(fields) > 2 {
			row.title = fields[2]
		}
		if len(fields) > 3 {
			row.acceptance = fields[3]
		}
		rows[fields[0]] = row
	}
	return rows
}

// specHeader returns the `Key: value` lines before the first `## ` heading of a spec.
func specHeader(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	header := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			break
		}
		if key, value, ok := strings.Cut(line, ":"); ok && headerLine.MatchString(line) {
			if _, seen := header[key]; !seen {
				header[key] = strings.TrimSpace(value)
			}
		}
	}
	return header, scanner.Err()
}

func loadChange(root, dir, name string, archived bool) (*change, error) {
	c := &change{name: name, dir: dir, archived: archived, files: map[string]bool{},
		headings: map[string]map[string]bool{}}
	for _, file := range []string{"proposal.md", "design.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(root, dir, file))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		c.files[file] = true
		text := string(data)
		c.headings[file] = headings(text)
		switch file {
		case "proposal.md":
			c.readProposal(text)
		case "tasks.md":
			c.readTasks(text)
		}
	}
	specs, err := filepath.Glob(filepath.Join(root, dir, "specs", "*", "spec.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(specs)
	for _, path := range specs {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(filepath.Join(root, dir))+"/")
		c.deltaFiles = append(c.deltaFiles, rel)
		c.readDelta(rel, string(data))
	}
	return c, nil
}

// headings returns the text of every `## ` and `### ` heading outside fenced code.
func headings(text string) map[string]bool {
	found := map[string]bool{}
	fence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		for _, prefix := range []string{"## ", "### "} {
			if strings.HasPrefix(line, prefix) {
				found[strings.TrimSpace(strings.TrimPrefix(line, prefix))] = true
			}
		}
	}
	return found
}

func (c *change) readProposal(text string) {
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "## ") {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || !headerLine.MatchString(line) {
			continue
		}
		// A header value continues on following lines until a blank line or the next header line.
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if strings.TrimSpace(next) == "" || headerLine.MatchString(next) || strings.HasPrefix(next, "#") {
				break
			}
			value += " " + strings.TrimSpace(next)
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Intent status":
			if c.intent == "" {
				c.intent = value
			}
		case "Delivery status":
			if c.delivery == "" {
				c.delivery = value
			}
		case "Ticket":
			for _, token := range regexp.MustCompile(`\bV[0-9]+-[0-9]+\b`).FindAllString(value, -1) {
				c.tickets = append(c.tickets, token)
			}
		case "Owning capability spec":
			for _, m := range externalRepo.FindAllStringSubmatch(value, -1) {
				if !strings.EqualFold(m[1], "corvint") {
					c.external = "beamfall/" + strings.ToLower(m[1])
					break
				}
			}
		}
	}
}

func (c *change) readTasks(text string) {
	lines := strings.Split(text, "\n")
	var current *task
	fence := false
	flush := func() {
		if current != nil {
			current.ids, current.all = namedIDs(current.text)
			current.proof = proofMarker.MatchString(current.text)
			current.commits = commitNames(current.text)
			c.tasks = append(c.tasks, *current)
			current = nil
		}
	}
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			fence = !fence
			flush()
			continue
		}
		if fence {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "- [") {
			flush()
			m := taskLine.FindStringSubmatch(trimmed)
			if m == nil {
				c.malformed = append(c.malformed, fmt.Sprintf("tasks.md:%d: %q is not `- [ ] N text` with a task number", i+1, trimmed))
				continue
			}
			current = &task{line: i + 1, number: m[2], ticked: m[1] != " ", text: trimmed}
			continue
		}
		if current != nil && strings.TrimSpace(line) != "" && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			current.text += " " + strings.TrimSpace(line)
			continue
		}
		flush()
	}
	flush()
}

// namedIDs returns the requirement IDs a task text names, with ranges expanded, and whether it
// says `Requirements: all`.
func namedIDs(text string) ([]string, bool) {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if !seen[id] && !ticketID.MatchString(id) {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	expand := func(prefix, low, high string) {
		var lo, hi int
		fmt.Sscanf(low, "%d", &lo)
		fmt.Sscanf(high, "%d", &hi)
		if hi < lo || hi-lo > 999 {
			return
		}
		for n := lo; n <= hi; n++ {
			add(fmt.Sprintf("%s-%0*d", prefix, len(low), n))
		}
	}
	for _, m := range idRange.FindAllStringSubmatch(text, -1) {
		if m[3] == "" || m[3] == m[1] {
			expand(m[1], m[2], m[4])
		}
	}
	for _, m := range prefixRange.FindAllStringSubmatch(text, -1) {
		expand(m[1], m[2], m[3])
	}
	for _, id := range requirementID.FindAllString(text, -1) {
		add(id)
	}
	return ids, allIDs.MatchString(text)
}

func commitNames(text string) []string {
	var names []string
	for _, token := range commitName.FindAllString(text, -1) {
		if strings.ContainsAny(token, "abcdef") && strings.ContainsAny(token, "0123456789") {
			names = append(names, token)
		}
	}
	return names
}

func (c *change) readDelta(file, text string) {
	kind := ""
	var current *deltaRequirement
	fence := false
	flush := func() {
		if current != nil {
			c.deltas = append(c.deltas, *current)
			current = nil
		}
	}
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fence = !fence
		}
		if fence {
			continue
		}
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			kind = ""
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			for _, k := range []string{"ADDED", "MODIFIED", "REMOVED"} {
				if heading == k+" Requirements" {
					kind = k
				}
			}
		case strings.HasPrefix(line, "### Requirement:"):
			flush()
			if kind == "" {
				c.badDeltas = append(c.badDeltas, fmt.Sprintf("%s:%d: a requirement outside an ADDED, MODIFIED or REMOVED Requirements section", file, i+1))
				continue
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, "### Requirement:"))
			id, _, _ := strings.Cut(rest, " ")
			if !requirementID.MatchString(id) || requirementID.FindString(id) != id || ticketID.MatchString(id) {
				c.badDeltas = append(c.badDeltas, fmt.Sprintf("%s:%d: %q names no requirement ID; write `### Requirement: <ID> <title>`", file, i+1, line))
				continue
			}
			current = &deltaRequirement{id: id, kind: kind, file: file, line: i + 1}
		case strings.HasPrefix(line, "### "):
			flush()
		case current != nil && strings.HasPrefix(line, "#### Scenario:"):
			current.scenario = true
		case current != nil && reasonWord.MatchString(line):
			current.reason = true
		}
	}
	flush()
}

// deltaKinds maps each delta ID of a change to its kind.
func (c *change) deltaKinds() map[string]string {
	kinds := map[string]string{}
	for _, d := range c.deltas {
		kinds[d.id] = d.kind
	}
	return kinds
}

// layoutRules is OSC-V1-001 over every active folder, plus the name rules of OSC-V1-006.
func layoutRules(r *repo) []finding {
	var out []finding
	out = append(out, r.badNames...)
	seen := map[string]string{}
	for _, c := range append(append([]*change{}, r.active...), r.archived...) {
		if other, dup := seen[c.name]; dup {
			out = append(out, finding{"OSC-V1-006", subjectChange(c.name),
				fmt.Sprintf("the name is used by two folders, %s and %s", other, c.dir)})
		}
		seen[c.name] = c.dir
	}
	for _, c := range r.active {
		subject := subjectChange(c.name)
		fail := func(format string, args ...any) {
			out = append(out, finding{"OSC-V1-001", subject, fmt.Sprintf(format, args...)})
		}
		for file, want := range map[string][]string{"proposal.md": proposalHeadings, "design.md": designHeadings} {
			if !c.files[file] {
				fail("%s/%s is missing", c.dir, file)
				continue
			}
			for _, heading := range want {
				if !hasHeading(c.headings[file], heading) {
					fail("%s has no `## %s` heading", file, heading)
				}
			}
		}
		if c.files["proposal.md"] {
			if c.intent == "" {
				fail("proposal.md has no `Intent status:` header line")
			}
			if c.delivery == "" {
				fail("proposal.md has no `Delivery status:` header line")
			}
		}
		if !c.files["tasks.md"] {
			fail("%s/tasks.md is missing", c.dir)
		} else {
			for _, m := range c.malformed {
				fail("%s", m)
			}
			if len(c.tasks) == 0 && len(c.malformed) == 0 {
				fail("tasks.md has no `- [ ] N text` item")
			}
			for _, t := range c.tasks {
				if len(t.ids) == 0 && !t.all {
					fail("tasks.md:%d: task %s names no requirement ID (or `Requirements: all`)", t.line, t.number)
				}
				if !t.proof {
					fail("tasks.md:%d: task %s names no `Proof:`", t.line, t.number)
				}
			}
		}
		if len(c.deltaFiles) == 0 {
			fail("%s has no specs/<capability>/spec.md delta", c.dir)
		}
		for _, bad := range c.badDeltas {
			fail("%s", bad)
		}
		perFile := map[string]int{}
		for _, d := range c.deltas {
			perFile[d.file]++
			switch {
			case d.kind == "REMOVED" && !d.scenario && !d.reason:
				fail("%s:%d: REMOVED requirement %s has neither a `#### Scenario:` block nor a reason line", d.file, d.line, d.id)
			case d.kind != "REMOVED" && !d.scenario:
				fail("%s:%d: %s requirement %s has no `#### Scenario:` block", d.file, d.line, d.kind, d.id)
			}
		}
		for _, file := range c.deltaFiles {
			if perFile[file] == 0 {
				fail("%s has no `### Requirement: <ID> <title>` under an ADDED, MODIFIED or REMOVED Requirements heading", file)
			}
		}
	}
	return out
}

// hasHeading reports whether a heading reads name, ignoring case, alone or followed by a space
// and more words, as in `## Rollback (TSC-V1-020)`.
func hasHeading(found map[string]bool, name string) bool {
	want := strings.ToLower(name)
	for heading := range found {
		h := strings.ToLower(heading)
		if h == want || strings.HasPrefix(h, want+" ") {
			return true
		}
	}
	return false
}

// retiredHere reports whether a REMOVED ID is marked retired in the canonical specs: its
// REQUIREMENTS.tsv text says retired, or its spec's delivery status does.
func (r *repo) retiredHere(id string) bool {
	row, ok := r.requirements[id]
	if !ok {
		return false
	}
	return retiredWord.MatchString(row.title) || strings.HasPrefix(r.specDelivery[row.file], "retired")
}

// inCanonical reports whether a delta ID's apply-stage obligation holds at the head.
func (r *repo) inCanonical(id, kind string) bool {
	if kind == "REMOVED" {
		return r.retiredHere(id)
	}
	_, ok := r.requirements[id]
	return ok
}

// taskIDs is the delta IDs a task names, in delta order for `Requirements: all`.
func taskIDs(c *change, t task) []string {
	kinds := c.deltaKinds()
	if t.all {
		ids := make([]string, 0, len(c.deltas))
		for _, d := range c.deltas {
			ids = append(ids, d.id)
		}
		return ids
	}
	var ids []string
	for _, id := range t.ids {
		if _, ok := kinds[id]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// applyRules is OSC-V1-005: tick order, and the canonical spec of every ticked task.
func applyRules(r *repo) ([]finding, []report) {
	var out []finding
	var reports []report
	for _, c := range append(append([]*change{}, r.active...), r.archived...) {
		subject := subjectChange(c.name)
		open := ""
		for _, t := range c.tasks {
			if !t.ticked {
				if open == "" {
					open = t.number
				}
				continue
			}
			if open != "" {
				out = append(out, finding{"OSC-V1-005", subject, fmt.Sprintf(
					"tasks.md:%d: task %s is ticked after unticked task %s; tick tasks in order", t.line, t.number, open)})
			}
		}
		kinds := c.deltaKinds()
		for _, t := range c.tasks {
			if !t.ticked {
				continue
			}
			ids := taskIDs(c, t)
			if len(ids) == 0 {
				continue
			}
			if c.external != "" {
				if len(t.commits) == 0 {
					out = append(out, finding{"OSC-V1-005", subject, fmt.Sprintf(
						"tasks.md:%d: task %s is ticked for a spec in %s but names no commit there that defines %s",
						t.line, t.number, c.external, strings.Join(ids, ", "))})
					continue
				}
				reports = append(reports, report{"external-spec-not-verified", subject, fmt.Sprintf(
					"task %s: %s defined by %s commit %s, not verified here",
					t.number, strings.Join(ids, ", "), c.external, strings.Join(t.commits, ", "))})
				continue
			}
			for _, id := range ids {
				if r.inCanonical(id, kinds[id]) {
					continue
				}
				what := "has no definition in a canonical spec under docs/specs/ (no REQUIREMENTS.tsv row); " +
					"copy the requirement into its owning spec and run make spec-requirements"
				if kinds[id] == "REMOVED" {
					what = "is not marked retired in its canonical spec under docs/specs/"
				}
				out = append(out, finding{"OSC-V1-005", subject, fmt.Sprintf(
					"tasks.md:%d: task %s is ticked but %s %s %s", t.line, t.number, kinds[id], id, what)})
			}
		}
	}
	return out, reports
}

// finished reports whether every task is ticked and every delta obligation holds (OSC-V1-006).
func (r *repo) finished(c *change) bool {
	if len(c.tasks) == 0 || len(c.malformed) > 0 {
		return false
	}
	for _, t := range c.tasks {
		if !t.ticked {
			return false
		}
	}
	if c.external != "" {
		return true
	}
	for _, d := range c.deltas {
		if !r.inCanonical(d.id, d.kind) {
			return false
		}
	}
	return true
}

// archiveStateRules is the tree part of OSC-V1-006.
func archiveStateRules(r *repo) []finding {
	var out []finding
	for _, c := range r.active {
		if r.finished(c) {
			out = append(out, finding{"OSC-V1-006", subjectChange(c.name), fmt.Sprintf(
				"every task is ticked and every delta ID is in the canonical specs: archive it with "+
					"`openspec archive %s --skip-specs` run from docs/specs, which moves it to %s/archive/%s-%s/",
				c.name, changesDir, r.now.Format("2006-01-02"), c.name)})
		}
	}
	for _, c := range r.archived {
		for _, t := range c.tasks {
			if !t.ticked {
				out = append(out, finding{"OSC-V1-006", subjectChange(c.name), fmt.Sprintf(
					"archived folder %s holds unticked task %s (tasks.md:%d); an archive never passes over an open task",
					c.dir, t.number, t.line)})
			}
		}
	}
	return out
}

// boundaryRules is OSC-V1-007.
func boundaryRules(root string) []finding {
	var out []finding
	fail := func(format string, args ...any) {
		out = append(out, finding{req: "OSC-V1-007", msg: fmt.Sprintf(format, args...)})
	}
	if info, err := os.Stat(filepath.Join(root, "openspec")); err == nil && info.IsDir() {
		fail("the repository root holds an openspec/ directory; the one OpenSpec root is %s/ and changes go "+
			"under %s/ (run openspec from docs/specs)", openspecRoot, changesDir)
	}
	if _, err := os.Lstat(filepath.Join(root, ".openspec.yaml")); err == nil {
		fail("the repository root holds .openspec.yaml; the one OpenSpec root is %s/", openspecRoot)
	}
	if _, err := os.Lstat(filepath.Join(root, openspecRoot, "specs")); err == nil {
		fail("%s/specs/ exists: the canonical spec belongs directly under docs/specs/ as a flat file; "+
			"run openspec archive with --skip-specs", openspecRoot)
	}
	config := openspecRoot + "/config.yaml"
	data, err := os.ReadFile(filepath.Join(root, config))
	if err != nil {
		fail("%s is missing; it makes %s/ the OpenSpec root with `schema: spec-driven`", config, openspecRoot)
		return out
	}
	schema := ""
	for _, line := range strings.Split(string(data), "\n") {
		if m := schemaLine.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			schema = m[1]
			break
		}
	}
	if schema != "spec-driven" {
		fail("%s names schema %q, want `schema: spec-driven`", config, schema)
	}
	return out
}

// healthRules is the check half of OSC-V1-008.
func healthRules(root string) []finding {
	var out []finding
	specs, err := filepath.Glob(filepath.Join(root, "docs/specs/*.md"))
	if err != nil {
		return nil
	}
	sort.Strings(specs)
	for _, path := range specs {
		if filepath.Base(path) == "README.md" {
			continue
		}
		rel := "docs/specs/" + filepath.Base(path)
		header, err := specHeader(path)
		if err != nil {
			out = append(out, finding{"OSC-V1-008", subjectSpec(rel), err.Error()})
			continue
		}
		value, ok := header["Health"]
		if !ok {
			out = append(out, finding{"OSC-V1-008", subjectSpec(rel),
				"its header has no `Health:` line (KEEP, MERGE, RETIRE or OUT-OF-1.0) before the first `## ` heading"})
			continue
		}
		class, _, _ := strings.Cut(value, " ")
		if !healthClasses[class] {
			out = append(out, finding{"OSC-V1-008", subjectSpec(rel), fmt.Sprintf(
				"`Health: %s` is not one of KEEP, MERGE, RETIRE or OUT-OF-1.0", value)})
			continue
		}
		if (class == "RETIRE" || class == "OUT-OF-1.0") && !strings.HasPrefix(header["Delivery status"], "retired") {
			out = append(out, finding{"OSC-V1-008", subjectSpec(rel), fmt.Sprintf(
				"`Health: %s` needs `Delivery status: retired`, got %q", class, header["Delivery status"])})
		}
	}
	return out
}

// acceptanceRules is OSC-V1-009 over every active proposal.
func acceptanceRules(r *repo) []finding {
	var out []finding
	for _, c := range r.active {
		if !c.files["proposal.md"] || c.intent == "" {
			continue
		}
		mode := r.acceptance[c.name]
		logged := mode == "owner" || mode == "auto"
		intent := strings.ToLower(c.intent)
		switch {
		case strings.HasPrefix(intent, "accepted") && !logged:
			out = append(out, finding{"OSC-V1-009", subjectChange(c.name), fmt.Sprintf(
				"proposal.md says the intent is accepted but %s has no `change` row for %s with mode owner or auto",
				acceptanceLog, c.name)})
		case logged:
		case !strings.HasPrefix(intent, "proposed"):
			out = append(out, finding{"OSC-V1-009", subjectChange(c.name), fmt.Sprintf(
				"with no acceptance log row the intent must read proposed, got %q", c.intent)})
		case !strings.HasPrefix(strings.ToLower(c.delivery), "not-started") &&
			!strings.HasPrefix(strings.ToLower(c.delivery), "experimental"):
			out = append(out, finding{"OSC-V1-009", subjectChange(c.name), fmt.Sprintf(
				"with no acceptance log row the delivery must read not-started or experimental, got %q", c.delivery)})
		}
	}
	return out
}
