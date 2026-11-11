// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type gitRepo struct {
	root string
}

// runCommand runs a command in dir and returns its standard output; standard error goes into
// the error.
func runCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (g gitRepo) git(args ...string) (string, error) {
	return runCommand(g.root, "git", args...)
}

func historyUnavailable(format string, args ...any) error {
	return &refusal{"history-unavailable", fmt.Sprintf(format, args...)}
}

// preflight refuses when the history the rules read is missing: no repository, no HEAD, a shallow
// clone, or a base that is not a commit (OSC-V1-011). It never lets a rule pass by default.
func (g gitRepo) preflight(base string) error {
	if out, err := g.git("rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(out) != "true" {
		return historyUnavailable("%s is not a Git work tree; the ID and archive rules read history", g.root)
	}
	if _, err := g.git("rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		return historyUnavailable("HEAD is not a commit; the ID and archive rules read history")
	}
	if out, err := g.git("rev-parse", "--is-shallow-repository"); err != nil || strings.TrimSpace(out) != "false" {
		return historyUnavailable("the clone is shallow, so the commit that added each change folder may be " +
			"missing; fetch full history (git fetch --unshallow, or actions/checkout with fetch-depth: 0)")
	}
	if base != "" {
		if _, err := g.git("rev-parse", "--verify", "-q", base+"^{commit}"); err != nil {
			return historyUnavailable("--base %s is not a commit in this clone; fetch it or name another base", base)
		}
	}
	return nil
}

// object returns the object name of path in the tree of rev. ok is false only when rev holds no
// such path (or rev is empty, the parent of a root commit); any other Git failure, such as a rev
// that is not a commit, is an error (OSC-V1-011).
func (g gitRepo) object(rev, path string) (name string, ok bool, err error) {
	if rev == "" {
		return "", false, nil
	}
	out, err := g.git("--literal-pathspecs", "ls-tree", "--full-tree", "-z", rev, "--", path)
	if err != nil {
		return "", false, historyUnavailable("cannot read %s at %s: %v", path, rev, err)
	}
	for _, entry := range strings.Split(out, "\x00") {
		meta, entryPath, found := strings.Cut(entry, "\t")
		if !found || entryPath != path {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			return "", false, historyUnavailable("cannot parse git ls-tree output %q for %s at %s", entry, path, rev)
		}
		return fields[2], true, nil
	}
	return "", false, nil
}

// exists reports whether rev holds path; a Git failure is an error, never absent.
func (g gitRepo) exists(rev, path string) (bool, error) {
	_, ok, err := g.object(rev, path)
	return ok, err
}

// showFile returns a file at a commit, and false when the commit does not hold it; a Git failure
// is an error, never absent.
func (g gitRepo) showFile(rev, path string) (string, bool, error) {
	name, ok, err := g.object(rev, path)
	if err != nil || !ok {
		return "", false, err
	}
	out, err := g.git("cat-file", "blob", name)
	if err != nil {
		return "", false, historyUnavailable("cannot read %s at %s: %v", path, rev, err)
	}
	return out, true, nil
}

type addition struct {
	commit string
	parent string
}

// folderAdditions maps each folder name directly under changes/ to the first commit that added a
// file in it, oldest first.
func (g gitRepo) folderAdditions() (map[string]addition, error) {
	out, err := g.git("log", "--reverse", "--diff-filter=A", "--no-renames", "--format=%x00%H %P", "--name-only", "--", changesDir+"/")
	if err != nil {
		return nil, historyUnavailable("cannot read the history of %s: %v", changesDir, err)
	}
	added := map[string]addition{}
	for _, record := range strings.Split(out, "\x00") {
		lines := strings.Split(strings.TrimSpace(record), "\n")
		if len(lines) == 0 || lines[0] == "" {
			continue
		}
		ids := strings.Fields(lines[0])
		a := addition{commit: ids[0]}
		if len(ids) > 1 {
			a.parent = ids[1]
		}
		for _, path := range lines[1:] {
			rest := strings.TrimPrefix(strings.TrimSpace(path), changesDir+"/")
			name, _, ok := strings.Cut(rest, "/")
			if !ok || name == "archive" {
				continue
			}
			if _, seen := added[name]; !seen {
				added[name] = a
			}
		}
	}
	return added, nil
}

