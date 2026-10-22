package store

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
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
		"--format=%H%n%B", "--max-count="+strconv.Itoa(limit+1), "--end-of-options", ref)
	if since != "" {
		args = append(args, "^"+since)
	}
	return args
}

// ScanCommits reads at most limit commits, newest first (TCD-V0-003). The
// output is read through MaxCloseoutScanBytes; a larger output stops git and
// is refused LIMIT_EXCEEDED.
func ScanCommits(root, ref, since string, limit int) (CommitScan, error) {
	c := exec.Command("git", CloseoutLogArgs(ref, since, limit)...)
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
		commit, message, _ := strings.Cut(rec, "\n")
		scan.Commits = append(scan.Commits, ScannedCommit{Commit: commit, Subject: subjectOf(message), Message: message})
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
