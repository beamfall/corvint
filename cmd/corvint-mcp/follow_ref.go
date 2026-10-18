package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// Follow-ref binding (MCPV0-039 to MCPV0-041, V1-1603).
const (
	followRefSchema = "corvint-mcp-follow-ref/0"
	// followRefMissing is the typed tool error and startup refusal for a ref
	// that does not resolve to a commit.
	followRefMissing = "follow-ref-missing"
	// followRefNotDedicated refuses a root that is not a linked worktree with
	// a detached HEAD.
	followRefNotDedicated = "follow-ref-not-dedicated"
	followRefGitTimeout   = 30 * time.Second
	followRefMoveTimeout  = 2 * time.Minute
)

// extractFollowRef removes the optional `--follow-ref REF` selector under the
// same closed rules as the other selectors.
func extractFollowRef(arguments []string) (remaining []string, ref string, ok bool) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--follow-ref" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if ref != "" || index+1 == len(arguments) || !validFollowRef(arguments[index+1]) {
			return nil, "", false
		}
		ref = arguments[index+1]
		index++
	}
	if ref != "" && slices.Contains(remaining, "--version") {
		return nil, "", false
	}
	return remaining, ref, true
}

func validFollowRef(ref string) bool {
	return ref != "" && len(ref) <= 256 && !strings.HasPrefix(ref, "-") && !strings.ContainsAny(ref, "\x00\n\r ")
}

// followRef keeps a dedicated detached worktree on the commit a ref names.
// Calls are serialized through bound: the move, its index refresh and the
// answer that follows them run under one lock, so a concurrent call cannot
// move the worktree under an answer.
type followRef struct {
	mu      sync.Mutex
	git     string
	ref     string
	indexed string
	// pinned is the registry's canonical root and its identities; every Git
	// command runs in pinned.Root() and nothing is mutated unless
	// pinned.SameRoot() still holds.
	pinned interface {
		Root() string
		SameRoot() bool
	}
	// refresh brings the index snapshot of root up to date and reports
	// whether it wrote one; tests replace it.
	refresh func(context.Context, string) (bool, error)
}

// followState is one call's binding: the commit the ref resolves to, the
// worktree head the answer describes, and whether they differ.
type followState struct {
	Commit, Head, State string
	Stale, Mutated      bool
}

// newFollowRef checks the startup contract against the registry's pinned
// canonical root: it is a linked worktree whose HEAD is detached, and ref
// resolves to a commit.
func newFollowRef(ctx context.Context, git string, registry *bridge.Registry, ref string) (*followRef, string) {
	f := &followRef{git: git, ref: ref, pinned: registry, refresh: refreshIndex}
	if marker, err := os.Lstat(filepath.Join(registry.Root(), ".git")); err != nil || !marker.Mode().IsRegular() || !registry.SameRoot() {
		return nil, followRefNotDedicated
	}
	if _, code, err := f.run(ctx, followRefGitTimeout, "symbolic-ref", "-q", "HEAD"); err != nil || code != 1 {
		return nil, followRefNotDedicated
	}
	if _, ok := f.resolve(ctx); !ok {
		return nil, followRefMissing
	}
	return f, ""
}

// run runs Git in the pinned root with the Core sanitized environment, so no
// inherited GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE or object override can
// redirect a read or the checkout to another repository.
func (f *followRef) run(ctx context.Context, bound time.Duration, arguments ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	command := exec.CommandContext(ctx, f.git, append([]string{"-C", f.pinned.Root(), "-c", "core.hooksPath=" + os.DevNull}, arguments...)...)
	command.Env = gokernel.SanitizedGitEnvironment()
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && ctx.Err() == nil {
		return stdout.Bytes(), exit.ExitCode(), nil
	}
	if err != nil {
		return nil, -1, err
	}
	return stdout.Bytes(), 0, nil
}

