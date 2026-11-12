// SPDX-License-Identifier: Apache-2.0

// Package testinventory reads and writes the committed test inventory of the
// seven blocking runners (docs/specs/test-inventory-v1.md):
// `.corvint/test-inventory.json`, profile `corvint-test-inventory/0`, one
// record per runner instance listing its tests by canonical test ID
// (CTI-V1-001). Only `corvint inventory refresh` writes it (TIV-V1-002); every
// other caller reads it and judges it against the tree, failing closed.
package testinventory

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

const (
	// Path is the repository-relative inventory file (TIV-V1-001).
	Path = ".corvint/test-inventory.json"
	// Profile names the inventory format (TIV-V1-001, TIV-V1-012).
	Profile = "corvint-test-inventory/0"
	// MaxBytes caps the inventory file: refresh refuses a larger one with
	// ReasonTooLarge and a reader judges one invalid (TIV-V1-009, TIV-V1-011).
	MaxBytes = 8 << 20
	// MaxErrorBytes bounds one recorded lister failure.
	MaxErrorBytes = 2048
)

// The lister kinds (TIV-V1-004).
const (
	KindStatic    = "static"
	KindExecuting = "executing"
)

// Inventory is one decoded inventory file. Instances are sorted by key and
// Failures by key. Extra holds the members a newer writer added that this
// reader does not know; it is kept and written back unchanged, never read
// (TIV-V1-012).
type Inventory struct {
	Profile   string         `json:"profile"`
	Instances []Instance     `json:"instances"`
	Failures  []Failure      `json:"failures"`
	Extra     jsontext.Value `json:",embed"`
}

// Instance is one runner instance: runner, root directory and config file
// (TIV-V1-003), bound to the tree it was listed from (TIV-V1-005).
type Instance struct {
	Key            string         `json:"key"`
	Runner         string         `json:"runner"`
	Root           string         `json:"root"`
	Config         string         `json:"config"`
	ConfigDigest   string         `json:"configDigest"`
	Lockfile       string         `json:"lockfile,omitzero"`
	LockfileDigest string         `json:"lockfileDigest,omitzero"`
	Lister         Lister         `json:"lister"`
	Commit         string         `json:"commit"`
	Files          []File         `json:"files"`
	Tests          []Test         `json:"tests"`
	Extra          jsontext.Value `json:",embed"`
}

// Lister names how an instance was listed (TIV-V1-004): Command is the argv
// an executing lister ran, from the instance root, and is absent for a
// static one. Receipt is the listing receipt an executing lister converted
// its output through, when it has one (Playwright: playwright-discovery/0).
type Lister struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Kind    string         `json:"kind"`
	Command []string       `json:"command,omitzero"`
	Receipt *Receipt       `json:"receipt,omitzero"`
	Extra   jsontext.Value `json:",embed"`
}

// Receipt is the identity of a lister's own listing receipt.
type Receipt struct {
	Profile      string         `json:"profile"`
	SourceDigest string         `json:"sourceDigest"`
	Extra        jsontext.Value `json:",embed"`
}

// File is one candidate test file under the instance root and its Git blob
// object ID at refresh (TIV-V1-005). A file with no test is still listed, so
// a later reader knows it held none rather than judging it new.
type File struct {
	Path  string         `json:"path"`
	Blob  string         `json:"blob"`
	Extra jsontext.Value `json:",embed"`
}

// Test is one listed test: its canonical ID and the file it lives in
// (TIV-V1-001). Duplicate and Reason carry testid.List's marks
// (CTI-V1-010, CTI-V1-011): a file-level ID with ReasonUnmapped names a file
// whose tests the lister could not name one by one. Kind and KindSource
// label a pytest test unit, integration, e2e or UNKNOWN and say where the
// label came from (PYR-V1-005); a label never changes selection in 1.0.
type Test struct {
	ID         string         `json:"id"`
	File       string         `json:"file"`
	Duplicate  bool           `json:"duplicate,omitzero"`
	Reason     string         `json:"reason,omitzero"`
	Kind       string         `json:"kind,omitzero"`
	KindSource string         `json:"kindSource,omitzero"`
	Extra      jsontext.Value `json:",embed"`
}

// Failure is one instance whose lister failed: it is absent from Instances,
// so a reader treats it as absent (TIV-V1-011).
type Failure struct {
	Key    string         `json:"key"`
	Runner string         `json:"runner"`
	Root   string         `json:"root"`
	Config string         `json:"config"`
	Lister Lister         `json:"lister"`
	Error  string         `json:"error"`
	Extra  jsontext.Value `json:",embed"`
}

