package sessiontest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// runnerLanguages maps each blocking runner other than go to the affected
// plugin language whose units it runs.
var runnerLanguages = map[string]string{
	testid.Vitest: "typescript", testid.Jest: "typescript", testid.Playwright: "typescript",
	testid.Pytest: "python", testid.RSpec: "ruby", testid.Minitest: "ruby",
}

// ReasonRunnerRemoved names the FULL run of a plan whose inventory or
// declaration names a runner instance the tree no longer declares
// (RNC-V1-004, owner decision R32).
const ReasonRunnerRemoved = "runner-removed"

// RemovedRunners lists, sorted, the runner instances named by undeclared
// (the inventory's records for instances the tree no longer declares,
// testinventory.Verdict.Undeclared) or by the declaration's instance
// entries, and the runners named by its runner entries, that no instance of
// instances matches.
func RemovedRunners(declared Declared, instances []contract.Instance, undeclared []string) []string {
	keys, runners := map[string]bool{}, map[string]bool{}
	for _, instance := range instances {
		keys[instance.Key()], runners[instance.Runner] = true, true
	}
	removed := map[string]bool{}
	for _, key := range undeclared {
		removed[key] = true
	}
	for _, entry := range declared.Instances {
		if !keys[entry.Key] {
			removed[entry.Key] = true
		}
	}
	for _, entry := range declared.Runners {
		if !runners[entry.Key] {
			removed[entry.Key] = true
		}
	}
	return sortedKeys(removed)
}

// goSuite reports whether the declared Go full suite runs: always for a
// declared go member, and for the built-in default only when the tree
// declares a Go instance (RNC-V1-013).
func goSuite(input Input) bool {
	if !input.Declared.GoDefault {
		return true
	}
	return slices.ContainsFunc(input.Instances, func(instance contract.Instance) bool { return instance.Runner == testid.Go })
}

// Unknown is one plan.unknown entry. Hint is the plan's optional operator
// guidance (AFP-V0-494); it never changes the manifest.
type Unknown struct {
	Detail string
	Reason string
	Hint   string
}

// PartitionFiles are the repository's AFP-V0-022/041 placement inputs from
// .github/cishards; a nil file is absent, which keeps the lexical partition.
type PartitionFiles struct {
	Allow  []byte
	Costs  []byte
	Slices []byte
}

// Input is everything the verdict and the manifest are a function of
// (STR-V0-003, STR-V0-018). PlanError is set when no plan could be produced;
// then only Base, Source, Declared, RunnerProfile and ShardCount are read.
type Input struct {
	Base     string
	Checks   []string
	Declared Declared
	// DirtyLanguages names each non-Go language whose plugin owns a
	// plan.dirty path; FULL covers it even when no unit names it.
	DirtyLanguages []string
	GoSelection    string // the AFP-V0-012 verdict line: run/FALLBACK/NOTHING
	// GoSlices maps a selected Go import path to its STR-V0-063 slice: the
	// anchored -run pattern of the tests that can observe the change, or
	// the reason the package runs whole. A package without an entry runs
	// whole without a reason.
	GoSlices map[string]GoSlice
	// Excluded lists each graph Go unit the caller left out of Universe
	// with its reason, as "<unit>: <reason>" (STR-V0-069).
	Excluded    []string
	GraphDigest string
	Partition   PartitionFiles
	PlanDigest  string
	PlanError   string
	// Placement is PlacementUniverse (or empty, the same),
	// PlacementBalanced or PlacementMeasured; the manifest records it
	// (STR-V0-018).
	Placement string
	// Producer is the writing binary's stamped identity, recorded as the
	// manifest's producer (STR-V0-8301); nil records none.
	Producer      *Producer
	RunnerProfile string
	Selected      []string // plan.selected unit ids
	// SelectionIncomplete states each part of the selection the caller
	// could not read; the manifest is INCOMPLETE for it (STR-V0-8706).
	SelectionIncomplete []string
	ShardCount          int
	// ShardPlan is the read corvint-test-shards/0 plan PlacementMeasured
	// places by; its partition digest is the manifest's (STR-V0-054).
	ShardPlan *ShardPlan
	Source    SourceIdentity
	Universe  []string // the graph's Go import paths
	Unknown   []Unknown
	// Instances are the runner instances the planned tree declares
	// (testinventory.Detect, RNC-V1-014). A language with no declared full
	// command runs each of its instances' full commands (RNC-V1-013), and
	// the built-in Go default runs only when a Go instance exists.
	Instances []contract.Instance
	// Removed are the runner instances the committed inventory or the
	// declaration names but the planned tree no longer declares
	// (RemovedRunners); any one runs every test (RNC-V1-004).
	Removed []string
	// LineUnits maps a selected Go import path to its GLS-V0-020 line
	// decision; LineIndex is the manifest's lineIndex record. Both are empty
	// without --line-index.
	LineUnits map[string]LineUnit
	LineIndex string
}

