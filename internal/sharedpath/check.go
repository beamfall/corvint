package sharedpath

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
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
// writes nothing: the merge probe uses `git merge-tree --write-tree`, which
// writes only unreachable objects, never a ref, index or worktree file.
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
	out, code, err := g.run(nil, nil, "merge-tree", "--write-tree", "--name-only", "--no-messages", "-z", base, head)
	if err != nil {
		return err
	}
	if code == 0 {
		return nil
	}
	if code != 1 {
		return errors.New(CodeGitUnavailable)
	}
	fields := bytes.Split(out, []byte{0})
	seen := map[string]bool{}
	for _, f := range fields[1:] {
		p := string(f)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if c.Class(p) == "generated" {
			r.ExemptConflicts = append(r.ExemptConflicts, p)
			continue
		}
		r.Findings = append(r.Findings, Finding{Code: "shared-path-merge-conflict", Path: p, Missing: missingMerge, Detail: "merging head into base conflicts outside the generated files"})
	}
	sort.Strings(r.ExemptConflicts)
	return nil
}

func readNotes(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.New(CodeNotesUnavailable)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New(CodeNotesUnavailable)
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxNotesBytes+1))
	if err != nil || len(raw) > maxNotesBytes {
		return nil, errors.New(CodeNotesUnavailable)
	}
	if raw == nil {
		raw = []byte{}
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