// idRules is OSC-V1-002.
func idRules(r *repo, g gitRepo) ([]finding, error) {
	additions, err := g.folderAdditions()
	if err != nil {
		return nil, err
	}
	head, err := g.git("rev-parse", "HEAD")
	if err != nil {
		return nil, historyUnavailable("cannot resolve HEAD: %v", err)
	}
	head = strings.TrimSpace(head)
	before := map[string]map[string]requirementRow{}
	rowsBefore := func(commit string) (map[string]requirementRow, error) {
		if rows, ok := before[commit]; ok {
			return rows, nil
		}
		text, _, err := g.showFile(commit, requirementsTSV)
		if err != nil {
			return nil, err
		}
		rows := parseRequirements(text)
		before[commit] = rows
		return rows, nil
	}
	addedBy := map[string][]string{}
	all := append(append([]*change{}, r.active...), r.archived...)
	for _, c := range all {
		for _, d := range c.deltas {
			if d.kind == "ADDED" {
				addedBy[d.id] = append(addedBy[d.id], c.dir)
			}
		}
	}
	var out []finding
	for _, c := range all {
		subject := subjectChange(c.name)
		parent := head
		where := "HEAD (the folder is not committed yet)"
		if a, ok := additions[c.name]; ok {
			parent = a.parent
			where = "the parent of " + short(a.commit) + ", the commit that added the folder"
		}
		earlier, err := rowsBefore(parent)
		if err != nil {
			return nil, err
		}
		for _, d := range c.deltas {
			switch d.kind {
			case "ADDED":
				if row, ok := earlier[d.id]; ok {
					out = append(out, finding{"OSC-V1-002", subject, fmt.Sprintf(
						"ADDED %s already has a REQUIREMENTS.tsv row in %s at %s; take a fresh ID", d.id, row.file, where)})
				}
				for _, other := range addedBy[d.id] {
					if other != c.dir {
						out = append(out, finding{"OSC-V1-002", subject, fmt.Sprintf(
							"ADDED %s is also ADDED by %s; take a fresh ID", d.id, other)})
					}
				}
			default:
				if _, ok := r.requirements[d.id]; !ok {
					out = append(out, finding{"OSC-V1-002", subject, fmt.Sprintf(
						"%s %s has no REQUIREMENTS.tsv row; only an existing requirement is modified or removed", d.kind, d.id)})
				}
			}
		}
	}
	return out, nil
}

