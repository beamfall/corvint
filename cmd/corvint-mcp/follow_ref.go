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
	"github.com/Beamfall/corvint/internal/rootalias"
)

// Follow-ref binding (MCPV0-039 to MCPV0-041, V1-1603; per root MMR-V0-010, MMR-V0-011).
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

// followSelector is one --follow-ref value: REF for the one plain root, whose
// alias is empty, or ALIAS=REF for the aliased root ALIAS (MMR-V0-010). A
// value is aliased only when the text before its first '=' is a valid alias,
// so a ref whose name holds '=' is spelled in full (refs/heads/NAME).
type followSelector struct{ alias, ref string }

// Reasons for refusing the --follow-ref arguments (MMR-V0-011); each follows
// "corvint-mcp: invalid arguments: " on stderr.
const (
	followReasonMissingValue     = "--follow-ref needs a value"
	followReasonValue            = "--follow-ref takes REF or ALIAS=REF, where REF is at most 256 bytes with no leading -, whitespace or NUL"
	followReasonVersion          = "--follow-ref cannot be combined with --version"
	followReasonRepeated         = "--follow-ref REF is given more than once; a plain --root follows one ref"
	followReasonBareWithAliases  = "--follow-ref REF needs exactly one plain --root; with aliased roots name the root as --follow-ref ALIAS=REF"
	followReasonAliasedWithPlain = "--follow-ref ALIAS=REF needs aliased --root ALIAS=ABSOLUTE_ROOT roots; a plain --root takes --follow-ref REF, and a ref whose name holds '=' is spelled in full (refs/heads/NAME)"
)

// extractFollowRef removes every `--follow-ref REF` or `--follow-ref
// ALIAS=REF` selector (MCPV0-039, MMR-V0-010). A missing or malformed value, a
// repeated bare REF or alias, or a selector beside --version is refused with
// its reason before the roots are read.
func extractFollowRef(arguments []string) (remaining []string, selectors []followSelector, reason string) {
	remaining = make([]string, 0, len(arguments))
	seen := map[string]bool{}
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--follow-ref" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if index+1 == len(arguments) {
			return nil, nil, followReasonMissingValue
		}
		value := arguments[index+1]
		// The bound applies to REF alone; an alias is bounded by rootalias.Valid,
		// as for --root (MMR-V0-010).
		alias, ref, _ := rootalias.Split(value)
		if !validFollowRef(ref) {
			return nil, nil, followReasonValue
		}
		if seen[alias] && alias == "" {
			return nil, nil, followReasonRepeated
		}
		if seen[alias] {
			return nil, nil, "--follow-ref repeats repository alias " + alias
		}
		seen[alias] = true
		selectors = append(selectors, followSelector{alias: alias, ref: ref})
		index++
	}
	if len(selectors) != 0 && slices.Contains(remaining, "--version") {
		return nil, nil, followReasonVersion
	}
	return remaining, selectors, ""
}

// bindFollowSelectors returns the ref each root follows, keyed by alias (the
// plain root's alias is empty). A bare REF needs exactly one plain root, and
// ALIAS=REF needs ALIAS among the declared aliased roots (MMR-V0-010).
func bindFollowSelectors(roots []rootDeclaration, selectors []followSelector) (map[string]string, string) {
	if len(selectors) == 0 {
		return nil, ""
	}
	plain := len(roots) == 1 && roots[0].alias == ""
	refs := make(map[string]string, len(selectors))
	for _, selector := range selectors {
		switch {
		case selector.alias == "" && !plain:
			return nil, followReasonBareWithAliases
		case selector.alias != "" && plain:
			return nil, followReasonAliasedWithPlain
		case selector.alias != "" && !slices.ContainsFunc(roots, func(root rootDeclaration) bool { return root.alias == selector.alias }):
			return nil, "--follow-ref " + selector.alias + "=" + selector.ref + " names undeclared repository alias " + selector.alias
		}
		refs[selector.alias] = selector.ref
	}
	return refs, ""
}

func validFollowRef(ref string) bool {
	return ref != "" && len(ref) <= 256 && !strings.HasPrefix(ref, "-") && !strings.ContainsAny(ref, "\x00\n\r ")
}

