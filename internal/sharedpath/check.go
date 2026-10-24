package sharedpath

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CheckProfile is the check-shared report profile (SPC-V0-002).
const CheckProfile = "corvint-shared-path-check/0"

// Finding names the commit, the path and the missing element. Missing is one
// of marker, separate-commit, listing, run-citation or clean-merge.
type Finding struct {
	Code    string `json:"code"`
	Commit  string `json:"commit,omitempty"`
	Path    string `json:"path"`
	Missing string `json:"missing"`
	Detail  string `json:"detail,omitempty"`
}

// SharedCommit is one commit of the range that changed a shared path.
type SharedCommit struct {
	Commit      string   `json:"commit"`
	Subject     string   `json:"subject"`
	Paths       []string `json:"paths"`
	Marker      bool     `json:"marker"`
	Listed      string   `json:"listed"`
	RunCitation string   `json:"runCitation,omitempty"`
}

// Report is the check-shared result.
type Report struct {
	Profile         string         `json:"profile"`
	Mutates         bool           `json:"mutates"`
	Base            string         `json:"base"`
	Head            string         `json:"head"`
	Commits         int            `json:"commits"`
	Shared          []SharedCommit `json:"shared"`
	Findings        []Finding      `json:"findings"`
	ExemptConflicts []string       `json:"exemptConflicts"`
	SkippedMerges   []string       `json:"skippedMerges"`
	NotObserved     []string       `json:"notObserved"`
	OK              bool           `json:"ok"`
}

// CheckRequest selects the checkout, policy and range.
type CheckRequest struct {
	Root, Base, Head, NotesFile string
	Config                      Config
}

const (
	missingMarker   = "marker"
	missingSeparate = "separate-commit"
	missingListing  = "listing"
	missingRun      = "run-citation"
	missingMerge    = "clean-merge"
)

// Check reads base..head and reports every shared-path policy finding. It
// writes nothing to the repository: the merge probe runs `git merge-tree
// --write-tree` in a private temporary repository (mergeConflicts).
func Check(ctx context.Context, request CheckRequest) (Report, error) {
	g := git{ctx: ctx, root: request.Root}
	if request.Head == "" {
		request.Head = "HEAD"
	}
	base, err := g.commit(request.Base)
	if err != nil {
		return Report{}, err
	}
	head, err := g.commit(request.Head)
	if err != nil {
		return Report{}, err
	}
	if err := refuseShallowMergeBase(g, base, head); err != nil {
		return Report{}, err
	}
	var notes []byte
	r := Report{Profile: CheckProfile, Base: base, Head: head, Shared: []SharedCommit{}, Findings: []Finding{}, ExemptConflicts: []string{}, SkippedMerges: []string{}, NotObserved: []string{}}
	if request.NotesFile != "" {
		if notes, err = readNotes(request.NotesFile); err != nil {
			return Report{}, err
		}
	} else {
		r.NotObserved = append(r.NotObserved, "notes-listing")
	}
	commits, err := g.commits(base, head)
	if err != nil {
		return Report{}, err
	}
	r.Commits = len(commits)
	c := request.Config
	for _, rc := range commits {
		if len(rc.parents) > 1 {
			r.SkippedMerges = append(r.SkippedMerges, rc.sha)
			continue
		}
		changes, err := g.changes(rc.sha)
		if err != nil {
			return Report{}, err
		}
		var shared, other []string
		for _, ch := range changes {
			switch c.Class(ch.path) {
			case "shared":
				shared = append(shared, ch.path)
			case "other":
				other = append(other, ch.path)
			}
		}
		if len(shared) == 0 {
			continue
		}
		m, err := g.message(rc.sha)
		if err != nil {
			return Report{}, err
		}
		sc := SharedCommit{Commit: rc.sha, Subject: m.subject(), Paths: shared, Marker: strings.HasPrefix(m.subject(), c.Marker), Listed: "NOT_OBSERVED"}
		citation, cited := m.runCitation(c.RunTrailer)
		sc.RunCitation = citation
		if notes != nil {
			sc.Listed = "NO"
			if listed(notes, rc.sha) {
				sc.Listed = "YES"
			}
		}
		r.Shared = append(r.Shared, sc)
		for _, p := range shared {
			if !sc.Marker {
				r.Findings = append(r.Findings, Finding{Code: "shared-path-unmarked", Commit: rc.sha, Path: p, Missing: missingMarker, Detail: "subject lacks the marker " + c.Marker})
			}
			if sc.Listed == "NO" {
				r.Findings = append(r.Findings, Finding{Code: "shared-path-unlisted", Commit: rc.sha, Path: p, Missing: missingListing, Detail: "the notes do not name this commit"})
			}
			switch {
			case !cited:
				r.Findings = append(r.Findings, Finding{Code: "shared-path-uncited", Commit: rc.sha, Path: p, Missing: missingRun, Detail: "no " + c.RunTrailer + " trailer"})
			case !passing(citation):
				r.Findings = append(r.Findings, Finding{Code: "shared-path-uncited", Commit: rc.sha, Path: p, Missing: missingRun, Detail: "the cited run is not PASS"})
			}
		}
		if sc.Marker {
			for _, p := range other {
				r.Findings = append(r.Findings, Finding{Code: "shared-path-mixed", Commit: rc.sha, Path: p, Missing: missingSeparate, Detail: "a marked shared-path commit also changes this non-shared path"})
			}
		}
	}
	if err := mergeProbe(g, c, base, head, &r); err != nil {
		return Report{}, err
	}
	r.OK = len(r.Findings) == 0
	return r, nil
}

