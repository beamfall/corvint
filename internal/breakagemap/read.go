package breakagemap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	executable := gitstatus.Executable()
	if !filepath.IsAbs(executable) {
		return nil, errors.New("Git unavailable")
	}
	argv := append([]string{executable, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", root}, args...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: gokernel.SanitizedGitEnvironment(), Timeout: 10 * time.Second, OutputLimit: MaxBytes + 1})
	if o.Err != nil || o.ExitStatus != 0 || len(o.Stdout) > MaxBytes {
		return nil, errors.New("immutable Git input unavailable or over bound")
	}
	return o.Stdout, nil
}

func bind(ctx context.Context, r Repository, root string) error {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return errors.New("checkout unavailable")
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return errors.New("checkout unavailable")
	}
	top, err := git(ctx, canonical, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(string(top)) != canonical {
		return errors.New("checkout must name its Git top level")
	}
	typ, err := git(ctx, canonical, "cat-file", "-t", r.Commit)
	if err != nil || strings.TrimSpace(string(typ)) != "commit" {
		return errors.New("commit unavailable")
	}
	tree, err := git(ctx, canonical, "rev-parse", r.Commit+"^{tree}")
	if err != nil || strings.TrimSpace(string(tree)) != r.Tree {
		return errors.New("tree pin mismatch")
	}
	roots, err := git(ctx, canonical, "rev-list", "--max-parents=0", r.Commit)
	if err != nil {
		return err
	}
	contains := func(raw []byte) bool {
		for _, v := range strings.Fields(string(raw)) {
			if v == r.Origin {
				return true
			}
		}
		return false
	}
	if !contains(roots) {
		return errors.New("origin pin mismatch")
	}
	headRoots, err := git(ctx, canonical, "rev-list", "--max-parents=0", "HEAD")
	if err != nil || !contains(headRoots) {
		return errors.New("checkout origin pin mismatch")
	}
	return nil
}

func readBlob(ctx context.Context, root, commit, p, expected string) ([]byte, error) {
	row, err := git(ctx, root, "ls-tree", "-z", commit, "--", p)
	if err != nil {
		return nil, err
	}
	fields := strings.SplitN(strings.TrimSuffix(string(row), "\x00"), "\t", 2)
	if len(fields) != 2 || fields[1] != p {
		return nil, errors.New("source missing")
	}
	meta := strings.Fields(fields[0])
	if len(meta) != 3 || (meta[0] != "100644" && meta[0] != "100755") || meta[1] != "blob" {
		return nil, errors.New("source is not a regular Git blob")
	}
	if expected != "" && meta[2] != expected {
		return nil, errors.New("blob pin mismatch")
	}
	b, err := git(ctx, root, "cat-file", "blob", meta[2])
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(b) || bytesContainNUL(b) {
		return nil, errors.New("source is not UTF-8 text")
	}
	return b, nil
}
func bytesContainNUL(b []byte) bool { return strings.IndexByte(string(b), 0) >= 0 }

type captured struct {
	source Source
	repo   Repository
	text   []byte
}

func (c captured) anchor(start, end int) Anchor {
	lines := strings.Split(string(c.text), "\n")
	return Anchor{Repository: c.repo.ID, Commit: c.repo.Commit, Tree: c.repo.Tree, Path: c.source.Path, Blob: c.source.Blob, Start: start, End: end, SpanSHA256: digest([]byte(strings.Join(lines[start-1:end], "\n")))}
}
