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
	"context"
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
// PathGlobs are the sorted, unique literal glob patterns the unit's files pass
// to a file-system glob (AFP-V0-401): a pattern with a leading "/" is anchored
// at the repository root, any other one follows a root known only at run time
// and matches a run of a dirty path's components. A dirty path one matches
// selects the unit with a PATH_GLOB_READER witness naming the pattern.
// PlacesFileNames reports a non-Go unit whose plugin certifies that a lone
// file-name token reaches a path only where namesFileOnlyWithin places it
// (AFP-V0-400): the shell plugin sets it for a script that changes to no
// literal directory and reads nothing recursively.
// PathTokensBounded reports that the plugin dropped the unit's tokens at its
// bound, so the unit's reads are unknown.
// Embeds reports a Go package whose non-test files carry a //go:embed
// directive, so a data file below its directory may be compiled into it.
// EmbedFiles lists, sorted, the repository files those directives resolve to
// exactly (AFP-V0-600): files compiled into the unit as its source text is,
// so a dirty one is owned by the unit instead of by none. A directive the
// plugin cannot resolve exactly contributes nothing.
// UnboundedReads names why no literal bounds what the unit reads (a
// root-locating call or a literal that climbs out of its package; AFP-V0-012
// rule (d)); LocatesRoot reports that a non-test file is the cause, so every
// unit that compiles this one is unbounded too.
// ReadScoped reports a unit whose tests' reads the repository declares
// (AFP-V0-023): ReadScope then holds the sorted, unique repository paths they
// read outside the unit's directory, a trailing "/" naming a subtree, and the
// unit is selected only on a dirty path in that scope instead of on any.
// Frontier holds the sorted, unique reasons that bear only on this unit: the
// plan names them only when the unit is reached (V1-0289) or narrowed away as
// a file-name reader (AFP-V0-072(4)); with FrontierBearsOnDependents only on a
// plan that follows the unit's dependents (AFP-V0-581); with
// FrontierBearsOnOwnFiles only on a plan with a dirty path directly in the
// unit's directory or an ancestor (TSP-V0-001).
// TestImports name the units only the unit's own tests import. A change there
// selects the unit's tests but reaches no importer of the unit, because an
// importer never compiles another unit's tests (`go list -deps -test`).
// Execs name the command units whose built binary the unit's code or tests
// run (AFP-V0-037): no import edge reaches the unit from the command, so a
// change to the command's build selects the unit through this edge instead.
// AdvisoryImports name units the unit reaches through a convention the
// plugin resolved without an explicit load, such as a Ruby constant that
// autoloading maps to a file (RUBY-AFFECTED-V0-009). A change there selects
// the unit like an import does, but the witness says the edge is advisory.
// RouteDispatchImports name units a test reaches only because the framework
// dispatches a request to them, with no constant naming them, such as every
// Rails controller for a request spec (RUBY-AFFECTED-V0-049). A change there
// selects the unit like an advisory edge, with a ROUTE_DISPATCH_BOOT witness.
type Unit struct {
	ID                   string   `json:"id"`
	Sources              []string `json:"sources"`
	Tests                []string `json:"tests"`
	Imports              []string `json:"imports"`
	TestImports          []string `json:"testImports,omitempty"`
	Execs                []string `json:"execs,omitempty"`
	AdvisoryImports      []string `json:"advisoryImports,omitempty"`
	RouteDispatchImports []string `json:"routeDispatchImports,omitempty"`
	PathTokens           []string `json:"pathTokens,omitempty"`
	PathGlobs            []string `json:"pathGlobs,omitempty"`
	PathTokensBounded    bool     `json:"pathTokensBounded,omitempty"`
	PlacesFileNames      bool     `json:"placesFileNames,omitempty"`
	Embeds               bool     `json:"embeds,omitempty"`
	EmbedFiles           []string `json:"embedFiles,omitempty"`
	UnboundedReads       string   `json:"unboundedReads,omitempty"`
	LocatesRoot          bool     `json:"locatesRoot,omitempty"`
	ReadScoped           bool     `json:"readScoped,omitempty"`
	ReadScope            []string `json:"readScope,omitempty"`
	Frontier             []string `json:"frontier,omitempty"`
	FrontierBears        string   `json:"frontierBears,omitempty"`
}

// FrontierBears values. The empty value names a unit's Frontier on every plan
// that reaches it (V1-0289).
const (
	// FrontierBearsOnDependents names it only on a plan that follows the
	// unit's dependents (AFP-V0-581).
	FrontierBearsOnDependents = "DEPENDENTS"
	// FrontierBearsOnOwnFiles names it only on a plan with a dirty path
	// directly in the unit's own directory or in an ancestor, the only places
	// its files and its module's build configuration live (TSP-V0-001).
	FrontierBearsOnOwnFiles = "OWN_FILES"
)

