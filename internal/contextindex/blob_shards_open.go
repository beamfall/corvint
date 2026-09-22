//go:build darwin || linux

package contextindex

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Pin each directory with a no-follow descriptor, then open the leaf without
// blocking. Directory symlink swaps cannot redirect reads or publication.
// os.Root follows a leaf symlink whose target stays inside the root even with
// O_NOFOLLOW, so the opened file must be the entry the name holds.
func openBlobShard(root, target string) (*os.File, error) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return nil, err
	}
	parent, err := openBlobShardDirectory(root, filepath.Dir(relative), false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	name := filepath.Base(relative)
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if !sameBlobShardEntry(parent, name, file) {
		file.Close()
		return nil, errors.New("symlinked blob shard refused")
	}
	return file, nil
}

func sameBlobShardEntry(parent *os.Root, name string, file *os.File) bool {
	entry, err := parent.Lstat(name)
	if err != nil {
		return false
	}
	opened, err := file.Stat()
	if err != nil {
		return false
	}
	return os.SameFile(entry, opened)
}

func openBlobShardDirectory(root, relative string, create bool) (*os.Root, error) {
	current, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		next, err := descendBlobShardDirectory(current, component, create)
		current.Close()
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func descendBlobShardDirectory(current *os.Root, component string, create bool) (*os.Root, error) {
	if !validComponent(component) {
		return nil, errors.New("blob shard directory component refused")
	}
	if create {
		if err := current.Mkdir(component, 0o755); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	file, err := current.OpenFile(component, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// OpenRoot has no NOFOLLOW option. /dev/fd pins the descriptor just opened;
	// resolving the original pathname again would reopen the symlink race.
	return os.OpenRoot("/dev/fd/" + strconv.FormatUint(uint64(file.Fd()), 10))
}

func publishBlobFact(root, target string, data []byte) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	directory, err := openBlobShardDirectory(root, filepath.Dir(relative), true)
	if err != nil {
		return err
	}
	defer directory.Close()
	return publishBlobFactAt(directory, filepath.Base(relative), data)
}

func publishBlobFactAt(directory *os.Root, name string, data []byte) error {
	if !validComponent(name) {
		return errors.New("blob shard filename refused")
	}
	temporary := "blob-" + rand.Text() + ".tmp"
	file, err := directory.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return err
	}
	defer directory.Remove(temporary)
	if err := writeAndClose(file, data); err != nil {
		return err
	}
	return directory.Rename(temporary, name)
}

// writeAndClose closes the file exactly once: on the happy path after Sync, so
// the close error is reported, and on a failed write or sync as cleanup.
func writeAndClose(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
