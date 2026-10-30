package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/plansnapshot"
	"github.com/Beamfall/corvint/internal/projectprofile"
)

// affectedProfile names the full wire `--full` emits (AFP-V0-003), which is
// also the N-1 default; affectedCompactProfile names the compact default wire
// (AFP-V0-035).
const (
	affectedProfile        = "affected-plan/0"
	affectedCompactProfile = "affected-plan/1"
)

// affectedReceipt is the stdout document. The plan is the selector's own
// canonical plan; provider is the derived, non-authoritative input an operator
// may copy into a Go live-test provider bundle (AFP-V0-004).
type affectedReceipt struct {
	Snapshot map[string]any   `json:"snapshot,omitempty"`
	PathList []string         `json:"path_list,omitempty"`
	Advice   affectedAdvice   `json:"advice"`
	Mutates  bool             `json:"mutates"`
	OK       bool             `json:"ok"`
	Plan     affected.Plan    `json:"plan"`
	Profile  string           `json:"profile"`
	Provider affectedProvider `json:"provider"`
	Range    affectedRange    `json:"range"`
	Revision string           `json:"revision"`
	Tool     string           `json:"tool"`
	Warning  string           `json:"warning,omitempty"`
}

// affectedRange records the committed half of the dirty set (AFP-V0-010).
// Base is the full commit id `--base` named, or "" for the worktree-only
// form; Paths is the sorted tree diff base..HEAD that was unioned into
// plan.dirty, so a reader can tell a committed path from a worktree one.
type affectedRange struct {
	Base  string   `json:"base"`
	Paths []string `json:"paths"`
}

// affectedInvocation is one parsed `affected` command line. Providers,
// Checkouts, and SelectionProfile are set only by the ETS-V0 flags.
type affectedInvocation struct {
	Snapshot  string
	PathList  []string // `--path` set planned at a clean HEAD (AFP-V0-094)
	Root      string
	Base      string
	Providers []string
	Checkouts []extevidence.Checkout
	// RepositoryBases maps a bound --repository ID to its --repository-base commit; e2e-safe only (AFU-V1-067).
	RepositoryBases     map[string]string
	SelectionProfile    string
	PlaywrightConfig    string
	PlaywrightDiscovery string
	// PlaywrightEnv holds each --playwright-env value, NAME=VALUE or NAME, in order (AFP-V0-382).
	PlaywrightEnv []string
	// PlaywrightList is set only by `affected discovery`: the Playwright JSON listing converted
	// into a playwright-discovery/0 receipt (TJAA-V0-019).
	PlaywrightList string
	// PlaywrightUngreppedList is the optional `affected discovery` listing taken with every
	// project grep and grepInvert removed, which verifies grep omissions (TJAA-V0-118).
	PlaywrightUngreppedList string
	// PlaywrightUngreppedConfig is the optional untracked wrapper config the ungrepped listing was
	// taken with instead of the bound config (TJAA-V0-124, TJAA-V0-125).
	PlaywrightUngreppedConfig string
	// AppmapImpacts are application-map-impact/0 report files routed into advice (ANL-V0-031).
	AppmapImpacts []string
	// Full selects the affected-plan/0 wire: every exclusion and every
	// selected test file (AFP-V0-035).
	Full bool
	// sharedVia selects with every witness chain held by reference, for a
	// receipt that is only ever compacted (AFP-V0-641); runAffected sets it
	// for the default wire. Every other caller gets each chain in Via.
	sharedVia bool
	// RequireClean refuses to plan over a worktree with any status entry
	// (AFP-V0-045).
	RequireClean bool
	// Cache is the graph cache directory, caller-named or the AFP-V0-436
	// default, resolved outside the repository root; empty means no cache is
	// read or written (AFP-V0-390).
	Cache string
	// NoCache turns off the default graph cache (AFP-V0-437).
	NoCache bool
	// Format is the --format value: "" or json writes the JSON document,
	// text the affected-plan/1 projection as lines (AFP-V0-928).
	Format string
	// CacheStatus is set only by `affected cache`, which reports the default
	// cache's location and size; CacheStatusFormat is json or doctor-plugin
	// (AFP-V0-439).
	CacheStatus       bool
	CacheStatusFormat string
	// CacheExcluded names the repository's Git and common directories, which
	// the opened cache directory is rechecked against (AFP-V0-390).
	CacheExcluded []string
	// PathsFrom names a file of --path values, one per line (AFP-V0-395).
	PathsFrom string
	// AuditChanges is set only by `affected audit`: the change manifest replayed
	// against observed outcomes (AFP-V0-445).
	AuditChanges string
}

type playwrightAffectedReceipt struct {
	Snapshot map[string]any            `json:"snapshot,omitempty"`
	Mutates  bool                      `json:"mutates"`
	OK       bool                      `json:"ok"`
	Plan     typescript.PlaywrightPlan `json:"plan"`
	Profile  string                    `json:"profile"`
	Range    affectedRange             `json:"range"`
	Revision string                    `json:"revision"`
	Tool     string                    `json:"tool"`
	Warning  string                    `json:"warning,omitempty"`
}

type affectedProvider struct {
	Go affectedGoProvider `json:"go"`
}

// affectedGoProvider is the go-live-plan/0 projection. Packages is populated
// only when State is RUNNABLE; every other state names why an operator must
// not paste the list into a bundle.
type affectedGoProvider struct {
	Packages []string `json:"packages"`
	State    string   `json:"state"`
	// noGoUnits marks a graph with no go: unit, whose empty selection is not
	// an advice gap (V1-2076). It is not on the wire.
	noGoUnits bool
	// compile is the sorted import paths of the reached Go units that declare
	// no test, which only a compile check covers (AFP-V0-468); compileOver
	// counts them instead when they exceed the pattern bound. Neither is on
	// the wire.
	compile     []string
	compileOver int
}

// affectedAdvice is the grounded check list (AFP-V0-009). Checks is ordered
// mandatory-first in source order, then the advisory Go test command, the
// compile-only check of reached untested packages (AFP-V0-468) and one compile
// check per changed Go module (AFP-V0-469); Unknown is
// the sorted frontier an operator must read before trusting the advisory half.
// Nothing here is derived from plan.excluded: an exclusion is never advice
// (AFP-V0-004). TestSelection is present only when a provider record was
// named (ETS-V0-002); it never adds to or removes from Checks.
type affectedAdvice struct {
	Checks        []affectedCheck `json:"checks"`
	Note          string          `json:"note"`
	Status        string          `json:"status"`
	TestSelection map[string]any  `json:"test_selection,omitempty"`
	// AppNavigation is present only when --appmap-impact was named (ANL-V0-031).
	AppNavigation []affectedAppNavigation `json:"app_navigation,omitempty"`
	Unknown       []string                `json:"unknown"`
}

