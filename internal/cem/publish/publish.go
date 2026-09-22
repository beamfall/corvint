// Package publish is the descriptor-rooted bounded reader and transactional
// output publisher for CEM artifacts.
//
// Every path is repository-root-relative and validated before any directory is
// created; ancestors are walked with no-follow checks so an output can never
// escape its root through a symlinked directory, an absolute path, or a
// dot-dot segment. Two-output publication is transactional: if the second
// output cannot land, the first's exact prior file is restored by atomic
// rename (or the fresh file removed when none existed) and no temporary or
// backup residue remains.
package publish

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Test seams: openInputFile and renameOutput let regressions inject a
// concurrent path swap between metadata capture and open, or a rename
// failure after earlier renames succeeded. Production behavior is identical.
var (
	openInputFile       = openBoundedInput
	renameOutput        = os.Rename
	syncOutputDirectory = syncDirectory
)

// Root is one validated publication root.
type Root struct {
	path string
	info os.FileInfo
}

// OpenRoot validates one existing directory as a publication root.
func OpenRoot(path string) (*Root, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, cemcode.New(cemcode.PublishFailed, "publication root does not resolve")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return nil, cemcode.New(cemcode.PublishFailed, "publication root is not a directory")
	}
	return &Root{path: resolved, info: info}, nil
}

// Path returns the root's resolved absolute path.
func (r *Root) Path() string { return r.path }

// resolve validates one root-relative POSIX path and returns its absolute
// location, requiring every existing ancestor to be a real directory.
func (r *Root) resolve(relative string) (string, error) {
	if err := wire.ValidatePath(relative); err != nil {
		return "", cemcode.New(cemcode.InvalidArguments, "output path must be a relative POSIX path: %v", err)
	}
	segments := strings.Split(relative, "/")
	current := r.path
	for _, segment := range segments[:len(segments)-1] {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			continue // missing ancestors are created at write time
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", cemcode.New(cemcode.PublishFailed, "output ancestor %q is not a real directory", segment)
		}
	}
	return filepath.Join(r.path, filepath.FromSlash(relative)), nil
}

// resolveOutput applies the repository-metadata boundary in addition to the
// ordinary relative-path and no-follow ancestor checks. A Root may itself be
// a verified Git directory, but a caller rooted at a worktree cannot select a
// nested .git segment as an output.
func (r *Root) resolveOutput(relative string) (string, error) {
	if err := wire.ValidatePath(relative); err != nil {
		return "", cemcode.New(cemcode.InvalidArguments, "output path must be a relative POSIX path: %v", err)
	}
	for _, segment := range strings.Split(relative, "/") {
		if strings.EqualFold(segment, ".git") {
			return "", cemcode.New(cemcode.InvalidArguments, "output path must not name Git metadata")
		}
	}
	return r.resolve(relative)
}

// ReadBounded reads one root-relative regular file without following a symlink
// at any component, rejecting content above bound bytes. An ancestor that is
// not a real directory is the caller's read refusal, not a publication one.
func (r *Root) ReadBounded(relative string, bound int, code string) ([]byte, error) {
	full, err := r.resolve(relative)
	if cemcode.CodeOf(err) == cemcode.PublishFailed {
		return nil, cemcode.New(code, "input %q has an ancestor that is not a real directory", relative)
	}
	if err != nil {
		return nil, err
	}
	return ReadBoundedFile(full, bound, code)
}

