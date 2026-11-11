// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// Options configures one refresh.
type Options struct {
	// Git runs Git in the repository; Git.Root is its absolute root.
	Git Git
	// Exec runs executing listers; nil is RunLister.
	Exec Exec
	// Runner, when set, refreshes only the instances of this runner.
	Runner string
	// Root, when set, refreshes only the instances rooted at this
	// repository-relative directory.
	Root string
}

// selected reports whether options select instance.
func (options Options) selected(instance Declared) bool {
	return (options.Runner == "" || instance.Runner == options.Runner) && (options.Root == "" || instance.Root == options.Root)
}

// Build lists every declared instance the options select from the working
// tree at HEAD into a new inventory (TIV-V1-001, TIV-V1-004, TIV-V1-005).
// An instance whose lister fails is omitted and its failure recorded
// (TIV-V1-011). With a runner or root filter the records of the other
// instances are kept from the current inventory when it is valid; an
// invalid or absent one keeps none, so those instances read as absent.
func Build(ctx context.Context, options Options) (Inventory, error) {
	inventory, _, _, err := build(ctx, options)
	return inventory, err
}

// build is Build that also returns the HEAD it listed at and the source
// digest it listed from.
func build(ctx context.Context, options Options) (Inventory, string, string, error) {
	if options.Exec == nil {
		options.Exec = RunLister
	}
	head, err := options.Git.Head(ctx)
	if err != nil {
		return Inventory{}, "", "", err
	}
	tree, err := NewWorkTree(ctx, options.Git)
	if err != nil {
		return Inventory{}, "", "", err
	}
	files, _ := tree.Files()
	declared := Detect(files, tree.Read)
	inventory := Inventory{Profile: Profile, Instances: []Instance{}, Failures: []Failure{}}
	if options.Runner != "" || options.Root != "" {
		if data, err := tree.Read(Path, MaxBytes); err == nil {
			if previous, err := Decode(data); err == nil {
				for _, instance := range previous.Instances {
					if !options.selected(Declared{Runner: instance.Runner, Root: instance.Root}) {
						inventory.Instances = append(inventory.Instances, instance)
					}
				}
				for _, failure := range previous.Failures {
					if !options.selected(Declared{Runner: failure.Runner, Root: failure.Root}) {
						inventory.Failures = append(inventory.Failures, failure)
					}
				}
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return Inventory{}, "", "", err
		}
	}
	input := listInput{Root: options.Git.Root, Tree: tree, Files: files, Declared: declared, Commit: head, Exec: options.Exec}
	before := sourceDigest(files)
	for _, instance := range declared {
		if !options.selected(instance) {
			continue
		}
		record, lister, err := listInstance(ctx, input, instance)
		afterLister(instance)
		if drift := checkDrift(ctx, options.Git, head, before, "the "+instance.Key+" lister ran"); drift != nil {
			return Inventory{}, "", "", drift
		}
		if err != nil {
			inventory.Failures = append(inventory.Failures, Failure{
				Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config,
				Lister: lister, Error: boundedError(err),
			})
			continue
		}
		inventory.Instances = append(inventory.Instances, record)
	}
	return inventory.canonical(), head, before, nil
}

// Refusal is a refresh that writes nothing: Code is ReasonDrift or
// ReasonTooLarge, and the previous inventory is left as it was.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Message }

// afterLister runs after each lister, before its drift check; beforeWrite
// runs before the final drift check ahead of the write. Tests replace them
// to move HEAD or a source at that point.
var (
	afterLister = func(Declared) {}
	beforeWrite = func() {}
)

// checkDrift refuses with ReasonDrift when HEAD or the working tree's
// source digest moved since listing began (TIV-V1-010); during names the
// step it checks after. The digest is read before HEAD, so a commit landing
// while the sources are read is caught.
func checkDrift(ctx context.Context, git Git, head, before, during string) error {
	tree, err := NewWorkTree(ctx, git)
	if err != nil {
		return err
	}
	files, _ := tree.Files()
	if sourceDigest(files) != before {
		return &Refusal{Code: ReasonDrift, Message: "the working tree changed while " + during + "; the inventory was not written"}
	}
	if now, err := git.Head(ctx); err != nil || now != head {
		return &Refusal{Code: ReasonDrift, Message: "HEAD moved while " + during + "; the inventory was not written"}
	}
	return nil
}

// tempPattern is the name writeAtomic gives its temporary file beside Path.
const tempPattern = ".test-inventory-*.tmp"

// sourceDigest is the SHA-256 of every listed file and its blob, the
// inventory file itself and writeAtomic's temporary files excluded.
func sourceDigest(files map[string]string) string {
	names := make([]string, 0, len(files))
	for name := range files {
		if name == Path {
			continue
		}
		if dir, base := path.Split(name); dir == path.Dir(Path)+"/" {
			if temp, _ := path.Match(tempPattern, base); temp {
				continue
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		fmt.Fprintf(digest, "%s\x00%s\n", name, files[name])
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// Result is what one refresh wrote.
type Result struct {
	Commit    string
	Inventory Inventory
	Bytes     int
	Changed   bool
}

// Refresh builds the inventory and replaces Path with it by writing a
// temporary file beside it and renaming it into place (TIV-V1-002). It
// refuses, leaving the previous file unchanged, on drift (ReasonDrift) and
// when the encoded inventory exceeds MaxBytes (ReasonTooLarge,
// TIV-V1-011). One final drift check runs just before the write, also when
// the options select no instance and no lister ran.
func Refresh(ctx context.Context, options Options) (Result, error) {
	inventory, head, before, err := build(ctx, options)
	if err != nil {
		return Result{}, err
	}
	data, err := Encode(inventory)
	if errors.Is(err, ErrTooLarge) {
		return Result{}, &Refusal{Code: ReasonTooLarge, Message: fmt.Sprintf("the refreshed inventory exceeds the %d-byte cap; the inventory was not written", MaxBytes)}
	}
	if err != nil {
		return Result{}, err
	}
	target := filepath.Join(options.Git.Root, filepath.FromSlash(Path))
	previous, _ := os.ReadFile(target)
	result := Result{Commit: head, Inventory: inventory, Bytes: len(data), Changed: !bytes.Equal(previous, data)}
	if !result.Changed {
		return result, nil
	}
	beforeWrite()
	if drift := checkDrift(ctx, options.Git, head, before, "refresh encoded the inventory"); drift != nil {
		return Result{}, drift
	}
	return result, writeAtomic(target, data)
}

// writeAtomic replaces target with data through a temporary file in the
// same directory, so a reader sees the old or the new file, never a part.
func writeAtomic(target string, data []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, target)
}

// boundedError is err's text cut to MaxErrorBytes on a rune boundary.
func boundedError(err error) string {
	text := strings.ToValidUTF8(err.Error(), "\uFFFD")
	if text == "" {
		text = fmt.Sprintf("%T", err)
	}
	for len(text) > MaxErrorBytes {
		_, size := utf8.DecodeLastRuneInString(text)
		text = text[:len(text)-size]
	}
	return text
}
