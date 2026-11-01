package store

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Closeout-debt commit scan (TCD-V0-002, TCD-V0-003): plain Git reads under
// the store's sanitized environment that run no hook, filter, textconv,
// merge driver or signature program.

// MaxCloseoutScanBytes bounds the `git log` output one scan reads.
var MaxCloseoutScanBytes int64 = 64 << 20

// ScannedCommit is one commit of the scan, newest first.
type ScannedCommit struct {
	Commit, Subject, Message string
	// Parents is the commit's parent count; more than one is a merge
	// (TCD-V0-731).
	Parents int
}

// CommitScan is the bounded result of ScanCommits.
type CommitScan struct {
	Commits   []ScannedCommit
	Truncated bool // more than limit commits are in the range
}

// CloseoutRevision resolves rev to a full commit ID in root. A revision that
// is empty, starts with `-` or holds a control byte, or that names no commit
// (rev-parse exit 1), is refused MALFORMED; any other Git failure, such as a
// corrupt object or a directory that is no repository, is UNSUPPORTED.
func CloseoutRevision(root, rev string) (string, error) {
	if rev == "" || strings.HasPrefix(rev, "-") || strings.IndexFunc(rev, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", wire.Errorf(wire.CodeMalformed, "revision", "revision must be non-empty, must not start with '-' and must hold no control byte")
	}
	c := exec.Command("git", append(append([]string{}, preflightGitConfig...), "-c", "credential.helper=", "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")...)
	c.Dir, c.Env = root, gitEnvironment()
	out, err := c.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", wire.Errorf(wire.CodeMalformed, "revision", "revision %q does not resolve to a commit", rev)
		}
		return "", wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// OriginSlug is the lower-cased `owner/repo` of remote.origin.url, or "" when
