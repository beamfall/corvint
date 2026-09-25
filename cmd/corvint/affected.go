package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/plansnapshot"
)

// affectedProfile names the wire this command emits (AFP-V0-003).
const affectedProfile = "affected-plan/0"

// affectedReceipt is the stdout document. The plan is the selector's own
// canonical plan; provider is the derived, non-authoritative input an operator
// may copy into a Go live-test provider bundle (AFP-V0-004).
type affectedReceipt struct {
	Snapshot map[string]any   `json:"snapshot,omitempty"`
	Advice   affectedAdvice   `json:"advice"`
	Mutates  bool             `json:"mutates"`
	OK       bool             `json:"ok"`
	Plan     affected.Plan    `json:"plan"`
	Profile  string           `json:"profile"`
	Provider affectedProvider `json:"provider"`
	Range    affectedRange    `json:"range"`
	Revision string           `json:"revision"`
	Tool     string           `json:"tool"`
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
	Snapshot            string
	Root                string
	Base                string
	Providers           []string
	Checkouts           []extevidence.Checkout
	SelectionProfile    string
	PlaywrightConfig    string
	PlaywrightDiscovery string
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
}

// affectedAdvice is the grounded check list (AFP-V0-009). Checks is ordered
// mandatory-first in source order, then the single advisory command; Unknown is
// the sorted frontier an operator must read before trusting the advisory half.
// Nothing here is derived from plan.excluded: an exclusion is never advice
// (AFP-V0-004). TestSelection is present only when a provider record was
// named (ETS-V0-002); it never adds to or removes from Checks.
type affectedAdvice struct {
	Checks        []affectedCheck `json:"checks"`
	Note          string          `json:"note"`
	Status        string          `json:"status"`
	TestSelection map[string]any  `json:"test_selection,omitempty"`
	Unknown       []string        `json:"unknown"`
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
	adviceMakefileName       = "Makefile"
	adviceAgentsName         = "AGENTS.md"
	adviceMaxSourceBytes     = 256 << 10
	adviceMaxMandatoryChecks = 16
	adviceNote               = "advice is static; no check was executed; mandatory checks remain required whatever the advisory list says"
	adviceNoGateUnknown      = "NO_REPOSITORY_GATE_DECLARED: no Makefile gate target or AGENTS.md Verify block"
	adviceMakefileReason     = "the repository Makefile declares a gate target, so the full gate stays mandatory"
	adviceAgentsReason       = "the repository AGENTS.md Verify block declares this command, so it stays mandatory"
	adviceLaunchReason       = "the repository AGENTS.md Verify block declares this command, but it launches a program or runs in the background and does not end on its own, so it is not a check"
	adviceVerifyHeading      = "Verify"
)

var adviceShellFences = map[string]bool{"sh": true, "bash": true, "console": true}

const (
	providerStateRunnable      = "RUNNABLE"
	providerStateEmpty         = "EMPTY_SELECTION"
	providerStateModuleUnknown = "MODULE_PATH_UNRESOLVED"
	providerStateBoundExceeded = "PACKAGE_BOUND_EXCEEDED"
	providerMaxPackagePatterns = 4091
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
	invocation, err := parseAffectedOptions(arguments[index+1:])
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
	return invocation, true, nil
}

// affectedOptionNames are the flags `affected` accepts, each taking one value
// as `--flag VALUE` or `--flag=VALUE`.
var affectedOptionNames = map[string]bool{"--snapshot": true, "--base": true, "--playwright-config": true, "--playwright-discovery": true, "--provider": true, "--provider-command": true, "--repository": true, "--selection-profile": true}