type affectedCheck struct {
	Command string `json:"command"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
	Source  string `json:"source"`
}

const (
	adviceStatus             = "PLAN_ONLY"
	adviceKindMandatory      = "mandatory"
	adviceKindAdvisory       = "advisory"
	adviceSourcePlan         = "affected-plan"
	adviceAdvisoryGoTest     = "GOTOOLCHAIN=local go test -count=1"
	adviceCompileGoTest      = adviceAdvisoryGoTest + " -run '^$'"
	adviceModuleGoVet        = "GOTOOLCHAIN=local go vet ./..."
	adviceMakefileName       = "Makefile"
	adviceAgentsName         = "AGENTS.md"
	adviceMaxSourceBytes     = 256 << 10
	adviceMaxMandatoryChecks = 16
	adviceNote               = "advice is static; no check was executed; mandatory checks remain required whatever the advisory list says"
	adviceFocusedCommand     = "corvint test --base %s --dry-run"
	adviceNoGateUnknown      = "NO_REPOSITORY_GATE_DECLARED: no Makefile gate target or AGENTS.md Verify block"
	adviceUnreadableUnknown  = "MANDATORY_DECLARATION_UNREADABLE: %s exists but is not a readable regular file"
	adviceMakefileReason     = "the repository Makefile declares a gate target, so the full gate stays mandatory at the project terminal boundary, not as the default next step"
	adviceAgentsReason       = "the repository AGENTS.md Verify block declares this command, so it stays mandatory at the project terminal boundary, not as the default next step"
	adviceLaunchReason       = "the repository AGENTS.md Verify block declares this command, but it launches a program or runs in the background and does not end on its own, so it is not a check"
	adviceAgentsLineReason   = "the repository AGENTS.md Verify line declares this command, so it stays mandatory at the project terminal boundary, not as the default next step"
)

const (
	providerStateRunnable      = "RUNNABLE"
	providerStateEmpty         = "EMPTY_SELECTION"
	providerStateModuleUnknown = "MODULE_PATH_UNRESOLVED"
	providerStateBoundExceeded = "PACKAGE_BOUND_EXCEEDED"
	providerMaxPackagePatterns = 4091
)

// The advisory RSpec check (AFP-V0-066).
const (
	adviceAdvisoryRSpec     = "bundle exec rspec"
	adviceRSpecUnitPrefix   = "ruby:rspec:"
	adviceMaxRSpecFiles     = 1024
	adviceNoRSpecGemfile    = "NO_ADVISORY_RSPEC_COMMAND: the plan selected RSpec units but the repository root has no Gemfile"
	adviceRSpecBoundUnknown = "NO_ADVISORY_RSPEC_COMMAND: %d selected spec files exceed the %d-file bound"
)

func parseAffectedInvocation(arguments []string) (affectedInvocation, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return affectedInvocation{}, false, nil
	}
	root := ""
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		value := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return affectedInvocation{}, false, nil
			}
			value, index = arguments[index+1], index+2
		} else {
			value, index = strings.TrimPrefix(arguments[index], "--root="), index+1
		}
		root = value
	}
	if index >= len(arguments) || arguments[index] != "affected" {
		return affectedInvocation{}, false, nil
	}
	rest, trailingRoot, err := extractAffectedTrailingRoot(arguments[index+1:])
	if err != nil {
		return affectedInvocation{}, true, err
	}
	if trailingRoot != "" {
		if root != "" {
			return affectedInvocation{}, true, argumentError("--root may be given once, before or after affected")
		}
		root = trailingRoot
	}
	invocation, err := parseAffectedOptions(rest)
	if err != nil {
		return affectedInvocation{}, true, err
	}
	if root == "" {
		root = "."
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		return affectedInvocation{}, true, err
	}
	invocation.Root = resolved
	if err := resolveAffectedAbbrevBase(&invocation); err != nil {
		return affectedInvocation{}, true, err
	}
	if err := resolveAffectedBatchAndCache(&invocation); err != nil {
		return affectedInvocation{}, true, err
	}
	if err := expandAffectedPathDirectories(&invocation); err != nil {
		return affectedInvocation{}, true, err
	}
	return invocation, true, nil
}

// affectedOptionNames are the flags `affected` accepts, each taking one value
// as `--flag VALUE` or `--flag=VALUE`.
var affectedOptionNames = map[string]bool{"--path": true, "--snapshot": true, "--base": true, "--appmap-impact": true, "--playwright-config": true, "--playwright-discovery": true, "--playwright-env": true, "--provider": true, "--provider-command": true, "--repository": true, "--repository-base": true, "--selection-profile": true, "--cache": true, "--paths-from": true}

// affectedFormatValues are the --format values `affected` accepts
// (AFP-V0-928, amending AFP-V0-201).
var affectedFormatValues = []string{"json", "text"}

// affectedFormatRefusal names why a --format value is refused, the values
// accepted, and every option `affected` accepts (AFP-V0-201, AFP-V0-928).
func affectedFormatRefusal(value string) error {
	return affectedFormatRefusalFor("got " + pythonRepr(value))
}

// affectedFormatRefusalFor is affectedFormatRefusal with the fault stated by
// the caller, such as a second --format (AFP-V0-928).
func affectedFormatRefusalFor(fault string) error {
	names := []string{"--format", "--full", "--no-cache", "--require-clean"}
	for name := range affectedOptionNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return argumentError("affected --format accepts " + strings.Join(affectedFormatValues, " or ") + ", " + fault + ": json writes one JSON document (affected-plan/1, or affected-plan/0 with --full), text the affected-plan/1 plan as lines; accepted options: " + strings.Join(names, ", "))
}

// parseAffectedOptions reads the flags after `affected`. `--base` must
// already be a full object id: a ref name is resolved by the caller, never
// here, so range.base is exactly what the operator asked for. `--provider`,
// `--repository`, and `--selection-profile` follow the impact bounds and
// errors (ETS-V0-001).
func parseAffectedOptions(rest []string) (affectedInvocation, error) {
	if len(rest) != 0 && rest[0] == "discovery" {
		return parseAffectedDiscoveryOptions(rest[1:])
	}
	if len(rest) != 0 && rest[0] == "cache" {
		return parseAffectedCacheStatusOptions(rest[1:])
	}
	if len(rest) != 0 && rest[0] == "audit" {
		return parseAffectedAuditOptions(rest[1:])
	}
	invocation := affectedInvocation{}
	baseSet := false
	for index := 0; index < len(rest); {
		if rest[index] == "--full" {
			if invocation.Full {
				return affectedInvocation{}, argumentError("--full may be given once")
			}
			invocation.Full = true
			index++
			continue
		}
		if rest[index] == "--no-cache" || strings.HasPrefix(rest[index], "--no-cache=") {
			if invocation.NoCache || rest[index] != "--no-cache" {
				return affectedInvocation{}, argumentError("argument --no-cache: does not accept a value or repetition")
			}
			invocation.NoCache = true
			index++
			continue
		}
		if rest[index] == "--require-clean" || strings.HasPrefix(rest[index], "--require-clean=") {
			if invocation.RequireClean || rest[index] != "--require-clean" {
				return affectedInvocation{}, argumentError("argument --require-clean: does not accept a value or repetition")
			}
			invocation.RequireClean = true
			index++
			continue
		}
		name, value, inline := strings.Cut(rest[index], "=")
		if name == "--format" {
			if !inline {
				if index+1 >= len(rest) {
					return affectedInvocation{}, affectedFormatRefusal("")
				}
				value = rest[index+1]
				index++
			}
			index++
			if invocation.Format != "" {
				return affectedInvocation{}, affectedFormatRefusalFor("given more than once (it may be given once)")
			}
			if !slices.Contains(affectedFormatValues, value) {
				return affectedInvocation{}, affectedFormatRefusal(value)
			}
			invocation.Format = value
			continue
		}
		if !affectedOptionNames[name] {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError(name + " requires exactly one value")
			}
			value = rest[index+1]
			if (name == "--path" || name == "--paths-from") && strings.HasPrefix(value, "-") {
				return affectedInvocation{}, argumentError(name + " requires a path value, got option " + pythonRepr(value) + "; write " + name + "=VALUE for a name that begins with -")
			}
			index++
		}
		index++
		var err error
		switch name {
		case "--path":
			err = addAffectedPath(&invocation, value)
			// One --path takes every following value up to the next option
			// (AFP-V0-509); a value that begins with - is an option, as above.
			for err == nil && index < len(rest) && !strings.HasPrefix(rest[index], "-") {
				err = addAffectedPath(&invocation, rest[index])
				index++
			}
		case "--paths-from":
			err = setAffectedOnce(&invocation.PathsFrom, name, value)
		case "--cache":
			err = setAffectedOnce(&invocation.Cache, name, value)
		case "--snapshot":
			if invocation.Snapshot != "" || value == "" {
				err = argumentError("--snapshot requires exactly one nonempty value")
			} else {
				invocation.Snapshot = value
			}
		case "--base":
			err = setAffectedBase(&invocation, &baseSet, value)
		case "--appmap-impact":
			err = addAffectedAppmapImpact(&invocation, value)
		case "--provider", "--provider-command":
			err = addAffectedProvider(&invocation, name, value)
		case "--repository":
			err = addAffectedCheckout(&invocation, value)
		case "--repository-base":
			err = addAffectedRepositoryBase(&invocation, value)
		case "--playwright-config":
			err = setAffectedPlaywrightConfig(&invocation, value)
		case "--playwright-env":
			err = addAffectedPlaywrightEnv(&invocation, value)
		case "--playwright-discovery":
			if invocation.PlaywrightDiscovery != "" || value == "" {
				err = argumentError("--playwright-discovery requires exactly one nonempty value")
			} else {
				invocation.PlaywrightDiscovery = value
			}
		default:
			err = setAffectedSelectionProfile(&invocation, value)
		}
		if err != nil {
			return affectedInvocation{}, err
		}
	}
	if invocation.Format == "text" && (invocation.Full || invocation.PlaywrightConfig != "") {
		return affectedInvocation{}, argumentError("--format text writes the affected-plan/1 plan and cannot be combined with --full or --playwright-config")
	}
	if invocation.Cache != "" && invocation.NoCache {
		return affectedInvocation{}, argumentError("--cache and --no-cache cannot be combined")
	}
	if invocation.Cache != "" && (invocation.Snapshot != "" || invocation.PlaywrightConfig != "") {
		return affectedInvocation{}, argumentError("--cache cannot be combined with --snapshot or --playwright-config")
	}
	if (len(invocation.PathList) != 0 || invocation.PathsFrom != "") && (invocation.Snapshot != "" || invocation.Base != "" || len(invocation.Providers) != 0 || invocation.PlaywrightConfig != "" || invocation.PlaywrightDiscovery != "" || len(invocation.AppmapImpacts) != 0) {
		return affectedInvocation{}, argumentError("--path and --paths-from cannot be combined with --snapshot, --base, --provider, --playwright-config, --playwright-discovery or --appmap-impact")
	}
	if invocation.Snapshot != "" && (invocation.Base != "" || len(invocation.Providers) != 0 || invocation.PlaywrightDiscovery != "") {
		return affectedInvocation{}, argumentError("--snapshot cannot be combined with --base, --provider or --playwright-discovery")
	}
	if invocation.Snapshot != "" && invocation.RequireClean {
		return affectedInvocation{}, argumentError("--require-clean cannot be combined with --snapshot, which never reads the worktree")
	}
	if len(invocation.AppmapImpacts) != 0 && (invocation.Base == "" || invocation.Snapshot != "" || invocation.PlaywrightConfig != "") {
		return affectedInvocation{}, argumentError("--appmap-impact requires --base and cannot be combined with --snapshot or --playwright-config")
	}
	// --appmap-impact binds a report to a declared --repository checkout without a selection
	// provider (ANL-V0-060); otherwise a checkout only feeds a provider.
	if len(invocation.Providers) == 0 && (invocation.SelectionProfile != "" || (len(invocation.Checkouts) != 0 && len(invocation.AppmapImpacts) == 0)) {
		return affectedInvocation{}, argumentError("--repository and --selection-profile require --provider")
	}
	if invocation.Full && invocation.PlaywrightConfig != "" {
		return affectedInvocation{}, argumentError("--full cannot be combined with --playwright-config")
	}
	if invocation.PlaywrightConfig != "" && len(invocation.Providers) != 0 {
		return affectedInvocation{}, argumentError("--playwright-config cannot be combined with --provider")
	}
	if len(invocation.PlaywrightEnv) != 0 && invocation.PlaywrightConfig == "" {
		return affectedInvocation{}, argumentError("--playwright-env requires --playwright-config")
	}
	if len(invocation.PlaywrightEnv) != 0 && invocation.Snapshot != "" {
		// A snapshot plan reads only the snapshot, never the invoking environment.
		return affectedInvocation{}, argumentError("--playwright-env cannot be combined with --snapshot")
	}
	if invocation.SelectionProfile == appflows.E2ESafeProfile {
		return invocation, checkAffectedE2ESafe(invocation)
	}
	if len(invocation.RepositoryBases) != 0 && len(invocation.AppmapImpacts) == 0 {
		return affectedInvocation{}, argumentError("--repository-base requires --selection-profile e2e-safe")
	}
	if len(invocation.AppmapImpacts) != 0 && (len(invocation.RepositoryBases) != 0 || len(invocation.Providers) == 0) {
		if err := checkAffectedRepositoryBases(invocation); err != nil {
			return affectedInvocation{}, err
		}
	}
	if invocation.PlaywrightDiscovery != "" && invocation.PlaywrightConfig == "" {
		return affectedInvocation{}, argumentError("--playwright-discovery requires --playwright-config")
	}
	if len(invocation.Providers) != 0 && invocation.SelectionProfile == "" {
		invocation.SelectionProfile = extevidence.ProfileStrict
	}
	return invocation, nil
}

// parseAffectedDiscoveryOptions reads `affected discovery --playwright-config PATH
// --playwright-list FILE [--playwright-ungrepped-list FILE [--playwright-ungrepped-config PATH]]`,
// each at most once (TJAA-V0-019, TJAA-V0-118, TJAA-V0-125).
func parseAffectedDiscoveryOptions(rest []string) (affectedInvocation, error) {
	invocation := affectedInvocation{}
	for index := 0; index < len(rest); index++ {
		name, value, inline := strings.Cut(rest[index], "=")
		if name != "--playwright-config" && name != "--playwright-list" && name != "--playwright-ungrepped-list" && name != "--playwright-ungrepped-config" {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError(name + " requires exactly one value")
			}
			index++
			value = rest[index]
		}
		if name == "--playwright-config" {
			if err := setAffectedPlaywrightConfig(&invocation, value); err != nil {
				return affectedInvocation{}, err
			}
		} else if name == "--playwright-ungrepped-list" {
			if invocation.PlaywrightUngreppedList != "" || value == "" {
				return affectedInvocation{}, argumentError("--playwright-ungrepped-list requires exactly one nonempty value")
			}
			invocation.PlaywrightUngreppedList = value
		} else if name == "--playwright-ungrepped-config" {
			if invocation.PlaywrightUngreppedConfig != "" || !affected.ValidRelativePath(value) {
				return affectedInvocation{}, argumentError("--playwright-ungrepped-config requires exactly one repository-relative canonical path")
			}
			invocation.PlaywrightUngreppedConfig = value
		} else if invocation.PlaywrightList != "" || value == "" {
			return affectedInvocation{}, argumentError("--playwright-list requires exactly one nonempty value")
		} else {
			invocation.PlaywrightList = value
		}
	}
	if invocation.PlaywrightConfig == "" || invocation.PlaywrightList == "" {
		return affectedInvocation{}, argumentError("affected discovery requires --playwright-config and --playwright-list")
	}
	if invocation.PlaywrightUngreppedConfig != "" && invocation.PlaywrightUngreppedList == "" {
		return affectedInvocation{}, argumentError("--playwright-ungrepped-config requires --playwright-ungrepped-list")
	}
	return invocation, nil
}

