package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Bounds of WorkingTreeState: more untracked files, or more untracked bytes,
// than these is LIMIT_EXCEEDED, so the caller treats the tree as uncacheable
// rather than hashing without bound (TOL-V0-047).
const (
	MaxWorkingTreeUntrackedFiles = 4096
	MaxWorkingTreeUntrackedBytes = 256 << 20
)

// WorkingTreeState digests what the working tree at root holds: HEAD's
// commit, the binary diff of tracked files against HEAD (no external diff,
// no textconv) and the path, type and content of every untracked file Git
// does not ignore. Any change to one of them changes the digest. It runs
// Git with repository hooks and the file-system monitor disabled and
// GIT_OPTIONAL_LOCKS=0, so it writes no index (TOL-V0-047). An unborn HEAD,
// a failed Git read or a bound refuses.
func WorkingTreeState(root string) (head string, digest wire.Digest, err error) {
	out, err := preflightGit(root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return "", "", wire.Errorf(wire.CodeMissingEvidence, "HEAD", "the working tree has no HEAD commit")
	}
	head = strings.TrimSpace(string(out))
	h := sha256.New()
	io.WriteString(h, "corvint-tasks-working-tree/0\nhead "+head+"\n")
	diff := exec.Command("git", append(append(append([]string{}, preflightGitConfig...), "-c", "credential.helper="),
		"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", "--full-index", "HEAD", "--")...)
	diff.Dir = root
	diff.Env = gitEnvironment()
	diff.Stdout = h
	var stderr bytes.Buffer
	diff.Stderr = &stderr
	if err := diff.Run(); err != nil {
		return "", "", wire.Errorf(wire.CodeUnsupported, "git", "git diff failed: %v", err)
	}
	listing, err := preflightGit(root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return "", "", err
	}
	names := strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00")
	if len(listing) == 0 {
		names = nil
	}
	if len(names) > MaxWorkingTreeUntrackedFiles {
		return "", "", wire.Errorf(wire.CodeLimitExceeded, "untracked", "the working tree holds more than %d untracked files", MaxWorkingTreeUntrackedFiles)
	}
	var total int64
	for _, name := range names {
		p := filepath.Join(root, filepath.FromSlash(name))
		st, err := os.Lstat(p)
		if err != nil {
			return "", "", wire.Annotatef(err, "untracked", "cannot read untracked file: %v", err)
		}
		io.WriteString(h, "untracked "+strconv.Quote(name)+" "+st.Mode().Type().String()+"\n")
		switch {
		case st.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return "", "", wire.Annotatef(err, "untracked", "cannot read untracked link: %v", err)
			}
			io.WriteString(h, "link "+strconv.Quote(target)+"\n")
		case st.Mode().IsRegular():
			total += st.Size()
			if total > MaxWorkingTreeUntrackedBytes {
				return "", "", wire.Errorf(wire.CodeLimitExceeded, "untracked", "the untracked files exceed %d bytes", MaxWorkingTreeUntrackedBytes)
			}
			sum, err := fileSha256(p)
			if err != nil {
				return "", "", wire.Annotatef(err, "untracked", "cannot read untracked file: %v", err)
			}
			io.WriteString(h, "content "+sum+" "+strconv.FormatInt(st.Size(), 10)+"\n")
		}
	}
	return head, wire.Digest(hex.EncodeToString(h.Sum(nil))), nil
}

func fileSha256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, MaxWorkingTreeUntrackedBytes+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
