package sessiontest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
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
	GraphDigest    string
	Partition      PartitionFiles
	PlanDigest     string
	PlanError      string
	// Placement is PlacementUniverse (or empty, the same) or
	// PlacementBalanced; the manifest records it (STR-V0-018).
	Placement     string
	RunnerProfile string
	Selected      []string // plan.selected unit ids
	ShardCount    int
	Source        SourceIdentity
	Universe      []string // the graph's Go import paths
	Unknown       []Unknown
}

// allowedUnknown is the closed AFP-V0-028 allowlist: every other plan.unknown
// entry forces FULL (STR-V0-003).
func allowedUnknown(entry Unknown) bool {
	switch entry.Reason {
	case "LANGUAGE_FRONTIER":
		return !strings.HasPrefix(entry.Detail, "go:") ||
			entry.Detail == "go:build-constraint-variants" || entry.Detail == "go:nested-module-frontier"
	case "UNOWNED_DIRTY_PATH", "NO_SELECTABLE_TEST":
		return true
	}
	return false
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
	default:
		return Manifest{}, fmt.Errorf("placement must be %s or %s", PlacementUniverse, PlacementBalanced)
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
		GraphDigest: input.GraphDigest, Incomplete: []string{}, PartitionDigest: PartitionDigest(input.Partition),
		Placement: placement, PlanDigest: input.PlanDigest, Profile: ManifestProfile,
		RunnerProfile: input.RunnerProfile, ShardCount: input.ShardCount, Source: input.Source,
		SourceDigest: sourceDigest, Units: []Unit{},
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
	for _, entry := range input.Unknown {
		// An unknown unit with no selectable test still needs its language's
		// declared full command when no adapter covers it (STR-V0-004).
		if language, _, ok := strings.Cut(entry.Detail, ":"); ok && entry.Reason == "NO_SELECTABLE_TEST" && language != "go" {
			languages[language] = true
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
		// A fallback need not name the language it widened for (a deleted
		// source file is only a path), so FULL runs every declared language
		// full command and covers every language owning a dirty path; an
		// unreadable declaration is INCOMPLETE.
		if input.Declared.Unreadable != "" {
			manifest.Incomplete = append(manifest.Incomplete, "the declared full scope is unreadable: "+input.Declared.Unreadable)
		}
		for _, language := range append(append([]string{}, input.Declared.Languages...), input.DirtyLanguages...) {
			languages[language] = true
		}
	} else if len(goRun) == 0 && len(languages) == 0 {
		manifest.Scope, manifest.Reason = ScopeNothing, strings.TrimPrefix(input.GoSelection, "NOTHING ")
	}
	if len(goRun) != 0 {
		manifest.placeGo(input, goRun)
	} else if manifest.Scope == ScopeFull && input.Declared.GoTest != GoTestAbsent {
		manifest.addDeclaredGo() // FULL with no Go universe still runs the declared Go suite
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
	declared := input.Declared
	if input.RunnerProfile == ProfileLocal {
		switch declared.GoTest {
		case GoTestAbsent:
			m.Incomplete = append(m.Incomplete, "go: the repository declares no go-test full-suite command")
			return
		case GoTestUnparsed:
			if m.Scope == ScopeFull {
				m.addDeclared("go-test")
				return
			}
			m.Incomplete = append(m.Incomplete, "go: the go-test recipe cannot be typed ("+declared.GoTestNote+"); run make go-test")
			return
		}
	}
	// Balanced placement runs the same cishards placement over the run set
	// instead of the universe: a package splits into the same slices either
	// way (the split test reads only that package's entries and the shard
	// count) and an unknown package costs the same table median, so the
	// execution units equal universe placement's and only their shards move.
	// Longest first into the least-loaded shard puts the first N units on N
	// different shards, so a run set of at least N units uses every shard.
	// FULL keeps the complete-universe partition in either mode.
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
				m.Units = append(m.Units, Unit{Adapter: AdapterGo, Shard: index, Slice: SliceWhole, Unit: "go:" + pkg})
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

// declaredOnly is the no-plan FULL run: every declared full-suite command,
// read independently of the planner, on shard 0 (STR-V0-003).
func (m *Manifest) declaredOnly(declared Declared) {
	if declared.Unreadable != "" {
		m.Incomplete = append(m.Incomplete, "the declared full scope is unreadable: "+declared.Unreadable)
		return
	}
	if !declared.Names() {
		m.Incomplete = append(m.Incomplete, "the repository declares no full-suite command")
		return
	}
	if declared.GoTest != GoTestAbsent {
		m.addDeclaredGo()
	}
	for _, language := range declared.Languages {
		m.addDeclared(language + "-test")
	}
}

// declaredLanguage runs a language with no qualified adapter through its
// declared full command, or records why it cannot (STR-V0-004).
func (m *Manifest) declaredLanguage(declared Declared, language string) {
	for _, named := range declared.Languages {
		if named == language {
			m.addDeclared(language + "-test")
			return
		}
	}
	m.Incomplete = append(m.Incomplete, language+": no qualified adapter and no declared "+language+"-test full command")
}

// addDeclaredGo runs the declared go-test target. The CI profile refuses it,
// because make go-test would drop the CI confinement, -race and environment
// (STR-V0-006).
func (m *Manifest) addDeclaredGo() {
	if m.RunnerProfile == ProfileCI {
		m.Incomplete = append(m.Incomplete, "go: the ci profile cannot run make go-test, which drops the CI confinement, -race and environment")
		return
	}
	m.addDeclared("go-test")
}

func (m *Manifest) addDeclared(target string) {
	m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:" + target})
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
