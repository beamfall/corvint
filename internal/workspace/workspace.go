// SPDX-License-Identifier: Apache-2.0

// Package workspace reads the corvint-workspace/0 declaration of the repositories one
// multi-repository command works across (docs/specs/workspace-declaration-v0.md, WSD-V0): each
// alias's checkout, the alias holding the manifest and default revisions. It names checkouts on
// this machine and carries no evidence: a verb that reads a root still pins the revision it read.
package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jsonend"
	"github.com/Beamfall/corvint/internal/rootalias"
)

const (
	// Profile names the schema of a workspace declaration (WSD-V0-001).
	Profile = "corvint-workspace/0"
	// DiscoveryPath is where a verb looks under --root when no workspace is named (WSD-V0-003).
	DiscoveryPath = ".corvint/workspace.json"
	// MaxBytes bounds one declaration file (WSD-V0-002).
	MaxBytes = 64 << 10
	// maxRevisionBytes bounds one default revision.
	maxRevisionBytes = 256
)

// Repo is one declared alias: its checkout and an optional default revision.
type Repo struct {
	Root     string `json:"root"`
	Revision string `json:"revision,omitempty"`
}

// Workspace is a decoded, validated declaration. Repos roots are absolute and clean.
type Workspace struct {
	Profile      string          `json:"profile"`
	Repos        map[string]Repo `json:"repos"`
	ManifestRepo string          `json:"manifest_repo,omitempty"`
	// File is the absolute path the declaration was read from.
	File string `json:"-"`
}

// Aliases returns the declared aliases in order.
func (w *Workspace) Aliases() []string {
	out := make([]string, 0, len(w.Repos))
	for alias := range w.Repos {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func invalid(file, format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)
	return &gokernel.Error{Code: "workspace-invalid", Message: fmt.Sprintf("workspace-invalid: %s: %s", file, detail)}
}

// Load reads and validates the declaration at file (WSD-V0-001, WSD-V0-002). Relative roots
// resolve against the directory holding the file.
func Load(file string) (*Workspace, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, invalid(file, "path is unusable")
	}
	raw, err := readBounded(abs)
	if err != nil {
		return nil, invalid(abs, "%v", err)
	}
	return decode(abs, raw)
}

// Discover reads root/.corvint/workspace.json when it exists (WSD-V0-003). An absent file is no
// workspace; a present one that is not a valid declaration refuses rather than being skipped.
func Discover(root string) (*Workspace, error) {
	file := filepath.Join(root, filepath.FromSlash(DiscoveryPath))
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return Load(file)
}

func readBounded(file string) ([]byte, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return nil, errors.New("is unreadable")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("is not a regular file")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.New("is unreadable")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, errors.New("is unreadable")
	}
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("exceeds %d bytes", MaxBytes)
	}
	return raw, nil
}

func decode(file string, raw []byte) (*Workspace, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	w := &Workspace{}
	if err := dec.Decode(w); err != nil {
		return nil, invalid(file, "not a %s object: %v", Profile, err)
	}
	if !jsonend.Done(dec) {
		return nil, invalid(file, "trailing data after the object")
	}
	if w.Profile != Profile {
		return nil, invalid(file, "profile must be %q", Profile)
	}
	if len(w.Repos) == 0 || len(w.Repos) > rootalias.MaxRoots {
		return nil, invalid(file, "repos must declare 1 to %d aliases", rootalias.MaxRoots)
	}
	dir := filepath.Dir(file)
	for _, alias := range w.Aliases() {
		r := w.Repos[alias]
		if !rootalias.Valid(alias) {
			return nil, invalid(file, "alias %q must match [a-z][a-z0-9-]{0,31}", alias)
		}
		root := filepath.FromSlash(r.Root)
		if r.Root != "" && !filepath.IsAbs(root) {
			root = filepath.Join(dir, root)
		}
		if !rootalias.ValidRoot(root) {
			return nil, invalid(file, "repos.%s.root must be a path, absolute or relative to the file's directory", alias)
		}
		if r.Revision != "" && !validRevision(r.Revision) {
			return nil, invalid(file, "repos.%s.revision is not a revision spelling", alias)
		}
		w.Repos[alias] = Repo{Root: root, Revision: r.Revision}
	}
	if w.ManifestRepo != "" {
		if _, ok := w.Repos[w.ManifestRepo]; !ok {
			return nil, invalid(file, "manifest_repo %q is not a declared alias", w.ManifestRepo)
		}
	}
	w.File = file
	return w, nil
}

// validRevision admits a bounded revision spelling that Git cannot read as an option.
func validRevision(rev string) bool {
	if len(rev) > maxRevisionBytes || strings.HasPrefix(rev, "-") {
		return false
	}
	for _, c := range rev {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return false
		}
	}
	return true
}
