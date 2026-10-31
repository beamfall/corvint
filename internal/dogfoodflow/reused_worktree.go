// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tracemigraterepo"
)

// staleOCMPatterns names the ignored OCM sidecars a dogfood run writes beside
// the change map: the numbered maps, their status and the intents list.
// The patterns match basenames read from .corvint, so a worktree path holding
// glob syntax cannot hide them.
var staleOCMPatterns = []string{"change.ocm.[0-9][0-9][0-9].json", "change.ocm-status.json", "change.ocm-intents"}

// requireFreshWorktree refuses, before any prepare or bind, a worktree whose
// private dogfood state was left by an earlier lane at the same path
// (DCW-V0-400, V1-3211): a worktree-local trace that names a commit no longer
// reachable from HEAD, or an ignored OCM sidecar last written before BASE was
// committed. Either one makes the later rebind refuse
// unsupported-query-trace-state or record-failed only after the bind commit.
// The run first retires that state itself (DCW-V0-406): it moves the sidecars,
// byte-preserved, into a new directory under the Git directory and quarantines
// the stranded traces with migrate-traces --apply and the digest of the dry run
// it just made, then continues. That apply runs only when the dry-run plan
// holds nothing but stranded traces: a plan that would also rekey reachable
// tree-keyed legacy traces is left to the operator. It refuses only when
// something is still stale after that, naming the exact manual recovery.
// Nothing is deleted. A trace store that cannot be evaluated is left to the
// steps that already report it. The dogfood inputs-check pre-flight passes
// retire=false: it only checks, and still refuses with the manual recovery
// (DCW-V0-400). The refusal names the reset-after-bind case, a lane that reset
// its own unpushed bind/seal commits, and its one-step recovery (DCW-V0-844).
func (f *flow) requireFreshWorktree(base, gitDir string, retire bool) {
	stranded, digest, legacy := f.strandedTraces()
	stale := f.staleOCMSidecars(base)
	if len(stranded) == 0 && len(stale) == 0 {
		return
	}
	// DCW-V0-810: the pre-flight runs no OCM step, and finish-lane moves stale
	// sidecars aside byte-preserved before it prepares, so sidecars alone are
	// a note there; a stranded trace, whose retirement is a migration, still
	// refuses.
	if !retire && len(stranded) == 0 {
		for _, name := range stale {
			f.say("%s: NOTE stale sidecar %s is ignored and was last written before BASE %s was committed; dogfood finish-lane moves it under %s/corvint/stale-ocm before it prepares (DCW-V0-406, DCW-V0-810)\n", f.prefix, name, base[:12], gitDir)
		}
		return
	}
	if retire {
		if legacy > 0 {
			digest = ""
		}
		f.retireStaleState(base, gitDir, stranded, digest, stale)
		stranded, digest, legacy = f.strandedTraces()
		stale = f.staleOCMSidecars(base)
		if len(stranded) == 0 && len(stale) == 0 {
			return
		}
	}
	f.say("%s: REFUSE reused-worktree-state\n", f.prefix)
	f.say("  this worktree path holds dogfood state from an earlier lane, an amended/rebased commit, or this lane's own bind/seal commits reset away before push, that could not be retired; nothing was prepared or bound\n")
	for _, revision := range stranded {
		f.say("  stale trace: the worktree-local trace store names %s, which HEAD does not reach (LTPM-V0-012)\n", revision)
	}
	for _, name := range stale {
		f.say("  stale sidecar: %s is ignored and was last written before BASE %s was committed\n", name, base[:12])
	}
	if len(stale) > 0 {
		aside := gitDir + "/corvint/stale-ocm"
		f.say("  fix: mkdir -p %s && mv %s %s/\n", shellQuote(aside), strings.Join(quoteAll(stale), " "), shellQuote(aside))
	}
	if retire && len(stranded) > 0 && legacy > 0 {
		f.say("  the same migration plan also rekeys %d reachable tree-keyed legacy trace(s), so it was not applied automatically (DCW-V0-406)\n", legacy)
	}
	if len(stranded) > 0 {
		f.say("  fix: corvint migrate-traces --apply --plan-digest %s (the digest of corvint migrate-traces --dry-run now; it quarantines the stranded traces byte-preserved). This is also the one-step recovery after a reset of this lane's own unpushed bind/seal commits (DCW-V0-844)\n", digest)
	}
	f.say("  then rerun the same command; each lane should use a fresh worktree (git worktree add) rather than a reused path (DCW-V0-044)\n")
	f.exit(2)
}

