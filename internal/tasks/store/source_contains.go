package store

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Source containment states (SCC-V0-001): whether a source revision
// contains a commit by ancestry, by a patch-equivalent commit, or not at all.
// UNKNOWN is the abstention for missing refs, missing history, a failed Git
// read or an exceeded bound; it never stands for ABSENT.
const (
	SourceAncestor   = "ANCESTOR"
	SourceEquivalent = "EQUIVALENT"
	SourceAbsent     = "ABSENT"
	SourceUnknown    = "UNKNOWN"
)

// Reason codes of an ABSENT or UNKNOWN answer (SCC-V0-002, SCC-V0-003).
const (
	SourceReasonPatchDiffers   = "PATCH_DIFFERS"
	SourceReasonNoEquivalent   = "NO_EQUIVALENT"
	SourceReasonMergeCommit    = "MERGE_COMMIT"
	SourceReasonMissingSource  = "MISSING_SOURCE"
	SourceReasonMissingCommit  = "MISSING_COMMIT"
	SourceReasonShallowHistory = "SHALLOW_HISTORY"
	SourceReasonScanBound      = "SCAN_BOUND"
	SourceReasonGitFailed      = "GIT_FAILED"
)

// SourceContainmentScanCommits is the named patch-id scan bound
// (SCC-V0-003): a source with more non-merge commits than this outside the
// commit's history is not scanned and the answer is UNKNOWN.
const SourceContainmentScanCommits = 10000

// sourceContainmentScanBound and sourceContainmentTimeout are variables only
// so tests can exercise the bound and the deadline.
var (
	sourceContainmentScanBound = SourceContainmentScanCommits
	sourceContainmentTimeout   = 60 * time.Second
)

// SourceContainment is one answer of SourceContainmentOf.
type SourceContainment struct {
	// Source and Commit are the resolved object ids, "" when unresolved.
	Source, Commit string
	State          string
	// Equivalent is the source commit whose patch-id equals Commit's.
	Equivalent string
	// PatchDiffers is a source commit with Commit's author and subject, or
	// its Change-Id, whose patch differs (a conflict-resolved rebase).
	PatchDiffers string
	// Reason is the ABSENT or UNKNOWN reason code, "" otherwise.
	Reason string
	Detail string
}

// SourceContainmentOf reports whether source contains commit (SCC-V0-001):
// ANCESTOR when commit is an ancestor of source; else EQUIVALENT when a
// non-merge source commit outside commit's history has commit's patch-id
// (git's --cherry-mark, as `git cherry` computes it); else ABSENT, naming a
// same-subject or same-Change-Id commit whose patch differs. It never
// fetches or writes. A shallow repository, a missing object, a failed read,
// the deadline or the scan bound yields UNKNOWN.
func SourceContainmentOf(root, source, commit string) SourceContainment {
	out := SourceContainment{State: SourceUnknown}
	var err error
	if out.Source, err = resolveObject(root, source, "commit"); err != nil {
		out.Source, out.Reason, out.Detail = "", SourceReasonMissingSource, "source "+source+" does not resolve to a commit"
		return out
	}
	if out.Commit, err = resolveObject(root, commit, "commit"); err != nil {
		out.Commit, out.Reason, out.Detail = "", SourceReasonMissingCommit, "commit "+commit+" does not resolve to a commit"
		return out
	}
	ancestor, err := isAncestor(root, out.Commit, out.Source)
	if err != nil {
		out.Reason, out.Detail = SourceReasonGitFailed, err.Error()
		return out
	}
	if ancestor {
		out.State = SourceAncestor
		return out
	}
	// A shallow clone hides history, so "not an ancestor" is unobservable.
	if shallow, err := gitOutput(root, "rev-parse", "--is-shallow-repository"); err != nil || strings.TrimSpace(string(shallow)) != "false" {
		out.Reason, out.Detail = SourceReasonShallowHistory, "the repository is shallow or its depth is unobservable; ancestry is unknown"
		return out
	}
	parents, err := gitOutput(root, "rev-list", "--parents", "-n", "1", "--end-of-options", out.Commit)
	if err != nil {
		out.Reason, out.Detail = SourceReasonGitFailed, err.Error()
		return out
	}
	fields := strings.Fields(string(parents))
	if len(fields) > 2 {
		out.State, out.Reason, out.Detail = SourceAbsent, SourceReasonMergeCommit, "a merge commit has no patch-id; only ancestry can contain it"
		return out
	}
	count, err := gitOutput(root, "rev-list", "--count", "--no-merges", out.Source, "^"+out.Commit, "--")
	if err != nil {
		out.Reason, out.Detail = SourceReasonGitFailed, err.Error()
		return out
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(count)))
	if err != nil || n > sourceContainmentScanBound {
		out.Reason, out.Detail = SourceReasonScanBound, "the source has "+strings.TrimSpace(string(count))+" commits outside the commit's history; the patch-id scan bound is "+strconv.Itoa(sourceContainmentScanBound)
		return out
	}
	args := []string{"log", "--no-merges", "--left-right", "--cherry-mark", "--format=%m%x1f%H%x1f%ae%x1f%s%x1f%(trailers:key=Change-Id,valueonly,separator=%x2C)",
		out.Source + "..." + out.Commit}
	if len(fields) == 2 {
		args = append(args, "^"+fields[1])
	}
	lines, err := sourceContainmentLog(root, append(args, "--"), n+1)
	if err != nil {
		out.Reason, out.Detail = SourceReasonGitFailed, err.Error()
		return out
	}
	var self []string
	for _, l := range lines {
		if l[1] == out.Commit {
			self = l
		}
	}
	for _, l := range lines {
		if l[0] == "=" && l[1] != out.Commit {
			out.State, out.Equivalent = SourceEquivalent, l[1]
			return out
		}
	}
	if self == nil {
		out.Reason, out.Detail = SourceReasonGitFailed, "the commit is missing from its own patch-id scan"
		return out
	}
	out.State, out.Reason = SourceAbsent, SourceReasonNoEquivalent
	for _, l := range lines {
		if l[0] != "<" {
			continue
		}
		if (l[2] == self[2] && l[3] == self[3]) || (self[4] != "" && l[4] == self[4]) {
			out.Reason, out.PatchDiffers = SourceReasonPatchDiffers, l[1]
			return out
		}
	}
	return out
}

// sourceContainmentLog runs one bounded, deadline-limited git log and
// splits each record into its five unit-separated fields.
func sourceContainmentLog(root string, args []string, maxLines int) ([][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sourceContainmentTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "credential.helper="}, args...)...)
	c.Dir, c.Env = root, gitEnvironment()
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	var out [][]string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 4096), 64<<10)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\x1f")
		if len(f) != 5 || len(f[0]) != 1 || len(out) >= maxLines {
			cancel()
			_ = c.Wait()
			return nil, errors.New("git log output exceeds the patch-id scan or is malformed")
		}
		out = append(out, f)
	}
	scanErr := sc.Err()
	if err := c.Wait(); err != nil || scanErr != nil || ctx.Err() != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git patch-id scan failed: %v %v %v", err, scanErr, ctx.Err())
	}
	return out, nil
}