// mergeProbe reports conflicts of merging head into base; conflicts
// confined to configured generated files are recorded as exempt, not findings.
func mergeProbe(g git, c Config, base, head string, r *Report) error {
	merge, err := mergeConflicts(g, base, head)
	if err != nil {
		return err
	}
	if merge.conflicted && len(merge.paths) == 0 {
		r.Findings = append(r.Findings, Finding{Code: "shared-path-merge-conflict", Missing: missingMerge, Detail: "merging head into base conflicts without naming a path"})
	}
	for _, p := range merge.paths {
		if c.Class(p) == "generated" {
			r.ExemptConflicts = append(r.ExemptConflicts, p)
			continue
		}
		r.Findings = append(r.Findings, Finding{Code: "shared-path-merge-conflict", Path: p, Missing: missingMerge, Detail: "merging head into base conflicts outside the generated files"})
	}
	sort.Strings(r.ExemptConflicts)
	return nil
}

// refuseShallowMergeBase refuses a shallow repository whose history between
// base and head is not provably inside the shallow boundary (SPC-V0-015,
// V1-1726): no merge base is visible; more than one merge base is visible,
// because Git merges several merge bases recursively through their own
// ancestry, which this check does not prove complete; a merge base is a boundary commit, whose
// parents the clone does not have, other than base or head itself; or a commit
// of the symmetric difference base...head is a boundary commit, so a side
// branch of the range is cut off. Each of these can make the range listing and
// the dry merge, which reads the same shallow file (privateMergeRepository),
// report a wrong result. A merge base that is base or head is exact: shallow
// ancestry is a subset of the full ancestry. A full repository is unaffected.
func refuseShallowMergeBase(g git, base, head string) error {
	out, err := g.ok("rev-parse", "--is-shallow-repository", "--path-format=absolute", "--git-path", "shallow")
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 2 || !filepath.IsAbs(lines[1]) {
		return errors.New(CodeGitUnavailable)
	}
	if lines[0] != "true" {
		return nil
	}
	raw, err := readRegular(lines[1], maxGitBytes)
	if err != nil {
		return errors.New(CodeGitUnavailable)
	}
	boundary := map[string]bool{}
	for _, sha := range strings.Fields(string(raw)) {
		boundary[sha] = true
	}
	out, code, err := g.run(nil, nil, "merge-base", "--all", base, head)
	if err != nil {
		return err
	}
	switch code {
	case 0:
	case 1:
		return errors.New(CodeShallowMergeBase)
	default:
		return errors.New(CodeGitUnavailable)
	}
	bases := strings.Fields(string(out))
	if len(bases) != 1 {
		return errors.New(CodeShallowMergeBase)
	}
	for _, sha := range bases {
		if !fullSHA.MatchString(sha) {
			return errors.New(CodeGitUnavailable)
		}
		if boundary[sha] && sha != base && sha != head {
			return errors.New(CodeShallowMergeBase)
		}
	}
	out, err = g.ok("rev-list", "--end-of-options", base+"..."+head)
	if err != nil {
		return err
	}
	for _, sha := range strings.Fields(string(out)) {
		if boundary[sha] {
			return errors.New(CodeShallowMergeBase)
		}
	}
	return nil
}

// mergeOutcome is the merge-tree probe of merging head into base: whether Git
// reported a conflict (exit status 1) and the distinct paths it named.
type mergeOutcome struct {
	conflicted bool
	paths      []string
}

