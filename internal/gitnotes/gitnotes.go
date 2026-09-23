// Package gitnotes anchors a committed Change Evidence Map to a commit in a
// Corvint notes ref and reads Git-native provenance beside it: the Git AI
// `refs/notes/ai` authorship log and the `Assisted-by` / `Agent-Logs-Url`
// commit trailers (FPK-V0-037..040, decision 0355).
//
// Every row it reports is repository-history: a relation read from commit
// history, never project authority (AGENTS.md invariant 3). Foreign text is
// carried only inside a row's bounded `untrusted` member, and no URL it holds
// is ever fetched (invariant 7). Only Anchor writes, and it writes only the
// Corvint notes ref.
package gitnotes

import (
	"context"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Frozen refs, labels, and bounds.
const (
	// Ref is the notes ref Anchor writes and Provenance reads the pointer from.
	Ref = "refs/notes/corvint"
	// ForeignRef is the Git AI authorship-log notes ref, read only.
	ForeignRef = "refs/notes/ai"
	// PointerSchema names the one pointer shape Anchor writes.
	PointerSchema = "corvint-cem-anchor/0"
	// Trust is contextindex.TrustRepositoryHistory: every row here is a
	// relation read from history, never proof and never project authority.
	Trust = "repository-history"
	// Authority is the row label that contextindex.TrustClass derives Trust from.
	Authority = "git-history"

	KindAnchor       = "cem-anchor-note"
	KindAINote       = "git-ai-authorship-note"
	KindAssistedBy   = "assisted-by-trailer"
	KindAgentLogsURL = "agent-logs-url-trailer"

	MaxNoteBytes     = 1 << 20  // one note body
	MaxListBytes     = 4 << 20  // one `git notes list`
	MaxTrailerBytes  = 64 << 10 // one commit's trailer block
	MaxTextBytes     = 256      // one untrusted string
	MaxListedEntries = 64       // untrusted files, agents, or trailers per commit
)

// Registered failure codes of the anchor mutation.
const (
	CodeMapUncommitted  = "anchor-map-uncommitted"
	CodeMapDirty        = "anchor-map-dirty"
	CodeBlobUnavailable = "anchor-blob-unavailable"
	CodeNoteConflict    = "anchor-note-conflict"
)

// committer is the fixed identity of the notes commit Anchor creates: the
// tool writes the note, so the note history names the tool.
var committer = []string{"-c", "user.name=Corvint", "-c", "user.email=corvint@localhost.invalid"}

type repository struct {
	root   string
	gitDir string
	budget *gitrun.Budget
}

// open validates root with the CEM repository boundary (worktree, reciprocal
// link, no alternates, no un-isolatable attributes).
func open(root string) (*repository, error) {
	budget := gitrun.NewDefaultBudget()
	opened, err := gitauth.Open(root, budget)
	if err != nil {
		return nil, err
	}
	return &repository{root: opened.Root, gitDir: opened.GitDir, budget: budget}, nil
}

func (r *repository) git(ctx context.Context, limit int, stdin []byte, args ...string) ([]byte, error) {
	prefix := []string{
		"--no-optional-locks", "--literal-pathspecs", "--git-dir=" + r.gitDir,
		"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "credential.helper=",
		"-c", "core.quotePath=false", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "log.showSignature=false", "-c", "trailer.separators=:",
	}
	options := gitrun.Options{Dir: r.root, Env: scrubbedEnv(), Stdin: stdin, StdoutLimit: limit}
	return gitrun.Run(ctx, r.budget, options, append(prefix, args...)...)
}

// scrubbedEnv mirrors the CEM seams' allowlist: no user or system config, no
// prompt, no lazy fetch, no replace objects, no inherited notes ref.
func scrubbedEnv() []string {
	result := make([]string, 0, 17)
	for _, key := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	return append(result,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull, "GIT_ASKPASS=",
		"GIT_ATTR_NOSYSTEM=1",
	)
}

// resolveCommit peels rev to one full commit object ID.
func (r *repository) resolveCommit(ctx context.Context, rev string) (string, error) {
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", cemcode.New(cemcode.RepositoryObjectUnavailable, "commit %q does not resolve", rev)
	}
	return strings.TrimSpace(string(out)), nil
}

// noteBlob returns the note blob attached to commit under ref, or "" when the
// ref or the note is absent.
func (r *repository) noteBlob(ctx context.Context, ref, commit string) (string, error) {
	out, err := r.git(ctx, MaxListBytes, nil, "notes", "--ref="+ref, "list")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == commit {
			return fields[0], nil
		}
	}
	return "", nil
}

func (r *repository) blob(ctx context.Context, oid string, limit int) ([]byte, error) {
	return r.git(ctx, limit, nil, "cat-file", "blob", oid)
}