// strandedTraces returns the revisions the worktree-local trace store names
// that HEAD does not reach, with the dry-run plan digest that retires them and
// the number of reachable legacy traces that same plan would rekey; a store
// that cannot be evaluated yields none.
func (f *flow) strandedTraces() ([]string, string, int) {
	result, err := tracemigraterepo.Evaluate(f.ctx, f.root, tracemigraterepo.Options{})
	if err != nil {
		return nil, "", 0
	}
	return result.StrandedRevisions, result.PlanDigest, len(result.Entries)
}

// retireStaleState moves each stale sidecar into a fresh directory under
// <git-dir>/corvint/stale-ocm, so no earlier aside is overwritten, and applies
// the trace migration whose dry-run digest is digest, unless digest is empty.
// Each retirement it completes is printed; one that fails or is skipped is
// left for the caller's refusal.
func (f *flow) retireStaleState(base, gitDir string, stranded []string, digest string, stale []string) {
	if len(stale) > 0 {
		parent := filepath.Join(gitDir, "corvint", "stale-ocm")
		aside := ""
		if err := os.MkdirAll(parent, 0o777); err == nil {
			aside, _ = os.MkdirTemp(parent, base[:12]+"-")
		}
		for _, name := range stale {
			if aside == "" || os.Rename(filepath.Join(f.root, filepath.FromSlash(name)), filepath.Join(aside, filepath.Base(name))) != nil {
				continue
			}
			f.say("%s: RETIRE stale sidecar %s moved to %s/ (DCW-V0-406)\n", f.prefix, name, filepath.ToSlash(aside))
		}
	}
	if len(stranded) > 0 && digest != "" {
		if _, err := tracemigraterepo.Evaluate(f.ctx, f.root, tracemigraterepo.Options{Apply: true, PlanDigest: &digest}); err == nil {
			for _, revision := range stranded {
				f.say("%s: RETIRE stale trace %s quarantined byte-preserved to .context-corvint/legacy-traces/ by migrate-traces --apply --plan-digest %s (DCW-V0-406, LTPM-V0-012)\n", f.prefix, revision, digest)
			}
		}
	}
}

// staleOCMSidecars lists, sorted and root-relative, each ignored untracked OCM
// sidecar whose modification time is before BASE's committer time: no run on
// that base can have written it.
func (f *flow) staleOCMSidecars(base string) []string {
	seconds, err := strconv.ParseInt(f.gitValue("show", "-s", "--format=%ct", base), 10, 64)
	if err != nil {
		return nil
	}
	committed := time.Unix(seconds, 0)
	var stale []string
	entries, _ := os.ReadDir(filepath.Join(f.root, ".corvint"))
	for _, entry := range entries {
		if !slices.ContainsFunc(staleOCMPatterns, func(pattern string) bool {
			matched, _ := path.Match(pattern, entry.Name())
			return matched
		}) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().Before(committed) {
			continue
		}
		name := ".corvint/" + entry.Name()
		if f.gitSucceeds("ls-files", "--error-unmatch", "--", name) || !f.gitSucceeds("check-ignore", "-q", "--", name) {
			continue
		}
		stale = append(stale, name)
	}
	sort.Strings(stale)
	return stale
}

// shellQuote single-quotes value for a POSIX shell when it holds anything but
// path-safe bytes, so a printed fix line pastes as one word per path.
func shellQuote(value string) string {
	if value != "" && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./") == "" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = shellQuote(value)
	}
	return quoted
}