// checkAffectedE2ESafe admits the e2e-safe profile only over one repository-relative selection
// provider file, and a discovery record, when named, only as a repository-relative file (AFU-V1-024).
// Each --repository needs --base and exactly one --repository-base for its ID (AFU-V1-067).
func checkAffectedE2ESafe(invocation affectedInvocation) error {
	if len(invocation.Providers) != 1 || !affected.ValidRelativePath(invocation.Providers[0]) {
		return argumentError("--selection-profile e2e-safe requires exactly one repository-relative --provider file")
	}
	if invocation.PlaywrightDiscovery != "" && !affected.ValidRelativePath(invocation.PlaywrightDiscovery) {
		return argumentError("--playwright-discovery must be a repository-relative canonical path under --selection-profile e2e-safe")
	}
	if len(invocation.Checkouts) != 0 && invocation.Base == "" {
		return argumentError("--repository under --selection-profile e2e-safe requires --base")
	}
	return checkAffectedRepositoryBases(invocation)
}

// checkAffectedRepositoryBases pairs each --repository with exactly one --repository-base
// (AFU-V1-067, ANL-V0-060).
func checkAffectedRepositoryBases(invocation affectedInvocation) error {
	for _, checkout := range invocation.Checkouts {
		if invocation.RepositoryBases[checkout.ID] == "" {
			return argumentError("argument --repository: repository id " + pythonRepr(checkout.ID) + " needs one --repository-base")
		}
	}
	if len(invocation.RepositoryBases) != len(invocation.Checkouts) {
		return argumentError("argument --repository-base: every id must be bound by --repository")
	}
	return nil
}

// addAffectedRepositoryBase admits one `--repository-base ID=FULL_COMMIT` (AFU-V1-067).
func addAffectedRepositoryBase(invocation *affectedInvocation, value string) error {
	id, commit, found := strings.Cut(value, "=")
	if !found || !extevidence.ValidRepositoryID(id) || !validGitObjectID(commit) {
		return argumentError("argument --repository-base: expected ID=FULL_COMMIT, got " + value)
	}
	if _, seen := invocation.RepositoryBases[id]; seen {
		return argumentError("argument --repository-base: repository id " + pythonRepr(id) + " is given twice")
	}
	if invocation.RepositoryBases == nil {
		invocation.RepositoryBases = map[string]string{}
	}
	invocation.RepositoryBases[id] = commit
	return nil
}

// affectedMaxPathList bounds the `--path` set, as the planning snapshot bounds
// its changed paths (AFP-V0-094).
const affectedMaxPathList = 4096

// addAffectedPath admits one canonical repository-relative `--path`; a
// repeated path collapses, so the plan's dirty set stays sorted and unique.
func addAffectedPath(invocation *affectedInvocation, value string) error {
	value = trimAffectedPathSlash(value)
	if !affected.ValidRelativePath(value) {
		return argumentError("--path must be a repository-relative canonical path, got " + pythonRepr(value))
	}
	invocation.PathList = affected.NormalizePaths(append(invocation.PathList, value))
	if len(invocation.PathList) > affectedMaxPathList {
		return argumentError(fmt.Sprintf("--path accepts at most %d paths", affectedMaxPathList))
	}
	return nil
}

// addAffectedPlaywrightEnv records one --playwright-env value: NAME=VALUE is a caller-supplied
// value and NAME reads that one variable from the invoking environment. A name is accepted once
// (AFP-V0-382).
func addAffectedPlaywrightEnv(invocation *affectedInvocation, value string) error {
	name, _, _ := strings.Cut(value, "=")
	if !typescript.ValidPlaywrightEnvironmentName(name) {
		return argumentError("--playwright-env requires NAME=VALUE or NAME, where NAME is letters, digits and underscores not starting with a digit")
	}
	for _, existing := range invocation.PlaywrightEnv {
		if previous, _, _ := strings.Cut(existing, "="); previous == name {
			return argumentError("--playwright-env names " + name + " more than once")
		}
	}
	invocation.PlaywrightEnv = append(invocation.PlaywrightEnv, value)
	return nil
}

// affectedPlaywrightEnvironment resolves each --playwright-env value. Only a variable the caller
// named is read from the invoking environment; no other variable is (AFP-V0-382).
func affectedPlaywrightEnvironment(values []string) []typescript.PlaywrightEnvironmentInput {
	inputs := make([]typescript.PlaywrightEnvironmentInput, 0, len(values))
	for _, value := range values {
		name, supplied, explicit := strings.Cut(value, "=")
		if explicit {
			inputs = append(inputs, typescript.NewPlaywrightEnvironmentInput(name, typescript.PlaywrightEnvironmentArgument, supplied, true))
			continue
		}
		observed, set := os.LookupEnv(name)
		inputs = append(inputs, typescript.NewPlaywrightEnvironmentInput(name, typescript.PlaywrightEnvironmentInvocation, observed, set))
	}
	return inputs
}

func setAffectedPlaywrightConfig(invocation *affectedInvocation, value string) error {
	if invocation.PlaywrightConfig != "" {
		return argumentError("--playwright-config requires exactly one value")
	}
	if !affected.ValidRelativePath(value) {
		return argumentError("--playwright-config must be a repository-relative canonical path")
	}
	extension := strings.ToLower(filepath.Ext(value))
	if extension != ".js" && extension != ".jsx" && extension != ".mjs" && extension != ".cjs" && extension != ".ts" && extension != ".tsx" {
		return argumentError("--playwright-config must name JavaScript or TypeScript source")
	}
	invocation.PlaywrightConfig = value
	return nil
}

func setAffectedBase(invocation *affectedInvocation, baseSet *bool, value string) error {
	if *baseSet {
		return argumentError("--base requires exactly one full commit id")
	}
	if !validGitObjectID(value) && !validAbbreviatedObjectID(value) {
		return argumentError("--base must be a full commit id or an unambiguous lowercase-hex prefix of at least 7 digits, got " + value)
	}
	*baseSet, invocation.Base = true, value
	return nil
}

// addAffectedProvider admits one `--provider FILE` or `--provider-command
// ARGV_JSON` under the bound and argv checks impact applies (EEP-TR-001).
func addAffectedProvider(invocation *affectedInvocation, name, value string) error {
	source, err := providerSource(name, value, len(invocation.Providers))
	if err != nil {
		return err
	}
	invocation.Providers = append(invocation.Providers, source)
	return nil
}

func addAffectedCheckout(invocation *affectedInvocation, value string) error {
	checkout, err := extevidence.ParseCheckout(value)
	if err != nil {
		return argumentError("argument --repository: " + err.Error())
	}
	if len(invocation.Checkouts) == extevidence.MaxCheckouts {
		return argumentError(fmt.Sprintf("argument --repository: at most %d checkouts", extevidence.MaxCheckouts))
	}
	for _, earlier := range invocation.Checkouts {
		if earlier.ID == checkout.ID {
			return argumentError("argument --repository: repository id " + pythonRepr(checkout.ID) + " is bound twice")
		}
	}
	invocation.Checkouts = append(invocation.Checkouts, checkout)
	return nil
}

func setAffectedSelectionProfile(invocation *affectedInvocation, value string) error {
	if invocation.SelectionProfile != "" {
		return argumentError("--selection-profile requires exactly one value")
	}
	if !extevidence.ValidSelectionProfile(value) && value != appflows.E2ESafeProfile {
		return argumentError("--selection-profile must be strict, coverage or e2e-safe, got " + value)
	}
	invocation.SelectionProfile = value
	return nil
}

