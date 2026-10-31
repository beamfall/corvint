// SPDX-License-Identifier: Apache-2.0
// dogfood-inherit-citations proposes a retroactive binding's citation plan
// from the sealed CEMs of the changes inside the range (DCW-V0-824, V1-3365).
// script/dogfood-bind-range.sh runs it for DOGFOOD_CITATIONS=auto-inherit.
//
// usage: dogfood-inherit-citations REPO BASE TARGET MAP PLAN UNINHERITED
//
// The constituent CEMs are every .corvint/changes/<bind>.cem.json in TARGET's
// tree and every .corvint/change.cem.json a commit in BASE..TARGET wrote. A
// hunk of MAP inherits the citations of a constituent hunk with the same hunk
// ID; otherwise of each constituent hunk on the same path whose added lines
// (or, for a pure removal, removed lines) overlap its own. Each inherited
// evidence span is moved to BASE by exact text search of the cited bytes,
// nearest the original offset, and written as a `bytes:S:E` plan row
// (DCW-V0-827). A hunk with no inherited row is written to UNINHERITED with
// its reason; it is never cited and stays unknown in the map, unless it is in
// a proposed class (classes.go): a byte-identical sealed-CEM rename
// (DCW-V0-829) or a hunk a registered generator reproduces (DCW-V0-828).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

const (
	changeMapPath   = ".corvint/change.cem.json"
	sealedDirectory = ".corvint/changes/"
	maxRowsPerHunk  = 4
	maxConstituents = 1024
)

type span struct{ Start, End int64 }

type lineRange struct{ Start, Count int64 }

type basis struct {
	EvidenceID string `json:"evidenceId"`
	Relation   string `json:"relation"`
}

type hunk struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Basis    []basis   `json:"basis"`
	NewRange lineRange `json:"newRange"`
}

type evidence struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	BlobOid string `json:"blobOid"`
	Span    *span  `json:"span"`
}

type document struct {
	BaseRevision string     `json:"baseRevision"`
	Hunks        []hunk     `json:"hunks"`
	Evidence     []evidence `json:"evidence"`
}

// constituent is one sealed CEM and the commit its canonical patch ends at,
// when that commit is known.
type constituent struct {
	doc      document
	commit   string
	evidence map[string]evidence
	diff     map[string]*patch.Hunk
	diffDone bool
}

type row struct {
	path       string
	start, end int64
	relation   string
}

type inheritor struct {
	repo, base, target string
	emptyTree          string
	blobs              map[string][]byte
	constituents       []*constituent
	byID               map[string][]*constituent

	// Proposed-class state (classes.go), loaded on first use.
	sealedAt               [2]map[string]string
	removedCEMs, addedCEMs map[string]bool
	specIndex              map[string]bool
	citations              int
	gitDir, privateIndex   string
}

