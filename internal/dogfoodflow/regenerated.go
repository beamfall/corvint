// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// batchRegenSubject is the subject script/batch-regen.sh commits under
// (ASD-V0-033); only a commit carrying it is replayed with the full run.
const batchRegenSubject = "chore(batch): regenerate derived files (make batch-regen, ASD-V0-033)"

// regenerationReplayLimit bounds how many commits one run replays, and
// regenerationReplayTimeout bounds each replay (DCW-V0-397).
const (
	regenerationReplayLimit   = 8
	regenerationReplayTimeout = 15 * time.Minute
)

// regeneration is the outcome of replaying one commit's regeneration.
type regeneration struct {
	ok bool
	// mode is the batch-regen.sh mode whose output equals the commit's tree.
	mode string
	// reason is why the commit is not regenerated, for the NOTE line.
	reason string
	// silent marks a commit never replayed because it cannot be a
	// regeneration: its parent has no batch-regen.sh or it changes Go source.
	silent bool
}

// regenerated reports whether commit is exactly what script/batch-regen.sh
// produces from parent (DCW-V0-397, V1-2981, V1-3214): a private clone outside
// the repository, reading its objects as an alternate, checks parent out
// detached, runs the parent's own script/batch-regen.sh --mechanical (and,
// for a commit under the batch-regen subject, the full --no-commit run), stages
// every change and compares the staged tree with commit's tree. Nothing in the
// repository, its index or its refs is written. A replay that cannot run, times
// out, or produces any other tree is not regenerated; a commit with one hand
// edit beside its repins therefore stays unbound and owes its citations.
func (f *flow) regenerated(parent, commit string) regeneration {
	if f.replays == nil {
		f.replays = map[string]regeneration{}
	}
	if known, seen := f.replays[commit]; seen {
		return known
	}
	if f.replayCount >= regenerationReplayLimit {
		return regeneration{reason: "replay-limit"}
	}
	result := f.replay(parent, commit)
	if !result.silent {
		f.replayCount++
	}
	f.replays[commit] = result
	return result
}

func (f *flow) replay(parent, commit string) regeneration {
	if !f.gitSucceeds("cat-file", "-e", parent+":script/batch-regen.sh") {
		return regeneration{reason: "no-batch-regen-script", silent: true}
	}
	// No regeneration step writes Go source, so such a commit is not replayed.
	for _, path := range nonEmpty(strings.Split(f.gitValue("diff-tree", "-r", "--no-commit-id", "--name-only", "--no-renames", parent, commit), "\n")) {
		if strings.HasSuffix(path, ".go") {
			return regeneration{reason: "go-source-changed", silent: true}
		}
	}
	tree := f.gitValue("rev-parse", commit+"^{tree}")
	modes := []string{"--mechanical"}
	if f.gitValue("show", "-s", "--format=%s", commit) == batchRegenSubject {
		modes = append(modes, "--no-commit")
	}
	scratch, err := os.MkdirTemp("", "corvint-regen-replay-")
	if err != nil {
		return regeneration{reason: "scratch-unavailable"}
	}
	defer os.RemoveAll(scratch)
	if resolved, err := filepath.EvalSymlinks(scratch); err == nil {
		scratch = resolved
	}
	if root, err := filepath.EvalSymlinks(f.root); err == nil && strings.HasPrefix(scratch+string(filepath.Separator), root+string(filepath.Separator)) {
		return regeneration{reason: "scratch-inside-worktree"}
	}
	clone := filepath.Join(scratch, "replay")
	objects := f.gitValue("rev-parse", "--path-format=absolute", "--git-path", "objects")
	if f.replayGit(scratch, "init", "-q", "--template=", clone) != nil ||
		os.WriteFile(filepath.Join(clone, ".git", "objects", "info", "alternates"), []byte(objects+"\n"), 0o600) != nil {
		return regeneration{reason: "scratch-unavailable"}
	}
	for _, mode := range modes {
		if f.replayGit(clone, "checkout", "-q", "-f", "--detach", parent) != nil || f.replayGit(clone, "clean", "-q", "-f", "-d", "-x") != nil {
			return regeneration{reason: "checkout-failed"}
		}
		ctx, cancel := context.WithTimeout(f.ctx, regenerationReplayTimeout)
		script := exec.CommandContext(ctx, "bash", "script/batch-regen.sh", mode)
		script.Dir = clone
		script.Env = append(withoutGitEnvironment(os.Environ()), "BATCH_REGEN_ROOT="+clone, "GIT_CEILING_DIRECTORIES="+scratch)
		failed := script.Run() != nil
		cancel()
		f.checkpoint()
		if failed {
			continue
		}
		if f.replayGit(clone, "add", "-A") != nil {
			continue
		}
		if staged, err := f.replayOutput(clone, "write-tree"); err == nil && staged == tree {
			return regeneration{ok: true, mode: mode}
		}
	}
	return regeneration{reason: "tree-differs"}
}

// replayGit and replayOutput run Git in the private replay clone with no
// configuration, hooks or credential helper of the user's.
func (f *flow) replayGit(dir string, args ...string) error {
	_, err := f.replayOutput(dir, args...)
	return err
}

func (f *flow) replayOutput(dir string, args ...string) (string, error) {
	f.checkpoint()
	command := exec.CommandContext(f.ctx, "git", append([]string{"-c", "credential.helper=", "-c", "core.hooksPath=" + os.DevNull}, args...)...)
	command.Dir = dir
	command.Env = gokernel.SanitizedGitEnvironment()
	output, err := command.Output()
	return chomp(string(output)), err
}

// withoutGitEnvironment drops every GIT_* variable, so the replayed script's
// Git commands address the replay clone and never the checked repository.
func withoutGitEnvironment(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GIT_") {
			kept = append(kept, entry)
		}
	}
	return kept
}