func runAffected(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	if invocation.PlaywrightList != "" {
		return runAffectedDiscovery(ctx, invocation, stdout, stderr)
	}
	if invocation.CacheStatus {
		return runAffectedCacheStatus(invocation, stdout, stderr)
	}
	if invocation.AuditChanges != "" {
		return runAffectedAudit(ctx, invocation, stdout, stderr)
	}
	invocation.sharedVia = !invocation.Full
	var receipt any
	var err error
	if invocation.Snapshot != "" {
		receipt, err = compileSnapshotAffected(ctx, invocation)
	} else if invocation.PlaywrightConfig != "" {
		receipt, err = compilePlaywrightAffected(ctx, invocation)
	} else {
		receipt, err = compileAffected(ctx, invocation)
	}
	if err == nil && !invocation.Full {
		receipt, err = compactAffectedReceipt(receipt)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if invocation.Format == "text" {
		compact, ok := receipt.(affectedCompactReceipt)
		if !ok {
			emitError(stderr, argumentError("--format text has no line form for this receipt"))
			return 2
		}
		if _, err := io.WriteString(stdout, renderAffectedText(compact)); err != nil {
			emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected plan"})
			return 2
		}
		return 0
	}
	// Streamed, so a large receipt is not held in a second full copy (AFP-V0-398).
	if err := gokernel.WriteCanonicalJSON(stdout, receipt); err != nil {
		if gokernel.IsCanonicalOutputFailure(err) {
			err = &gokernel.Error{Code: "output-failed", Message: "cannot write affected receipt"}
		}
		emitError(stderr, err)
		return 2
	}
	return 0
}

func compilePlaywrightAffected(ctx context.Context, invocation affectedInvocation) (playwrightAffectedReceipt, error) {
	root := invocation.Root
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return playwrightAffectedReceipt{}, affectedGitExecutableRefusal()
	}
	revision, err := affectedHeadRevision(ctx, gitExecutable, root)
	if err != nil {
		return playwrightAffectedReceipt{}, err
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if invocation.RequireClean && len(dirty) != 0 {
		return playwrightAffectedReceipt{}, affectedMixedWorktreeRefusal(revision, dirty)
	}
	committed, err := affectedRangePaths(ctx, gitExecutable, root, invocation.Base)
	if err != nil {
		return playwrightAffectedReceipt{}, err
	}
	allDirty := affected.NormalizePaths(append(append([]string{}, dirty...), committed...))
	ignored, release, err := holdPlaywrightIgnored(ctx, gitExecutable, root)
	if err != nil {
		return playwrightAffectedReceipt{}, err
	}
	defer release()
	discovery := readPlaywrightDiscovery(root, invocation.PlaywrightDiscovery)
	comparison := revision
	if invocation.Base != "" {
		if comparison, err = affectedRevision(ctx, gitExecutable, root, invocation.Base); err != nil {
			return playwrightAffectedReceipt{}, affectedBaseRefusal(invocation.Base)
		}
	}
	before := playwrightBaseContent(ctx, gitExecutable, root, comparison, allDirty)
	plan, err := typescript.SelectPlaywrightWithEnvironment(root, invocation.PlaywrightConfig, revision, allDirty, discovery, before, affectedPlaywrightEnvironment(invocation.PlaywrightEnv))
	if err != nil {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-playwright-affected", Message: err.Error()}
	}
	recheck, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if !equalStringSlices(dirty, recheck) {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "worktree changed while the plan was compiled"}
	}
	if err := playwrightIgnoredDrift(ctx, gitExecutable, root, ignored, "plan"); err != nil {
		return playwrightAffectedReceipt{}, err
	}
	if revisionAfter, err := affectedHeadRevision(ctx, gitExecutable, root); err != nil || revisionAfter != revision {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "HEAD changed while the plan was compiled"}
	}
	sourceDigest, err := typescript.ObservePlaywrightSources(root, invocation.PlaywrightConfig)
	if err != nil || sourceDigest != plan.SourceDigest {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "source changed while the plan was compiled"}
	}
	if !bytes.Equal(discovery, readPlaywrightDiscovery(root, invocation.PlaywrightDiscovery)) {
		return playwrightAffectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "discovery receipt changed while the plan was compiled"}
	}
	return playwrightAffectedReceipt{
		Mutates: false, OK: true, Plan: plan, Profile: typescript.PlaywrightProfile,
		Range: affectedRange{Base: invocation.Base, Paths: committed}, Revision: revision, Tool: "affected",
		Warning: affectedMixedWarning(revision, dirty, committed),
	}, nil
}

// runAffectedDiscovery writes the canonical playwright-discovery/0 receipt converted from a
// Playwright JSON listing, bound to HEAD, the config bytes and the source digest, and refuses
// HEAD or source drift while it ran (TJAA-V0-019). It reads only; it never runs Playwright.
func runAffectedDiscovery(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	raw, err := compilePlaywrightDiscovery(ctx, invocation)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(raw, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write discovery receipt"})
		return 2
	}
	return 0
}

func compilePlaywrightDiscovery(ctx context.Context, invocation affectedInvocation) ([]byte, error) {
	root := invocation.Root
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, affectedGitExecutableRefusal()
	}
	revision, err := affectedHeadRevision(ctx, gitExecutable, root)
	if err != nil {
		return nil, err
	}
	listing, err := readPlaywrightList(root, invocation.PlaywrightList, "--playwright-list")
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-playwright-discovery", Message: err.Error()}
	}
	var ungrepped []byte
	if invocation.PlaywrightUngreppedList != "" {
		if ungrepped, err = readPlaywrightList(root, invocation.PlaywrightUngreppedList, "--playwright-ungrepped-list"); err != nil {
			return nil, &gokernel.Error{Code: "unsupported-playwright-discovery", Message: err.Error()}
		}
	}
	wrapper := invocation.PlaywrightUngreppedConfig
	if wrapper != "" {
		if err := refuseTrackedPlaywrightWrapper(ctx, gitExecutable, root, revision, wrapper); err != nil {
			return nil, err
		}
	}
	ignored, release, err := holdPlaywrightIgnored(ctx, gitExecutable, root, wrapper)
	if err != nil {
		return nil, err
	}
	defer release()
	raw, err := typescript.PlaywrightDiscoveryFromWrappedLists(root, invocation.PlaywrightConfig, revision, listing, ungrepped, wrapper)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-playwright-discovery", Message: err.Error()}
	}
	if _, state := typescript.VerifyPlaywrightDiscovery(root, invocation.PlaywrightConfig, revision, raw); state != "MATCHED" {
		return nil, &gokernel.Error{Code: "unsupported-affected-drift", Message: "source changed while the discovery receipt was produced"}
	}
	playwrightDiscoverySourcesVerified()
	if err := playwrightIgnoredDrift(ctx, gitExecutable, root, ignored, "discovery receipt"); err != nil {
		return nil, err
	}
	// HEAD is read after the last source observation, so a commit that lands while sources are
	// read, even one with an unchanged tree, is refused rather than stamped.
	if revisionAfter, err := affectedHeadRevision(ctx, gitExecutable, root); err != nil || revisionAfter != revision {
		return nil, &gokernel.Error{Code: "unsupported-affected-drift", Message: "HEAD changed while the discovery receipt was produced"}
	}
	return raw, nil
}

// holdPlaywrightIgnored excludes the worktree's ignored paths that HEAD does
// not track from every source walk of root until release, so generated report,
// trace or coverage output is no graph input and raises no frontier
// (TJAA-V0-100), and the .corvint/generated-paths.json entries as well (AFP-V0-444). Each extra
// path, such as the untracked ungrepped wrapper config (TJAA-V0-125), is held too, so it enters
// no graph and no source digest the later plan recomputes after it is deleted. It returns the
// held Git and declared list the caller refuses a change of as drift, without the extras.
func holdPlaywrightIgnored(ctx context.Context, gitExecutable, root string, extra ...string) ([]string, func(), error) {
	held, err := playwrightHeldPaths(ctx, gitExecutable, root)
	if err != nil {
		return nil, nil, err
	}
	entries := slices.Clone(held)
	for _, relative := range extra {
		if relative != "" {
			entries = append(entries, relative)
		}
	}
	release, err := affected.HoldIgnoredExclusion(root, entries)
	if err != nil {
		return nil, nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	return held, release, nil
}

// refuseTrackedPlaywrightWrapper refuses an ungrepped wrapper config that revision tracks: the
// wrapper is an operator's transient file, and a tracked one would be repository source the
// discovery hold would hide from the graph (TJAA-V0-125). A Git failure refuses as well.
func refuseTrackedPlaywrightWrapper(ctx context.Context, gitExecutable, root, revision, wrapper string) error {
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	output, err := hermeticGitCommand(deadline, gitExecutable, root, "ls-tree", "-z", "--name-only", revision, "--", wrapper).Output()
	if refusal := gitExitOwnershipRefusal(err); refusal != nil {
		return refusal
	}
	if err != nil {
		return &gokernel.Error{Code: "unsupported-affected-status", Message: "cannot read whether HEAD tracks --playwright-ungrepped-config"}
	}
	if len(output) != 0 {
		return &gokernel.Error{Code: "unsupported-playwright-discovery", Message: "--playwright-ungrepped-config " + wrapper + " is tracked by HEAD; it must be an untracked file deleted after discovery"}
	}
	return nil
}

// playwrightHeldPaths is Git's ignored list followed by the declared generated
// or vendored entries (AFP-V0-444): the exclusion a Playwright walk holds.
func playwrightHeldPaths(ctx context.Context, gitExecutable, root string) ([]string, error) {
	ignored, err := affected.IgnoredPaths(ctx, gitExecutable, root)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	_, generated, err := affected.ReadGeneratedDeclaration(root)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	return append(slices.Clone(ignored), generated...), nil
}

// playwrightIgnoredDrift refuses a change of the held ignored and declared
// generated paths while the sources were observed under their exclusion
// (TJAA-V0-100, AFP-V0-444).
func playwrightIgnoredDrift(ctx context.Context, gitExecutable, root string, held []string, product string) error {
	after, err := playwrightHeldPaths(ctx, gitExecutable, root)
	if err != nil {
		return err
	}
	if !equalStringSlices(held, after) {
		return &gokernel.Error{Code: "unsupported-affected-drift", Message: "ignored or declared generated paths changed while the " + product + " was compiled"}
	}
	return nil
}

// affectedGraphContext is the context the plan path hands affected.Build: the
// caller's own, so its deadline and cancellation bound the walk (AFP-V0-151);
// tests wrap it to cancel mid-walk.
var affectedGraphContext = func(ctx context.Context) context.Context { return ctx }

// affectedPlanSourcesObserved runs after the plan path's last source
// observation and before its final status and HEAD reads; tests inject a
// commit there (AFP-V0-150).
var affectedPlanSourcesObserved = func() {}

// playwrightDiscoverySourcesVerified runs after the producer's last source observation; tests
// replace it to inject drift at that point.
var playwrightDiscoverySourcesVerified = func() {}

// readPlaywrightList reads one bounded regular listing file named by flag, relative to root
// unless absolute.
func readPlaywrightList(root, name, flag string) ([]byte, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New(flag + " must name a readable regular file")
	}
	if info.Size() > typescript.PlaywrightListMaxBytes {
		return nil, fmt.Errorf("%s is over the %d-byte bound", flag, typescript.PlaywrightListMaxBytes)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, errors.New(flag + " must name a readable regular file")
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, typescript.PlaywrightListMaxBytes+1))
}