// Key is the instance key `<runner>@<root>` (TIV-V1-003).
func Key(runner, root string) string { return runner + "@" + root }

// ErrInvalid is wrapped by every Decode refusal: the file cannot be read as
// this profile (TIV-V1-009).
var ErrInvalid = errors.New("invalid test inventory")

// ErrTooLarge is returned when an encoded inventory exceeds MaxBytes
// (TIV-V1-011).
var ErrTooLarge = fmt.Errorf("test inventory exceeds %d bytes", MaxBytes)

// Decode reads an inventory: at most MaxBytes, exactly one JSON object with
// no duplicate member, profile Profile, and every record valid. Members it
// does not know are kept in Extra (TIV-V1-012). Any other input is refused
// with ErrInvalid.
func Decode(data []byte) (Inventory, error) {
	if len(data) > MaxBytes {
		return Inventory{}, fmt.Errorf("%w: %d bytes exceed the %d-byte cap", ErrInvalid, len(data), MaxBytes)
	}
	var probe struct {
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Inventory{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if probe.Profile != Profile {
		return Inventory{}, fmt.Errorf("%w: unknown profile %q", ErrInvalid, probe.Profile)
	}
	var inventory Inventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		return Inventory{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := inventory.validate(); err != nil {
		return Inventory{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return inventory, nil
}

// Encode writes inventory in canonical form: every list sorted, two-space
// indented, one trailing newline, so equal inventories give equal bytes. It
// refuses an invalid inventory and one over MaxBytes (ErrTooLarge).
func Encode(inventory Inventory) ([]byte, error) {
	inventory = inventory.canonical()
	if err := inventory.validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	data, err := json.Marshal(inventory, json.Deterministic(true), jsontext.Multiline(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}

// canonical sorts every list and turns nil lists into empty ones.
func (inventory Inventory) canonical() Inventory {
	out := inventory
	out.Instances = append([]Instance{}, inventory.Instances...)
	out.Failures = append([]Failure{}, inventory.Failures...)
	for index := range out.Instances {
		instance := &out.Instances[index]
		instance.Files = append([]File{}, instance.Files...)
		instance.Tests = append([]Test{}, instance.Tests...)
		sort.Slice(instance.Files, func(i, j int) bool { return instance.Files[i].Path < instance.Files[j].Path })
		sort.Slice(instance.Tests, func(i, j int) bool { return instance.Tests[i].ID < instance.Tests[j].ID })
	}
	sort.Slice(out.Instances, func(i, j int) bool { return out.Instances[i].Key < out.Instances[j].Key })
	sort.Slice(out.Failures, func(i, j int) bool { return out.Failures[i].Key < out.Failures[j].Key })
	return out
}

func (inventory Inventory) validate() error {
	if inventory.Profile != Profile {
		return fmt.Errorf("unknown profile %q", inventory.Profile)
	}
	if inventory.Instances == nil || inventory.Failures == nil {
		return errors.New("instances and failures must be arrays")
	}
	seen := map[string]bool{}
	for index, instance := range inventory.Instances {
		if err := instance.validate(); err != nil {
			return fmt.Errorf("instance %q: %w", instance.Key, err)
		}
		if index > 0 && inventory.Instances[index-1].Key >= instance.Key {
			return fmt.Errorf("instance %q is out of order or repeated", instance.Key)
		}
		seen[instance.Key] = true
	}
	for index, failure := range inventory.Failures {
		if err := validateIdentity(failure.Key, failure.Runner, failure.Root, failure.Config); err != nil {
			return fmt.Errorf("failure %q: %w", failure.Key, err)
		}
		if err := failure.Lister.validate(); err != nil {
			return fmt.Errorf("failure %q: %w", failure.Key, err)
		}
		if failure.Error == "" || len(failure.Error) > MaxErrorBytes {
			return fmt.Errorf("failure %q: error must hold 1 to %d bytes", failure.Key, MaxErrorBytes)
		}
		if seen[failure.Key] {
			return fmt.Errorf("failure %q is also a listed instance", failure.Key)
		}
		if index > 0 && inventory.Failures[index-1].Key >= failure.Key {
			return fmt.Errorf("failure %q is out of order or repeated", failure.Key)
		}
	}
	return nil
}

// validateIdentity checks an instance's runner, root, config and key
// (TIV-V1-003): a known runner, a clean root, a config under the root, and
// the key `<runner>@<root>`.
func validateIdentity(key, runner, root, config string) error {
	if !testid.KnownRunner(runner) {
		return fmt.Errorf("unknown runner %q", runner)
	}
	if !testid.CleanFile(root) {
		return fmt.Errorf("root %q is not a clean repository-relative path", root)
	}
	if !testid.CleanFile(config) || config == "." || !under(root, config) {
		return fmt.Errorf("config %q is not a file under root %q", config, root)
	}
	if key != Key(runner, root) {
		return fmt.Errorf("key must be %q", Key(runner, root))
	}
	return nil
}

func (instance Instance) validate() error {
	if err := validateIdentity(instance.Key, instance.Runner, instance.Root, instance.Config); err != nil {
		return err
	}
	if !objectID(instance.ConfigDigest) {
		return errors.New("configDigest is not a Git object ID")
	}
	if (instance.Lockfile == "") != (instance.LockfileDigest == "") {
		return errors.New("lockfile and lockfileDigest go together")
	}
	if instance.Lockfile != "" && (!testid.CleanFile(instance.Lockfile) || !objectID(instance.LockfileDigest)) {
		return fmt.Errorf("lockfile %q is not a clean path with a Git object ID", instance.Lockfile)
	}
	if err := instance.Lister.validate(); err != nil {
		return err
	}
	if !objectID(instance.Commit) {
		return errors.New("commit is not a full Git object ID")
	}
	if instance.Files == nil || instance.Tests == nil {
		return errors.New("files and tests must be arrays")
	}
	files := map[string]bool{}
	for index, file := range instance.Files {
		if !testid.CleanFile(file.Path) || file.Path == "." || !under(instance.Root, file.Path) {
			return fmt.Errorf("file %q is not under root %q", file.Path, instance.Root)
		}
		if !objectID(file.Blob) {
			return fmt.Errorf("file %q blob is not a Git object ID", file.Path)
		}
		if index > 0 && instance.Files[index-1].Path >= file.Path {
			return fmt.Errorf("file %q is out of order or repeated", file.Path)
		}
		files[file.Path] = true
	}
	for index, test := range instance.Tests {
		id, err := testid.Parse(test.ID)
		if err != nil {
			return err
		}
		if id.Runner != instance.Runner {
			return fmt.Errorf("test %q is not a %s test", test.ID, instance.Runner)
		}
		if !files[test.File] {
			return fmt.Errorf("test %q names unlisted file %q", test.ID, test.File)
		}
		if owner := ownerOf(instance.Runner, test.File); owner != id.File {
			return fmt.Errorf("test %q does not live in %q", test.ID, test.File)
		}
		if test.Reason != "" && (test.Reason != testid.ReasonUnmapped || !id.IsFileLevel()) {
			return fmt.Errorf("test %q has reason %q; only a file-level ID carries %q", test.ID, test.Reason, testid.ReasonUnmapped)
		}
		if index > 0 && instance.Tests[index-1].ID >= test.ID {
			return fmt.Errorf("test %q is out of order or repeated", test.ID)
		}
	}
	return nil
}

func (lister Lister) validate() error {
	if lister.Name == "" || lister.Version == "" {
		return errors.New("lister name and version are required")
	}
	switch lister.Kind {
	case KindStatic:
		if lister.Command != nil {
			return errors.New("a static lister runs no command")
		}
	case KindExecuting:
		if len(lister.Command) == 0 {
			return errors.New("an executing lister names its command")
		}
	default:
		return fmt.Errorf("lister kind %q is neither static nor executing", lister.Kind)
	}
	if lister.Receipt != nil && (lister.Receipt.Profile == "" || lister.Receipt.SourceDigest == "") {
		return errors.New("a lister receipt names its profile and source digest")
	}
	return nil
}

// ownerOf is the canonical ID file part of a test living in file: the
// package directory for Go (CTI-V1-003), the file itself otherwise.
func ownerOf(runner, file string) string {
	if runner == testid.Go {
		return path.Dir(file)
	}
	return file
}

// under reports whether the clean path file is root or lies below it.
func under(root, file string) bool {
	return root == "." || file == root || strings.HasPrefix(file, root+"/")
}

// objectID reports whether text is a full lowercase SHA-1 or SHA-256 Git
// object ID.
func objectID(text string) bool {
	if len(text) != 40 && len(text) != 64 {
		return false
	}
	return bytes.IndexFunc([]byte(text), func(r rune) bool { return !('0' <= r && r <= '9' || 'a' <= r && r <= 'f') }) < 0
}
