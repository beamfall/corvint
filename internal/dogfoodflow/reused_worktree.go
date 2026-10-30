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
// (DCW-V0-396, V1-3211): a worktree-local trace that names a commit no longer
// reachable from HEAD, or an ignored OCM sidecar last written before BASE was
// committed. Either one makes the later rebind refuse
// unsupported-query-trace-state or record-failed only after the bind commit.
// The refusal names the exact recovery: the sidecars move aside under the Git
// directory, and migrate-traces retires the stranded traces with the digest
// its dry run printed. A trace store that cannot be evaluated is left to the
// steps that already report it.
func (f *flow) requireFreshWorktree(base, gitDir string) {
	var stranded []string
	digest := ""
	if result, err := tracemigraterepo.Evaluate(f.ctx, f.root, tracemigraterepo.Options{}); err == nil {
		stranded, digest = result.StrandedRevisions, result.PlanDigest
	}
	stale := f.staleOCMSidecars(base)
	if len(stranded) == 0 && len(stale) == 0 {
		return
	}
	f.say("%s: REFUSE reused-worktree-state\n", f.prefix)
	f.say("  this worktree path holds dogfood state from an earlier lane or an amended/rebased commit; nothing was prepared or bound\n")
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
	if len(stranded) > 0 {
		f.say("  fix: corvint migrate-traces --apply --plan-digest %s (the digest of corvint migrate-traces --dry-run now; it quarantines the stranded traces byte-preserved)\n", digest)
	}
	f.say("  then rerun the same command; each lane should use a fresh worktree (git worktree add) rather than a reused path (DCW-V0-044)\n")
	f.exit(2)
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