// parseAffectedOptions reads the flags after `affected`. `--base` must
// already be a full object id: a ref name is resolved by the caller, never
// here, so range.base is exactly what the operator asked for. `--provider`,
// `--repository`, and `--selection-profile` follow the impact bounds and
// errors (ETS-V0-001).
func parseAffectedOptions(rest []string) (affectedInvocation, error) {
	invocation := affectedInvocation{}
	baseSet := false
	for index := 0; index < len(rest); {
		name, value, inline := strings.Cut(rest[index], "=")
		if !affectedOptionNames[name] {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError(name + " requires exactly one value")
			}
			value = rest[index+1]
			index++
		}
		index++
		var err error
		switch name {
		case "--snapshot":
			if invocation.Snapshot != "" || value == "" {
				err = argumentError("--snapshot requires exactly one nonempty value")
			} else {
				invocation.Snapshot = value
			}
		case "--base":
			err = setAffectedBase(&invocation, &baseSet, value)
		case "--provider", "--provider-command":
			err = addAffectedProvider(&invocation, name, value)
		case "--repository":
			err = addAffectedCheckout(&invocation, value)
		case "--playwright-config":
			err = setAffectedPlaywrightConfig(&invocation, value)
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
	if invocation.Snapshot != "" && (invocation.Base != "" || len(invocation.Providers) != 0 || invocation.PlaywrightDiscovery != "") {
		return affectedInvocation{}, argumentError("--snapshot cannot be combined with --base, --provider or --playwright-discovery")
	}
	if len(invocation.Providers) == 0 && (len(invocation.Checkouts) != 0 || invocation.SelectionProfile != "") {
		return affectedInvocation{}, argumentError("--repository and --selection-profile require --provider")
	}
	if invocation.PlaywrightConfig != "" && len(invocation.Providers) != 0 {
		return affectedInvocation{}, argumentError("--playwright-config cannot be combined with --provider")
	}
	if invocation.SelectionProfile == appflows.E2ESafeProfile {
		return invocation, checkAffectedE2ESafe(invocation)
	}
	if invocation.PlaywrightDiscovery != "" && invocation.PlaywrightConfig == "" {
		return affectedInvocation{}, argumentError("--playwright-discovery requires --playwright-config")
	}
	if len(invocation.Providers) != 0 && invocation.SelectionProfile == "" {
		invocation.SelectionProfile = extevidence.ProfileStrict
	}
	return invocation, nil
}

// checkAffectedE2ESafe admits the e2e-safe profile only over one repository-relative selection
// provider file, and a discovery record, when named, only as a repository-relative file (AFU-V1-024).
func checkAffectedE2ESafe(invocation affectedInvocation) error {
	if len(invocation.Providers) != 1 || len(invocation.Checkouts) != 0 || !affected.ValidRelativePath(invocation.Providers[0]) {
		return argumentError("--selection-profile e2e-safe requires exactly one repository-relative --provider file and no --repository")
	}
	if invocation.PlaywrightDiscovery != "" && !affected.ValidRelativePath(invocation.PlaywrightDiscovery) {
		return argumentError("--playwright-discovery must be a repository-relative canonical path under --selection-profile e2e-safe")
	}
	return nil
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
	if !validGitObjectID(value) {
		return argumentError("--base must be a full commit id, got " + value)
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
	var receipt any
	var err error
	if invocation.Snapshot != "" {
		receipt, err = compileSnapshotAffected(ctx, invocation)
	} else if invocation.PlaywrightConfig != "" {
		receipt, err = compilePlaywrightAffected(ctx, invocation)
	} else {
		receipt, err = compileAffected(ctx, invocation)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected receipt"})
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
	committed, err := affectedRangePaths(ctx, gitExecutable, root, invocation.Base)
	if err != nil {
		return playwrightAffectedReceipt{}, err
	}
	allDirty := affected.NormalizePaths(append(append([]string{}, dirty...), committed...))
	discovery := readPlaywrightDiscovery(root, invocation.PlaywrightDiscovery)
	plan, err := typescript.SelectPlaywright(root, invocation.PlaywrightConfig, revision, allDirty, discovery)
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
	}, nil
}

func readPlaywrightDiscovery(root, name string) []byte {
	if name == "" {
		return nil
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > typescript.PlaywrightDiscoveryMaxBytes {
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
	root := invocation.Root
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return affectedReceipt{}, affectedGitExecutableRefusal()
	}
	revision, err := affectedHeadRevision(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, err
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	committed, err := affectedRangePaths(ctx, gitExecutable, root, invocation.Base)
	if err != nil {
		return affectedReceipt{}, err
	}
	graph, err := affected.Build(root, affectedLanguages()...)
	if err != nil {
		return affectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-graph", Message: err.Error()}
	}
	// The status and the graph are two observations of one mutable worktree.
	// A dirty set or HEAD that changed while the graph was built would bind a
	// plan to inputs nobody observed together; refuse rather than publish.
	recheck, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return affectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	if !equalStringSlices(dirty, recheck) {
		return affectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "worktree changed while the plan was compiled"}
	}
	if revisionAfter, err := affectedHeadRevision(ctx, gitExecutable, root); err != nil || revisionAfter != revision {
		return affectedReceipt{}, &gokernel.Error{Code: "unsupported-affected-drift", Message: "HEAD changed while the plan was compiled"}
	}
	plan := affected.Select(graph, affected.NormalizePaths(append(append([]string{}, dirty...), committed...)))
	provider := providerGoProjection(graph, plan)
	advice := compileAffectedAdvice(root, plan, provider)
	if invocation.SelectionProfile == appflows.E2ESafeProfile {
		advice.TestSelection, err = affectedE2ESafeSelection(ctx, invocation, revision, graph, plan, advice)
		if err != nil {
			return affectedReceipt{}, err
		}
	} else if len(invocation.Providers) != 0 {
		input := affectedSelectionInput(invocation, plan, dirty, advice)
		input.CheckoutStatus = func(ctx context.Context, dir string) ([]string, error) {
			return affected.DirtyPaths(ctx, gitExecutable, dir)
		}
		advice.TestSelection = extevidence.Selection(ctx, root, revision, invocation.Providers, invocation.Checkouts, input)
	}
	return affectedReceipt{
		Advice:   advice,
		Mutates:  false,
		OK:       true,
		Plan:     plan,
		Profile:  affectedProfile,
		Provider: affectedProvider{Go: provider},
		Range:    affectedRange{Base: invocation.Base, Paths: committed},
		Revision: revision,
		Tool:     "affected",
	}, nil
}

// affectedE2ESafeSelection runs the e2e-safe profile over the plan and hands its closed output to
// the receipt as the test_selection member (AFU-V1-019..024).
func affectedE2ESafeSelection(ctx context.Context, invocation affectedInvocation, revision string, graph *affected.Graph, plan affected.Plan, advice affectedAdvice) (map[string]any, error) {
	mandatory := affectedSelectionInput(invocation, plan, nil, advice).Mandatory
	selection := appflows.SelectE2E(ctx, invocation.Root, appflows.E2EInput{
		Provider: invocation.Providers[0], Discovery: invocation.PlaywrightDiscovery, Revision: revision,
		Base: invocation.Base, Graph: graph, Plan: plan, Mandatory: mandatory,
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

// compileAffectedAdvice joins the repository-declared mandatory gate, the
// advisory Go projection, and the unknown frontier into one static list. It
// executes nothing and reads nothing derived from plan.excluded (AFP-V0-009).
func compileAffectedAdvice(root string, plan affected.Plan, provider affectedGoProvider) affectedAdvice {
	checks, mandatoryUnknown, truncated := mandatoryAffectedChecks(root)
	unknown := append([]string{}, mandatoryUnknown...)
	if !declaresMandatoryCheck(checks) && !truncated {
		unknown = append(unknown, adviceNoGateUnknown)
	}
	advisory, advisoryUnknown := advisoryAffectedChecks(plan, provider)
	checks = append(checks, advisory...)
	unknown = append(unknown, advisoryUnknown...)
	for _, entry := range plan.Unknown {
		unknown = append(unknown, entry.Reason+": "+entry.Detail)
	}
	return affectedAdvice{Checks: checks, Note: adviceNote, Status: adviceStatus, Unknown: sortedUniqueStrings(unknown)}
}

// mandatoryAffectedChecks reads only repository-owned declarations from the
// working tree at the root: a Makefile gate target and the AGENTS.md Verify
// block. Both reads are bounded, and the list is deduplicated in order of
// appearance and capped at adviceMaxMandatoryChecks. A declared command that
// does not end on its own is advisory and follows the mandatory ones. A read
// that hits the bound, or a declaration the cap drops, is reported in the
// returned unknown list rather than silently disappearing; truncated tells the
// caller a source was cut short, so it never also claims no gate was declared.
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
	makefile, makefileTruncated := readAdviceSource(filepath.Join(root, adviceMakefileName), adviceMaxSourceBytes)
	if makefileTruncated {
		truncated = true
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_TRUNCATED: %s exceeded %d bytes", adviceMakefileName, adviceMaxSourceBytes))
	}
	if makefileDeclaresGate(makefile) {
		admit(affectedCheck{Command: "make gate", Kind: adviceKindMandatory, Reason: adviceMakefileReason, Source: adviceMakefileName})
	}
	agents, agentsTruncated := readAdviceSource(filepath.Join(root, adviceAgentsName), adviceMaxSourceBytes)
	if agentsTruncated {
		truncated = true
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_TRUNCATED: %s exceeded %d bytes", adviceAgentsName, adviceMaxSourceBytes))
	}
	commands, unrecognized := agentsVerifyCommands(agents)
	for _, command := range commands {
		admit(agentsCheck(command))
	}
	for _, heading := range unrecognized {
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_UNRECOGNIZED: %s heading %q is not %q, so its commands are not checks", adviceAgentsName, heading, adviceVerifyHeading))
	}
	if cappedSource != "" {
		unknown = append(unknown, fmt.Sprintf("MANDATORY_DECLARATION_CAPPED: %s declared more than %d commands", cappedSource, adviceMaxMandatoryChecks))
	}
	return append(checks, launches...), unknown, truncated
}

// agentsCheck is a Verify-block command as a check: advisory when it does not
// end on its own, mandatory otherwise.
func agentsCheck(command string) affectedCheck {
	if nonTerminatingCommand(command) {
		return affectedCheck{Command: command, Kind: adviceKindAdvisory, Reason: adviceLaunchReason, Source: adviceAgentsName}
	}
	return affectedCheck{Command: command, Kind: adviceKindMandatory, Reason: adviceAgentsReason, Source: adviceAgentsName}
}

// adviceLaunchers are the commands whose only job is to open a program the
// caller then leaves running.
var adviceLaunchers = map[string]bool{"open": true, "xdg-open": true}

// nonTerminatingCommand reports a command that launches a program or runs in
// the background. Any other command stays a check: requiring too much is safe.
func nonTerminatingCommand(command string) bool {
	fields := strings.Fields(command)
	background := strings.HasSuffix(command, "&") && !strings.HasSuffix(command, "&&")
	return background || adviceLaunchers[fields[0]]
}

// declaresMandatoryCheck reports whether any check is mandatory.
func declaresMandatoryCheck(checks []affectedCheck) bool {
	for _, check := range checks {
		if check.Kind == adviceKindMandatory {
			return true
		}
	}
	return false
}

// advisoryAffectedChecks proposes the Go packages the plan selected. Any state
// other than RUNNABLE yields no command and one unknown line naming the state.
func advisoryAffectedChecks(plan affected.Plan, provider affectedGoProvider) ([]affectedCheck, []string) {
	if provider.State != providerStateRunnable {
		return nil, []string{"NO_ADVISORY_GO_COMMAND: provider.go.state is " + provider.State}
	}
	command := "GOTOOLCHAIN=local go test -count=1 " + shellQuoteJoin(provider.Packages)
	reason := fmt.Sprintf("the plan selected %d unit(s) over %d dirty path(s), so these packages are the plausible first pass", len(plan.Selected), len(plan.Dirty))
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
func readAdviceSource(path string, limit int64) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", false
	}
	if int64(len(body)) > limit {
		return string(body[:limit]), true
	}
	return string(body), false
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

// agentsVerifyCommands returns the command lines of every fenced
// sh/bash/console block under a heading whose text is exactly "Verify", in any
// case, with shell comments removed. It also returns, in order, each other
// heading containing "verify" that has such a block, whose commands are not
// taken as checks.
func agentsVerifyCommands(body string) (commands, unrecognized []string) {
	commands, unrecognized = []string{}, []string{}
	heading, underVerify, inBlock := "", false, false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		fence := adviceShellFences[strings.ToLower(strings.TrimPrefix(trimmed, "```"))]
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "#"):
			heading = strings.TrimSpace(strings.Trim(trimmed, "#"))
			underVerify = strings.EqualFold(heading, adviceVerifyHeading)
		case inBlock && strings.HasPrefix(trimmed, "```"):
			inBlock = false
		case strings.HasPrefix(trimmed, "```") && fence && !underVerify && strings.Contains(strings.ToLower(heading), "verify"):
			unrecognized = appendOnce(unrecognized, heading)
		case strings.HasPrefix(trimmed, "```"):
			inBlock = underVerify && fence
		case inBlock:
			commands = appendNonEmpty(commands, stripShellComment(strings.TrimPrefix(trimmed, "$ ")))
		}
	}
	return commands, unrecognized
}

// stripShellComment removes a shell comment: a "#" that begins a word outside
// quotes runs to the end of the line.
func stripShellComment(line string) string {
	quote := byte(0)
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case quote != 0 && char == quote:
			quote = 0
		case quote == '\'':
		case char == '\\':
			index++
		case quote == 0 && (char == '\'' || char == '"'):
			quote = char
		case quote == 0 && char == '#' && (index == 0 || line[index-1] == ' ' || line[index-1] == '\t'):
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func appendNonEmpty(values []string, value string) []string {
	if value == "" {
		return values
	}
	return append(values, value)
}

func appendOnce(values []string, value string) []string {
	for _, present := range values {
		if present == value {
			return values
		}
	}
	return append(values, value)
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
	packages := providerGoPackages(plan)
	if len(packages) == 0 {
		return affectedGoProvider{Packages: []string{}, State: providerStateEmpty}
	}
	if len(packages) > providerMaxPackagePatterns {
		return affectedGoProvider{Packages: []string{}, State: providerStateBoundExceeded}
	}
	return affectedGoProvider{Packages: packages, State: providerStateRunnable}
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
	command := exec.CommandContext(deadline, gitExecutable, "--no-optional-locks", "-C", root, "rev-parse", "--verify", "--quiet", spec+"^{commit}")
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "LANG=C", "LC_ALL=C",
	}
	output, err := command.Output()
	revision := string(bytes.TrimSpace(output))
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
		graph, err := affected.Build(root, affectedLanguages()...)
		if err != nil {
			return nil, snapshotAffectedError()
		}
		plan := affected.Select(graph, snapshot.Paths)
		provider := providerGoProjection(graph, plan)
		receipt = affectedReceipt{Snapshot: snapshot.Scope(), Advice: compileAffectedAdvice(root, plan, provider), OK: true, Plan: plan, Profile: affectedProfile, Provider: affectedProvider{Go: provider}, Range: affectedRange{Base: snapshot.Base, Paths: snapshot.Paths}, Revision: snapshot.Commit, Tool: "affected"}
	}
	if err := snapshot.Validate(ctx, invocation.Root); err != nil {
		return nil, snapshotAffectedError()
	}
	return receipt, nil
}

func snapshotAffectedError() error {
	return &gokernel.Error{Code: "unsupported-planning-snapshot", Message: "immutable planning snapshot is missing, mismatched, incomplete or unsupported"}
}