func readPlaywrightDiscovery(root, name string) []byte {
	if name == "" {
		return nil
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	// An oversize regular file is read only up to one byte past the bound, so the profile reports
	// it as MALFORMED with its reason rather than as MISSING (TJAA-V0-020).
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	file, err := os.Open(name)
	if err != nil {
		return nil
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, typescript.PlaywrightDiscoveryMaxBytes+1))
	if err != nil {
		return nil
	}
	return body
}

// compileAffected is read-only: one bounded git status, one HEAD identity
// read, one source walk, no test execution, no ledger write (AFP-V0-001).
// With a base, one bounded tree diff base..HEAD joins the dirty set
// (AFP-V0-010); the base is immutable, so it is read once and needs no
// drift recheck.
func compileAffected(ctx context.Context, invocation affectedInvocation) (affectedReceipt, error) {
	receipt, _, err := compileAffectedGraph(ctx, invocation)
	return receipt, err
}

// compileAffectedGraph is compileAffected that also returns the graph the plan
// was selected over, so `corvint test` reads the Go universe from the same
// observation (STR-V0-001, STR-V0-007).
func compileAffectedGraph(ctx context.Context, invocation affectedInvocation) (affectedReceipt, *affected.Graph, error) {
	root := invocation.Root
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return affectedReceipt{}, nil, affectedGitExecutableRefusal()
	}
	revision, err := affectedHeadRevision(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, nil, err
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if invocation.RequireClean && len(dirty) != 0 {
		return affectedReceipt{}, nil, affectedMixedWorktreeRefusal(revision, dirty)
	}
	if len(invocation.PathList) != 0 && len(dirty) != 0 {
		return affectedReceipt{}, nil, affectedPathListMixedRefusal(revision, dirty)
	}
	committed, err := affectedRangePaths(ctx, gitExecutable, root, invocation.Base)
	if err != nil {
		return affectedReceipt{}, nil, err
	}
	graph, err := affectedBuildGraph(affectedGraphContext(ctx), invocation)
	if err != nil {
		return affectedReceipt{}, nil, &gokernel.Error{Code: "unsupported-affected-graph", Message: err.Error()}
	}
	changed := affected.NormalizePaths(append(append([]string{}, dirty...), committed...))
	if len(invocation.PathList) != 0 {
		changed = invocation.PathList
	}
	var ignored []string
	if len(invocation.PathList) != 0 {
		if graph, ignored, err = affectedPathListGraph(ctx, gitExecutable, root, revision, invocation, graph); err != nil {
			return affectedReceipt{}, nil, err
		}
	}
	graph, plan, provider, advice := planAffectedGraph(ctx, affectedPlanInputs{
		gitExecutable: gitExecutable, repository: root, tree: root, base: invocation.Base, revision: revision,
		additive: len(invocation.PathList) == 0, shared: invocation.sharedVia,
	}, graph, changed)
	if invocation.SelectionProfile == appflows.E2ESafeProfile {
		advice.TestSelection, err = affectedE2ESafeSelection(ctx, invocation, revision, graph, plan, advice)
		if err != nil {
			return affectedReceipt{}, nil, err
		}
	} else if len(invocation.Providers) != 0 {
		input := affectedSelectionInput(invocation, plan, dirty, advice)
		input.CheckoutStatus = func(ctx context.Context, dir string) ([]string, error) {
			return affected.DirtyPaths(ctx, gitExecutable, dir)
		}
		advice.TestSelection = extevidence.Selection(ctx, root, revision, invocation.Providers, invocation.Checkouts, input)
	}
	recheckCheckouts, err := affectedAppNavigations(ctx, gitExecutable, invocation, revision, dirty, &advice)
	if err != nil {
		return affectedReceipt{}, nil, err
	}
	affectedPlanSourcesObserved()
	// The status, the graph and every later source read (prior read scopes,
	// advice, provider and e2e selection, app navigations) are observations of
	// one mutable worktree. A dirty set or HEAD that changed while they were
	// read would bind a plan to inputs nobody observed together, so both are
	// read again after the last source observation; refuse rather than publish
	// (AFP-V0-150, V1-1087).
	recheck, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if !equalStringSlices(dirty, recheck) {
		return affectedReceipt{}, nil, &gokernel.Error{Code: "unsupported-affected-drift", Message: "worktree changed while the plan was compiled"}
	}
	if len(invocation.PathList) != 0 {
		if err := playwrightIgnoredDrift(ctx, gitExecutable, root, ignored, "plan"); err != nil {
			return affectedReceipt{}, nil, err
		}
	}
	if revisionAfter, err := affectedHeadRevision(ctx, gitExecutable, root); err != nil || revisionAfter != revision {
		return affectedReceipt{}, nil, &gokernel.Error{Code: "unsupported-affected-drift", Message: "HEAD changed while the plan was compiled"}
	}
	if err := recheckCheckouts(); err != nil {
		return affectedReceipt{}, nil, err
	}
	return affectedReceipt{
		PathList: invocation.PathList,
		Advice:   advice,
		Mutates:  false,
		OK:       true,
		Plan:     plan,
		Profile:  affectedProfile,
		Provider: affectedProvider{Go: provider},
		Range:    affectedRange{Base: invocation.Base, Paths: committed},
		Revision: revision,
		Tool:     "affected",
		Warning:  affectedMixedWarning(revision, dirty, committed),
	}, graph, nil
}

// affectedE2ESafeSelection runs the e2e-safe profile over the plan and hands its closed output to
// the receipt as the test_selection member (AFU-V1-019..024).
func affectedE2ESafeSelection(ctx context.Context, invocation affectedInvocation, revision string, graph *affected.Graph, plan affected.Plan, advice affectedAdvice) (map[string]any, error) {
	mandatory := affectedSelectionInput(invocation, plan, nil, advice).Mandatory
	repositories := []appflows.E2ERepositoryInput{}
	for _, checkout := range invocation.Checkouts {
		repositories = append(repositories, appflows.E2ERepositoryInput{ID: checkout.ID, Source: checkout.Source, Base: invocation.RepositoryBases[checkout.ID]})
	}
	selection := appflows.SelectE2E(ctx, invocation.Root, appflows.E2EInput{
		Provider: invocation.Providers[0], Discovery: invocation.PlaywrightDiscovery, Revision: revision,
		Base: invocation.Base, Graph: graph, Plan: plan, Mandatory: mandatory, Repositories: repositories,
	})
	encoded, err := json.Marshal(selection)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, json.Unmarshal(encoded, &out)
}

// affectedSelectionMaxItems bounds every test_selection list; the rest is
// counted in test_selection.omitted (ETS-V0-010).
const affectedSelectionMaxItems = 64

// affectedSelectionInput hands the plan to the selector: every changed path,
// the uncommitted subset no record can describe, why the scope is not
// bounded, and the mandatory checks it must echo unchanged (ETS-V0-009).
func affectedSelectionInput(invocation affectedInvocation, plan affected.Plan, dirty []string, advice affectedAdvice) extevidence.SelectionInput {
	incomplete := []string{}
	for _, entry := range plan.Unknown {
		incomplete = append(incomplete, entry.Reason+": "+entry.Detail)
	}
	if plan.Scope != affected.ScopeBounded && len(incomplete) == 0 {
		incomplete = append(incomplete, "plan scope is "+plan.Scope)
	}
	mandatory := []any{}
	for _, check := range advice.Checks {
		if check.Kind == adviceKindMandatory {
			mandatory = append(mandatory, map[string]any{"command": check.Command, "kind": check.Kind, "reason": check.Reason, "source": check.Source})
		}
	}
	return extevidence.SelectionInput{
		Changed: plan.Dirty, Worktree: affected.NormalizePaths(dirty), Incomplete: sortedUniqueStrings(incomplete),
		Mandatory: mandatory, Profile: invocation.SelectionProfile, Limit: affectedSelectionMaxItems,
	}
}

// affectedRangePaths resolves the requested base to a commit in this
// repository and captures the tree diff base..HEAD. An empty base is the
// worktree-only form and yields an empty, non-nil list.
// affectedPriorReadScopes reads, when the change touches the read-scope
// declaration, the declaration it is measured from: the base commit with
// --base, else HEAD, from immutable Git objects (AFP-V0-050). It is nil,
// unknown, otherwise and on any failure, which keeps an edit to the
// declaration bearing on every declared unit.
func affectedPriorReadScopes(ctx context.Context, gitExecutable, root, base, revision string, changed []string) map[string][]string {
	touched := false
	for _, path := range changed {
		touched = touched || path == affected.ReadScopesPath
	}
	if !touched {
		return nil
	}
	commit := revision
	if base != "" {
		resolved, err := affectedRevision(ctx, gitExecutable, root, base)
		if err != nil {
			return nil
		}
		commit = resolved
	}
	scopes, err := golang.CommittedReadScopes(ctx, root, commit)
	if err != nil {
		return nil
	}
	return scopes
}

// affectedPlanInputs are what planAffectedGraph measures a change from: the
// repository whose Git objects hold the base, the tree the graph was built
// over (the worktree, or a review's materialized snapshot), and whether the
// plan records ADN-V0-001 additive changes (not in --path mode, ADN-V0-009).
type affectedPlanInputs struct {
	gitExecutable, repository, tree, base, revision string
	additive, shared                                bool
}

