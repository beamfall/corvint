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
// parents are first and second: every non-merge commit reachable from second
// and not from first, so a batch of lane merges yields every lane's commits
// (TCD-V0-731).
func LaneLogArgs(first, second string, limit int) []string {
	args := CloseoutLogArgs(second, first, limit)
	i := slices.Index(args, "--end-of-options")
	return slices.Insert(args, i, "--no-merges")
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

// ScanCommitsFrom is ScanCommits after the newest skip commits of the range,
// so a scan cut at its limit continues with skip+limit (TCD-V0-740).
func ScanCommitsFrom(root, ref, since string, skip, limit int) (CommitScan, error) {
	args := CloseoutLogArgs(ref, since, limit)
	if skip > 0 {
		args = slices.Insert(args, slices.Index(args, "--end-of-options"), "--skip="+strconv.Itoa(skip))
	}
	return scanLog(root, args, limit)
}

// runScanGit runs one bounded closeout git read, with stdin as its input
// when non-empty, through MaxCloseoutScanBytes; a larger output stops git
// and is refused LIMIT_EXCEEDED.
func runScanGit(root, stdin string, argv []string) ([]byte, error) {
	c := exec.Command("git", argv...)
	c.Dir, c.Env = root, gitEnvironment()
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	c.Stderr = &stderr
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	if err = c.Start(); err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	out, readErr := io.ReadAll(io.LimitReader(pipe, MaxCloseoutScanBytes+1))
	over := int64(len(out)) > MaxCloseoutScanBytes
	if over {
		_ = c.Process.Kill()
	}
	waitErr := c.Wait()
	switch {
	case over:
		return nil, wire.Errorf(wire.CodeLimitExceeded, "git log", "the commit scan output exceeds %d bytes; lower --limit or pass --since", MaxCloseoutScanBytes)
	case readErr != nil:
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", readErr)
	case waitErr != nil:
		return nil, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", waitErr)
	}
	return out, nil
}

// BuildLogDir is the build-log directory of TCD-V0-741.
const BuildLogDir = "docs/build-log/"

// CommitChange is what one non-merge commit changes (TCD-V0-741): every
// path it changes against its parent, and the lines it adds under
// BuildLogDir. A merge commit has neither.
type CommitChange struct {
	Paths, BuildLogAdded []string
}

// BuildLogOnly reports that the commit changes at least one path and every
// path it changes is under BuildLogDir.
func (c *CommitChange) BuildLogOnly() bool {
	return c != nil && len(c.Paths) > 0 && !slices.ContainsFunc(c.Paths, func(p string) bool { return !strings.HasPrefix(p, BuildLogDir) })
}

// CommitChanges reads the CommitChange of each of commits in two bounded
// `git log --no-walk --stdin` reads (TCD-V0-741). A quoted path never counts
// as a build-log path.
func CommitChanges(root string, commits []string) (map[string]*CommitChange, error) {
	out := make(map[string]*CommitChange, len(commits))
	if len(commits) == 0 {
		return out, nil
	}
	stdin := strings.Join(commits, "\n") + "\n"
	base := append(append([]string{}, preflightGitConfig...), "-c", "credential.helper=", "log", "--no-walk=unsorted", "--stdin",
		"--no-show-signature", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--format=%x00%H")
	records := func(raw []byte, each func(*CommitChange, string)) {
		for _, rec := range strings.Split(string(raw), "\x00") {
			commit, body, _ := strings.Cut(rec, "\n")
			if commit = strings.TrimSpace(commit); commit == "" {
				continue
			}
			ch := out[commit]
			if ch == nil {
				ch = &CommitChange{}
				out[commit] = ch
			}
			for _, line := range strings.Split(body, "\n") {
				each(ch, line)
			}
		}
	}
	names, err := runScanGit(root, stdin, append(slices.Clone(base), "--name-only"))
	if err != nil {
		return nil, err
	}
	records(names, func(ch *CommitChange, line string) {
		if line != "" {
			ch.Paths = append(ch.Paths, line)
		}
	})
	patch, err := runScanGit(root, stdin, append(slices.Clone(base), "-p", "--unified=0", "--", BuildLogDir))
	if err != nil {
		return nil, err
	}
	records(patch, func(ch *CommitChange, line string) {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++ ") {
			ch.BuildLogAdded = append(ch.BuildLogAdded, line[1:])
		}
	})
	return out, nil
}

func scanLog(root string, argv []string, limit int) (CommitScan, error) {
	out, err := runScanGit(root, "", argv)
	if err != nil {
		return CommitScan{}, err
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
// namespaces are further ref prefixes listed the same way, such as the
// fetched refs/lanes/ heads (TCD-V0-774).
func LaneHeads(root, base string, namespaces ...string) ([]LaneHead, error) {
	rootsOut, err := gitOutput(root, "rev-list", "--max-parents=0", "--end-of-options", base)
	if err != nil {
		return nil, err
	}
	roots := strings.Fields(string(rootsOut))
	args := []string{"for-each-ref", "--format=%(objectname) %(refname)", "--no-merged=" + base}
	for _, r := range roots {
		args = append(args, "--contains="+r)
	}
	out, err := gitOutput(root, append(append(args, "refs/heads/"), namespaces...)...)
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

// LaneMention is one lane commit whose message names a screened token, with
// the head the walk reached it from (TCD-V0-774).
type LaneMention struct {
	Ref string
	ScannedCommit
}

// LaneMentionScan is the bounded result of ScanLaneMentions.
type LaneMentionScan struct {
	// Mentions is sorted by head ref, newest first within a head.
	Mentions []LaneMention
	// Truncated is more than limit commits naming a token; the kept ones
	// are the newest of the walk, and Last is the oldest kept one.
	Truncated bool
	Last      string
}

// ScanLaneMentions reads, in one `git log` walk over every head and not from
// base, at most limit commits whose message holds one of tokens as a fixed,
// case-insensitive string, each attributed to the first head that reached it
// (TCD-V0-774). The match is only a prefilter: the caller classifies every
// message it returns. No head or no token reads nothing. A detached worktree
// head is walked by its commit.
func ScanLaneMentions(root, base string, heads []LaneHead, tokens []string, limit int) (LaneMentionScan, error) {
	var scan LaneMentionScan
	if len(heads) == 0 || len(tokens) == 0 {
		return scan, nil
	}
	args := append(append([]string{}, preflightGitConfig...), "-c", "credential.helper=", "log", "--no-show-signature", "--no-color", "--encoding=UTF-8", "-z",
		"--source", "--fixed-strings", "--regexp-ignore-case", "--format=%H %S%n%B", "--max-count="+strconv.Itoa(limit+1))
	for _, t := range tokens {
		args = append(args, "--grep="+t)
	}
	args = append(args, "--end-of-options")
	source := make(map[string]string, len(heads))
	for _, h := range heads {
		arg := h.Ref
		if !strings.HasPrefix(arg, "refs/") {
			arg = h.Commit
		}
		source[arg] = h.Ref
		args = append(args, arg)
	}
	out, err := runScanGit(root, "", append(args, "^"+base))
	if err != nil {
		return scan, err
	}
	for _, rec := range strings.Split(string(out), "\x00") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		if len(scan.Mentions) == limit {
			scan.Truncated = true
			break
		}
		header, message, _ := strings.Cut(rec, "\n")
		commit, src, _ := strings.Cut(header, " ")
		ref, ok := source[src]
		if !ok {
			return LaneMentionScan{}, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: lane commit %s has unknown source %q", commit, src)
		}
		scan.Mentions = append(scan.Mentions, LaneMention{Ref: ref, ScannedCommit: ScannedCommit{Commit: commit, Subject: subjectOf(message), Message: message}})
		scan.Last = commit
	}
	sort.SliceStable(scan.Mentions, func(i, j int) bool { return scan.Mentions[i].Ref < scan.Mentions[j].Ref })
	return scan, nil
}