// mergeConflicts probes merging head into base, in merge-tree order: the
// conflicted file entries, then the paths named by CONFLICT messages, so a
// conflict without an unmerged file entry (a directory rename split) still
// counts (SPC-V0-010). Merge-tree runs inside a private temporary bare
// repository that reads the repository's objects as an alternate and is removed
// afterwards. It has no repository configuration, attributes or worktree, so
// no custom merge or filter driver runs, nothing is written into the checkout
// and the repository's object store is never written (AGENTS.md invariant 4;
// SPC-V0-003, SPC-V0-009).
func mergeConflicts(g git, base, head string) (mergeOutcome, error) {
	env, cleanup, err := privateMergeRepository(g)
	if err != nil {
		return mergeOutcome{}, err
	}
	defer cleanup()
	out, code, err := g.run(nil, env, "merge-tree", "--write-tree", "--name-only", "-z", base, head)
	if err != nil {
		return mergeOutcome{}, err
	}
	switch code {
	case 0:
		return mergeOutcome{}, nil
	case 1:
	default:
		return mergeOutcome{}, errors.New(CodeGitUnavailable)
	}
	paths, ok := conflictPaths(out)
	if !ok {
		return mergeOutcome{}, errors.New(CodeGitUnavailable)
	}
	return mergeOutcome{conflicted: true, paths: paths}, nil
}

// privateMergeRepository creates a temporary bare repository with the
// repository's object format, its object store as the only alternate and its
// shallow file, and returns the environment that points Git at it.
func privateMergeRepository(g git) ([]string, func(), error) {
	out, err := g.ok("rev-parse", "--path-format=absolute", "--git-path", "objects", "--git-path", "shallow", "--show-object-format")
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 3 {
		return nil, nil, errors.New(CodeGitUnavailable)
	}
	store, shallow, format := lines[0], lines[1], lines[2]
	if store == "" || !filepath.IsAbs(store) || !singleAlternate(store) || !filepath.IsAbs(shallow) {
		return nil, nil, errors.New(CodeGitUnavailable)
	}
	if format != "sha1" && format != "sha256" {
		return nil, nil, errors.New(CodeGitUnavailable)
	}
	scratch, err := os.MkdirTemp("", "corvint-shared-merge-")
	if err != nil {
		return nil, nil, errors.New(CodeGitUnavailable)
	}
	cleanup := func() { _ = os.RemoveAll(scratch) }
	repository := filepath.Join(scratch, "repository.git")
	if _, err := g.ok("init", "--quiet", "--bare", "--template=", "--object-format="+format, repository); err != nil {
		cleanup()
		return nil, nil, err
	}
	env := []string{"GIT_DIR=" + repository, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + store}
	if info, err := os.Lstat(shallow); err == nil && info.Mode().IsRegular() {
		env = append(env, "GIT_SHALLOW_FILE="+shallow)
	}
	return env, cleanup, nil
}

// singleAlternate reports whether store is one GIT_ALTERNATE_OBJECT_DIRECTORIES
// entry: no list separator (':' on POSIX, ';' on Windows, Git's PATH_SEP), quote
// or line break anywhere. Git for Windows' C:/... passes: ':' is not its separator.
func singleAlternate(store string) bool {
	return !strings.ContainsAny(store, string(filepath.ListSeparator)+"\"\n\r")
}

// conflictPaths parses `merge-tree --write-tree --name-only -z` output: the
// tree, the conflicted file entries, an empty field, then messages of a path
// count, that many paths, a type and a text. It returns the distinct file
// entries and the paths of messages whose type starts with CONFLICT.
func conflictPaths(out []byte) ([]string, bool) {
	fields := strings.Split(string(out), "\x00")
	if len(fields) < 2 || fields[len(fields)-1] != "" {
		return nil, false
	}
	fields = fields[1 : len(fields)-1]
	paths := []string{}
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	i := 0
	for ; i < len(fields) && fields[i] != ""; i++ {
		add(fields[i])
	}
	for i++; i < len(fields); {
		n, err := strconv.Atoi(fields[i])
		if err != nil || n < 0 || n > len(fields)-i-3 {
			return nil, false
		}
		if strings.HasPrefix(fields[i+n+1], "CONFLICT") {
			for _, p := range fields[i+1 : i+n+1] {
				add(p)
			}
		}
		i += n + 3
	}
	return paths, true
}

func readNotes(file string) ([]byte, error) {
	raw, err := readRegular(file, maxNotesBytes)
	if err != nil {
		return nil, errors.New(CodeNotesUnavailable)
	}
	return raw, nil
}

var hexToken = regexp.MustCompile(`\b[0-9a-fA-F]{7,64}\b`)

// listed reports whether the notes name sha by a whole hexadecimal token of at
// least seven characters that is a prefix of it.
func listed(notes []byte, sha string) bool {
	for _, token := range hexToken.FindAll(notes, -1) {
		if strings.HasPrefix(sha, strings.ToLower(string(token))) {
			return true
		}
	}
	return false
}

// passing reports whether a run citation's first word is PASS.
func passing(citation string) bool {
	fields := strings.Fields(citation)
	return len(fields) >= 2 && fields[0] == "PASS"
}