// planAffectedGraph is the one planning tail of `corvint affected` and of a
// review's snapshot plan (AFP-V0-930, V1-3147): the prior read-scope
// declaration (AFP-V0-050), the additive-change records (ADN-V0-001), the
// selection, the Go provider projection and the advice. Both callers go
// through it, so a plan member added here reaches review.affected too.
func planAffectedGraph(ctx context.Context, in affectedPlanInputs, graph *affected.Graph, changed []string) (*affected.Graph, affected.Plan, affectedGoProvider, affectedAdvice) {
	graph = graph.WithReadScopeBase(affectedPriorReadScopes(ctx, in.gitExecutable, in.repository, in.base, in.revision, changed))
	if in.additive {
		graph = graph.WithGoAdditive(affectedGoAdditiveAt(ctx, in.gitExecutable, in.repository, in.tree, in.base, in.revision, graph, changed))
	}
	plan := affectedSelect(graph, changed, in.shared)
	provider := providerGoProjection(graph, plan)
	advice := withFocusedNextStep(compileAffectedAdvice(in.tree, graph, plan, provider), in.base, plan)
	return graph, plan, provider, advice
}

// affectedGoAdditive reads, for each changed Go package, whether the change
// only adds top-level declarations and which importers reference one, against
// the commit the change is measured from: the base commit with --base, else
// HEAD (ADN-V0-001). A base that cannot be resolved records every candidate
// as a fallback, which selects as before.
func affectedGoAdditive(ctx context.Context, gitExecutable, root, base, revision string, graph *affected.Graph, changed []string) []affected.GoAdditive {
	return affectedGoAdditiveAt(ctx, gitExecutable, root, root, base, revision, graph, changed)
}

// affectedGoAdditiveAt is affectedGoAdditive for a graph built over tree,
// which is root itself or a review's snapshot of HEAD materialized elsewhere;
// the commit is always read from root's Git objects.
func affectedGoAdditiveAt(ctx context.Context, gitExecutable, root, tree, base, revision string, graph *affected.Graph, changed []string) []affected.GoAdditive {
	commit := revision
	if base != "" {
		resolved, err := affectedRevision(ctx, gitExecutable, root, base)
		if err != nil {
			resolved = ""
		}
		commit = resolved
	}
	return golang.AdditiveChangesAt(ctx, root, tree, commit, graph, changed)
}

func affectedRangePaths(ctx context.Context, gitExecutable, root, base string) ([]string, error) {
	if base == "" {
		return []string{}, nil
	}
	if _, err := affectedRevision(ctx, gitExecutable, root, base); err != nil {
		return nil, affectedBaseRefusal(base)
	}
	paths, err := affected.RangePaths(ctx, gitExecutable, root, base)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	return paths, nil
}

// affectedPathListGraph returns the --path graph and the ignored paths HEAD
// does not track that it was checked against. When a TypeScript unit lists
// such a file, as a TypeScript walk lists a gitignored
// playwright-report/ bundle, and no other plugin's unit does, the graph is built
// again, uncached, under the TJAA-V0-100 exclusion, so the file is no graph input
// (AFP-V0-440, GitHub #1774). The rebuild is used only when nothing outside the
// TypeScript units changed. Every other case, and every
// affectedPathListIgnoredCheck refusal, still refuses (AFP-V0-094).
func affectedPathListGraph(ctx context.Context, gitExecutable, root, revision string, invocation affectedInvocation, graph *affected.Graph) (*affected.Graph, []string, error) {
	ignored, err := affected.IgnoredPaths(ctx, gitExecutable, root)
	if err != nil {
		return nil, nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if affectedGraphListsIgnored(graph, ignored) {
		release, err := affected.HoldIgnoredExclusion(root, ignored)
		if err != nil {
			return nil, nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
		}
		rebuilt, err := affected.Build(affectedGraphContext(ctx), root, affectedLanguages()...)
		release()
		if err != nil {
			return nil, nil, &gokernel.Error{Code: "unsupported-affected-graph", Message: err.Error()}
		}
		// The exclusion holds for every plugin's walk, but only the TypeScript
		// adapter turns an import of an excluded file into an unresolved import
		// (TJAA-V0-100). Any other change, such as an ignored .go file the Go
		// adapter could not parse, keeps the refusal over the first graph.
		same, err := affectedGraphsAgreeOutsideTypeScript(graph, rebuilt)
		if err != nil {
			return nil, nil, &gokernel.Error{Code: "unsupported-affected-graph", Message: err.Error()}
		}
		if same {
			graph = rebuilt
		}
	}
	return graph, ignored, affectedPathListIgnoredCheck(revision, graph, invocation.PathList, ignored)
}

// affectedGraphsAgreeOutsideTypeScript reports whether two graphs' canonical
// forms are equal once every TypeScript unit, frontier reason and unobserved
// entry is removed (AFP-V0-440).
func affectedGraphsAgreeOutsideTypeScript(before, after *affected.Graph) (bool, error) {
	project := func(graph *affected.Graph) ([]byte, error) {
		body, err := graph.Canonical()
		if err != nil {
			return nil, err
		}
		var form map[string]any
		if err := json.Unmarshal(body, &form); err != nil {
			return nil, err
		}
		for _, key := range []string{"units", "frontier", "unobserved"} {
			entries, _ := form[key].([]any)
			kept := []any{}
			for _, entry := range entries {
				name, _ := entry.(string)
				if unit, ok := entry.(map[string]any); ok {
					name, _ = unit["id"].(string)
				}
				if !strings.HasPrefix(name, typescript.Language{}.Name()+":") {
					kept = append(kept, entry)
				}
			}
			form[key] = kept
		}
		return json.Marshal(form)
	}
	first, err := project(before)
	if err != nil {
		return false, err
	}
	second, err := project(after)
	if err != nil {
		return false, err
	}
	return bytes.Equal(first, second), nil
}

// affectedGraphListsIgnored reports whether a TypeScript unit lists an ignored
// file and no unit of another plugin does (AFP-V0-440): only the TypeScript
// adapter resolves an import of an excluded file as unresolved (TJAA-V0-100).
func affectedGraphListsIgnored(graph *affected.Graph, ignored []string) bool {
	if len(ignored) == 0 {
		return false
	}
	entries := affected.NormalizeIgnored(ignored)
	listed := false
	for _, id := range graph.UnitIDs() {
		unit, ok := graph.Unit(id)
		if !ok {
			continue
		}
		for _, file := range append(append([]string{}, unit.Sources...), unit.Tests...) {
			if affected.IgnoredMatch(entries, file) {
				if !strings.HasPrefix(id, typescript.Language{}.Name()+":") {
					return false
				}
				listed = true
			}
		}
	}
	return listed
}

// affectedPathListIgnoredCheck refuses a --path plan whose disk graph reads, or
// whose list names, an ignored file HEAD does not track: such a file is not a
// dirty path, yet a graph built over it is not HEAD's (AFP-V0-094). A Go
// unit's graph also reads its testdata, its whole directory when it embeds,
// and its declared read scope, so an ignored file there refuses too (V1-2086).
func affectedPathListIgnoredCheck(revision string, graph *affected.Graph, listed, ignored []string) error {
	if len(ignored) == 0 {
		return nil
	}
	paths := append([]string{}, listed...)
	areas := []string{}
	for _, id := range graph.UnitIDs() {
		if unit, ok := graph.Unit(id); ok {
			paths = append(append(paths, unit.Sources...), unit.Tests...)
			areas = append(areas, ignoredCheckAreas(id, unit)...)
		}
	}
	for _, path := range affected.NormalizePaths(paths) {
		for _, entry := range ignored {
			if path == entry || (strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)) {
				return affectedPathListIgnoredRefusal(revision, path)
			}
		}
	}
	for _, entry := range ignored {
		for _, area := range areas {
			if ignoredEntryOverlaps(entry, area) {
				return affectedPathListIgnoredRefusal(revision, entry)
			}
		}
	}
	return nil
}

// ignoredCheckAreas lists the paths a Go unit reads beyond its own files: its
// directory's testdata, the directory's subtree when it embeds or declares a
// read scope (affected.InReadScope admits it even with no entries), and each
// declared read-scope entry ("/"-suffixed for a subtree).
func ignoredCheckAreas(id string, unit affected.Unit) []string {
	files := append(append([]string{}, unit.Sources...), unit.Tests...)
	if !strings.HasPrefix(id, "go:") || len(files) == 0 {
		return nil
	}
	directory := path.Dir(files[0])
	prefix := directory + "/"
	if directory == "." {
		prefix = ""
	}
	areas := []string{prefix + "testdata/"}
	if unit.Embeds || unit.ReadScoped {
		areas = append(areas, prefix)
	}
	return append(areas, unit.ReadScope...)
}

// ignoredEntryOverlaps reports whether an ignored entry ("/"-suffixed for a
// directory) is, lies under, or contains area ("/"-suffixed for a subtree, ""
// for the repository root).
func ignoredEntryOverlaps(entry, area string) bool {
	if area == "" || entry == area || entry == strings.TrimSuffix(area, "/") {
		return true
	}
	if strings.HasSuffix(area, "/") && strings.HasPrefix(entry, area) {
		return true
	}
	return strings.HasSuffix(entry, "/") && strings.HasPrefix(area, entry)
}

// compileAffectedAdvice joins the repository-declared mandatory gate, the
// advisory Go projection, the advisory RSpec and pytest commands, and the
// unknown frontier into one static list. It executes nothing and reads
// nothing derived from plan.excluded (AFP-V0-009). graph may be nil; the
// pytest command then adds no source root (AFP-V0-475).
func compileAffectedAdvice(root string, graph *affected.Graph, plan affected.Plan, provider affectedGoProvider) affectedAdvice {
	checks, mandatoryUnknown, truncated := mandatoryAffectedChecks(root)
	unknown := append([]string{}, mandatoryUnknown...)
	if !declaresMandatoryCheck(checks) && !truncated {
		unknown = append(unknown, adviceNoGateUnknown)
	}
	modules, moduleUnknown := advisoryGoModuleChecks(root, plan)
	advisory, advisoryUnknown := advisoryAffectedChecks(plan, provider, len(modules) > 0)
	checks = append(checks, advisory...)
	checks = append(checks, modules...)
	unknown = append(unknown, advisoryUnknown...)
	unknown = append(unknown, moduleUnknown...)
	rspec, rspecUnknown := advisoryRSpecCheck(root, plan)
	checks = append(checks, rspec...)
	unknown = append(unknown, rspecUnknown...)
	pytest, pytestUnknown := advisoryPytestCheck(root, graph, plan)
	checks = append(checks, pytest...)
	unknown = append(unknown, pytestUnknown...)
	for _, entry := range plan.Unknown {
		unknown = append(unknown, entry.Reason+": "+entry.Detail)
	}
	return affectedAdvice{Checks: checks, Note: adviceNote, Status: adviceStatus, Unknown: sortedUniqueStrings(unknown)}
}

