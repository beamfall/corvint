// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// batchRegenSubject is the subject script/batch-regen.sh commits under
// (ASD-V0-033); only a commit carrying it is replayed with the full run.
const batchRegenSubject = "chore(batch): regenerate derived files (make batch-regen, ASD-V0-033)"

// regenerationReplayLimit bounds how many commits one run replays, and
// regenerationReplayTimeout bounds each commit's replay, every mode together
// (DCW-V0-397).
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
	// A regeneration step writes only the Go source its parent's scripts name,
	// such as the Pi adapter literal a host package bump repins, so a commit
	// changing any other Go file is not replayed.
	for _, path := range nonEmpty(strings.Split(f.gitValue("diff-tree", "-r", "--no-commit-id", "--name-only", "--no-renames", parent, commit), "\n")) {
		if strings.HasSuffix(path, ".go") && !f.gitSucceeds("grep", "-q", "-F", "-e", path, parent, "--", "script/") {
			return regeneration{reason: "go-source-changed", silent: true}
		}
	}
	tree := f.gitValue("rev-parse", commit+"^{tree}")
	modes := []string{"--mechanical"}
	if f.gitValue("show", "-s", "--format=%s", commit) == batchRegenSubject {
		modes = append(modes, "--no-commit")
	}
	// The scratch parent is checked before anything is created in it: no
	// replay writes inside the worktree or its Git directories.
	temporary, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return regeneration{reason: "scratch-unavailable"}
	}
	for _, protected := range []string{f.root, f.gitValue("rev-parse", "--absolute-git-dir"), f.gitValue("rev-parse", "--path-format=absolute", "--git-common-dir")} {
		if resolved, err := filepath.EvalSymlinks(protected); err == nil && strings.HasPrefix(temporary+string(filepath.Separator), resolved+string(filepath.Separator)) {
			return regeneration{reason: "scratch-inside-worktree"}
		}
	}
	scratch, err := os.MkdirTemp(temporary, "corvint-regen-replay-")
	if err != nil {
		return regeneration{reason: "scratch-unavailable"}
	}
	defer os.RemoveAll(scratch)
	clone := filepath.Join(scratch, "replay")
	objects := f.gitValue("rev-parse", "--path-format=absolute", "--git-path", "objects")
	if f.replayGit(scratch, "init", "-q", "--template=", clone) != nil ||
		os.WriteFile(filepath.Join(clone, ".git", "objects", "info", "alternates"), []byte(objects+"\n"), 0o600) != nil {
		return regeneration{reason: "scratch-unavailable"}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return regeneration{reason: "bash-unavailable"}
	}
	deadline := time.Now().Add(regenerationReplayTimeout)
	environment := replayEnvironment(os.Environ(), clone, scratch)
	for _, mode := range modes {
		if f.replayGit(clone, "checkout", "-q", "-f", "--detach", parent) != nil || f.replayGit(clone, "clean", "-q", "-f", "-d", "-x") != nil {
			return regeneration{reason: "checkout-failed"}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return regeneration{reason: "timeout"}
		}
		// The script runs in its own process group, so a timeout or an
		// interruption also stops the builds and tests it started.
		run := procgroup.Run(f.ctx, procgroup.Spec{
			Argv: []string{bash, "script/batch-regen.sh", mode}, Dir: clone, Env: environment,
			Timeout: remaining, ShutdownTimeout: 5 * time.Second,
			OutputLimit: 64 << 10, OverflowPolicy: procgroup.OverflowTruncate,
		})
		f.checkpoint()
		if run.TimedOut {
			return regeneration{reason: "timeout"}
		}
		if run.Err != nil || !run.ExitObserved || run.ExitStatus != 0 {
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

// replayEnvironment is the replayed script's environment: every GIT_*
// variable is dropped, so its Git commands address the replay clone and never
// the checked repository, with no global or system Git configuration; every
// BATCH_REGEN_* override, such as BATCH_REGEN_STEPS replacing the steps, and
// the BASH_ENV and ENV startup files are dropped, so only the parent's own
// script decides what is regenerated.
func replayEnvironment(environment []string, clone, scratch string) []string {
	kept := make([]string, 0, len(environment)+5)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "GIT_") && !strings.HasPrefix(name, "BATCH_REGEN_") && name != "BASH_ENV" && name != "ENV" {
			kept = append(kept, entry)
		}
	}
	return append(kept, "BATCH_REGEN_ROOT="+clone, "GIT_CEILING_DIRECTORIES="+scratch,
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
}
