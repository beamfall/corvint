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
	"time"

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
	started := time.Now()
	scopes, err := committedDeclaration(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	// Checking each declared directory lists its ancestors, so a large
	// declaration would exhaust the frozen default and leave every base
	// unknown (V1-1417): the second pass's operation count scales with the
	// declaration, and both passes share one default wall budget.
	remaining := gitrun.DefaultTotalBudget - time.Since(started)
	if remaining <= 0 {
		return nil, fmt.Errorf("%s at %s: Git verification budget elapsed", affected.ReadScopesPath, commit)
	}
	repository, err := gitauth.Open(root, gitrun.NewBudget(readScopeOperations(scopes), remaining))
	if err != nil {
		return nil, err
	}
	defer repository.BeginObjectSession()()
	source, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil, err
	}
	frontier := map[string]bool{}
	workspace, err := workspaceDirectories(affected.FSSource(source), frontier)
	if err != nil {
		return nil, err
	}
	if frontier[FrontierWorkspaceUnparsed] {
		// A go.work the strict reader refuses leaves ownership unknown, so the
		// base declaration is unknown too (AFP-V0-190).
		return nil, fmt.Errorf("go.work at %s is outside the grammar the manifest reader admits", commit)
	}
	listed := make(map[string]module, len(workspace))
	for _, directory := range workspace {
		listed[directory] = module{dir: directory, listed: true}
	}
	for directory := range scopes {
		held, err := committedPackageDirectory(source, listed, directory)
		if err != nil {
			return nil, err
		}
		if !held {
			return nil, fmt.Errorf("%s at %s declares %q, which holds no Go package", affected.ReadScopesPath, commit, directory)
		}
	}
	return scopes, nil
}

// committedDeclaration reads and validates the declaration at commit under
// the frozen default budget; an absent declaration is an empty map.
func committedDeclaration(ctx context.Context, root, commit string) (map[string][]string, error) {
	repository, err := gitauth.Open(root, gitrun.NewBudget(readScopeBaseOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return nil, err
	}
	defer repository.BeginObjectSession()()
	source, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil, err
	}
	scopes, err := readScopes(affected.FSSource(source))
	if err != nil || scopes != nil {
		return scopes, err
	}
	return map[string][]string{}, nil
}

// readScopeBaseOperations is the frozen default Git operation count and
// readScopeComponentOperations the second pass's allowance per declared path
// component: listing that ancestor once and reading at most one go.mod blob
// there. A test lowers them to reach the boundary with a small fixture.
var (
	readScopeBaseOperations      = gitrun.DefaultOperations
	readScopeComponentOperations = 2
)

// readScopeOperations bounds the second pass: the default count for the
// commit, go.work and go.mod reads, plus each declared directory's path
// components.
func readScopeOperations(scopes map[string][]string) int {
	operations := readScopeBaseOperations
	for directory := range scopes {
		operations += readScopeComponentOperations * (strings.Count(directory, "/") + 1)
	}
	return operations
}

// committedPackageDirectory reports whether directory holds a Go package the
// observer would list. A Git failure (an exhausted budget, say) is returned
// rather than read as "no package", so it is never misreported as an invalid
// declaration (V1-1417).
func committedPackageDirectory(source fs.FS, listed map[string]module, directory string) (bool, error) {
	if !walkedDirectory(directory) {
		return false, nil
	}
	owner, observed, err := committedOwner(source, listed, directory)
	if err != nil || !observed || inTestdata(relativeTo(directory, owner.dir)) {
		return false, err
	}
	entries, err := fs.ReadDir(source, directory)
	if err != nil {
		return false, gitFailure(err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// gitFailure is err when the revision source failed (a Git failure or an
// exhausted bound), and nil for an absent path or an entry of the wrong type.
func gitFailure(err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
		return nil
	}
	return err
}

// committedOwner is enclosingModule over the commit: the nearest listed module
// or go.mod at or above directory, as observeModules assigns them. A go.mod
// below a listed module's testdata is fixture data and owns nothing; any other
// unlisted go.mod, or one that cannot be checked, owns its subtree for a module
// this plugin does not observe. A coded Git failure is returned instead.
func committedOwner(source fs.FS, listed map[string]module, directory string) (module, bool, error) {
	for {
		if owner, known := listed[directory]; known {
			return owner, true, nil
		}
		info, err := fs.Stat(source, path.Join(directory, "go.mod"))
		switch {
		case errors.Is(err, fs.ErrNotExist), err == nil && info.IsDir():
			// No manifest here: the observer collects only go.mod files.
		case err != nil:
			return module{}, false, gitFailure(err)
		case !info.Mode().IsRegular() || !fixtureOfObservedModule(directory, listed):
			return module{}, false, nil
		}
		if directory == "." {
			return module{}, false, nil
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