// LineUnit is one Go package's line-coverage decision (GLS-V0-020): Mode is
// exclude, compile or tests, Pattern the anchored -run pattern of a tests
// unit, and Reason what the manifest names.
type LineUnit struct {
	Mode    string
	Pattern string
	Reason  string
}

// GoSlice is one package's STR-V0-063 test-function slice: Pattern is the
// anchored `-run` alternation when the selection is test-function precise,
// else empty with Reason saying why the package runs whole.
type GoSlice struct {
	Pattern string
	Reason  string
	// FullReason retains a named whole-package obligation when FULL drops
	// the selection's narrowing pattern. Ordinary slicing reasons stay local
	// to narrowed plans, preserving existing fallback receipts.
	FullReason string
}

// goUnit is one Go package's execution unit on a shard: the selected slice
// when the input proves one, else the whole package with its reason.
func goUnit(slices map[string]GoSlice, pkg string, shard int) Unit {
	slice := slices[pkg]
	if slice.Pattern != "" {
		return Unit{Adapter: AdapterGo, Flag: "-run", Pattern: slice.Pattern, Reason: slice.Reason, ReasonCode: ReasonCode(slice.Reason), Shard: shard, Slice: SliceSelected, Unit: "go:" + pkg}
	}
	return Unit{Adapter: AdapterGo, Reason: slice.Reason, ReasonCode: ReasonCode(slice.Reason), Shard: shard, Slice: SliceWhole, Unit: "go:" + pkg}
}

// ModuleUnitPrefix starts the unit id of a nested Go module's execution unit
// (STR-V0-064): "go-module:<directory>".
const ModuleUnitPrefix = "go-module:"

// driverNamespace is the affected plan's namespace of make targets and CI
// jobs (AZU-V0-001), which no language adapter runs.
const driverNamespace = "driver"

// moduleReason is the stated reason a nested-module unit runs whole.
const moduleReason = "a nested Go module the root does not build: its packages are not indexed, so its own go test ./... runs whole from its directory (AFP-V0-650)"

// moduleDirectory returns the module directory of an AFP-V0-650
// unobserved-module frontier, "go:unobserved-module:<directory>:<path>". The
// directory must be a clean, local, slash-separated path and the only split
// whose path lies below it; anything else is not admitted, so the entry
// stays outside the allowlist and the run is FULL.
func moduleDirectory(entry Unknown) (string, bool) {
	return gateselect.UnobservedModule(entry.Reason, entry.Detail)
}

// binaryConsumer returns the file of a well-formed AFP-V0-762
// UNSELECTED_BINARY_CONSUMER entry (gateselect.BinaryConsumer). No Go run,
// narrowed or FULL, runs it, so on every runner profile `corvint test` names it
// as a check still required and not run instead of widening to FULL for it
// (STR-V0-750, V1-4008); the deciding driver admits the same entry (V1-4361).
func binaryConsumer(entry Unknown) (string, bool) {
	return gateselect.BinaryConsumer(entry.Reason, entry.Detail)
}

// BinaryConsumerCheck is the still-required check that names a binary
// consumer file (STR-V0-750).
func BinaryConsumerCheck(file string) string {
	return file + " (UNSELECTED_BINARY_CONSUMER: it runs the built binary of a Go command this change reaches; run it by its own driver, since no Go test run covers it)"
}

// ModuleCheck is the still-required check that names a nested module's own
// suite on the ci profile (AFP-V0-2742): CI runs each nested module's tests
// and vet in go-static (CIN-V0-008), never in a Go shard, so
// `corvint test --profile ci` neither runs it nor widens to FULL for it.
func ModuleCheck(directory string) string {
	return "go -C " + directory + " test ./... (go:unobserved-module: a nested Go module the root does not build; CI runs its own suite in a job outside the Go shards, so corvint test --profile ci does not run it)"
}

