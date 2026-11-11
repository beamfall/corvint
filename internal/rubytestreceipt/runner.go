package rubytestreceipt

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MaxSpecFileBytes bounds one snapshot file; it equals the bound discovery
	// reads a retained spec or helper file under, so a file the run bound is
	// always one discovery can compare (LPCV-V0-092, LPCV-V0-094).
	MaxSpecFileBytes     = 4 << 20
	MaxReportBytes       = 32 << 20
	MaxRunnerOutputBytes = 1 << 20
	MaxRSpecIDBytes      = 512
	ReportDirectory      = ".corvint/rspec"
	// DefaultTimeout bounds one RSpec run.
	DefaultTimeout = 30 * time.Minute
	// FormatterCorvint names the Corvint-shipped RSpec JSON formatter
	// (LPCV-V0-136); FormatterStock is RSpec's own json formatter.
	FormatterCorvint = "corvint-rspec-json/0"
	FormatterStock   = "stock"
	FormatterClass   = "CorvintRSpecJsonFormatter"
)

// MaxSnapshotEntries bounds every directory entry a spec-tree walk visits,
// whatever its type; tests may lower it.
var MaxSnapshotEntries = 16384

// IsHelperPath reports whether a project-relative name is, or lies under, a
// conventional helper path: spec/spec_helper.rb, spec/rails_helper.rb or
// spec/support.
func IsHelperPath(name string) bool {
	return name == "spec/spec_helper.rb" || name == "spec/rails_helper.rb" || name == "spec/support" || strings.HasPrefix(name, "spec/support/")
}

// CurrentHelpers lists, sorted and worktree-relative, the conventional helper
// files of projectDirectory as they are now, under the same paths and entry
// bound as a run. A helper path that is not a regular file, or a tree over the
// bound, is an error: the current set cannot be compared (LPCV-V0-094).
func CurrentHelpers(root *os.Root, projectDirectory string) ([]string, error) {
	prefix := ""
	if projectDirectory != "." {
		prefix = projectDirectory + "/"
	}
	info, err := root.Lstat(filepath.FromSlash(prefix + "spec"))
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errNotDirectory
	}
	helpers := []string{}
	visit := func(name string, mode fs.FileMode) error {
		if !IsHelperPath(strings.TrimPrefix(name, prefix)) {
			return nil
		}
		if !mode.IsRegular() {
			return errors.New("a helper path is not a regular file")
		}
		if strings.HasSuffix(name, ".rb") {
			helpers = append(helpers, name)
		}
		return nil
	}
	for _, name := range []string{prefix + "spec/spec_helper.rb", prefix + "spec/rails_helper.rb"} {
		info, err := root.Lstat(filepath.FromSlash(name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := visit(name, info.Mode().Type()); err != nil {
			return nil, err
		}
	}
	if err := WalkTree(context.Background(), root, prefix+"spec/support", visit); err != nil {
		return nil, err
	}
	SortStrings(helpers)
	return helpers, nil
}

// WalkTree visits every non-directory entry under start without following a
// symlink, counting every entry against maxSnapshotEntries. A missing start is
// empty; a start that is not a real directory is errNotDirectory.
func WalkTree(ctx context.Context, root *os.Root, start string, visit func(name string, mode fs.FileMode) error) error {
	info, err := root.Lstat(filepath.FromSlash(start))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errNotDirectory
	}
	pending, entries := []string{start}, 0
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		children, err := listDirectory(root, name, &entries)
		if err != nil {
			return err
		}
		for _, child := range children {
			childName := name + "/" + child.Name()
			if child.IsDir() {
				pending = append(pending, childName)
				continue
			}
			if err := visit(childName, child.Type()); err != nil {
				return err
			}
		}
	}
	return nil
}

// listDirectory reads one directory in chunks, refusing it once the walk's
// entry count passes maxSnapshotEntries, and only when the opened directory is
// the one a no-follow Lstat admitted.
func listDirectory(root *os.Root, name string, entries *int) ([]fs.DirEntry, error) {
	admitted, err := root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	if !admitted.IsDir() {
		return nil, errNotDirectory
	}
	directory, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil || !os.SameFile(admitted, opened) {
		return nil, errors.New("directory changed while it was opened")
	}
	var children []fs.DirEntry
	for {
		batch, err := directory.ReadDir(256)
		*entries += len(batch)
		if *entries > MaxSnapshotEntries {
			return nil, ErrSnapshotOverBound
		}
		children = append(children, batch...)
		if errors.Is(err, io.EOF) || err == nil && len(batch) == 0 {
			return children, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

var (
	ErrOverBound         = errors.New("file is over its bound")
	ErrSnapshotOverBound = errors.New("spec snapshot is over its bound")
	errNotDirectory      = errors.New("not a real directory")
)
