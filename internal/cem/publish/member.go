package publish

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// Held member access (CEM-V1-244, V1-2106). A caller that judges a root by its
// resolved location, such as an absolute map or report outside the
// repository, must not re-resolve that location by pathname afterwards: a
// parent swapped in between (for a symlink or another directory) would
// redirect the read or write. These functions open the root once as a
// directory handle, require the handle to be the very directory the root
// resolved, and reach the member only through that handle.

// hold opens r as a directory handle and refuses with code when the handle is
// not the directory r resolved.
func (r *Root) hold(code, name string) (*os.Root, error) {
	root, err := os.OpenRoot(r.path)
	if err != nil {
		return nil, cemcode.New(code, "directory of %q could not be opened", name)
	}
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(opened, r.info) {
		_ = root.Close()
		return nil, cemcode.New(code, "directory of %q changed after it was checked", name)
	}
	return root, nil
}

// directMember reports whether name is one plain entry of a directory.
func directMember(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// ReadBoundedMember reads the regular file name directly inside r through a
// held handle of r's resolved directory, with ReadBounded's bounds: no symlink
// is followed, the opened descriptor must be the file whose metadata was
// checked, and content above bound bytes is refused with code.
func (r *Root) ReadBoundedMember(name string, bound int, code string) ([]byte, error) {
	if !directMember(name) {
		return nil, cemcode.New(code, "input %q is not a direct member of its directory", name)
	}
	root, err := r.hold(code, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := root.Lstat(name)
	if err != nil {
		return nil, cemcode.New(code, "input %q is missing", name)
	}
	if !before.Mode().IsRegular() {
		return nil, cemcode.New(code, "input %q is not a regular file", name)
	}
	if before.Size() > int64(bound) {
		return nil, cemcode.New(code, "input %q exceeds %d bytes", name, bound)
	}
	if r.BeforeReadOpen != nil {
		r.BeforeReadOpen(name)
	}
	file, err := openBoundedMember(root, name)
	if err != nil {
		return nil, cemcode.New(code, "input %q could not be opened", name)
	}
	defer func() { _ = file.Close() }()
	return readOpened(file, before, bound, code, name)
}

// readOpened reads an opened input whose identity must match before, through
// a bound-plus-one limiter.
func readOpened(file *os.File, before os.FileInfo, bound int, code, name string) ([]byte, error) {
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, cemcode.New(code, "input %q changed while being read", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil || len(data) > bound || int64(len(data)) != before.Size() {
		return nil, cemcode.New(code, "input %q could not be read completely", name)
	}
	return data, nil
}

// PublishMember atomically writes output.Relative, a direct member of
// output.Root, with owner-only permissions through a held handle of the root's
// resolved directory. A final path that is not a regular file is refused.
func PublishMember(output Output) error {
	name := output.Relative
	if !directMember(name) {
		return cemcode.New(cemcode.InvalidArguments, "output %q is not a direct member of its directory", name)
	}
	root, err := output.Root.hold(cemcode.PublishFailed, name)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if info, statErr := root.Lstat(name); statErr == nil && !info.Mode().IsRegular() {
		return cemcode.New(cemcode.PublishFailed, "final output path %q is not a regular file", name)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return cemcode.New(cemcode.PublishFailed, "temporary output name cannot be generated")
	}
	staged := name + ".tmp-" + hex.EncodeToString(suffix)
	file, err := root.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "temporary output for %q cannot be created", name)
	}
	written, writeErr := file.Write(output.Data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || written != len(output.Data) || syncErr != nil || closeErr != nil {
		_ = root.Remove(staged)
		return cemcode.New(cemcode.PublishFailed, "temporary output for %q cannot be written", name)
	}
	if err := root.Rename(staged, name); err != nil {
		_ = root.Remove(staged)
		return cemcode.New(cemcode.PublishFailed, "output %q could not be published", name)
	}
	if err := syncHeldDirectory(root); err != nil {
		return cemcode.New(cemcode.PublishFailed, "output %q was published but could not be synced", name)
	}
	return nil
}
