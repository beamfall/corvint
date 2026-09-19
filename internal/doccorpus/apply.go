package doccorpus

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// Publication is confined to a pinned parent and never replaces a competing
// destination. The original inode remains named as recovery evidence, including
// writes from an editor that held it open across publication. There is a brief
// absent-destination window; this is not a crash-atomic filesystem transaction.
func applyCorpusPage(root, page string, before, next []byte, checkpoint func(string)) (string, error) {
	if !validCorpusPage(page) {
		return "", fail("invalid maintenance page")
	}
	repository, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer repository.Close()
	parent, err := corpusParent(repository, path.Dir(page))
	if err != nil {
		return "", err
	}
	defer parent.Close()
	pinned, err := parent.Stat(".")
	if err != nil {
		return "", err
	}
	name := path.Base(page)
	original, err := parent.Lstat(name)
	if err != nil || !original.Mode().IsRegular() {
		return "", fail("maintenance page is not regular")
	}
	current, err := testvaliditydoc.ReadFile(parent, name)
	if err != nil || !bytes.Equal(current, before) {
		return "", fail("maintenance page changed")
	}
	temporary := ".corvint-corpus-stage-" + rand.Text()
	file, err := parent.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer parent.Remove(temporary)
	if err = file.Chmod(original.Mode().Perm()); err == nil {
		_, err = file.Write(next)
	}
	if err == nil {
		err = file.Sync()
	}
	closed := file.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		return "", err
	}
	if checkpoint != nil {
		checkpoint("before-capture")
	}
	liveParent, err := corpusParent(repository, path.Dir(page))
	if err != nil {
		return "", fail("maintenance parent changed")
	}
	liveInfo, statErr := liveParent.Stat(".")
	_ = liveParent.Close()
	if statErr != nil || !os.SameFile(pinned, liveInfo) {
		return "", fail("maintenance parent changed")
	}
	// The random backup name is reserved by exclusive creation before the move.
	recovery := ".corvint-corpus-recovery-" + rand.Text()
	reserved, err := parent.OpenFile(recovery, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_ = reserved.Close()
	recoveryPath := path.Join(path.Dir(page), recovery)
	if err = parent.Rename(name, recovery); err != nil {
		_ = parent.Remove(recovery)
		return "", err
	}
	refuse := func(reason string) (string, error) {
		// Link is no-clobber: a competing destination always wins. Keep the captured
		// inode even after restoration, since an existing writer may still own it.
		_ = parent.Link(recovery, name)
		return recoveryPath, fail(fmt.Sprintf("%s; captured page retained at %s", reason, recoveryPath))
	}
	captured, err := parent.Lstat(recovery)
	if err != nil || !os.SameFile(original, captured) || !captured.Mode().IsRegular() {
		return refuse("maintenance destination replaced")
	}
	capturedBytes, err := testvaliditydoc.ReadFile(parent, recovery)
	if err != nil || !bytes.Equal(capturedBytes, before) {
		return refuse("maintenance destination changed")
	}
	if checkpoint != nil {
		checkpoint("before-publish")
	}
	if err = parent.Link(temporary, name); err != nil {
		return refuse("maintenance publication refused")
	}
	return recoveryPath, nil
}

func corpusParent(repository *os.Root, relative string) (*os.Root, error) {
	current, err := repository.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if relative == "." {
		return current, nil
	}
	for _, component := range strings.Split(relative, "/") {
		info, err := current.Lstat(component)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, fail("unsafe maintenance parent")
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			current.Close()
			return nil, err
		}
		opened, err := next.Stat(".")
		current.Close()
		if err != nil || !os.SameFile(info, opened) {
			next.Close()
			return nil, fail("maintenance parent changed")
		}
		current = next
	}
	return current, nil
}

func validCorpusPage(page string) bool {
	if !validPath(page) || path.Ext(page) != ".md" {
		return false
	}
	for _, component := range strings.Split(page, "/") {
		if strings.EqualFold(component, ".git") {
			return false
		}
	}
	return true
}
