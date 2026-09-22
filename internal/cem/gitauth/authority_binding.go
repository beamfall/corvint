package gitauth

import (
	"context"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// authorityFile retains the parent descriptors that bound this one read. It
// deliberately shares no directory handles across files or clean checks.
type authorityFile struct {
	*os.File
	parent *os.Root
	leaf   string
	before os.FileInfo
	edges  []authorityEdge
	closed bool
}

type authorityEdge struct {
	parent *os.Root
	child  *os.Root
	name   string
	before os.FileInfo
}

// Go 1.27's final-name OpenRoot open lacks O_DIRECTORY/O_NONBLOCK. Retaining
// "/." makes the named entry a directory component, so a FIFO substitution
// fails directory lookup instead of blocking before our identity witness.
func openAuthorityRoot(path string) (*os.Root, error) { return os.OpenRoot(path + "/.") }

func openAuthorityDirectory(parent *os.Root, name string) (*os.Root, error) {
	return parent.OpenRoot(name + "/.")
}

func sameAuthorityMetadata(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return false
	}
	bs, bn, bok := authorityChangeTime(before)
	as, an, aok := authorityChangeTime(after)
	return bok && aok && bs == as && bn == an
}

func openAuthorityFile(root *os.Root, path string) (*authorityFile, error) {
	return openAuthorityFileContext(context.Background(), root, path)
}

func openAuthorityFileContext(ctx context.Context, root *os.Root, path string) (_ *authorityFile, err error) {
	if wire.ValidatePath(path) != nil {
		return nil, unavailable("authority path unavailable")
	}
	bound := &authorityFile{parent: root}
	defer func() {
		if err != nil {
			bound.Close()
		}
	}()
	parts := strings.Split(path, "/")
	for _, name := range parts[:len(parts)-1] {
		if ctx.Err() != nil {
			return nil, unavailable("authority snapshot cancelled")
		}
		before, e := bound.parent.Lstat(name)
		if e != nil || !before.IsDir() {
			return nil, unavailable("authority parent unavailable")
		}
		child, e := openAuthorityDirectory(bound.parent, name)
		if e != nil {
			return nil, unavailable("authority parent unavailable")
		}
		edge := authorityEdge{parent: bound.parent, child: child, name: name, before: before}
		bound.edges = append(bound.edges, edge)
		opened, e := child.Stat(".")
		after, afterErr := bound.parent.Lstat(name)
		if e != nil || afterErr != nil || !sameAuthorityMetadata(before, opened) || !sameAuthorityMetadata(before, after) {
			return nil, unavailable("authority parent changed during open")
		}
		bound.parent = child
	}
	if ctx.Err() != nil {
		return nil, unavailable("authority snapshot cancelled")
	}
	bound.leaf = parts[len(parts)-1]
	before, e := bound.parent.Lstat(bound.leaf)
	if e != nil || !before.Mode().IsRegular() {
		return nil, unavailable("authority worktree file unavailable")
	}
	bound.File, e = openAuthorityLeaf(bound.parent, bound.leaf)
	if e != nil {
		return nil, unavailable("authority worktree file unavailable")
	}
	opened, e := bound.File.Stat()
	after, afterErr := bound.parent.Lstat(bound.leaf)
	if e != nil || afterErr != nil || !sameAuthorityMetadata(before, opened) || !sameAuthorityMetadata(before, after) {
		return nil, unavailable("authority file changed during open")
	}
	bound.before = opened
	return bound, nil
}

func (f *authorityFile) validate(ctx context.Context) error {
	// Walk from the caller's root toward the leaf, so a detached parent cannot
	// validate a child through its old descriptor and conceal namespace drift.
	for _, edge := range f.edges {
		if ctx.Err() != nil {
			return unavailable("authority snapshot cancelled")
		}
		named, err := edge.parent.Lstat(edge.name)
		if err != nil || !sameAuthorityMetadata(edge.before, named) {
			return unavailable("authority parent changed during read")
		}
	}
	if ctx.Err() != nil {
		return unavailable("authority snapshot cancelled")
	}
	named, err := f.parent.Lstat(f.leaf)
	opened, statErr := f.File.Stat()
	if err != nil || statErr != nil || !sameAuthorityMetadata(f.before, named) || !sameAuthorityMetadata(f.before, opened) {
		return unavailable("authority file changed during read")
	}
	return nil
}

func (f *authorityFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	var err error
	if f.File != nil {
		err = f.File.Close()
	}
	for i := len(f.edges) - 1; i >= 0; i-- {
		if e := f.edges[i].child.Close(); err == nil {
			err = e
		}
	}
	f.edges = nil
	return err
}