// ReadBoundedFile reads one absolute-path regular file with the same bounds.
// The opened descriptor's identity must match the pre-open metadata, so a
// path swapped for a symlink between the checks reads nothing else, and all
// bytes flow through a bound-plus-one limiter so growth after the size check
// can never allocate beyond the bound.
func ReadBoundedFile(path string, bound int, code string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, cemcode.New(code, "input %q is missing", path)
	}
	if !before.Mode().IsRegular() {
		return nil, cemcode.New(code, "input %q is not a regular file", path)
	}
	if before.Size() > int64(bound) {
		return nil, cemcode.New(code, "input %q exceeds %d bytes", path, bound)
	}
	file, err := openInputFile(path)
	if err != nil {
		return nil, cemcode.New(code, "input %q could not be opened", path)
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, cemcode.New(code, "input %q changed while being read", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil || len(data) > bound || int64(len(data)) != before.Size() {
		return nil, cemcode.New(code, "input %q could not be read completely", path)
	}
	return data, nil
}

// Output names one root-relative publication target.
type Output struct {
	Root     *Root
	Relative string
	Data     []byte
}

// Publish atomically writes one output with owner-only permissions, refusing a
// symlink as the final path.
func Publish(output Output) error {
	staged, final, err := stage(output)
	if err != nil {
		return err
	}
	if err := renameOutput(staged, final); err != nil {
		_ = os.Remove(staged)
		return cemcode.New(cemcode.PublishFailed, "output %q could not be published", output.Relative)
	}
	if err := syncOutputDirectory(filepath.Dir(final)); err != nil {
		return cemcode.New(cemcode.PublishFailed, "output %q was published but could not be synced", output.Relative)
	}
	return nil
}

// PublishPair publishes two outputs transactionally: both land, or neither
// does. Both outputs are fully staged before either final path is touched,
// the first's prior file is set aside by atomic rename so a failed pair
// restores the exact prior inode, and a restoration failure is reported —
// never swallowed. The two targets must be distinct files. A crash after the
// prior file is set aside but before the first output lands leaves only the
// backup; callers holding the output's Lock repair that with
// RecoverPairBackup before treating the first output as missing.
func PublishPair(first, second Output) error {
	stagedFirst, finalFirst, err := stage(first)
	if err != nil {
		return err
	}
	stagedSecond, finalSecond, err := stage(second)
	if err != nil {
		_ = os.Remove(stagedFirst)
		return err
	}
	if finalFirst == finalSecond {
		_ = os.Remove(stagedFirst)
		_ = os.Remove(stagedSecond)
		return cemcode.New(cemcode.PublishFailed, "pair outputs %q and %q select the same file", first.Relative, second.Relative)
	}
	backup, hadPrior, err := backupPrior(finalFirst, first.Relative)
	if err != nil {
		_ = os.Remove(stagedFirst)
		_ = os.Remove(stagedSecond)
		return err
	}
	if err := renameOutput(stagedFirst, finalFirst); err != nil {
		_ = os.Remove(stagedFirst)
		_ = os.Remove(stagedSecond)
		if restoreErr := restorePrior(finalFirst, backup, hadPrior); restoreErr != nil {
			return cemcode.New(cemcode.PublishFailed,
				"output %q could not be published and its prior file could not be restored", first.Relative)
		}
		return cemcode.New(cemcode.PublishFailed, "output %q could not be published", first.Relative)
	}
	if aliased(finalFirst, finalSecond) {
		_ = os.Remove(stagedSecond)
		restoreErr := restorePrior(finalFirst, backup, hadPrior)
		if restoreErr != nil {
			return cemcode.New(cemcode.PublishFailed,
				"pair outputs %q and %q alias one file and the first could not be restored", first.Relative, second.Relative)
		}
		return cemcode.New(cemcode.PublishFailed, "pair outputs %q and %q alias one file", first.Relative, second.Relative)
	}
	if err := renameOutput(stagedSecond, finalSecond); err != nil {
		_ = os.Remove(stagedSecond)
		if restoreErr := restorePrior(finalFirst, backup, hadPrior); restoreErr != nil {
			return cemcode.New(cemcode.PublishFailed,
				"second output %q could not be published and first output %q could not be restored", second.Relative, first.Relative)
		}
		return cemcode.New(cemcode.PublishFailed, "second output %q could not be published; first was rolled back", second.Relative)
	}
	firstErr := syncOutputDirectory(filepath.Dir(finalFirst))
	secondErr := syncOutputDirectory(filepath.Dir(finalSecond))
	if firstErr != nil || secondErr != nil {
		return cemcode.New(cemcode.PublishFailed,
			"outputs %q and %q were published but could not be synced", first.Relative, second.Relative)
	}
	if hadPrior {
		if err := os.Remove(backup); err != nil {
			return cemcode.New(cemcode.PublishFailed,
				"outputs %q and %q were published but the prior-file backup could not be removed", first.Relative, second.Relative)
		}
		if err := syncOutputDirectory(filepath.Dir(backup)); err != nil {
			return cemcode.New(cemcode.PublishFailed,
				"outputs %q and %q were published but the prior-file backup removal could not be synced", first.Relative, second.Relative)
		}
	}
	return nil
}

// aliased reports whether the second final path already refers to the file
// just published at the first final path — the case-folding or
// normalization-insensitive filesystem alias that byte comparison misses.
func aliased(finalFirst, finalSecond string) bool {
	firstInfo, firstErr := os.Lstat(finalFirst)
	secondInfo, secondErr := os.Lstat(finalSecond)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

// backupPrior moves an existing prior first output aside by atomic rename in
// the same directory so a failed pair can restore the exact prior file.
func backupPrior(final, relative string) (string, bool, error) {
	info, err := os.Lstat(final)
	if err != nil {
		return "", false, nil
	}
	if !info.Mode().IsRegular() {
		return "", false, cemcode.New(cemcode.PublishFailed, "output %q exists and is not a regular file", relative)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", false, cemcode.New(cemcode.PublishFailed, "backup name for %q cannot be generated", relative)
	}
	backup := final + ".bak-" + hex.EncodeToString(suffix)
	if err := os.Rename(final, backup); err != nil {
		return "", false, cemcode.New(cemcode.PublishFailed, "prior output %q could not be set aside", relative)
	}
	if err := syncOutputDirectory(filepath.Dir(final)); err != nil {
		if restoreErr := restorePrior(final, backup, true); restoreErr != nil {
			return "", false, cemcode.New(cemcode.PublishFailed,
				"prior output %q could not be set aside durably or restored", relative)
		}
		return "", false, cemcode.New(cemcode.PublishFailed, "prior output %q could not be set aside durably", relative)
	}
	return backup, true, nil
}

// restorePrior undoes backupPrior: any freshly published first output is
// replaced (or removed) so the final path holds exactly the prior state.
func restorePrior(final, backup string, hadPrior bool) error {
	if hadPrior {
		if err := os.Rename(backup, final); err != nil {
			return err
		}
		return syncOutputDirectory(filepath.Dir(final))
	}
	err := os.Remove(final)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncOutputDirectory(filepath.Dir(final))
}

// stage validates the final path, creates missing ancestors inside the root,
// and writes a private temporary file beside the final location.
func stage(output Output) (string, string, error) {
	final, err := output.Root.resolveOutput(output.Relative)
	if err != nil {
		return "", "", err
	}
	if info, statErr := os.Lstat(final); statErr == nil {
		if !info.Mode().IsRegular() {
			return "", "", cemcode.New(cemcode.PublishFailed, "final output path %q is not a regular file", output.Relative)
		}
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return "", "", cemcode.New(cemcode.PublishFailed, "output directory for %q cannot be created", output.Relative)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", "", cemcode.New(cemcode.PublishFailed, "temporary output name cannot be generated")
	}
	staged := final + ".tmp-" + hex.EncodeToString(suffix)
	file, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", cemcode.New(cemcode.PublishFailed, "temporary output for %q cannot be created", output.Relative)
	}
	written, writeErr := file.Write(output.Data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || written != len(output.Data) || syncErr != nil || closeErr != nil {
		_ = os.Remove(staged)
		return "", "", cemcode.New(cemcode.PublishFailed, "temporary output for %q cannot be written", output.Relative)
	}
	return staged, final, nil
}