// archiveHistoryRules is the history part of OSC-V1-006: an archive commit is a pure rename dated
// within one day of the commit, and no later commit edits the archived folder.
func archiveHistoryRules(r *repo, g gitRepo) ([]finding, error) {
	if len(r.archived) == 0 {
		return nil, nil
	}
	out, err := g.git("log", "--reverse", "--no-renames", "--format=%x00%H %ct", "--name-only", "--", changesDir+"/archive/")
	if err != nil {
		return nil, historyUnavailable("cannot read the history of %s/archive: %v", changesDir, err)
	}
	type touch struct {
		commit string
		time   time.Time
	}
	touches := map[string][]touch{}
	for _, record := range strings.Split(out, "\x00") {
		lines := strings.Split(strings.TrimSpace(record), "\n")
		if len(lines) == 0 || lines[0] == "" {
			continue
		}
		fields := strings.Fields(lines[0])
		if len(fields) < 2 {
			continue
		}
		seconds, _ := strconv.ParseInt(fields[1], 10, 64)
		t := touch{commit: fields[0], time: time.Unix(seconds, 0).UTC()}
		seen := map[string]bool{}
		for _, path := range lines[1:] {
			rest := strings.TrimPrefix(strings.TrimSpace(path), changesDir+"/archive/")
			dir, _, ok := strings.Cut(rest, "/")
			if ok && !seen[dir] {
				seen[dir] = true
				touches[dir] = append(touches[dir], t)
			}
		}
	}
	var findings []finding
	for _, c := range r.archived {
		subject := subjectChange(c.name)
		dir := filepath.Base(c.dir)
		active := changesDir + "/" + c.name
		list := touches[dir]
		date, err := time.Parse("2006-01-02", c.date)
		if err != nil {
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf("archive date %q is not a date", c.date)})
			continue
		}
		within := func(at time.Time) bool {
			day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
			diff := date.Sub(day)
			return diff >= -24*time.Hour && diff <= 24*time.Hour
		}
		if len(list) == 0 {
			// Not committed yet: compare the work tree with the active folder at HEAD.
			held, err := g.exists("HEAD", active)
			if err != nil {
				return nil, err
			}
			if !held {
				findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
					"%s is not committed and HEAD holds no active folder %s to have been moved from", c.dir, active)})
				continue
			}
			same, err := worktreeEqualsTree(g, c.dir, "HEAD:"+active)
			if err != nil {
				return nil, err
			}
			if !same {
				findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
					"%s differs from %s at HEAD: an archive is a pure rename (openspec archive %s --skip-specs)",
					c.dir, active, c.name)})
			}
			if _, err := os.Stat(filepath.Join(r.root, active)); err == nil {
				findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
					"%s still exists beside its archive %s", active, c.dir)})
			}
			if !within(r.now) {
				findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
					"archive date %s is more than one day from today (%s UTC)", c.date, r.now.Format("2006-01-02"))})
			}
			continue
		}
		first := list[0]
		archivedTree, hasArchived, err := g.object(first.commit, c.dir)
		if err != nil {
			return nil, err
		}
		activeTree, hasActive := "", false
		if parents, err := g.git("rev-list", "--parents", "-n", "1", first.commit); err != nil {
			return nil, historyUnavailable("cannot read the parents of %s: %v", short(first.commit), err)
		} else if ids := strings.Fields(parents); len(ids) > 1 {
			if activeTree, hasActive, err = g.object(ids[1], active); err != nil {
				return nil, err
			}
		}
		stillActive, err := g.exists(first.commit, active)
		if err != nil {
			return nil, err
		}
		switch {
		case !hasActive:
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"archive commit %s has no active folder %s in its first parent to move", short(first.commit), active)})
		case !hasArchived || archivedTree != activeTree:
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"archive commit %s is not a pure rename of %s: the archived content differs; "+
					"archive with openspec archive %s --skip-specs and edit nothing in that commit",
				short(first.commit), active, c.name)})
		case stillActive:
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"archive commit %s copies %s instead of moving it", short(first.commit), active)})
		}
		if !within(first.time) {
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"archive date %s is more than one day from archive commit %s (%s UTC)",
				c.date, short(first.commit), first.time.Format("2006-01-02"))})
		}
		for _, later := range list[1:] {
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"commit %s edits archived folder %s; an archived change is never edited", short(later.commit), c.dir)})
		}
		dirty, err := g.git("status", "--porcelain", "--", c.dir)
		if err != nil {
			return nil, historyUnavailable("cannot read the work-tree status of %s: %v", c.dir, err)
		}
		if strings.TrimSpace(dirty) != "" {
			findings = append(findings, finding{"OSC-V1-006", subject, fmt.Sprintf(
				"the work tree edits archived folder %s; an archived change is never edited", c.dir)})
		}
	}
	return findings, nil
}

// worktreeEqualsTree compares the files of a work-tree directory with a Git tree by blob name.
func worktreeEqualsTree(g gitRepo, dir, tree string) (bool, error) {
	listing, err := g.git("ls-tree", "-r", tree)
	if err != nil {
		return false, err
	}
	want := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		want[path] = fields[2]
	}
	have := map[string]string{}
	base := filepath.Join(g.root, dir)
	err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, path)
		header := fmt.Sprintf("blob %d\x00", len(data))
		sum := sha1.Sum(append([]byte(header), data...))
		have[filepath.ToSlash(rel)] = fmt.Sprintf("%x", sum)
		return nil
	})
	if err != nil {
		return false, err
	}
	if len(want) != len(have) {
		return false, nil
	}
	for path, blob := range want {
		if have[path] != blob {
			return false, nil
		}
	}
	return true, nil
}

type commitRecord struct {
	commit   string
	parents  []string
	trailers map[string][]string
	paths    []string
}