// withFocusedNextStep puts the focused selected-test command first among the
// advisory checks when the plan has a base and dirty paths (AFP-V0-496): the
// default next step is the narrowed run, and the declared exhaustive commands
// stay the terminal-boundary checks a project policy may require.
func withFocusedNextStep(advice affectedAdvice, base string, plan affected.Plan) affectedAdvice {
	if base == "" || len(plan.Dirty) == 0 {
		return advice
	}
	focused := affectedCheck{Command: fmt.Sprintf(adviceFocusedCommand, shellQuoteJoin([]string{base})), Kind: adviceKindAdvisory, Reason: "prints the selected focused test argv for this plan and runs nothing; run the selected units, not the whole suite", Source: adviceSourcePlan}
	at := 0
	for at < len(advice.Checks) && advice.Checks[at].Kind != adviceKindAdvisory {
		at++
	}
	checks := append([]affectedCheck{}, advice.Checks[:at]...)
	checks = append(checks, focused)
	advice.Checks = append(checks, advice.Checks[at:]...)
	return advice
}

// mandatoryAffectedChecks reads only repository-owned declarations from the
// working tree at the root: a Makefile gate target and the AGENTS.md Verify
// block. Both reads are bounded, and the list is deduplicated in order of
// appearance and capped at adviceMaxMandatoryChecks. A declared command that
// does not end on its own is advisory and follows the mandatory ones. A read
// that hits the bound, or a declaration the cap drops, is reported in the
// returned unknown list rather than silently disappearing, as is a declaration
// path that exists but is not a readable regular file; truncated tells the
// caller a source was cut short or unread, so it never also claims no gate was
// declared.
func mandatoryAffectedChecks(root string) (checks []affectedCheck, unknown []string, truncated bool) {
	checks = []affectedCheck{}
	launches := []affectedCheck{}
	unknown = []string{}
	seen := map[string]bool{}
	cappedSource := ""
	admit := func(check affectedCheck) {
		if check.Command == "" || seen[check.Command] {
			return
		}
		if len(checks)+len(launches) >= adviceMaxMandatoryChecks {
			if cappedSource == "" {
				cappedSource = check.Source
			}
			return
		}
		seen[check.Command] = true
		if check.Kind == adviceKindAdvisory {
			launches = append(launches, check)
			return
		}
		checks = append(checks, check)
	}
	makefile, makefileTruncated, makefileUnreadable := readAdviceSource(filepath.Join(root, adviceMakefileName), adviceMaxSourceBytes)
	if makefileTruncated {
		truncated = true
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_TRUNCATED: %s exceeded %d bytes", adviceMakefileName, adviceMaxSourceBytes))
	}
	if makefileUnreadable {
		truncated = true
		unknown = append(unknown, fmt.Sprintf(adviceUnreadableUnknown, adviceMakefileName))
	}
	if makefileDeclaresGate(makefile) {
		admit(affectedCheck{Command: "make gate", Kind: adviceKindMandatory, Reason: adviceMakefileReason, Source: adviceMakefileName})
	}
	agents, agentsTruncated, agentsUnreadable := readAdviceSource(filepath.Join(root, adviceAgentsName), adviceMaxSourceBytes)
	if agentsTruncated {
		truncated = true
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_TRUNCATED: %s exceeded %d bytes", adviceAgentsName, adviceMaxSourceBytes))
	}
	if agentsUnreadable {
		truncated = true
		unknown = append(unknown, fmt.Sprintf(adviceUnreadableUnknown, adviceAgentsName))
	}
	commands, unrecognized := projectprofile.VerifyCommands(agents)
	for _, command := range commands {
		admit(agentsCheck(command))
	}
	scopeConditionalChecks(checks, projectprofile.VerifyScopes(agents))
	for _, heading := range unrecognized {
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_UNRECOGNIZED: %s heading %q is not %q, so its commands are not checks", adviceAgentsName, heading, projectprofile.VerifyHeading))
	}
	if cappedSource != "" {
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_CAPPED: %s declared more than %d commands", cappedSource, adviceMaxMandatoryChecks))
	}
	return append(checks, launches...), unknown, truncated
}

// scopeConditionalChecks relabels a mandatory check conditional when a
// scoping sentence of the AGENTS.md Verify section names its exact command in
// backticks, and puts that sentence in the reason (AFP-V0-009, decision 0531).
// A check no sentence names stays mandatory.
func scopeConditionalChecks(checks []affectedCheck, scopes []string) {
	for index, check := range checks {
		if check.Kind != adviceKindMandatory {
			continue
		}
		if sentence := projectprofile.ScopedBy(check.Command, scopes); sentence != "" {
			checks[index].Kind = adviceKindConditional
			checks[index].Reason = fmt.Sprintf(adviceConditionalReason, check.Source, sentence)
		}
	}
}

// A scoped declared command is conditional (AFP-V0-009, decision 0531).
const (
	adviceKindConditional   = "conditional"
	adviceConditionalReason = "the repository %s declares this command, but its AGENTS.md Verify section scopes it, so run it only when this condition holds: %s"
)

// agentsCheck is a declared AGENTS.md command as a check: advisory when it
// does not end on its own, mandatory otherwise. An inline **Verify:** line
// declares with the same authority as a Verify block (AFP-V0-196).
func agentsCheck(declared projectprofile.DeclaredCommand) affectedCheck {
	command := declared.Command
	if projectprofile.NonTerminatingCommand(command) {
		return affectedCheck{Command: command, Kind: adviceKindAdvisory, Reason: adviceLaunchReason, Source: adviceAgentsName}
	}
	reason := adviceAgentsReason
	if declared.Inline {
		reason = adviceAgentsLineReason
	}
	return affectedCheck{Command: command, Kind: adviceKindMandatory, Reason: reason, Source: adviceAgentsName}
}

// declaresMandatoryCheck reports whether any check is mandatory or
// conditional: a scoped gate is still a declared gate.
func declaresMandatoryCheck(checks []affectedCheck) bool {
	for _, check := range checks {
		if check.Kind == adviceKindMandatory || check.Kind == adviceKindConditional {
			return true
		}
	}
	return false
}

// advisoryAffectedChecks proposes the Go packages the plan selected, then a
// compile-only check of the reached Go packages that declare no test
// (AFP-V0-468). Any state other than RUNNABLE yields no test command and one
// unknown line naming the state, except an empty selection over a graph with
// no Go unit or over an empty dirty set (V1-2126), which yields neither, and
// an empty selection that a compile or module check still covers
// (moduleChecked), whose gap the NO_SELECTABLE_TEST plan unknown names.
func advisoryAffectedChecks(plan affected.Plan, provider affectedGoProvider, moduleChecked bool) ([]affectedCheck, []string) {
	if provider.State == providerStateEmpty && (provider.noGoUnits || len(plan.Dirty) == 0) {
		return nil, nil
	}
	var checks []affectedCheck
	var unknown []string
	if provider.State == providerStateRunnable {
		command := adviceAdvisoryGoTest + " " + shellQuoteJoin(provider.Packages)
		reason := fmt.Sprintf("the plan selected %d unit(s) over %d dirty path(s), so these packages are the plausible first pass", len(plan.Selected), len(plan.Dirty))
		checks = append(checks, affectedCheck{Command: command, Kind: adviceKindAdvisory, Reason: reason, Source: adviceSourcePlan})
	}
	if len(provider.compile) > 0 {
		reason := fmt.Sprintf("%d changed or reached Go package(s) declare no test, so no test command covers them; this compiles them and runs nothing", len(provider.compile))
		checks = append(checks, affectedCheck{Command: adviceCompileGoTest + " " + shellQuoteJoin(provider.compile), Kind: adviceKindAdvisory, Reason: reason, Source: adviceSourcePlan})
	}
	if provider.compileOver > 0 {
		unknown = append(unknown, fmt.Sprintf("NO_ADVISORY_GO_COMPILE_COMMAND: %d untested Go packages exceed the %d-pattern bound", provider.compileOver, providerMaxPackagePatterns))
	}
	covered := provider.State == providerStateEmpty && (len(provider.compile) > 0 || moduleChecked)
	if provider.State != providerStateRunnable && !covered {
		unknown = append(unknown, "NO_ADVISORY_GO_COMMAND: provider.go.state is "+provider.State)
	}
	return checks, unknown
}

// advisoryGoModuleChecks proposes one compile check per Go module whose go.mod
// or go.sum is dirty (AFP-V0-469): `go vet ./...` from the module directory
// type-checks every package and test file of that module against the changed
// requirements without running a test. Behaviour under the new requirements
// still needs the module's tests, which the plan leaves to the mandatory
// gate. A dirty go.mod or go.sum whose directory holds no readable go.mod
// declaring a module yields a named unknown instead. A directory the go tool
// ignores (testdata, vendor, `_`-prefixed) holds fixtures, not a module of
// this repository, and adds neither unless the governing go.work uses it. A
// module the governing go.work does not use is vetted with GOWORK=off, since
// the go tool refuses ./... there in workspace mode; an unreadable governing
// go.work is a named unknown.
func advisoryGoModuleChecks(root string, plan affected.Plan) ([]affectedCheck, []string) {
	directories := map[string][]string{}
	for _, dirty := range plan.Dirty {
		base := path.Base(dirty)
		if base != "go.mod" && base != "go.sum" {
			continue
		}
		directory := path.Dir(dirty)
		directories[directory] = append(directories[directory], dirty)
	}
	var checks []affectedCheck
	var unknown []string
	ordered := make([]string, 0, len(directories))
	for directory := range directories {
		ordered = append(ordered, directory)
	}
	sort.Strings(ordered)
	for _, directory := range ordered {
		paths := strings.Join(directories[directory], ", ")
		workspace, uses, workReason := golang.GoverningWorkspace(root, directory)
		member := workReason == "" && slices.Contains(uses, directory) && !goIgnoredDirectory(path.Dir(workspace))
		if goIgnoredDirectory(directory) && !member {
			continue
		}
		if workReason != "" {
			unknown = append(unknown, fmt.Sprintf("NO_ADVISORY_GO_MODULE_COMMAND: %s changed but its governing %s is %s", paths, workspace, workReason))
			continue
		}
		module, err := golang.ReadModulePath(filepath.Join(root, filepath.FromSlash(directory)))
		if err != nil || module == "" {
			unknown = append(unknown, fmt.Sprintf("NO_ADVISORY_GO_MODULE_COMMAND: %s changed but %s declares no readable module", paths, path.Join(directory, "go.mod")))
			continue
		}
		command := adviceModuleGoVet
		if workspace != "" && !member {
			command = "GOWORK=off " + command
		}
		if directory != "." {
			command = "(cd " + shellQuoteJoin([]string{directory}) + " && " + command + ")"
		}
		reason := fmt.Sprintf("%s changed the requirements of module %s, which no package unit owns, so this type-checks every package and test of the module and runs nothing; behaviour under the new requirements still needs the module's tests", paths, module)
		checks = append(checks, affectedCheck{Command: command, Kind: adviceKindAdvisory, Reason: reason, Source: adviceSourcePlan})
	}
	return checks, unknown
}