// ModuleCovered reports whether every dirty path is named by a well-formed
// AFP-V0-650 unobserved-module frontier (STR-V0-064). `corvint test` then
// reads an empty root selection as NOTHING for the root rather than the
// AFP-V0-012 empty-selection fallback: no observed module builds those
// paths, and each module's own unit runs them. An empty dirty set is never
// covered.
func ModuleCovered(dirty []string, unknown []Unknown) bool {
	covered := map[string]bool{}
	for _, entry := range unknown {
		if directory, ok := moduleDirectory(entry); ok {
			covered[strings.TrimPrefix(entry.Detail, "go:unobserved-module:"+directory+":")] = true
		}
	}
	for _, path := range dirty {
		if !covered[path] {
			return false
		}
	}
	return len(dirty) != 0
}

// allowedUnknown is the closed AFP-V0-028 allowlist, the one the deciding
// driver shares (gateselect.AllowedUnknown, AFP-V0-982): every other
// plan.unknown entry forces FULL (STR-V0-003).
func allowedUnknown(entry Unknown) bool {
	return gateselect.AllowedUnknown(entry.Reason, entry.Detail)
}

// frontierSuite is gateselect.FrontierSuite, the language whose declared suite
// a bearing non-Go frontier adds for both selectors (AFP-V0-2750).
func frontierSuite(entry Unknown) (string, bool) {
	return gateselect.FrontierSuite(entry.Reason, entry.Detail)
}