func (g gitRepo) commits(base string) ([]commitRecord, error) {
	out, err := g.git("log", "--reverse", "--no-merges", "--no-renames",
		"--format=%x00%H%x1f%P%x1f%(trailers:only,unfold)%x1f", "--name-only", base+"..HEAD")
	if err != nil {
		return nil, historyUnavailable("cannot list the commits of %s..HEAD: %v", base, err)
	}
	var records []commitRecord
	for _, chunk := range strings.Split(out, "\x00") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		parts := strings.SplitN(chunk, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		rec := commitRecord{commit: parts[0], parents: strings.Fields(parts[1]), trailers: map[string][]string{}}
		for _, line := range strings.Split(parts[2], "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			rec.trailers[strings.ToLower(strings.TrimSpace(key))] = append(rec.trailers[strings.ToLower(strings.TrimSpace(key))], strings.TrimSpace(value))
		}
		for _, path := range strings.Split(parts[3], "\n") {
			if path = strings.TrimSpace(path); path != "" {
				rec.paths = append(rec.paths, path)
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

// touchedChanges returns the change names whose folder a commit touches: a path under
// changes/<name>/ or changes/archive/YYYY-MM-DD-<name>/.
func touchedChanges(paths []string) []string {
	seen := map[string]bool{}
	var names []string
	for _, path := range paths {
		rest, ok := strings.CutPrefix(path, changesDir+"/")
		if !ok {
			continue
		}
		dir, inner, ok := strings.Cut(rest, "/")
		if !ok {
			continue
		}
		if dir == "archive" {
			dated, _, ok := strings.Cut(inner, "/")
			m := archivedName.FindStringSubmatch(dated)
			if !ok || m == nil {
				continue
			}
			dir = m[2]
		}
		if !seen[dir] {
			seen[dir] = true
			names = append(names, dir)
		}
	}
	return names
}

// commitRules is OSC-V1-003 and OSC-V1-004 over BASE..HEAD. Only a commit that opts in to a
// change is in scope: it touches a change folder, carries a `Change:` trailer, or carries a
// `Corvint-Ticket:` that some proposal's `Ticket:` header names. Every other commit gets no
// finding.
func commitRules(r *repo, g gitRepo, base string) ([]finding, error) {
	records, err := g.commits(base)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	byTicket := map[string][]string{}
	for _, c := range append(append([]*change{}, r.active...), r.archived...) {
		known[c.name] = true
		for _, ticket := range c.tickets {
			byTicket[ticket] = append(byTicket[ticket], c.name)
		}
	}
	var out []finding
	for _, rec := range records {
		var code []string
		for _, path := range rec.paths {
			if !strings.HasPrefix(path, "docs/") {
				code = append(code, path)
			}
		}
		if len(code) == 0 {
			continue
		}
		touched := touchedChanges(rec.paths)
		var ticketed []string
		for _, ticket := range rec.trailers["corvint-ticket"] {
			ticketed = append(ticketed, byTicket[strings.TrimSpace(ticket)]...)
		}
		if len(touched) == 0 && len(rec.trailers["change"]) == 0 && len(ticketed) == 0 {
			continue // out of scope: the commit opts in to no change
		}
		bound := map[string]bool{}
		var missing []string
		for _, name := range touched {
			if known[name] {
				bound[name] = true
			} else {
				missing = append(missing, name)
			}
		}
		for _, name := range ticketed {
			bound[name] = true
		}
		named := false
		for _, name := range rec.trailers["change"] {
			if known[name] {
				bound[name] = true
				continue
			}
			named = true
			out = append(out, finding{req: "OSC-V1-003", msg: fmt.Sprintf(
				"commit %s: trailer `Change: %s` names no active or archived change folder", short(rec.commit), name)})
		}
		accepted := false
		for _, id := range rec.trailers["requirement"] {
			row, ok := r.requirements[id]
			if ok && (row.acceptance == "owner" || row.acceptance == "standing") {
				accepted = true
				continue
			}
			out = append(out, finding{req: "OSC-V1-003", msg: fmt.Sprintf(
				"commit %s: trailer `Requirement: %s` names no requirement accepted as owner or standing in REQUIREMENTS.tsv",
				short(rec.commit), id)})
		}
		if len(bound) == 0 && !accepted {
			if !named {
				out = append(out, finding{req: "OSC-V1-003", msg: fmt.Sprintf(
					"commit %s changes %s outside docs/ and touches change folder(s) %s, which no active or archived "+
						"folder holds at HEAD, so it binds to no change: add `Change: <name>`, a `Corvint-Ticket:` "+
						"naming a proposal's ticket, or `Requirement: <accepted ID>`",
					short(rec.commit), listPaths(code), strings.Join(missing, ", "))})
			}
			continue
		}
		names := make([]string, 0, len(bound))
		for name := range bound {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			held := false
			if len(rec.parents) > 0 {
				if held, err = g.exists(rec.parents[0], changesDir+"/"+name+"/proposal.md"); err != nil {
					return nil, err
				}
			}
			if !held {
				out = append(out, finding{req: "OSC-V1-004", msg: fmt.Sprintf(
					"commit %s changes %s for change %s, but its first parent holds no active %s/%s/proposal.md: "+
						"commit the proposal before the code", short(rec.commit), listPaths(code), name, changesDir, name)})
			}
		}
	}
	return out, nil
}

// lineLocator is the line number of a `file.md:N:` locator in a finding message; the baseline key
// drops it so an edit that only moves a line keeps its entry.
var lineLocator = regexp.MustCompile(`(\.(?:md|tsv|yaml)):[0-9]+:`)

// baselineKey is the baseline entry that matches f: `<rule ID> <subject>: <key>`. A finding with no
// subject has no key and is never baselined.
func baselineKey(f finding) string {
	if f.subject == "" {
		return ""
	}
	return f.req + " " + f.subject + ": " + lineLocator.ReplaceAllString(f.msg, "$1:")
}

var baselineRule = regexp.MustCompile(`^OSC-V1-[0-9]{3}$`)

// loadBaseline reads the baseline entries. A malformed line is a finding.
func loadBaseline(root, path string) (map[string]bool, []finding, error) {
	data, err := os.ReadFile(filepath.Join(root, path))
	if os.IsNotExist(err) {
		return map[string]bool{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	entries, malformed := parseBaseline(string(data), path)
	return entries, malformed, nil
}

// parseBaseline reads `<rule ID> change <name>: <key>` and `<rule ID> spec docs/specs/<file>.md:
// <key>` lines and `#` comments; it returns the entries and a finding per malformed or duplicate
// line.
func parseBaseline(text, path string) (map[string]bool, []finding) {
	entries := map[string]bool{}
	var out []finding
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule, rest, _ := strings.Cut(line, " ")
		kind, rest, _ := strings.Cut(rest, " ")
		value, key, _ := strings.Cut(rest, ": ")
		ok := baselineRule.MatchString(rule) && strings.TrimSpace(key) != "" &&
			((kind == "change" && changeName.MatchString(value)) ||
				(kind == "spec" && strings.HasPrefix(value, "docs/specs/") && strings.HasSuffix(value, ".md") &&
					!strings.Contains(strings.TrimPrefix(value, "docs/specs/"), "/")))
		if !ok {
			out = append(out, finding{req: "OSC-V1-011", msg: fmt.Sprintf(
				"%s:%d: %q is not `<rule ID> change <name>: <finding>` or `<rule ID> spec docs/specs/<file>.md: <finding>`",
				path, i+1, line)})
			continue
		}
		if entries[line] {
			out = append(out, finding{req: "OSC-V1-011", msg: fmt.Sprintf("%s:%d: duplicate entry %q", path, i+1, line)})
		}
		entries[line] = true
	}
	return entries, out
}

// baselineGrowth fails every baseline entry the range adds: the baseline only shrinks. When the
// file was absent at base every entry is added; any other Git failure is an error.
func baselineGrowth(g gitRepo, base, path string, entries map[string]bool) ([]finding, error) {
	text, _, err := g.showFile(base, path)
	if err != nil {
		return nil, err
	}
	before, _ := parseBaseline(text, path)
	var added []string
	for entry := range entries {
		if !before[entry] {
			added = append(added, entry)
		}
	}
	sort.Strings(added)
	var out []finding
	for _, entry := range added {
		out = append(out, finding{req: "OSC-V1-011", msg: fmt.Sprintf(
			"baseline entry %q was added since %s; the baseline only shrinks, so fix the finding instead", entry, base)})
	}
	return out, nil
}