func main() {
	if len(os.Args) != 7 {
		fmt.Fprintln(os.Stderr, "usage: dogfood-inherit-citations REPO BASE TARGET MAP PLAN UNINHERITED")
		os.Exit(2)
	}
	summary, err := run(os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
	if err != nil {
		fmt.Fprintf(os.Stderr, "dogfood-inherit-citations: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(summary)
}

func run(repo, base, target, mapPath, planPath, uninheritedPath string) (string, error) {
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		return "", err
	}
	var current document
	if err := json.Unmarshal(raw, &current); err != nil {
		return "", fmt.Errorf("map: %w", err)
	}
	in := &inheritor{repo: repo, base: base, target: target, blobs: map[string][]byte{}, byID: map[string][]*constituent{}}
	if in.emptyTree, err = in.git(nil, "hash-object", "-t", "tree", "--stdin"); err != nil {
		return "", fmt.Errorf("empty tree: %w", err)
	}
	if err := in.loadConstituents(); err != nil {
		return "", err
	}
	mine := in.canonicalDiff(base, target)
	var plan, missing bytes.Buffer
	inherited, proposed, rows := 0, 0, 0
	for i, h := range current.Hunks {
		got, reason := in.inherit(h, mine[h.ID])
		if len(got) > 0 {
			inherited++
		} else if got, _ = in.propose(mine[h.ID]); len(got) > 0 {
			proposed++
		} else {
			fmt.Fprintf(&missing, "%d\t%s\t%s\t%s\n", i+1, h.ID, planField(h.Path), reason)
			continue
		}
		for _, r := range got {
			rows++
			fmt.Fprintf(&plan, "%d\t%s\tbytes:%d:%d\t%s\n", i+1, r.path, r.start, r.end, r.relation)
		}
	}
	if err := os.WriteFile(planPath, plan.Bytes(), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(uninheritedPath, missing.Bytes(), 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf("hunks=%d inherited=%d proposed=%d rows=%d uninherited=%d constituents=%d",
		len(current.Hunks), inherited, proposed, rows, len(current.Hunks)-inherited-proposed, len(in.constituents)), nil
}

// git runs one Git command in the repository with no caller stdin (V1-3367).
func (in *inheritor) git(stdin []byte, args ...string) (string, error) {
	out, err := in.gitBytes(stdin, args...)
	return strings.TrimSpace(string(out)), err
}

func (in *inheritor) gitBytes(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", in.repo}, args...)...)
	cmd.Stdin = bytes.NewReader(stdin)
	return cmd.Output()
}

func (in *inheritor) loadConstituents() error {
	seen := map[[32]byte]bool{}
	add := func(blobRef, commit string) {
		if len(in.constituents) >= maxConstituents {
			return
		}
		raw, err := in.gitBytes(nil, "cat-file", "blob", blobRef)
		if err != nil {
			return
		}
		digest := sha256.Sum256(raw)
		if seen[digest] {
			return
		}
		seen[digest] = true
		c := &constituent{commit: commit, evidence: map[string]evidence{}}
		if json.Unmarshal(raw, &c.doc) != nil {
			return
		}
		for _, e := range c.doc.Evidence {
			c.evidence[e.ID] = e
		}
		in.constituents = append(in.constituents, c)
		for _, h := range c.doc.Hunks {
			if len(h.Basis) > 0 {
				in.byID[h.ID] = append(in.byID[h.ID], c)
			}
		}
	}
	listing, err := in.git(nil, "ls-tree", "-r", "--name-only", "--end-of-options", in.target, "--", sealedDirectory)
	if err != nil {
		return fmt.Errorf("ls-tree %s: %w", in.target, err)
	}
	for _, path := range strings.Split(listing, "\n") {
		name, ok := strings.CutPrefix(path, sealedDirectory)
		if !ok || !strings.HasSuffix(name, ".cem.json") || strings.Contains(name, "/") {
			continue
		}
		commit, _ := in.git(nil, "rev-parse", "--verify", "--quiet", "--end-of-options", strings.TrimSuffix(name, ".cem.json")+"^{commit}")
		add(in.target+":"+path, commit)
	}
	commits, err := in.git(nil, "log", "--format=%H", "--end-of-options", in.base+".."+in.target, "--", changeMapPath)
	if err != nil {
		return fmt.Errorf("log %s..%s: %w", in.base, in.target, err)
	}
	for _, commit := range strings.Fields(commits) {
		add(commit+":"+changeMapPath, commit)
	}
	return nil
}

// canonicalDiff parses the canonical patch with the producer's diff options,
// so its hunk IDs are the map's; an unreadable patch yields no hunk bodies.
func (in *inheritor) canonicalDiff(base, target string) map[string]*patch.Hunk {
	raw, err := in.gitBytes(nil, "--attr-source="+in.emptyTree,
		"diff", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/",
		"--ignore-submodules=none", "--end-of-options", base, target,
		"--", ".", ":(exclude)"+changeMapPath)
	if err != nil {
		return nil
	}
	parsed, err := patch.Parse(raw)
	if err != nil {
		return nil
	}
	hunks := map[string]*patch.Hunk{}
	for _, h := range parsed.Hunks {
		hunks[h.ID] = h
	}
	return hunks
}

// inherit returns the relocated citations for one map hunk, or the reason
// it inherits none.
func (in *inheritor) inherit(h hunk, mine *patch.Hunk) ([]row, string) {
	var sources []hunkSource
	for _, c := range in.byID[h.ID] {
		for _, theirs := range c.doc.Hunks {
			if theirs.ID == h.ID && len(theirs.Basis) > 0 {
				sources = append(sources, hunkSource{c, theirs})
			}
		}
	}
	reason := "no-constituent-hunk"
	if len(sources) == 0 && mine != nil {
		added, removed := changedLines(mine)
		for _, c := range in.constituents {
			if c.commit == "" || c.doc.BaseRevision == "" {
				continue
			}
			for _, theirs := range c.doc.Hunks {
				if theirs.Path != h.Path {
					continue
				}
				if !c.diffDone {
					c.diff, c.diffDone = in.canonicalDiff(c.doc.BaseRevision, c.commit), true
				}
				body := c.diff[theirs.ID]
				if body == nil {
					continue
				}
				theirAdded, theirRemoved := changedLines(body)
				if !overlaps(added, theirAdded) && (len(added) > 0 || !overlaps(removed, theirRemoved)) {
					continue
				}
				if len(theirs.Basis) == 0 {
					reason = "constituent-hunk-unknown"
					continue
				}
				sources = append(sources, hunkSource{c, theirs})
			}
		}
	}
	if len(sources) > 0 {
		reason = "evidence-absent-at-base"
	}
	var rows []row
	seen := map[row]bool{}
	for _, source := range sources {
		for _, b := range source.hunk.Basis {
			e, ok := source.c.evidence[b.EvidenceID]
			if !ok || e.Span == nil {
				continue
			}
			r, ok := in.relocate(e)
			if !ok {
				continue
			}
			r.relation = b.Relation
			if seen[r] || len(rows) >= maxRowsPerHunk {
				continue
			}
			seen[r] = true
			rows = append(rows, r)
		}
	}
	return rows, reason
}

type hunkSource struct {
	c    *constituent
	hunk hunk
}

// changedLines returns a hunk's trimmed added and removed lines longer than
// three bytes, so blank and brace-only lines never make an overlap.
func changedLines(h *patch.Hunk) (added, removed map[string]bool) {
	added, removed = map[string]bool{}, map[string]bool{}
	for _, line := range h.Body {
		text := strings.TrimSpace(string(line.Payload))
		if len(text) <= 3 {
			continue
		}
		switch line.Prefix {
		case '+':
			added[text] = true
		case '-':
			removed[text] = true
		}
	}
	return added, removed
}

func overlaps(a, b map[string]bool) bool {
	for text := range a {
		if b[text] {
			return true
		}
	}
	return false
}

// relocate finds the cited bytes in BASE's blob at the same path, choosing
// the occurrence nearest the original offset.
func (in *inheritor) relocate(e evidence) (row, bool) {
	if e.Path == "" || planField(e.Path) != e.Path || e.BlobOid == "" {
		return row{}, false
	}
	source, ok := in.blob(e.BlobOid)
	if !ok || e.Span.Start < 0 || e.Span.End <= e.Span.Start || e.Span.End > int64(len(source)) {
		return row{}, false
	}
	text := source[e.Span.Start:e.Span.End]
	if len(bytes.TrimSpace(text)) == 0 {
		return row{}, false
	}
	data, ok := in.blob(in.base + ":" + e.Path)
	if !ok {
		return row{}, false
	}
	best, at := int64(-1), int64(0)
	for {
		index := bytes.Index(data[at:], text)
		if index < 0 {
			break
		}
		position := at + int64(index)
		if best < 0 || distance(position, e.Span.Start) < distance(best, e.Span.Start) {
			best = position
		}
		at = position + 1
	}
	if best < 0 {
		return row{}, false
	}
	return row{path: e.Path, start: best, end: best + int64(len(text))}, true
}

func distance(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

func (in *inheritor) blob(ref string) ([]byte, bool) {
	if data, ok := in.blobs[ref]; ok {
		return data, data != nil
	}
	data, err := in.gitBytes(nil, "cat-file", "blob", ref)
	if err != nil {
		data = nil
	}
	in.blobs[ref] = data
	return data, data != nil
}

// planField keeps a plan field free of the TAB, newline and other control
// bytes the plan format refuses.
func planField(value string) string {
	var out strings.Builder
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			out.WriteByte('?')
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
