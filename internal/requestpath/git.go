package requestpath

import (
	"bytes"
	"context"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

// GitSource reads committed regular files at one commit; it never reads the working tree, follows a
// symlink or submodule, or touches the network (AMAP-V0-137).
type GitSource struct {
	ctx    context.Context
	root   string
	commit string
}

func (g GitSource) git(limit int, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, refuse("request-path-source-unavailable", "Git is unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "--literal-pathspecs", "-c", "core.fsmonitor=false", "-c",
		"core.quotePath=false", "-c", "credential.helper=", "-C", g.root}, args...)
	o := procgroup.Run(g.ctx, procgroup.Spec{Argv: argv, Dir: g.root, Env: gokernel.SanitizedGitEnvironment(),
		Timeout: 120 * time.Second, OutputLimit: limit})
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, refuse("request-path-source-unavailable", "a Git read failed")
	}
	return o.Stdout, nil
}

// OpenGit resolves revision to a commit of the repository at root.
func OpenGit(ctx context.Context, root, revision string) (GitSource, error) {
	g := GitSource{ctx: ctx, root: root}
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\n\r") {
		return g, refuse("request-path-invalid-revision", "the revision is not a commit")
	}
	out, err := g.git(4096, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return g, refuse("request-path-invalid-revision", "the revision is not a commit")
	}
	g.commit = strings.TrimSpace(string(out))
	return g, nil
}

// Commit is the resolved commit.
func (g GitSource) Commit() string { return g.commit }

// CleanPath reports p as a clean repository-relative path, or false.
func CleanPath(p string) (string, bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	c := path.Clean(p)
	if p == "" || strings.HasPrefix(c, "/") || c == "." || c == ".." || strings.HasPrefix(c, "../") || strings.ContainsAny(c, "\x00\n\r") {
		return "", false
	}
	return c, true
}

// Read returns the first candidate that is a committed regular file at the commit.
func (g GitSource) Read(candidates []string) (string, string, bool, error) {
	clean := []string{}
	for _, c := range candidates {
		if p, ok := CleanPath(c); ok {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return "", "", false, nil
	}
	out, err := g.git(1<<20, append([]string{"ls-tree", "-z", "-l", "--full-tree", g.commit, "--"}, clean...)...)
	if err != nil {
		return "", "", false, err
	}
	type blob struct {
		oid  string
		size int
	}
	present := map[string]blob{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			continue
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			return "", "", false, refuse("request-path-source-unavailable", "the tree is unreadable")
		}
		present[name] = blob{oid: fields[2], size: size}
	}
	for _, c := range clean {
		b, ok := present[c]
		if !ok {
			continue
		}
		if b.size > MaxFileBytes {
			return "", "", false, refuse("request-path-bound-exceeded", "%s exceeds %d bytes", c, MaxFileBytes)
		}
		data, err := g.git(b.size+1024, "cat-file", "blob", b.oid)
		if err != nil {
			return "", "", false, err
		}
		return c, string(data), true, nil
	}
	return "", "", false, nil
}
