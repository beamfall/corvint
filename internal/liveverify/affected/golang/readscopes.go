package golang

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file reads the project-owned read-scope declaration (AFP-V0-023). Its
// grammar is mirrored by tools/gate-affected-select (readscopes.go) and by the
// protected confinement wrapper .github/testconfine, which enforces it; the
// three cannot share code because each is built apart from this module.

// FrontierReadScopesInvalid reports a read-scope declaration that is present
// but unreadable or invalid. No package is then declared, so every root-locating
// package stays an unbounded reader and the plan widens.
const FrontierReadScopesInvalid = "go:test-read-scopes-invalid"

// ReadScopesProfile is the declaration's closed profile name.
const ReadScopesProfile = "corvint-test-read-scopes/0"

const (
	maxReadScopeBytes    = 1 << 20
	maxReadScopePackages = 4096
	maxReadScopeEntries  = 256
)

type readScopeFile struct {
	Profile  string              `json:"profile"`
	Packages map[string][]string `json:"packages"`
}

// readScopes returns the declared scopes by package directory: nil with no
// declaration, an error for one that is unreadable, oversized, not a regular
// file, or invalid in any member.
func readScopes(root *affected.Source) (map[string][]string, error) {
	body, err := root.ReadBounded(affected.ReadScopesPath, maxReadScopeBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var declared readScopeFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != ReadScopesProfile || declared.Packages == nil || len(declared.Packages) > maxReadScopePackages {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d packages", affected.ReadScopesPath, ReadScopesProfile, maxReadScopePackages)
	}
	for directory, entries := range declared.Packages {
		if !affected.ValidRelativePath(directory) || entries == nil || len(entries) > maxReadScopeEntries {
			return nil, fmt.Errorf("%s: package %q", affected.ReadScopesPath, directory)
		}
		previous := ""
		for _, entry := range entries {
			if !affected.ValidReadScopeEntry(entry) || entry <= previous {
				return nil, fmt.Errorf("%s: package %q entry %q", affected.ReadScopesPath, directory, entry)
			}
			previous = entry
		}
	}
	return declared.Packages, nil
}

// CommittedReadScopes reads the read-scope declaration of commit from verified
// immutable Git objects, never the worktree, as the base a declaration edit is
// measured from (AFP-V0-050). An absent declaration is an empty map. A commit
// or declaration that cannot be read, or a declaration that is invalid,
// returns an error; the caller then keeps the edit bearing on every declared
// unit. Like matchReadScopes, a declared directory must hold a package at the
// commit: one the observer's walk reaches, that the observed module set owns
// (a nested module's packages are not units), outside that module's testdata,
// that directly holds a regular .go file.
func CommittedReadScopes(ctx context.Context, root, commit string) (map[string][]string, error) {
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	defer repository.BeginObjectSession()()
	source, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil, err
	}
	base := affected.FSSource(source)
	scopes, err := readScopes(base)
	if err != nil {
		return nil, err
	}
	if scopes == nil {
		scopes = map[string][]string{}
	}
	workspace, err := workspaceDirectories(base, map[string]bool{})
	if err != nil {
		return nil, err
	}
	listed := make(map[string]module, len(workspace))
	for _, directory := range workspace {
		listed[directory] = module{dir: directory, listed: true}
	}
	for directory := range scopes {
		if !committedPackageDirectory(source, listed, directory) {
			return nil, fmt.Errorf("%s at %s declares %q, which holds no Go package", affected.ReadScopesPath, commit, directory)
		}
	}
	return scopes, nil
}

func committedPackageDirectory(source fs.FS, listed map[string]module, directory string) bool {
	if !walkedDirectory(directory) {
		return false
	}
	owner, observed := committedOwner(source, listed, directory)
	if !observed || inTestdata(relativeTo(directory, owner.dir)) {
		return false
	}
	entries, err := fs.ReadDir(source, directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}

// committedOwner is enclosingModule over the commit: the nearest listed module
// or go.mod at or above directory, as observeModules assigns them. A go.mod
// below a listed module's testdata is fixture data and owns nothing; any other
// unlisted go.mod, or one that cannot be checked, owns its subtree for a module
// this plugin does not observe.
func committedOwner(source fs.FS, listed map[string]module, directory string) (module, bool) {
	for {
		if owner, known := listed[directory]; known {
			return owner, true
		}
		info, err := fs.Stat(source, path.Join(directory, "go.mod"))
		switch {
		case errors.Is(err, fs.ErrNotExist), err == nil && info.IsDir():
			// No manifest here: the observer collects only go.mod files.
		case err != nil || !info.Mode().IsRegular() || !fixtureOfObservedModule(directory, listed):
			return module{}, false
		}
		if directory == "." {
			return module{}, false
		}
		directory = path.Dir(directory)
	}
}

// walkedDirectory reports whether the observer's walk descends into directory:
// no component is hidden or skipped (affected.SkippedDirectories) unless this
// plugin admits it.
func walkedDirectory(directory string) bool {
	if directory == "." {
		return true
	}
	for _, name := range strings.Split(directory, "/") {
		if strings.HasPrefix(name, ".") || (affected.SkippedDirectories[name] && !slices.Contains(includedDirectories, name)) {
			return false
		}
	}
	return true
}

// applyReadScopes marks each declared unit, given each unit's directory. Any
// failure, including a declaration naming a directory that holds no observed
// package, declares nothing and widens every plan instead.
func applyReadScopes(root *affected.Source, units []affected.Unit, directories []string, frontier map[string]bool) {
	scopes, err := readScopes(root)
	if err == nil {
		err = matchReadScopes(scopes, units, directories)
	}
	if err != nil {
		frontier[FrontierReadScopesInvalid] = true
		for index := range units {
			units[index].ReadScoped, units[index].ReadScope = false, nil
		}
	}
}

func matchReadScopes(scopes map[string][]string, units []affected.Unit, directories []string) error {
	matched := 0
	for index, directory := range directories {
		entries, declared := scopes[directory]
		if !declared {
			continue
		}
		units[index].ReadScoped, units[index].ReadScope = true, append([]string{}, entries...)
		matched++
	}
	if matched != len(scopes) {
		return fmt.Errorf("%s declares a directory that holds no Go package", affected.ReadScopesPath)
	}
	return nil
}
