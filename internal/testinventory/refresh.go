// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	if options.Exec == nil {
		options.Exec = RunLister
	}
	head, err := options.Git.Head(ctx)
	if err != nil {
		return Inventory{}, err
	}
	tree, err := NewWorkTree(ctx, options.Git)
	if err != nil {
		return Inventory{}, err
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
			return Inventory{}, err
		}
	}
	input := listInput{Root: options.Git.Root, Tree: tree, Files: files, Declared: declared, Commit: head, Exec: options.Exec}
	for _, instance := range declared {
		if !options.selected(instance) {
			continue
		}
		record, lister, err := listInstance(ctx, input, instance)
		if err != nil {
			inventory.Failures = append(inventory.Failures, Failure{
				Key: instance.Key, Runner: instance.Runner, Root: instance.Root, Config: instance.Config,
				Lister: lister, Error: boundedError(err),
			})
			continue
		}
		inventory.Instances = append(inventory.Instances, record)
	}
	return inventory.canonical(), nil
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
