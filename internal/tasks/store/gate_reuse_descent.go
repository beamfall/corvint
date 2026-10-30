package store

import (
	"strconv"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// maxProtectedCommits bounds the intent branch history a descendant reuse
// reads; a tree older than this is not found, so the reuse is refused.
const maxProtectedCommits = 100000

// maxDescentPairs bounds the ancestry checks between commits that carry the
// candidate tree and commits that carry the source tree.
const maxDescentPairs = 16

// protectedHistory maps each tree on the protected intent branch, and on its
// configured remote-tracking upstream, to the commits that carry it
// (TCD-V0-151). It never fetches and never writes a ref.
type protectedHistory struct {
	root    string
	trees   map[string][]string
	commits map[string]string
}

// loadProtectedHistory reads the intent branch history once. A branch with
// no local ref and no upstream yields an empty history, which refuses every
// descendant reuse.
func loadProtectedHistory(root, branch string) (*protectedHistory, error) {
	h := &protectedHistory{root: root, trees: map[string][]string{}, commits: map[string]string{}}
	if branch == "" {
		return h, nil
	}
	local := "refs/heads/" + branch
	var refs []string
	if _, err := gitOutput(root, "show-ref", "--verify", "--quiet", "--end-of-options", local); err == nil {
		refs = append(refs, local)
	}
	if up := configuredUpstream(root, local); up != "" {
		refs = append(refs, up)
	}
	if len(refs) == 0 {
		return h, nil
	}
	args := append([]string{"log", "--format=%H %T", "-n", strconv.Itoa(maxProtectedCommits), "--end-of-options"}, refs...)
	out, err := gitOutput(root, args...)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git log of the intent branch failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		commit, tree, ok := strings.Cut(line, " ")
		if ok {
			h.trees[tree] = append(h.trees[tree], commit)
			h.commits[commit] = tree
		}
	}
	return h, nil
}

// descent is "" when a commit carrying seal is an ancestor of, or is, a
// commit carrying pass on the protected history; otherwise it names the
// refusal: the source is not on the protected branch, or the candidate is
// not its ancestor (TCD-V0-152).
func (h *protectedHistory) descent(seal, pass string) (string, error) {
	passCommits := h.trees[pass]
	if len(passCommits) == 0 {
		return "the reused result's tree " + pass + " is not a commit on the protected intent branch", nil
	}
	checked := 0
	for _, s := range h.trees[seal] {
		for _, c := range passCommits {
			if checked == maxDescentPairs {
				break
			}
			checked++
			ok, err := isAncestor(h.root, s, c)
			if err != nil {
				return "", err
			}
			if ok {
				return "", nil
			}
		}
	}
	return "the candidate tree " + seal + " is not an ancestor of the reused result's tree " + pass + " on the protected intent branch", nil
}

// commitDescent is "" when commit is on the protected history carrying tree
// and a commit carrying seal is an ancestor of, or is, commit; otherwise it
// names the refusal (TCD-V0-155). A CI run is bound to its head commit, so
// a rewound branch that dropped that commit refuses even when another commit
// still carries the same tree.
func (h *protectedHistory) commitDescent(seal, commit, tree string) (string, error) {
	if h.commits[commit] != tree {
		return "the CI run's head commit " + commit + " is not a commit carrying tree " + tree + " on the protected intent branch", nil
	}
	for i, s := range h.trees[seal] {
		if i == maxDescentPairs {
			break
		}
		ok, err := isAncestor(h.root, s, commit)
		if err != nil {
			return "", err
		}
		if ok {
			return "", nil
		}
	}
	return "the candidate tree " + seal + " is not an ancestor of the CI run's head commit " + commit + " on the protected intent branch", nil
}

// descentRoots maps a store's state directory to the git checkout its
// receipt audit reads ancestry from when that is not the primary worktree;
// only package tests set it.
var descentRoots sync.Map

// descentRoot is the checkout whose intent branch the receipt audit reads.
func descentRoot(repo *intent.Repository) string {
	if v, ok := descentRoots.Load(repo.StateDir); ok {
		return v.(string)
	}
	return repo.PrimaryWorktree
}

// descentOracle answers the receipt audit's ancestry question from git at
// root, reading each branch's history once per audit (TCD-V0-153).
func descentOracle(root string) func(seal, pass, branch string) (string, error) {
	cache := map[string]*protectedHistory{}
	return func(seal, pass, branch string) (string, error) {
		h, ok := cache[branch]
		if !ok {
			var err error
			if h, err = loadProtectedHistory(root, branch); err != nil {
				return "", err
			}
			cache[branch] = h
		}
		return h.descent(seal, pass)
	}
}

// ciDescentOracle answers the receipt audit's CI head-commit question from
// git at root, reading each branch's history once per audit (TCD-V0-157).
func ciDescentOracle(root string) func(seal, commit, tree, branch string) (string, error) {
	cache := map[string]*protectedHistory{}
	return func(seal, commit, tree, branch string) (string, error) {
		h, ok := cache[branch]
		if !ok {
			var err error
			if h, err = loadProtectedHistory(root, branch); err != nil {
				return "", err
			}
			cache[branch] = h
		}
		return h.commitDescent(seal, commit, tree)
	}
}

// intentBranch is the queue's declared intent branch, "" when unreadable.
func intentBranch(proof *journal.Result) string {
	q, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return ""
	}
	return q.IntentBranch
}

// descendantResults rechecks, for complete, every reused result the attempt
// names at a tree other than its candidate (TCD-V0-151); results at the
// candidate need no ancestry and are not listed.
func descendantResults(root string, proof *journal.Result, a *snapshot.Attempt, results map[wire.Digest][]byte) (map[wire.Digest]bool, error) {
	out := map[wire.Digest]bool{}
	if a == nil || a.CandidateTreeOid == nil {
		return out, nil
	}
	var h *protectedHistory
	for d, raw := range results {
		g, err := snapshot.DecodeGateResult(raw)
		if err != nil || g.ReusedFrom == nil || g.CandidateTreeOid == *a.CandidateTreeOid {
			continue
		}
		if h == nil {
			if h, err = loadProtectedHistory(root, intentBranch(proof)); err != nil {
				return nil, err
			}
		}
		why, err := h.descent(*a.CandidateTreeOid, g.CandidateTreeOid)
		if err != nil {
			return nil, err
		}
		out[d] = why == ""
	}
	return out, nil
}
