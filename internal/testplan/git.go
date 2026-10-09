package testplan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	// maxBoundFile is the provider document bound (LPCV-V0-051) reused for one bound source.
	maxBoundFile = 4 << 20
	// maxBoundTotal bounds all bound sources read at the evaluated revision.
	maxBoundTotal = 64 << 20
	pathChunk     = 512
)

// Freshness reasons TCN-V0-004 adds to the LPCV-V0-053 binding when it compares with a Git
// revision instead of the worktree.
const reasonUncommitted = "retained-bound-path-uncommitted"

// repo reads Git only: committed objects at the evaluated revision and, for the uncommitted rule,
// the worktree status. It never writes (--no-optional-locks) and never uses the network.
type repo struct {
	ctx  context.Context
	root string
}

var errGit = errors.New("Git source unavailable")

func (r repo) git(limit int, stdin []byte, args ...string) ([]byte, error) {
	return gitIn(r.ctx, r.root, limit, stdin, args...)
}

// gitIn runs one bounded Git read in dir with frozen hooks and credentials.
func gitIn(ctx context.Context, dir string, limit int, stdin []byte, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, errGit
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", dir}, args...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 60 * time.Second, OutputLimit: limit})
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, errGit
	}
	return o.Stdout, nil
}

// uncommitted names every path Git reports as changed or untracked in the worktree. The status
// runs through gitstatus.Status on private metadata, so no repository-defined clean or process
// filter can execute; a refusal there is a coded refusal here.
func (r repo) uncommitted() (map[string]bool, error) {
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		return gitIn(ctx, dir, limit, nil, args...)
	}
	status, err := gitstatus.Status(r.ctx, r.root, 16<<20, run, "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=all")
	if err != nil {
		return nil, invalidArguments("%s", "Git status of the bound sources failed: "+gitstatus.RefusalMessage(err))
	}
	changed := map[string]bool{}
	for _, entry := range bytes.Split(status, []byte{0}) {
		if len(entry) > 3 {
			changed[string(entry[3:])] = true
		}
	}
	return changed, nil
}

// top is the resolved top of the worktree holding root.
func (r repo) top() (string, error) {
	out, err := r.git(4096, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := strings.TrimSuffix(string(out), "\n")
	resolved, err := filepath.EvalSymlinks(top)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", errGit
	}
	return resolved, nil
}

// resolve names the commit a revision denotes.
func (r repo) resolve(revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\n") {
		return "", errGit
	}
	out, err := r.git(4096, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// boundSource is one bound path at the evaluated revision: the SHA-256 hex of its committed
// content, or the LPCV-V0-053 reason it cannot be compared.
type boundSource struct {
	digest string
	reason string
}

// boundSources reads every repository-relative path at rev. A path changed in the worktree
// against HEAD is unverifiable (retained-bound-path-uncommitted); a path absent at rev is a
// mismatch; a non-regular or oversized entry is unreadable.
func (r repo) boundSources(rev string, paths []string) (map[string]boundSource, error) {
	out := map[string]boundSource{}
	type entry struct {
		oid  string
		size int
	}
	listed := map[string]entry{}
	changed, err := r.uncommitted()
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if changed[path] {
			out[path] = boundSource{reason: reasonUncommitted}
		}
	}
	for start := 0; start < len(paths); start += pathChunk {
		chunk := paths[start:min(start+pathChunk, len(paths))]
		// Status cannot see an edit to an entry marked assume-unchanged (a lowercase tag) or
		// skip-worktree (S), so such an entry is unverifiable too. ls-files reads the index only.
		args := append([]string{"--literal-pathspecs", "ls-files", "-v", "-z", "--"}, chunk...)
		tagged, err := r.git(16<<20, nil, args...)
		if err != nil {
			return nil, err
		}
		for _, entry := range bytes.Split(tagged, []byte{0}) {
			if len(entry) > 2 && entry[0] != 'H' {
				out[string(entry[2:])] = boundSource{reason: reasonUncommitted}
			}
		}
		args = append([]string{"--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", rev, "--"}, chunk...)
		listing, err := r.git(16<<20, nil, args...)
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
			if len(line) == 0 {
				continue
			}
			meta, name, ok := strings.Cut(string(line), "\t")
			fields := strings.Fields(meta)
			if !ok || len(fields) != 4 {
				return nil, errGit
			}
			if _, done := out[name]; done {
				continue
			}
			if _, done := listed[name]; done {
				continue
			}
			size, err := strconv.Atoi(fields[3])
			if fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") || err != nil || size > maxBoundFile {
				out[name] = boundSource{reason: "retained-bound-source-unreadable"}
				continue
			}
			listed[name] = entry{oid: fields[2], size: size}
		}
	}
	// Read every listed blob through one bounded cat-file --batch.
	request, total := &bytes.Buffer{}, 0
	oids := map[string]bool{}
	for _, e := range listed {
		if oids[e.oid] {
			continue
		}
		oids[e.oid] = true
		total += e.size + 128
		request.WriteString(e.oid + "\n")
	}
	if total > maxBoundTotal {
		return nil, boundExceeded("the provider documents bind more than 64 MiB of sources")
	}
	contents := map[string]string{}
	if request.Len() > 0 {
		raw, err := r.git(total+1024, request.Bytes(), "cat-file", "--batch")
		if err != nil {
			return nil, err
		}
		for len(raw) > 0 {
			header, rest, ok := bytes.Cut(raw, []byte{'\n'})
			fields := strings.Fields(string(header))
			if !ok || len(fields) != 3 || fields[1] != "blob" {
				return nil, errGit
			}
			size, err := strconv.Atoi(fields[2])
			if err != nil || size+1 > len(rest) {
				return nil, errGit
			}
			sum := sha256.Sum256(rest[:size])
			contents[fields[0]] = hex.EncodeToString(sum[:])
			raw = rest[size+1:]
		}
	}
	for name, e := range listed {
		digest, ok := contents[e.oid]
		if !ok {
			return nil, errGit
		}
		out[name] = boundSource{digest: digest}
	}
	return out, nil
}