// goIgnoredDirectory reports a repository-relative directory the go tool's
// ./... patterns skip: one with a testdata, vendor or `_`- or `.`-prefixed
// component.
func goIgnoredDirectory(directory string) bool {
	for _, component := range strings.Split(directory, "/") {
		if component == "testdata" || component == "vendor" || strings.HasPrefix(component, "_") || (strings.HasPrefix(component, ".") && component != ".") {
			return true
		}
	}
	return false
}

// advisoryRSpecCheck proposes the RSpec spec files the plan selected, run
// through Bundler (AFP-V0-066). It adds nothing when the plan selected no RSpec
// unit; otherwise a missing root Gemfile or a selection past the file bound
// yields one unknown line instead of a command.
func advisoryRSpecCheck(root string, plan affected.Plan) ([]affectedCheck, []string) {
	units := 0
	seen := map[string]bool{}
	files := []string{}
	for _, selection := range plan.Selected {
		if !strings.HasPrefix(selection.UnitID, adviceRSpecUnitPrefix) {
			continue
		}
		units++
		for _, file := range selection.Tests {
			if !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	if info, err := os.Lstat(filepath.Join(root, "Gemfile")); err != nil || !info.Mode().IsRegular() {
		return nil, []string{adviceNoRSpecGemfile}
	}
	if !adviceDeclaresRSpecGem(root) {
		return nil, []string{adviceNoRSpecGem}
	}
	if len(files) > adviceMaxRSpecFiles {
		return nil, []string{fmt.Sprintf(adviceRSpecBoundUnknown, len(files), adviceMaxRSpecFiles)}
	}
	sort.Strings(files)
	// A leading `./` keeps a dash-leading spec file from being read as an
	// RSpec option; selected tests are repository-relative slash paths.
	operands := make([]string, len(files))
	for index, file := range files {
		operands[index] = "./" + file
	}
	command := adviceAdvisoryRSpec + " " + shellQuoteJoin(operands)
	reason := fmt.Sprintf("the plan selected %d RSpec unit(s) and the repository root has a Gemfile declaring an rspec gem, so these spec files are the plausible first pass", units)
	return []affectedCheck{{Command: command, Kind: adviceKindAdvisory, Reason: reason, Source: adviceSourcePlan}}, nil
}

// shellQuoteJoin POSIX single-quotes every value and joins them with spaces,
// so an advisory command stays safe to paste into a shell even when a package
// path carries a shell metacharacter (AFP-V0-009).
func shellQuoteJoin(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

// readAdviceSource returns at most limit bytes of path and whether the file
// held more than that. "" and false when the file is absent or unreadable. A
// truncated read is still parsed: a declaration inside the bound is found,
// one beyond it is not.
func readAdviceSource(path string, limit int64) (body string, truncated, unreadable bool) {
	// Only a path that does not exist is absent. Anything else that cannot be
	// read as a regular file (a directory, a FIFO, a dangling or special
	// symlink target, a read error) is unreadable, and the open never blocks.
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return "", false, false
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", false, true
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", false, true
	}
	defer func() { _ = file.Close() }()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return "", false, true
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", false, true
	}
	if int64(len(raw)) > limit {
		return string(raw[:limit]), true, false
	}
	return string(raw), false, false
}

func makefileDeclaresGate(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		rest, isTarget := strings.CutPrefix(line, "gate:")
		if isTarget && !strings.HasPrefix(rest, "=") {
			return true
		}
	}
	return false
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func affectedLanguages() []affected.Language {
	return languages.All()
}

// providerGoProjection derives the exact import paths a go-live-plan/0 bundle
// accepts. Packages stay empty unless the projection is RUNNABLE: an
// unresolved module path makes unit identities directories rather than import
// paths, an empty selection is rejected by the bundle reader, and the plan
// wire admits at most 4,091 exact patterns (AFP-V0-003).
func providerGoProjection(graph *affected.Graph, plan affected.Plan) affectedGoProvider {
	for _, frontier := range graph.Frontier() {
		if frontier == golang.FrontierModulePath {
			return affectedGoProvider{Packages: []string{}, State: providerStateModuleUnknown}
		}
	}
	compile := providerGoUntested(graph, plan)
	compileOver := 0
	if len(compile) > providerMaxPackagePatterns {
		compile, compileOver = nil, len(compile)
	}
	packages := providerGoPackages(plan)
	if len(packages) == 0 {
		return affectedGoProvider{Packages: []string{}, State: providerStateEmpty, noGoUnits: !graphHasGoUnit(graph), compile: compile, compileOver: compileOver}
	}
	if len(packages) > providerMaxPackagePatterns {
		return affectedGoProvider{Packages: []string{}, State: providerStateBoundExceeded, compile: compile, compileOver: compileOver}
	}
	return affectedGoProvider{Packages: packages, State: providerStateRunnable, compile: compile, compileOver: compileOver}
}

// providerGoUntested is the sorted import paths of the Go units that declare
// no test file and that a dirty source or an import path reaches: a changed
// package without tests and the untested packages importing it, which Select
// drops and no go test command covers (AFP-V0-468).
func providerGoUntested(graph *affected.Graph, plan affected.Plan) []string {
	if len(plan.Dirty) == 0 {
		return nil
	}
	var packages []string
	for _, id := range graph.UntestedCodeReached(plan.Dirty) {
		if importPath, isGo := strings.CutPrefix(id, "go:"); isGo {
			packages = append(packages, importPath)
		}
	}
	return packages
}

func graphHasGoUnit(graph *affected.Graph) bool {
	for _, id := range graph.UnitIDs() {
		if strings.HasPrefix(id, "go:") {
			return true
		}
	}
	return false
}

func providerGoPackages(plan affected.Plan) []string {
	seen := map[string]bool{}
	packages := []string{}
	for _, selection := range plan.Selected {
		importPath, isGo := strings.CutPrefix(selection.UnitID, "go:")
		if !isGo || seen[importPath] {
			continue
		}
		seen[importPath] = true
		packages = append(packages, importPath)
	}
	sort.Strings(packages)
	return packages
}

const affectedRevisionDeadline = 10 * time.Second

func affectedHeadRevision(ctx context.Context, gitExecutable, root string) (string, error) {
	return affectedRevision(ctx, gitExecutable, root, "HEAD")
}

// affectedRevision resolves one revision spec to a full commit id with a
// bounded, hermetic rev-parse. The spec is HEAD or an already-validated full
// object id, never operator text.
func affectedRevision(ctx context.Context, gitExecutable, root, spec string) (string, error) {
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", spec+"^{commit}")
	output, err := command.Output()
	revision := string(bytes.TrimSpace(output))
	if refusal := gitExitOwnershipRefusal(err); refusal != nil {
		return "", refusal
	}
	if err != nil || !validGitObjectID(revision) {
		return "", affectedHeadRefusal()
	}
	return revision, nil
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func compileSnapshotAffected(ctx context.Context, invocation affectedInvocation) (any, error) {
	name := invocation.Snapshot
	if !filepath.IsAbs(name) {
		name = filepath.Join(invocation.Root, name)
	}
	snapshot, err := plansnapshot.ReadFile(name)
	if err != nil {
		return nil, snapshotAffectedError()
	}
	root, cleanup, err := snapshot.Materialize(ctx, invocation.Root)
	if err != nil {
		return nil, snapshotAffectedError()
	}
	defer cleanup()
	var receipt any
	if invocation.PlaywrightConfig != "" {
		plan, err := typescript.SelectPlaywright(root, invocation.PlaywrightConfig, snapshot.Commit, snapshot.Paths, nil)
		if err != nil {
			return nil, snapshotAffectedError()
		}
		receipt = playwrightAffectedReceipt{Snapshot: snapshot.Scope(), OK: true, Plan: plan, Profile: typescript.PlaywrightProfile, Range: affectedRange{Base: snapshot.Base, Paths: snapshot.Paths}, Revision: snapshot.Commit, Tool: "affected"}
	} else {
		graph, err := affected.Build(ctx, root, affectedLanguages()...)
		if err != nil {
			return nil, snapshotAffectedError()
		}
		plan := affectedSelect(graph, snapshot.Paths, invocation.sharedVia)
		provider := providerGoProjection(graph, plan)
		receipt = affectedReceipt{Snapshot: snapshot.Scope(), Advice: withFocusedNextStep(compileAffectedAdvice(root, graph, plan, provider), snapshot.Base, plan), OK: true, Plan: plan, Profile: affectedProfile, Provider: affectedProvider{Go: provider}, Range: affectedRange{Base: snapshot.Base, Paths: snapshot.Paths}, Revision: snapshot.Commit, Tool: "affected"}
	}
	if err := snapshot.Validate(ctx, invocation.Root); err != nil {
		return nil, snapshotAffectedError()
	}
	return receipt, nil
}

// affectedSelect selects the plan, with its witness chains held by reference
// when shared is set: the default wire states them through viaLinks, so a
// deep chain then costs memory linear in the reached units, not the sum of
// chain lengths (V1-2979, AFP-V0-641). Such a plan must be compacted, never
// encoded whole.
func affectedSelect(graph *affected.Graph, changed []string, shared bool) affected.Plan {
	if shared {
		return affected.SelectLinked(graph, changed)
	}
	return affected.Select(graph, changed)
}

func snapshotAffectedError() error {
	return &gokernel.Error{Code: "unsupported-planning-snapshot", Message: "immutable planning snapshot is missing, mismatched, incomplete or unsupported"}
}