// followRef keeps a dedicated detached worktree on the commit a ref names.
// Calls are serialized through bound: the move, its index refresh and the
// answer that follows them run under one lock, so a concurrent call cannot
// move the worktree under an answer.
type followRef struct {
	mu  sync.Mutex
	git string
	ref string
	// alias names the root a multi-root binding follows (MMR-V0-010); it is
	// empty for the plain root.
	alias   string
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
	// lockHook, when set, runs in bound just before the lock is taken; tests
	// use it to observe a caller waiting on this binding.
	lockHook func()
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

// openFollowRefs binds each aliased root a ref names, in alias order, under
// the newFollowRef startup contract (MMR-V0-010). A refusal names the alias.
func openFollowRefs(ctx context.Context, git string, repositories map[string]*bridge.Registry, aliases []string, refs map[string]string) (map[string]*followRef, string) {
	if len(refs) == 0 {
		return nil, ""
	}
	follows := make(map[string]*followRef, len(refs))
	for _, alias := range aliases {
		ref, declared := refs[alias]
		if !declared {
			continue
		}
		follow, refusal := newFollowRef(ctx, git, repositories[alias], ref)
		if refusal != "" {
			return nil, refusal + ": repository " + alias
		}
		follow.alias = alias
		follows[alias] = follow
	}
	return follows, ""
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
	if f.lockHook != nil {
		f.lockHook()
	}
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
// response; a multi-root binding also names its repository alias (MMR-V0-010).
func (s followState) block(alias, ref string) map[string]any {
	value := map[string]any{
		"schema": followRefSchema, "ref": ref, "commit": s.Commit, "head": s.Head,
		"state": s.State, "stale": s.Stale, "mutates": s.Mutated,
	}
	if alias != "" {
		value[repositoryArgument] = alias
	}
	raw, err := gokernel.CanonicalJSON(value)
	if err != nil {
		panic(err)
	}
	return map[string]any{"type": "text", "text": string(raw)}
}

// withFollowRef appends the follow's block to a tool result or tool error.
func withFollowRef(state *followState, follow *followRef) func(map[string]any, *protocol.RPCError) (map[string]any, *protocol.RPCError) {
	return func(response map[string]any, failure *protocol.RPCError) (map[string]any, *protocol.RPCError) {
		if state == nil || response == nil {
			return response, failure
		}
		content, _ := response["content"].([]any)
		response["content"] = append(content, state.block(follow.alias, follow.ref))
		return response, failure
	}
}

// followRefToolNotice is appended to the description of every tool a call
// binds to --follow-ref (MCPV0-114).
const followRefToolNotice = " Under --follow-ref each call first moves this dedicated worktree to the ref's commit (git checkout --detach) and reindexes it; a dirty or branch-bound worktree is never modified."

// followRefTools returns the advertised tools with every tool a call binds
// to the ref marked as a writer of the dedicated worktree: readOnlyHint is
// false and the description states the checkout and reindex. The move never
// overwrites a dirty tree or ignored file (MCPV0-040), so destructiveHint
// stays false, and a repeated call against an unchanged ref changes nothing,
// so idempotentHint stays true. corvint.help never binds the ref and keeps
// its read-only annotation (MCPV0-100). The input descriptors are not
// modified (MCPV0-114, V1-3030).
func followRefTools(tools []bridge.ToolDescriptor) []bridge.ToolDescriptor {
	return markFollowRefTools(tools, followRefToolNotice)
}

// followRefRootsTools is followRefTools for multi-root mode: a call binds to a
// ref only when it names a following alias, so the sentence names those
// aliases in byte order (MMR-V0-010).
func followRefRootsTools(tools []bridge.ToolDescriptor, aliases []string) []bridge.ToolDescriptor {
	return markFollowRefTools(tools, " Under --follow-ref a call naming repository "+strings.Join(aliases, " or ")+
		" first moves that repository's dedicated worktree to its ref's commit (git checkout --detach) and reindexes it; a dirty or branch-bound worktree is never modified.")
}

func markFollowRefTools(tools []bridge.ToolDescriptor, notice string) []bridge.ToolDescriptor {
	bound := make([]bridge.ToolDescriptor, len(tools))
	for i, tool := range tools {
		if tool.Name != bridge.ToolHelp {
			tool.Annotations.ReadOnlyHint = false
			tool.Description += notice
		}
		bound[i] = tool
	}
	return bound
}