// no origin is configured or its URL has no two path segments.
func OriginSlug(root string) string {
	out, err := gitOutput(root, "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	u := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(string(out)), "/"), ".git")
	parts := strings.FieldsFunc(u, func(r rune) bool { return r == '/' || r == ':' })
	if len(parts) < 2 {
		return ""
	}
	return wire.FoldToken(parts[len(parts)-2] + "/" + parts[len(parts)-1])
}

// CloseoutLogArgs is the `git log` argv for a scan of at most limit commits
// reachable from ref and not from since (empty for none). Both are resolved
// commit IDs; no --first-parent, so second-parent constituents count.
// --encoding=UTF-8 overrides i18n.logOutputEncoding, whose UTF-16 output would
// hold NUL bytes the -z split treats as record ends.
func CloseoutLogArgs(ref, since string, limit int) []string {
	args := append(append([]string{}, preflightGitConfig...), "-c", "credential.helper=", "log", "--no-show-signature", "--no-color", "--encoding=UTF-8", "-z",
		"--format=%H %P%n%B", "--max-count="+strconv.Itoa(limit+1), "--end-of-options", ref)
	if since != "" {
		args = append(args, "^"+since)
	}
	return args
}

// ScanCommits reads at most limit commits, newest first (TCD-V0-003). The
// output is read through MaxCloseoutScanBytes; a larger output stops git and
// is refused LIMIT_EXCEEDED.
func ScanCommits(root, ref, since string, limit int) (CommitScan, error) {
	return scanLog(root, CloseoutLogArgs(ref, since, limit), limit)
}

// LaneLogArgs is the `git log` argv for the lane commits of a merge whose
// parents are first and second: the first-parent chain from second that is
// not reachable from first (TCD-V0-731).
func LaneLogArgs(first, second string, limit int) []string {
	args := CloseoutLogArgs(second, first, limit)
	i := slices.Index(args, "--end-of-options")
	return slices.Insert(args, i, "--first-parent")
}

// ScanLaneCommits reads at most limit lane commits of merge, newest first
// (TCD-V0-731). A merge that does not resolve or has fewer than two parents
// is refused MISSING_EVIDENCE: its lane range is unknown.
func ScanLaneCommits(root, merge string, limit int) (CommitScan, error) {
	if _, err := wire.ParseOID("mergeCommit", merge); err != nil {
		return CommitScan{}, err
	}
	out, _, err := boundedGit(root, 4096, "rev-list", "--parents", "--max-count=1", "--end-of-options", merge)
	f := strings.Fields(string(out))
	if err != nil || len(f) < 3 {
		return CommitScan{}, wire.Errorf(wire.CodeMissingEvidence, "mergeCommit", "merge commit %s does not resolve locally with two parents", merge)
	}
	return scanLog(root, LaneLogArgs(f[1], f[2], limit), limit)
}

func scanLog(root string, argv []string, limit int) (CommitScan, error) {
	c := exec.Command("git", argv...)
	c.Dir, c.Env = root, gitEnvironment()
	var stderr bytes.Buffer
	c.Stderr = &stderr
	pipe, err := c.StdoutPipe()
	if err != nil {
		return CommitScan{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	if err = c.Start(); err != nil {
		return CommitScan{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	out, readErr := io.ReadAll(io.LimitReader(pipe, MaxCloseoutScanBytes+1))
	over := int64(len(out)) > MaxCloseoutScanBytes
	if over {
		_ = c.Process.Kill()
	}
	waitErr := c.Wait()
	switch {
	case over:
		return CommitScan{}, wire.Errorf(wire.CodeLimitExceeded, "git log", "the commit scan output exceeds %d bytes; lower --limit or pass --since", MaxCloseoutScanBytes)
	case readErr != nil:
		return CommitScan{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", readErr)
	case waitErr != nil:
		return CommitScan{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", waitErr)
	}
	var scan CommitScan
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		if len(scan.Commits) == limit {
			scan.Truncated = true
			break
		}
		header, message, _ := strings.Cut(rec, "\n")
		ids := strings.Fields(header)
		if len(ids) == 0 {
			continue
		}
		scan.Commits = append(scan.Commits, ScannedCommit{Commit: ids[0], Subject: subjectOf(message), Message: message, Parents: len(ids) - 1})
	}
	return scan, nil
}

// CommitTime is commit's committer time in root (TCD-V0-168).
func CommitTime(root, commit string) (time.Time, error) {
	out, err := gitOutput(root, "show", "-s", "--no-show-signature", "--format=%ct", "--end-of-options", commit)
	if err != nil {
		return time.Time{}, err
	}
	sec, e := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if e != nil {
		return time.Time{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: committer time %q", strings.TrimSpace(string(out)))
	}
	return time.Unix(sec, 0).UTC(), nil
}

// subjectOf is the message's first paragraph joined by spaces, as `%s`.
func subjectOf(message string) string {
	para, _, _ := strings.Cut(strings.TrimLeft(message, "\n"), "\n\n")
	return strings.Join(strings.Fields(strings.ReplaceAll(para, "\n", " ")), " ")
}

// MaxLaneHeads bounds the local branch and worktree heads LaneHeads returns.
const MaxLaneHeads = 256

// LaneHead is one local branch or detached worktree head (TLF-V0-887).
type LaneHead struct{ Ref, Commit string }

// LaneHeads lists the local branches and detached worktree heads of root
// that base does not contain and that share a root commit with base, each
// commit once, sorted by ref; an unrelated lineage is no lane of base. More
// than MaxLaneHeads is refused LIMIT_EXCEEDED rather than silently cut.
func LaneHeads(root, base string) ([]LaneHead, error) {
	rootsOut, err := gitOutput(root, "rev-list", "--max-parents=0", "--end-of-options", base)
	if err != nil {
		return nil, err
	}
	roots := strings.Fields(string(rootsOut))
	args := []string{"for-each-ref", "--format=%(objectname) %(refname)", "--no-merged=" + base}
	for _, r := range roots {
		args = append(args, "--contains="+r)
	}
	out, err := gitOutput(root, append(args, "refs/heads/")...)
	if err != nil {
		return nil, err
	}
	wt, err := gitOutput(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var heads []LaneHead
	for _, line := range strings.Split(string(out), "\n") {
		commit, ref, ok := strings.Cut(line, " ")
		if ok && !seen[commit] {
			seen[commit] = true
			heads = append(heads, LaneHead{Ref: ref, Commit: commit})
		}
	}
	for _, block := range strings.Split(string(wt), "\n\n") {
		var path, commit string
		detached := false
		for _, line := range strings.Split(block, "\n") {
			switch k, v, _ := strings.Cut(line, " "); k {
			case "worktree":
				path = v
			case "HEAD":
				commit = v
			case "detached":
				detached = true
			}
		}
		if detached && commit != "" && !seen[commit] && !IsAncestor(root, commit, base) && slices.ContainsFunc(roots, func(r string) bool { return IsAncestor(root, r, commit) }) {
			seen[commit] = true
			heads = append(heads, LaneHead{Ref: "worktree:" + path, Commit: commit})
		}
	}
	if len(heads) > MaxLaneHeads {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "git", "%d local heads exceed %d", len(heads), MaxLaneHeads)
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].Ref < heads[j].Ref })
	return heads, nil
}
