// Package pathbrief builds the path brief of `corvint context --path` (TBC-V1-009 to TBC-V1-011)
// and the requirement-link file it reads (TBC-V1-012). It reads committed objects and never the
// working tree, ranks nothing, and writes nothing except through the one-time export.
package pathbrief

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// errGit reports a Git read that failed; callers turn it into a typed refusal or a missing source.
var errGit = errors.New("Git source unavailable")

// repo reads committed objects of one repository root.
type repo struct {
	ctx  context.Context
	root string
}

// runProcess runs one child process; tests replace it to force an outcome.
var runProcess = procgroup.Run

func (r repo) git(limit int, stdin []byte, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, errGit
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", r.root}, args...)
	o := runProcess(r.ctx, procgroup.Spec{Argv: argv, Dir: r.root, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 120 * time.Second, InputLimit: len(stdin) + 1, OutputLimit: limit})
	if errors.Is(o.Err, procgroup.ErrPlatformUnsupported) {
		return nil, procgroup.ErrPlatformUnsupported
	}
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, errGit
	}
	return o.Stdout, nil
}

// resolve names the commit a revision denotes.
func (r repo) resolve(revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") {
		return "", errGit
	}
	out, err := r.git(4096, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// tree maps each tracked file at rev to its blob id. Prefixes narrow the listing; none lists the
// whole tree. Submodule entries are left out: they are not files of this repository.
func (r repo) tree(rev string, prefixes ...string) (map[string]string, error) {
	args := append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", rev, "--"}, prefixes...)
	out, err := r.git(256<<20, nil, args...)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, errGit
		}
		if fields[1] == "blob" {
			files[name] = fields[2]
		}
	}
	return files, nil
}

// blob returns the bytes of one object, or false when the repository does not hold it.
func (r repo) blob(oid string) ([]byte, bool) {
	if !validOID(oid) {
		return nil, false
	}
	out, err := r.git(64<<20, nil, "cat-file", "blob", oid)
	return out, err == nil
}

// objects answers one `git cat-file --batch-check` for names such as `SHA:path`: the object id of
// each, or "" when it is missing or the name cannot be put on one line.
func (r repo) objects(names []string) ([]string, error) {
	var stdin bytes.Buffer
	asked := make([]bool, len(names))
	for i, name := range names {
		if name == "" || strings.ContainsAny(name, "\n\r") {
			continue
		}
		asked[i] = true
		stdin.WriteString(name)
		stdin.WriteByte('\n')
	}
	oids := make([]string, len(names))
	if stdin.Len() == 0 {
		return oids, nil
	}
	out, err := r.git(256<<20, stdin.Bytes(), "cat-file", "--batch-check=%(objectname)")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	next := 0
	for i := range names {
		if !asked[i] {
			continue
		}
		if next >= len(lines) {
			return nil, errGit
		}
		if validOID(lines[next]) {
			oids[i] = lines[next]
		}
		next++
	}
	return oids, nil
}

// committerTimes returns the committer time in Unix seconds of each commit that exists.
func (r repo) committerTimes(shas []string) (map[string]int64, error) {
	times := map[string]int64{}
	for start := 0; start < len(shas); start += 256 {
		end := min(start+256, len(shas))
		args := append([]string{"show", "-s", "--no-walk=unsorted", "--format=%H %ct", "--end-of-options"}, shas[start:end]...)
		out, err := r.git(1<<20, nil, args...)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			sha, stamp, ok := strings.Cut(line, " ")
			seconds, err := strconv.ParseInt(stamp, 10, 64)
			if ok && err == nil && validOID(sha) {
				times[sha] = seconds
			}
		}
	}
	return times, nil
}

func validOID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	for _, c := range oid {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
