package sessiontest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/gateselect"
)

// Unknown is one plan.unknown entry.
type Unknown struct {
	Detail string
	Reason string
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
	Placement     string
	RunnerProfile string
	Selected      []string // plan.selected unit ids
	ShardCount    int
	// ShardPlan is the read corvint-test-shards/0 plan PlacementMeasured
	// places by; its partition digest is the manifest's (STR-V0-054).
	ShardPlan *ShardPlan
	Source    SourceIdentity
	Universe  []string // the graph's Go import paths
	Unknown   []Unknown
}

// GoSlice is one package's STR-V0-063 test-function slice: Pattern is the
// anchored `-run` alternation when the selection is test-function precise,
// else empty with Reason saying why the package runs whole.
type GoSlice struct {
	Pattern string
	Reason  string
}

// goUnit is one Go package's execution unit on a shard: the selected slice
// when the input proves one, else the whole package with its reason.
func goUnit(slices map[string]GoSlice, pkg string, shard int) Unit {
	slice := slices[pkg]
	if slice.Pattern != "" {
		return Unit{Adapter: AdapterGo, Flag: "-run", Pattern: slice.Pattern, Reason: slice.Reason, Shard: shard, Slice: SliceSelected, Unit: "go:" + pkg}
	}
	return Unit{Adapter: AdapterGo, Reason: slice.Reason, Shard: shard, Slice: SliceWhole, Unit: "go:" + pkg}
}

// ModuleUnitPrefix starts the unit id of a nested Go module's execution unit
// (STR-V0-064): "go-module:<directory>".
const ModuleUnitPrefix = "go-module:"

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
		Placement: placement, PlanDigest: input.PlanDigest, Profile: ManifestProfile,
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
		manifest.declaredOnly(input.Declared)
		return manifest.bounded(), nil
	}
	fullReason := ""
	goRun := []string{}
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
	for _, id := range input.Selected {
		if language, _, ok := strings.Cut(id, ":"); ok && language != "go" {
			languages[language] = true
		}
	}
	modules := map[string]bool{}
	for _, entry := range input.Unknown {
		// An unknown unit with no selectable test still needs its language's
		// declared full command when no adapter covers it (STR-V0-004).
		if language, _, ok := strings.Cut(entry.Detail, ":"); ok && entry.Reason == "NO_SELECTABLE_TEST" && language != "go" {
			languages[language] = true
		}
		if directory, ok := moduleDirectory(entry); ok && input.RunnerProfile != ProfileCI {
			// The nested module's own go test ./... covers the path
			// (STR-V0-064); the ci profile's confined runner is not
			// qualified for it, so there it stays outside the allowlist.
			modules[directory] = true
			continue
		}
		if allowedUnknown(entry) {
			continue // an allowlisted frontier names a category, not a unit (AFP-V0-028)
		}
		if fullReason == "" {
			fullReason = fmt.Sprintf("plan.unknown entry outside the AFP-V0-028 allowlist: %s %s", entry.Reason, entry.Detail)
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
	if fullReason != "" {
		manifest.Scope, manifest.Reason = ScopeFull, fullReason
		goRun = append([]string{}, input.Universe...)
		input.GoSlices = nil // FULL runs every package whole (STR-V0-063)
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
	} else if len(goRun) == 0 && len(languages) == 0 && len(modules) == 0 {
		manifest.Scope, manifest.Reason = ScopeNothing, strings.TrimPrefix(input.GoSelection, "NOTHING ")
	}
	if len(goRun) != 0 {
		manifest.placeGo(input, goRun)
	} else if manifest.Scope == ScopeFull {
		manifest.addDeclared("go") // FULL with no Go universe still runs the declared Go full suite
	}
	for _, directory := range sortedKeys(modules) {
		// FULL runs the root universe, never a nested module, so a module
		// with a dirty path keeps its unit in either scope (STR-V0-064).
		manifest.Units = append(manifest.Units, Unit{
			Adapter: AdapterGo, Dir: directory, Reason: moduleReason, Shard: 0, Slice: SliceWhole, Unit: ModuleUnitPrefix + directory,
		})
	}
	for _, language := range sortedKeys(languages) {
		manifest.declaredLanguage(input.Declared, language)
	}
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
				m.Units = append(m.Units, Unit{
					Adapter: AdapterGo, Flag: slice.Flag, Pattern: slice.Pattern, Shard: index,
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
			slicesOf[slice.Package] = append(slicesOf[slice.Package], Unit{
				Adapter: AdapterGo, Flag: slice.Flag, Pattern: slice.Pattern, Shard: shard.Index,
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

// declaredOnly is the no-plan FULL run: the declared Go full suite and every
// declared language full command, read independently of the planner, on
// shard 0; every no-suite assertion is recorded (STR-V0-003).
func (m *Manifest) declaredOnly(declared Declared) {
	m.addDeclared("go")
	for _, entry := range declared.Languages {
		m.declaredLanguage(declared, entry.Language)
	}
}

// declaredLanguage runs a language with no qualified adapter through its
// declared full command, records the project's assertion that it has no
// suite, or is INCOMPLETE when the declaration does not name it (STR-V0-004).
// The ci profile runs only what the CI full branch runs, which is Go, so it
// never launches a declared command (a make target included); it reports the
// command as still required and not run (STR-V0-012).
func (m *Manifest) declaredLanguage(declared Declared, language string) {
	entry, ok := declared.Command(language)
	switch {
	case !ok:
		m.Incomplete = append(m.Incomplete, language+": no qualified adapter and "+DeclarationPath+" declares no "+language+" full command")
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
		return m.Units[i].Adapter > m.Units[j].Adapter // go before declared
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
