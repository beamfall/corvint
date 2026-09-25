// Package affected selects the verification units reachable from a dirty
// worktree.
//
// The package is language-agnostic by construction. A language participates by
// implementing Language; selection, witnesses, exclusion certificates, and
// widening are then computed identically for every plugin, so adding a language
// cannot change what a selection means.
//
// This package performs no execution. It observes source text and a bounded Git
// status, and produces a plan. A plan is an observation about the dependency
// graph it names, never a claim that unselected tests cannot fail.
package affected

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidUnit reports a unit that is not in canonical form.
	ErrInvalidUnit = errors.New("affected: unit is not canonical")
	// ErrDuplicateUnit reports two units sharing one identity.
	ErrDuplicateUnit = errors.New("affected: duplicate unit identity")
	// ErrDuplicateOwner reports one path claimed by two units.
	ErrDuplicateOwner = errors.New("affected: duplicate path ownership")
	// ErrInvalidLanguage reports a plugin that broke the seam contract.
	ErrInvalidLanguage = errors.New("affected: language plugin is invalid")
)

// MaxUnits bounds one graph so a pathological tree cannot exhaust memory.
const MaxUnits = 200_000

// MaxPathsPerUnit bounds the file lists carried by one unit.
const MaxPathsPerUnit = 20_000

// Unit is one language-agnostic verification unit: the smallest thing a runtime
// provider can be asked to verify. A Go package, a Python module, and a
// JavaScript file are all units.
//
// Every path is repository-relative, slash-separated, sorted, and unique.
// Imports name other Unit identities. An import that a plugin could not resolve
// to a repository unit is omitted here and reported through Result.Frontier,
// except that the Go plugin keeps an import under an observed module that names
// no unit (a deleted package) as an edge to that absent identity.
// PathTokens are the sorted, unique path-shaped tokens of the string literals
// the unit's own files carry; every dirty path selects the units whose tokens
// name it (AFP-V0-021). A plugin that reads no literals leaves it empty.
// PathTokensBounded reports that the plugin dropped the unit's tokens at its
// bound, so the unit's reads are unknown.
// Embeds reports a Go package whose non-test files carry a //go:embed
// directive, so a data file below its directory may be compiled into it.
// TestImports name the units only the unit's own tests import. A change there
// selects the unit's tests but reaches no importer of the unit, because an
// importer never compiles another unit's tests (`go list -deps -test`).
type Unit struct {
	ID                string   `json:"id"`
	Sources           []string `json:"sources"`
	Tests             []string `json:"tests"`
	Imports           []string `json:"imports"`
	TestImports       []string `json:"testImports,omitempty"`
	PathTokens        []string `json:"pathTokens,omitempty"`
	PathTokensBounded bool     `json:"pathTokensBounded,omitempty"`
	Embeds            bool     `json:"embeds,omitempty"`
}

// Result is what one Language plugin observed for a repository.
//
// Frontier carries the reason codes for everything the plugin could not resolve
// exactly. A non-empty frontier widens the plan's scope to UNKNOWN rather than
// silently narrowing selection.
type Result struct {
	Units    []Unit
	Frontier []string
}

// Language is the seam a second language plugs into.
//
// A plugin observes source text only. It must not execute the repository, shell
// out to a package manager, or read outside root.
type Language interface {
	// Name is the plugin's namespace. Every unit it returns has the identity
	// prefix Name() + ":".
	Name() string
	// Owns reports whether a repository-relative path is source text this
	// plugin is responsible for. A dirty path owned by no plugin widens scope.
	Owns(path string) bool
	// Units observes the repository rooted at the absolute path root.
	Units(root string) (Result, error)
}

func validUnit(unit Unit, namespace string) error {
	if !strings.HasPrefix(unit.ID, namespace+":") || len(unit.ID) == len(namespace)+1 {
		return ErrInvalidUnit
	}
	if !utf8.ValidString(unit.ID) {
		return ErrInvalidUnit
	}
	if err := validPathList(unit.Sources); err != nil {
		return err
	}
	if err := validPathList(unit.Tests); err != nil {
		return err
	}
	if err := validIdentifierList(unit.Imports); err != nil {
		return err
	}
	if err := validIdentifierList(unit.TestImports); err != nil {
		return err
	}
	return validIdentifierList(unit.PathTokens)
}

func validPathList(values []string) error {
	if len(values) > MaxPathsPerUnit {
		return ErrInvalidUnit
	}
	previous := ""
	for _, value := range values {
		if !ValidRelativePath(value) || value <= previous {
			return ErrInvalidUnit
		}
		previous = value
	}
	return nil
}

func validIdentifierList(values []string) error {
	if len(values) > MaxPathsPerUnit {
		return ErrInvalidUnit
	}
	previous := ""
	for _, value := range values {
		if value == "" || !utf8.ValidString(value) || value <= previous {
			return ErrInvalidUnit
		}
		previous = value
	}
	return nil
}

// ValidRelativePath reports whether value is the canonical repository-relative
// form this package accepts everywhere: non-empty, valid UTF-8, slash
// separated, no leading slash, no "." or ".." component, and no empty component.
func ValidRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.HasPrefix(value, "/") {
		return false
	}
	if strings.ContainsRune(value, 0) || strings.Contains(value, `\`) {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}