func (f *followRef) resolve(ctx context.Context) (string, bool) {
	out, code, err := f.run(ctx, followRefGitTimeout, "rev-parse", "--verify", "--quiet", "--end-of-options", f.ref+"^{commit}")
	if err != nil || code != 0 {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// bound syncs the worktree to the ref and produces the answer under one lock.
func (f *followRef) bound(ctx context.Context, answer func(followState, string) (map[string]any, *protocol.RPCError)) (map[string]any, *protocol.RPCError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, code := f.sync(ctx)
	return answer(state, code)
}

// sync binds the worktree to the ref before an answer; the caller holds f.mu.
// The root identity, a detached HEAD and a clean tree are checked on every
// call, whether or not the ref moved. A moved ref on a clean, detached
// worktree checks the new commit out without overwriting ignored files and
// refreshes the index; a changed root, a dirty or attached worktree, or a
// checkout collision is never modified and the answer is flagged stale. It
// returns followRefMissing when the ref no longer resolves.
func (f *followRef) sync(ctx context.Context) (followState, string) {
	state := followState{State: "CURRENT"}
	if out, code, err := f.run(ctx, followRefGitTimeout, "rev-parse", "--verify", "--quiet", "HEAD"); err == nil && code == 0 {
		state.Head = strings.TrimSpace(string(out))
	}
	commit, ok := f.resolve(ctx)
	if !ok {
		state.State, state.Stale = "MISSING", true
		return state, followRefMissing
	}
	state.Commit = commit
	if !f.pinned.SameRoot() {
		state.State, state.Stale = "ROOT_CHANGED", true
		return state, ""
	}
	if _, code, err := f.run(ctx, followRefGitTimeout, "symbolic-ref", "-q", "HEAD"); err != nil || code != 1 {
		state.State, state.Stale = "NOT_DETACHED", true
		return state, ""
	}
	status, code, err := f.run(ctx, followRefGitTimeout, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil || code != 0 || len(status) != 0 {
		state.State, state.Stale = "DIRTY", true
		return state, ""
	}
	if state.Head != commit {
		if !f.pinned.SameRoot() {
			state.State, state.Stale = "ROOT_CHANGED", true
			return state, ""
		}
		if _, code, err := f.run(ctx, followRefMoveTimeout, "checkout", "--detach", "--quiet", "--no-overwrite-ignore", commit); err != nil || code != 0 {
			state.State, state.Stale = "CHECKOUT_FAILED", true
			return state, ""
		}
		state.Head, state.State, state.Mutated = commit, "REFRESHED", true
	}
	if f.indexed != commit {
		if !f.pinned.SameRoot() {
			state.State, state.Stale = "ROOT_CHANGED", true
			return state, ""
		}
		wrote, err := f.refresh(ctx, f.pinned.Root())
		state.Mutated = state.Mutated || wrote
		if err != nil {
			state.State, state.Stale = "INDEX_FAILED", true
			return state, ""
		}
		f.indexed = commit
	}
	return state, ""
}

// refreshIndex rebuilds the root's index snapshot unless a fresh one exists,
// as `corvint index --if-stale` does, in process.
func refreshIndex(ctx context.Context, root string) (bool, error) {
	if _, fresh, err := contextindex.ProbeSnapshot(ctx, root); err == nil && fresh {
		return false, nil
	}
	index, err := contextindex.BuildForSnapshot(ctx, root)
	if err != nil {
		return false, err
	}
	if _, err := contextindex.WriteSnapshot(index); err != nil {
		return false, err
	}
	return true, nil
}

// block is the server-authored follow-ref content block echoed in every
// response.
func (s followState) block(ref string) map[string]any {
	raw, err := gokernel.CanonicalJSON(map[string]any{
		"schema": followRefSchema, "ref": ref, "commit": s.Commit, "head": s.Head,
		"state": s.State, "stale": s.Stale, "mutates": s.Mutated,
	})
	if err != nil {
		panic(err)
	}
	return map[string]any{"type": "text", "text": string(raw)}
}

// withFollowRef appends the follow-ref block to a tool result or tool error.
func withFollowRef(state *followState, ref string) func(map[string]any, *protocol.RPCError) (map[string]any, *protocol.RPCError) {
	return func(response map[string]any, failure *protocol.RPCError) (map[string]any, *protocol.RPCError) {
		if state == nil || response == nil {
			return response, failure
		}
		content, _ := response["content"].([]any)
		response["content"] = append(content, state.block(ref))
		return response, failure
	}
}