// Result is what one Language plugin observed for a repository.
//
// Frontier carries the reason codes for everything the plugin could not resolve
// exactly. A non-empty frontier widens the scope of every plan the plugin takes
// part in to UNKNOWN rather than silently narrowing selection.
//
// Unobserved names, sorted and unique, the repository-relative directories
// whose source files the plugin owns but deliberately builds no unit for, such
// as a nested Go module outside the observed module set. A dirty path the
// plugin claims at or below one of them, which no unit declares, is reported
// as that directory's frontier rather than as an unindexed source path
// (AFP-V0-191).
//
// Runnable names, sorted and unique, the subset of Unobserved that the
// plugin's own runner can test as a root of its own: for Go, a nested module
// that no observed module builds, whose own `go test` covers its files. A
// dirty path at or below one of them is that directory's unobserved-module
// frontier, which `corvint test` turns into an execution unit run from the
// directory (AFP-V0-650).
type Result struct {
	Units      []Unit
	Frontier   []string
	Unobserved []string
	Runnable   []string
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
	Units(ctx context.Context, root string) (Result, error)
}

// PathReader is implemented by a plugin that records path tokens, so that its
// units read any dirty path (AFP-V0-021, rule (d)). Its frontier then takes
// part in every plan with a dirty path, whichever plugin owns it.
type PathReader interface {
	ReadsAnyPath() bool
}

// FrontierReach is implemented by a plugin that classifies its frontier reasons
// by the paths their hidden edge can bind (AFP-V0-511). A reason is the first
// two colon-separated fields of a frontier entry, `<plugin>:<category>`. A
// plugin that does not implement it, and every reason it does not name, is
// path-open: the hidden edge may bind any path, so the reason bears on a dirty
// path no plugin owns. Both lists are validated at Build: each reason carries
// the plugin's own namespace and no further field. A plugin that resolves
// against files it does not own, such as every repository file, must not
// implement it.
type FrontierReach interface {
	// SourceBoundFrontiers names the reasons whose hidden edge the plugin's
	// own resolution binds only to a path its Owns claims, one of its units
	// lists, or one the site's unit records as a path token, so that the
	// dirty path reaches the unit by rule (d) instead. Such a reason bears on
	// no dirty path that no plugin owns and no ResolutionInput reads.
	SourceBoundFrontiers() []string
	// ResolutionInput reports a repository-relative path the plugin reads
	// although its Owns does not claim it: a manifest, build or test-runner
	// configuration, or a foreign asset it scans. A change there may rebind
	// any site the plugin resolved, so a dirty path it reports keeps every
	// frontier entry of the plugin in the plan. Plugins answer with the same
	// predicate their Units pass to Source.Files, applied to the base name.
	ResolutionInput(path string) bool
	// FixtureClosedFrontiers names the path-open reasons whose hidden edge
	// cannot bind a file below a `testdata` directory a Go unit hosts in which
	// the plugin lists no path and raises no site: the binding is to a test
	// runner, to a build graph between the plugin's own units, or to a file in
	// an ancestor directory of the site's own.
	FixtureClosedFrontiers() []string
}

func validUnit(unit Unit, namespace string) error {
	if !strings.HasPrefix(unit.ID, namespace+":") || len(unit.ID) == len(namespace)+1 {
		return ErrInvalidUnit
	}
	if !utf8.ValidString(unit.ID) || !utf8.ValidString(unit.UnboundedReads) {
		return ErrInvalidUnit
	}
	if unit.LocatesRoot && unit.UnboundedReads == "" {
		return ErrInvalidUnit
	}
	if err := validReadScope(unit.ReadScoped, unit.ReadScope); err != nil {
		return err
	}
	if err := validPathList(unit.Sources); err != nil {
		return err
	}
	if err := validPathList(unit.Tests); err != nil {
		return err
	}
	if err := validPathList(unit.EmbedFiles); err != nil {
		return err
	}
	if err := validIdentifierList(unit.Imports); err != nil {
		return err
	}
	if err := validIdentifierList(unit.TestImports); err != nil {
		return err
	}
	if err := validIdentifierList(unit.Execs); err != nil {
		return err
	}
	if err := validIdentifierList(unit.AdvisoryImports); err != nil {
		return err
	}
	if err := validIdentifierList(unit.RouteDispatchImports); err != nil {
		return err
	}
	if err := validIdentifierList(unit.Frontier); err != nil {
		return err
	}
	if err := validFrontierBears(unit); err != nil {
		return err
	}
	if err := validIdentifierList(unit.PathGlobs); err != nil {
		return err
	}
	return validIdentifierList(unit.PathTokens)
}

// validFrontierBears accepts the empty, DEPENDENTS and OWN_FILES values.
func validFrontierBears(unit Unit) error {
	switch unit.FrontierBears {
	case "", FrontierBearsOnDependents, FrontierBearsOnOwnFiles:
	default:
		return ErrInvalidUnit
	}
	return nil
}

// validReadScope accepts a declared scope of canonical paths, each optionally
// naming a subtree with one trailing "/", in strictly ascending order.
func validReadScope(scoped bool, scope []string) error {
	if (!scoped && len(scope) != 0) || len(scope) > MaxPathsPerUnit {
		return ErrInvalidUnit
	}
	previous := ""
	for _, entry := range scope {
		if !ValidReadScopeEntry(entry) || entry <= previous {
			return ErrInvalidUnit
		}
		previous = entry
	}
	return nil
}

// ValidReadScopeEntry reports one declared read-scope entry (AFP-V0-023): a
// canonical repository-relative path, optionally with one trailing "/" naming
// the subtree below it, that is neither the root .git directory nor inside it,
// because a dirty set never names a path there.
func ValidReadScopeEntry(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	return ValidRelativePath(trimmed) && trimmed != ".git" && !strings.HasPrefix(trimmed, ".git/")
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