// PartitionDigest binds the cishards profile and the three repository
// placement files (STR-V0-007).
func PartitionDigest(files PartitionFiles) string {
	hash := sha256.New()
	hash.Write([]byte("corvint-test-partition/0\n" + cishards.ProfileDigest() + "\n"))
	for _, file := range []struct {
		name string
		raw  []byte
	}{{"package-costs.json", files.Costs}, {"test-split-allow.json", files.Allow}, {"test-slices.json", files.Slices}} {
		hash.Write([]byte(file.name + "\x00"))
		if file.raw == nil {
			hash.Write([]byte("absent"))
		} else {
			hash.Write([]byte(strconv.Itoa(len(file.raw)) + "\x00"))
			hash.Write(file.raw)
		}
		hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

// Build derives the verdict and the manifest. It is pure: every shard and
// host reaches the same bytes from the same Input.
func Build(input Input) (Manifest, error) {
	if input.ShardCount < 1 || input.ShardCount > cishards.MaxShards {
		return Manifest{}, fmt.Errorf("shard count must be 1 to %d", cishards.MaxShards)
	}
	placement := input.Placement
	switch placement {
	case "":
		placement = PlacementUniverse
	case PlacementUniverse, PlacementBalanced:
	case PlacementMeasured:
		if input.ShardPlan == nil {
			return Manifest{}, fmt.Errorf("placement %s needs a shard plan", PlacementMeasured)
		}
	default:
		return Manifest{}, fmt.Errorf("placement must be %s, %s or %s", PlacementUniverse, PlacementBalanced, PlacementMeasured)
	}
	partitionDigest := PartitionDigest(input.Partition)
	if placement == PlacementMeasured {
		partitionDigest = input.ShardPlan.PartitionDigest
	}
	sourceDigest, err := Digest(input.Source)
	if err != nil {
		return Manifest{}, err
	}
	declaredDigest, err := Digest(input.Declared)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Base: input.Base, Checks: nonNil(input.Checks), DeclaredDigest: declaredDigest,
		GraphDigest: input.GraphDigest, Incomplete: []string{}, NoSuite: []string{}, PartitionDigest: partitionDigest,
		Placement: placement, PlanDigest: input.PlanDigest, Producer: input.Producer, Profile: ManifestProfile,
		RunnerProfile: input.RunnerProfile, ShardCount: input.ShardCount, Source: input.Source,
		SourceDigest: sourceDigest, Units: []Unit{},
	}
	if len(input.Excluded) != 0 {
		manifest.Excluded = slices.Sorted(slices.Values(input.Excluded))
	}
	if input.Declared.Unreadable != "" {
		// Every scope needs the declaration: the Go environment and flags
		// locally, and the language commands and no-suite assertions in
		// FULL; nothing runs on a guessed scope (STR-V0-003).
		manifest.Scope, manifest.Reason = ScopeFull, "the declared full scope is unreadable"
		if input.PlanError != "" {
			manifest.Reason = "no plan: " + input.PlanError
		}
		manifest.Incomplete = append(manifest.Incomplete, "the declared full scope is unreadable: "+input.Declared.Unreadable)
		return manifest.bounded(), nil
	}
	if input.PlanError != "" {
		manifest.Scope, manifest.Reason = ScopeFull, "no plan: "+input.PlanError
		manifest.declaredOnly(input)
		return manifest.bounded(), nil
	}
	fullReason := ""
	goRun := []string{}
	// attributionOf reads from the same verdict whether the Go selector
	// attributed every UNOWNED_DIRTY_PATH (gateselect.UnownedPathFallback).
	switch verdict, detail, _ := strings.Cut(input.GoSelection, " "); verdict {
	case "FALLBACK":
		fullReason = detail
	case "run":
		goRun = strings.Fields(detail)
	case "NOTHING":
	default:
		fullReason = "no AFP-V0-012 attribution verdict"
	}
	languages := map[string]bool{}
	provided := map[string]bool{}
	for _, id := range input.Selected {
		// A make target or CI job (AZU-V0-001) is run by make or CI, not by
		// a language adapter; the affected plan reports it.
		if strings.HasPrefix(id, "declared-consumer:") {
			// Each consumer has its own project-owned argv, rather than a
			// language-wide full command. The caller pins it in Declared.
			languages[id] = true
		} else if input.RunnerProfile != ProfileCI && providerUnit(input.Declared, id) {
			// STR-V0-8702: a declared provider runs the selected file itself,
			// so the file alone does not need its language's full command.
			// Everything else that names the language still adds it. The ci
			// profile runs only Go (STR-V0-012), so there the unit names its
			// language's declared command as a check, as before.
			provided[id] = true
		} else if language, _, ok := strings.Cut(id, ":"); ok && language != "go" && language != driverNamespace {
			languages[language] = true
		}
	}
	if len(input.Removed) != 0 {
		// RNC-V1-004, owner decision R32: a removed runner runs every test.
		fullReason = ReasonRunnerRemoved + ": " + strings.Join(input.Removed, ", ") + " is named by the inventory or " + CommandsPath + " but no longer declared, so every test runs"
	}
	modules := map[string]bool{}
	attribution := attributionOf(input)
	for _, entry := range input.Unknown {
		// SLF-V1-007: the one decision SelectionVerdictOf reports. A
		// NO_SELECTABLE_TEST unit of another language adds its declared
		// full command (STR-V0-004); a nested module adds its own suite
		// (STR-V0-064), or on the ci profile names it as a check
		// (AFP-V0-2742); a binary consumer is named as a check, since FULL
		// would not run it either (STR-V0-750); an unowned path no Go rule
		// attributes runs every test (RNC-V1-004, owner decision R32); an
		// allowlisted or flow-binding entry widens nothing (AFP-V0-028,
		// AFU-V1-1652); a bearing non-Go frontier adds its language's
		// declared suite (AFP-V0-2750); every other entry forces FULL.
		action := decideUnknown(entry, attribution, input.RunnerProfile)
		switch {
		case action.suite != "":
			languages[action.suite] = true
		case action.module != "":
			modules[action.module] = true
		case action.check != "":
			if !slices.Contains(manifest.Checks, action.check) {
				manifest.Checks = append(manifest.Checks, action.check)
			}
		case action.full != "" && fullReason == "":
			fullReason = action.full
		}
	}
	universe := map[string]bool{}
	for _, pkg := range input.Universe {
		universe[pkg] = true
	}
	for _, pkg := range goRun {
		if fullReason == "" && !universe[pkg] {
			fullReason = "selected package outside the Go universe: " + pkg
		}
	}
	manifest.Scope = ScopeNarrowed
	manifest.LineIndex = input.LineIndex
	if fullReason == "" && len(input.LineUnits) != 0 {
		goRun = applyLineUnits(&manifest, &input, goRun)
	}
	if fullReason != "" {
		manifest.Scope, manifest.Reason = ScopeFull, fullReason
		goRun = append([]string{}, input.Universe...)
		// FULL removes narrowing patterns, but named obligations and their
		// triggering paths still explain why a package must run (AZU-V0-9900).
		whole := make(map[string]GoSlice, len(input.GoSlices))
		for pkg, slice := range input.GoSlices {
			if slice.FullReason != "" {
				whole[pkg] = GoSlice{Reason: slice.FullReason, FullReason: slice.FullReason}
			}
		}
		input.GoSlices = whole // FULL runs every package whole (STR-V0-063)
		// A fallback need not name the language it widened for (a deleted
		// source file is only a path), so FULL runs every declared language
		// full command, records every declared no-suite assertion and covers
		// every language owning a dirty path.
		for _, entry := range input.Declared.Languages {
			languages[entry.Language] = true
		}
		for _, language := range input.DirtyLanguages {
			languages[language] = true
		}
		for _, instance := range input.Instances {
			if language, ok := runnerLanguages[instance.Runner]; ok {
				languages[language] = true
			}
		}
	} else if len(goRun) == 0 && len(languages) == 0 && len(modules) == 0 && len(provided) == 0 {
		manifest.Scope, manifest.Reason = ScopeNothing, strings.TrimPrefix(input.GoSelection, "NOTHING ")
	}
	if len(goRun) != 0 {
		manifest.placeGo(input, goRun)
	} else if manifest.Scope == ScopeFull && goSuite(input) {
		manifest.addDeclared("go") // FULL with no Go universe still runs the declared Go full suite
	}
	for _, directory := range sortedKeys(modules) {
		// FULL runs the root universe, never a nested module, so a module
		// with a dirty path keeps its unit in either scope (STR-V0-064).
		manifest.Units = append(manifest.Units, Unit{
			Adapter: AdapterGo, Dir: directory, Reason: moduleReason, ReasonCode: ReasonNestedModule, Shard: 0, Slice: SliceWhole, Unit: ModuleUnitPrefix + directory,
		})
	}
	for _, id := range sortedKeys(provided) {
		// A selected file keeps its provider unit in either scope: FULL adds
		// the declared full commands, it does not drop a selected test.
		manifest.Units = append(manifest.Units, Unit{Adapter: AdapterProvider, Shard: 0, Slice: SliceWhole, Unit: id})
	}
	for _, language := range sortedKeys(languages) {
		manifest.declaredLanguage(input, language)
	}
	manifest.Incomplete = append(manifest.Incomplete, input.SelectionIncomplete...)
	return manifest.bounded(), nil
}

// placeGo expands the Go run set into execution units on the unchanged
// complete-universe partition (STR-V0-002, STR-V0-007), or, for balanced
// placement of a narrowed run, partitions the run set's own execution units
// (STR-V0-008).
func (m *Manifest) placeGo(input Input, run []string) {
	// Balanced placement runs the same cishards placement over the run set
	// instead of the universe: a package splits into the same slices either
	// way (the split test reads only that package's entries and the shard
	// count) and an unknown package costs the same table median, so the
	// execution units equal universe placement's and only their shards move.
	// Longest first into the least-loaded shard puts the first N units on N
	// different shards, so a run set of at least N units uses every shard.
	// FULL keeps the complete-universe partition in either mode.
	if m.Placement == PlacementMeasured {
		m.placeMeasured(*input.ShardPlan, input.ShardCount, run, input.GoSlices)
		return
	}
	placed, partition := input.Universe, "universe"
	if m.Placement == PlacementBalanced && m.Scope == ScopeNarrowed {
		set := map[string]bool{}
		for _, pkg := range run {
			set[pkg] = true
		}
		placed, partition = sortedKeys(set), "balanced"
	}
	shards, err := cishards.PlanFrom(placed, input.ShardCount, input.Partition.Costs, input.Partition.Allow, input.Partition.Slices)
	if err != nil {
		m.Incomplete = append(m.Incomplete, "go: the "+partition+" partition is unavailable: "+err.Error())
		return
	}
	wanted := map[string]bool{}
	for _, pkg := range run {
		wanted[pkg] = true
	}
	for index, shard := range shards {
		for _, pkg := range shard.Packages {
			if wanted[pkg] {
				m.Units = append(m.Units, goUnit(input.GoSlices, pkg, index))
			}
		}
		for _, slice := range shard.Slices {
			if wanted[slice.Package] {
				reason := partitionObligationReason(input.GoSlices[slice.Package])
				m.Units = append(m.Units, Unit{
					Adapter: AdapterGo, Flag: slice.Flag, Pattern: slice.Pattern, Shard: index,
					Reason: reason, ReasonCode: ReasonCode(reason),
					Slice: fmt.Sprintf("%d/%d", slice.Index, slice.Count), Unit: "go:" + slice.Package,
				})
			}
		}
	}
}

// placeMeasured puts every Go package of the run set, NARROWED or FULL,
// whole on the shard the shard plan assigns it (STR-V0-054), or, for a
// package the plan carries as test slices, each slice on its planned shard
// (STR-V0-057). A plan for another shard count or one that omits a run-set
// package places nothing: the run is INCOMPLETE rather than run on a guessed
// shard.
func (m *Manifest) placeMeasured(plan ShardPlan, count int, run []string, slices map[string]GoSlice) {
	if plan.Count != count {
		m.Incomplete = append(m.Incomplete, fmt.Sprintf("go: the shard plan has %d shards, the run %d", plan.Count, count))
		return
	}
	shardOf := map[string]int{}
	slicesOf := map[string][]Unit{}
	for _, shard := range plan.Shards {
		for _, file := range shard.Files {
			shardOf[file] = shard.Index
		}
		for _, slice := range shard.Slices {
			reason := partitionObligationReason(slices[slice.Package])
			slicesOf[slice.Package] = append(slicesOf[slice.Package], Unit{
				Adapter: AdapterGo, Flag: slice.Flag, Pattern: slice.Pattern, Shard: shard.Index,
				Reason: reason, ReasonCode: ReasonCode(reason),
				Slice: fmt.Sprintf("%d/%d", slice.Index, slice.Count), Unit: "go:" + slice.Package,
			})
		}
	}
	set := map[string]bool{}
	for _, pkg := range run {
		set[pkg] = true
	}
	packages := sortedKeys(set)
	missing := []string{}
	for _, pkg := range packages {
		if _, ok := shardOf[pkg]; !ok && len(slicesOf[pkg]) == 0 {
			missing = append(missing, pkg)
		}
	}
	if len(missing) != 0 {
		m.Incomplete = append(m.Incomplete, fmt.Sprintf("go: the shard plan omits %d run-set packages, first %s", len(missing), missing[0]))
		return
	}
	for _, pkg := range packages {
		if units, ok := slicesOf[pkg]; ok {
			m.Units = append(m.Units, units...)
			continue
		}
		m.Units = append(m.Units, goUnit(slices, pkg, shardOf[pkg]))
	}
}

// Only the explicit ownership obligation survives CI partitioning. Ordinary
// package slicing reasons were historically omitted from partitioned units.
func partitionObligationReason(slice GoSlice) string {
	if slice.FullReason != "" {
		return slice.Reason
	}
	return ""
}

// declaredOnly is the no-plan FULL run: the declared Go full suite and every
// declared language full command, read independently of the planner, on
// shard 0; every no-suite assertion is recorded (STR-V0-003).
func (m *Manifest) declaredOnly(input Input) {
	if goSuite(input) {
		m.addDeclared("go")
	}
	languages := map[string]bool{}
	for _, entry := range input.Declared.Languages {
		languages[entry.Language] = true
	}
	for _, instance := range input.Instances {
		if language, ok := runnerLanguages[instance.Runner]; ok {
			languages[language] = true
		}
	}
	for _, language := range sortedKeys(languages) {
		m.declaredLanguage(input, language)
	}
}

// declaredLanguage runs a language with no qualified adapter through its
// declared full command, records the project's assertion that it has no
// suite, or is INCOMPLETE when the declaration does not name it (STR-V0-004).
// The ci profile runs only what the CI full branch runs, which is Go, so it
// never launches a declared command (a make target included); it reports the
// command as still required and not run (STR-V0-012).
//
// A language the declaration does not name runs the full command of each
// runner instance of the language the tree declares instead, one
// `declared:<runner>@<root>` unit each (RNC-V1-013, RNC-V1-014); only a
// language with neither is INCOMPLETE.
func (m *Manifest) declaredLanguage(input Input, language string) {
	declared := input.Declared
	entry, ok := declared.Command(language)
	var instances []string
	for _, instance := range input.Instances {
		if runnerLanguages[instance.Runner] == language {
			instances = append(instances, instance.Key())
		}
	}
	source := declared.Source
	if source == "" {
		source = CommandsPath
	}
	switch {
	case !ok && len(instances) != 0:
		for _, key := range instances {
			if m.RunnerProfile == ProfileCI {
				if check := key + " full command"; !slices.Contains(m.Checks, check) {
					m.Checks = append(m.Checks, check)
				}
				continue
			}
			m.addDeclared(key)
		}
	case !ok:
		m.Incomplete = append(m.Incomplete, language+": no qualified adapter and "+source+" declares no "+language+" full command")
	case entry.None != "":
		m.NoSuite = append(m.NoSuite, language+": "+entry.None)
	case m.RunnerProfile == ProfileCI:
		if command := strings.Join(entry.Argv, " "); !slices.Contains(m.Checks, command) {
			m.Checks = append(m.Checks, command)
		}
	default:
		m.addDeclared(language)
	}
}

// addDeclared adds the whole-suite unit of one declared command: "go" is the
// declaration's typed `go test` full suite, any other name a declared
// language's argv (STR-V0-006).
func (m *Manifest) addDeclared(name string) {
	m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:" + name})
}

// bounded applies the unit and size bounds (STR-V0-015): an exceeded bound
// keeps no units and is INCOMPLETE, never a narrower run.
func (m Manifest) bounded() Manifest {
	sort.SliceStable(m.Units, func(i, j int) bool {
		if m.Units[i].Shard != m.Units[j].Shard {
			return m.Units[i].Shard < m.Units[j].Shard
		}
		return adapterOrder(m.Units[i].Adapter) < adapterOrder(m.Units[j].Adapter)
	})
	if len(m.Units) > MaxUnits {
		m.Incomplete = append(m.Incomplete, fmt.Sprintf("%d execution units exceed the %d bound", len(m.Units), MaxUnits))
		m.Units = []Unit{}
	}
	if _, err := Encode(m); err != nil {
		m.Incomplete = append(m.Incomplete, fmt.Sprintf("the manifest exceeds %d bytes", MaxDocumentBytes))
		m.Units = []Unit{}
	}
	return m
}

// adapterOrder places a shard's Go units first, then its provider units,
// then its declared units.
func adapterOrder(adapter string) int {
	switch adapter {
	case AdapterGo:
		return 0
	case AdapterProvider:
		return 1
	}
	return 2
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string{}, values...)
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// applyLineUnits narrows a NARROWED run set by its line decisions
// (GLS-V0-020): an exclude package leaves the run and is listed in Excluded,
// a compile package builds and vets only unless a slice already runs tests,
// and a tests package runs the union of its kept tests and any slice.
func applyLineUnits(m *Manifest, input *Input, run []string) []string {
	if input.GoSlices == nil {
		input.GoSlices = map[string]GoSlice{}
	}
	kept := make([]string, 0, len(run))
	for _, pkg := range run {
		line, ok := input.LineUnits[pkg]
		if !ok {
			kept = append(kept, pkg)
			continue
		}
		static := input.GoSlices[pkg]
		switch line.Mode {
		case "exclude":
			m.Excluded = append(m.Excluded, "go:"+pkg+": "+line.Reason)
			continue
		case "compile":
			if static.Pattern == "" || static.Pattern == "^$" {
				input.GoSlices[pkg] = GoSlice{Pattern: "^$", Reason: line.Reason}
			}
		case "tests":
			if pattern, ok := unionPatterns(static.Pattern, line.Pattern); ok {
				reason := line.Reason
				if static.Pattern != "" && static.Pattern != "^$" {
					reason += "; with " + static.Reason
				}
				input.GoSlices[pkg] = GoSlice{Pattern: pattern, Reason: reason}
			}
		}
		kept = append(kept, pkg)
	}
	sort.Strings(m.Excluded)
	return kept
}

// unionPatterns joins two anchored name-list patterns, "^(A|B)$", where an
// empty or "^$" static pattern adds nothing; any other static pattern cannot
// be joined, so the package stays as the static slice left it.
func unionPatterns(static, line string) (string, bool) {
	names := func(p string) ([]string, bool) {
		inner, ok := strings.CutPrefix(p, "^(")
		if !ok {
			return nil, false
		}
		inner, ok = strings.CutSuffix(inner, ")$")
		if !ok || inner == "" {
			return nil, false
		}
		return strings.Split(inner, "|"), true
	}
	lineNames, ok := names(line)
	if !ok {
		return "", false
	}
	all := append([]string(nil), lineNames...)
	switch static {
	case "", "^$":
	default:
		more, ok := names(static)
		if !ok {
			return "", false
		}
		all = append(all, more...)
	}
	sort.Strings(all)
	return "^(" + strings.Join(slices.Compact(all), "|") + ")$", true
}

// runnerNeutralFlags are the go test flags that change neither the built
// test binary nor the test-binary arguments a test can observe beyond
// linecov.TestRuntime (GLS-V0-020): parallelism and failing fast. -timeout
// is bound into the identity as the test runtime's deadline. The values say
// whether the flag takes a value.
var runnerNeutralFlags = map[string]bool{"p": true, "timeout": true, "failfast": false}

// runnerVerbosityFlags override the -test.v=test2json collection runs every
// test under: a declared flag follows `corvint test`'s -json, so -v=false
// turns testing.Verbose off and -json=false drops the test2json verbosity.
// Any spelling or value is refused rather than modelled (GLS-V0-020).
var runnerVerbosityFlags = map[string]bool{"v": true, "json": true, "test.v": true}

// runnerGoEnv are the go.env variables a profile may set and still narrow:
// Go toolchain settings the runner identity reads (GOTOOLCHAIN through
// GOVERSION, GOOS, GOARCH, CGO_ENABLED, GOEXPERIMENT, GOFLAGS, GOWORK and the
// linecov.ArchFeatureVars) or that change no executed code (caches, module
// download and telemetry settings). Any other one, an application setting,
// GODEBUG, a runtime setting such as GOMAXPROCS or a C toolchain, is visible
// to the tests but not to collection, so it is refused (GLS-V0-020).
var runnerGoEnv = map[string]bool{
	"GOTOOLCHAIN": true, "GOOS": true, "GOARCH": true, "CGO_ENABLED": true, "GOEXPERIMENT": true, "GOFLAGS": true, "GOWORK": true, "GOENV": true,
	"GOCACHE": true, "GOCACHEPROG": true, "GOMODCACHE": true, "GOPATH": true, "GOTMPDIR": true, "GOBIN": true,
	"GOPROXY": true, "GOSUMDB": true, "GONOPROXY": true, "GONOSUMDB": true, "GOPRIVATE": true, "GOINSECURE": true, "GOVCS": true, "GOAUTH": true,
	"GOTELEMETRY": true, "GOTELEMETRYDIR": true,
}

// RunnerEnv is the environment the Go runner identity of a profile is read
// under (GLS-V0-020), and the go test -timeout its tests run under, which
// names the identity's test runtime (linecov.TestRuntime): environ with the
// profile's Go environment, and every build flag the profile passes on the
// command line added to the effective GOFLAGS (`go env GOFLAGS` under that
// environment at root, so a setting saved with go env -w stays), as the
// index's runner names them. A flag that is neither one of those nor neutral
// (a test-behaviour, coverage or exec flag, -count other than 1, or -v,
// -json or -test.v in any spelling or value), a value GOFLAGS cannot carry,
// or a -timeout that is not a duration, is refused: the identity could not
// show it, so the line selection keeps the static plan (runner-unavailable).
// So is a go.env variable outside runnerGoEnv.
func RunnerEnv(root, profile string, declared Declared, environ []string) ([]string, time.Duration, error) {
	goEnv, goFlags := declared.GoEnv, declared.GoFlags
	if profile == ProfileCI {
		goEnv, goFlags = ciGoEnv, ciGoFlags
	}
	for _, kv := range goEnv {
		name, _, _ := strings.Cut(kv, "=")
		if !runnerGoEnv[name] && !slices.Contains(linecov.ArchFeatureVars, name) {
			return nil, 0, fmt.Errorf("go.env variable %s is visible to the tests but not to line coverage collection, and the line runner identity cannot carry it", name)
		}
	}
	env := append(append([]string{}, environ...), goEnv...)
	timeout := linecov.DefaultTestTimeout
	var build []string
	for i := 0; i < len(goFlags); i++ {
		word := goFlags[i]
		if !strings.HasPrefix(word, "-") {
			return nil, 0, fmt.Errorf("go flag %q is not a flag the line runner identity can carry", word)
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(word, "-"), "-"), "=")
		if runnerVerbosityFlags[name] {
			return nil, 0, fmt.Errorf("go flag -%s overrides the -test.v=test2json verbosity line coverage collection runs every test under, and the line runner identity cannot carry it", name)
		}
		valued, neutral := runnerNeutralFlags[name]
		if !neutral {
			if name == "count" && inline && value == "1" {
				continue
			}
			var known bool
			if valued, known = linecov.BuildFlags[name]; !known {
				return nil, 0, fmt.Errorf("go flag -%s may change which code a test executes, and the line runner identity cannot carry it", name)
			}
		}
		if valued && !inline {
			if i+1 == len(goFlags) {
				return nil, 0, fmt.Errorf("go flag -%s has no value", name)
			}
			i++
			value, inline = goFlags[i], true
		}
		if name == "timeout" {
			d, err := time.ParseDuration(value)
			if err != nil {
				return nil, 0, fmt.Errorf("go flag -timeout value %q is not a duration", value)
			}
			timeout = d
		}
		if neutral {
			continue
		}
		if strings.ContainsAny(value, " \t\n\r'\"\\") {
			return nil, 0, fmt.Errorf("go flag -%s value %q cannot be carried in GOFLAGS", name, value)
		}
		if inline {
			build = append(build, "-"+name+"="+value)
		} else {
			build = append(build, "-"+name)
		}
	}
	if len(build) == 0 {
		return env, timeout, nil
	}
	cmd := exec.Command("go", "env", "GOFLAGS")
	cmd.Dir, cmd.Env = root, env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, 0, fmt.Errorf("go env GOFLAGS: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	flags := strings.TrimSpace(string(out))
	return append(env, "GOFLAGS="+strings.TrimSpace(flags+" "+strings.Join(build, " "))), timeout, nil
}
