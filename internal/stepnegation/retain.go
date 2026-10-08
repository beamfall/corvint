package stepnegation

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// EvidenceDirectory is the worktree-relative retention directory, separate
// from .corvint/test-evidence so its discovery is unchanged (LPCV-V0-067).
const EvidenceDirectory = ".corvint/strength-evidence"

// ErrBusy is returned when another writer holds the identity's lock.
var ErrBusy = errors.New("negate-evidence-busy")

// FileName is the retained file name of a test identity.
func FileName(key TestKey) string { return key.Digest() + ".json" }

// Lock takes the per-identity writer lock, creating the retention directory
// if needed. A held lock refuses ErrBusy. The release closes the lock.
func Lock(worktree string, key TestKey) (func(), error) {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return nil, fmt.Errorf("worktree cannot be opened: %w", err)
	}
	defer root.Close()
	directory, err := openRetention(root, true)
	if err != nil {
		return nil, err
	}
	release, err := lockFile(directory, key.Digest()+".lock")
	if err != nil {
		directory.Close()
		return nil, err
	}
	return func() {
		release()
		directory.Close()
	}, nil
}

// Load reads the retained document of key, or returns nil when none exists
// or the retained bytes are not one canonical document. It never follows a
// symlinked component and never creates anything.
func Load(worktree string, key TestKey) (*Document, error) {
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return nil, fmt.Errorf("worktree cannot be opened: %w", err)
	}
	defer root.Close()
	directory, err := openRetention(root, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	info, err := directory.Lstat(FileName(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("retained step-negation document is not a regular file")
	}
	file, err := directory.Open(FileName(key))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	document, err := Decode(data)
	if err != nil {
		return nil, nil
	}
	return &document, nil
}

// Merge returns the document to retain (LPCV-V0-067). A previous document
// with an equal common binding and inventory keeps the entries next did not
// measure; any other previous document is replaced whole. Each measured
// entry whose plan digest equals the stored one reports planReused.
func Merge(previous *Document, next Document) Document {
	if previous == nil {
		return next
	}
	stored := map[int]Step{}
	for _, step := range previous.Steps {
		stored[step.Ordinal] = step
	}
	for index, step := range next.Steps {
		if old, ok := stored[step.Ordinal]; ok && step.Ordinal != 0 && step.PlanDigest != "" && old.PlanDigest == step.PlanDigest {
			next.Steps[index].PlanReused = true
		}
	}
	if previous.Binding != next.Binding || !sameInventory(previous.Inventory, next.Inventory) {
		return next
	}
	measured := map[int]bool{}
	for _, step := range next.Steps {
		measured[step.Ordinal] = true
	}
	merged := append([]Step{}, next.Steps...)
	for _, step := range previous.Steps {
		if step.Ordinal != 0 && !measured[step.Ordinal] {
			merged = append(merged, step)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Ordinal < merged[j].Ordinal })
	next.Steps = merged
	return next
}

func sameInventory(left, right Inventory) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return bytes.Equal(a, b)
}

// Retain atomically writes document as the identity's retained file: an
// exclusive 0600 temporary, fsync and rename inside 0700 directories with no
// symlinked component. The caller holds Lock.
func Retain(worktree string, document Document) error {
	data, err := Encode(document)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(worktree)
	if err != nil {
		return fmt.Errorf("worktree cannot be opened: %w", err)
	}
	defer root.Close()
	directory, err := openRetention(root, true)
	if err != nil {
		return err
	}
	defer directory.Close()
	name := FileName(document.Binding.Test.Key())
	if info, err := directory.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("retained step-negation path is not a regular file")
	}
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	temporary := "." + strings.TrimSuffix(name, ".json") + "-" + hex.EncodeToString(suffix) + ".tmp"
	file, err := directory.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("retained step-negation document cannot be created: %w", err)
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = directory.Rename(temporary, name)
	}
	if err != nil {
		_ = directory.Remove(temporary)
		return fmt.Errorf("retained step-negation document cannot be written: %w", err)
	}
	return nil
}

func openRetention(worktree *os.Root, create bool) (*os.Root, error) {
	parent := worktree
	for _, component := range strings.Split(EvidenceDirectory, "/") {
		next, err := openDirectory(parent, component, create)
		if parent != worktree {
			parent.Close()
		}
		if err != nil {
			return nil, err
		}
		parent = next
	}
	return parent, nil
}

// openDirectory opens name only when it is a real directory, the same file
// as the no-follow Lstat that admitted it, creating it 0700 when asked.
func openDirectory(parent *os.Root, name string, create bool) (*os.Root, error) {
	if create {
		if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("%s cannot be created: %w", name, err)
		}
	}
	admitted, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !admitted.IsDir() {
		return nil, fmt.Errorf("%s is not a real directory", name)
	}
	opened, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be opened: %w", name, err)
	}
	current, err := opened.Stat(".")
	if err != nil || !os.SameFile(admitted, current) {
		opened.Close()
		return nil, fmt.Errorf("%s changed while it was opened", name)
	}
	return opened, nil
}
