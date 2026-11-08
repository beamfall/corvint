package linecov

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"time"
)

// SelectionProfile names the lineSelection member of an affected-plan/1
// receipt (GLS-V0-010).
const SelectionProfile = "affected-line-selection/0"

// The selection states and FALLBACK reasons (GLS-V0-011).
const (
	StateNarrowed     = "NARROWED"
	StateShadow       = "SHADOW"
	ReasonUnqualified = "unqualified"
	StateFallback     = "FALLBACK"

	ReasonIndexUnreadable      = "index-unreadable"
	ReasonIndexFormat          = "index-format"
	ReasonModuleMismatch       = "module-mismatch"
	ReasonRunnerMismatch       = "runner-mismatch"
	ReasonRunnerUnavailable    = "runner-unavailable"
	ReasonNotAncestor          = "index-not-ancestor"
	ReasonWorktreeDirty        = "worktree-dirty"
	ReasonStaticScopeUnknown   = "static-scope-unknown"
	ReasonFallbackScopeUnknown = "fallback-scope-unknown"
	ReasonGitUnavailable       = "git-unavailable"
	// ReasonEnvironmentUnbound: the index records no environment bindings
	// (collected before GLS-V0-017, or by hand), so no test can be excluded.
	ReasonEnvironmentUnbound = "environment-unbound"
)

// The categories a unit runs whole for besides a path class (GLS-V0-013).
const (
	CategoryUnordered      = "unordered"
	CategoryOrderSensitive = "order-sensitive"
	CategoryStaticRead     = "static-read"
	// CategoryMustRun: the static plan pins the package as a declared
	// must-run unit (AFP-V0-2484), which coverage cannot discharge.
	CategoryMustRun = "must-run"
	// CategoryEnvironment: the package's in-order record was observed under
	// another environment, so what the order reaches now is unknown
	// (GLS-V0-018).
	CategoryEnvironment = "environment"
	// CategorySuiteUnrecorded: the unit's test binary declares no named test
	// (a TestMain-only package) and the index has no whole in-order record
	// of it, so what its TestMain, init and package variables execute is
	// unknown (GLS-V0-008).
	CategorySuiteUnrecorded = "suite-unrecorded"
)

// suiteUnrecorded reports whether a directory whose test binary declares no
// named test (names is empty, not nil) has no whole in-order record: only
// that record shows what the binary executes, so without it the unit is
// kept whole, never excluded (GLS-V0-008).
func (ix *Index) suiteUnrecorded(dir string, names []string) bool {
	if names == nil || len(names) > 0 {
		return false
	}
	id, ok := ix.byName[dir+"\x00"+InOrder]
	return !ok || ix.Tests[id].Incomplete
}

// The reasons a test is kept (GLS-V0-013, GLS-V0-018).
const (
	WhySelected    = "selected"
	WhyStale       = "stale"
	WhyDirect      = "direct"
	WhyUnrecorded  = "unrecorded"
	WhyEnvironment = "environment"
)

// Invalidation is the condition every exclusion entry names (LPCV-V0-952).
const Invalidation = "any recorded blob OID changes"

// Selection is the affected-line-selection/0 document.
type Selection struct {
	Profile      string              `json:"profile"`
	State        string              `json:"state"`
	Reason       string              `json:"reason,omitempty"`
	Detail       string              `json:"detail,omitempty"`
	Index        *SelectionIndex     `json:"index,omitempty"`
	Base         string              `json:"base"`
	Head         string              `json:"head"`
	Paths        []SelectionPath     `json:"paths"`
	Units        []UnitSelection     `json:"units"`
	Exclusion    *SelectionExclusion `json:"exclusion,omitempty"`
	Counts       SelectionCounts     `json:"counts"`
	LoadMillis   float64             `json:"loadMillis"`
	SelectMillis float64             `json:"selectMillis"`
	StaticMillis float64             `json:"staticMillis"`
	GitMillis    float64             `json:"gitMillis"`
}

// SelectionIndex identifies the index a selection read.
type SelectionIndex struct {
	Format string `json:"format"`
	Commit string `json:"commit"`
	Runner string `json:"runner"`
	Digest string `json:"digest"`
}

// SelectionPath is one analysed path, its side and class (GLS-V0-012).
type SelectionPath struct {
	Path   string `json:"path"`
	Side   string `json:"side"`
	Class  string `json:"class"`
	Detail string `json:"detail,omitempty"`
}

// The sides of an analysed path.
const (
	SideDrift  = "drift"
	SideChange = "change"
)

// UnitSelection is one Go unit's decision (GLS-V0-013).
type UnitSelection struct {
	Unit     string     `json:"unit"`
	Mode     Mode       `json:"mode"`
	Run      string     `json:"run,omitempty"`
	Category string     `json:"category,omitempty"`
	Witness  string     `json:"witness,omitempty"`
	Declared int        `json:"declared"`
	Kept     []KeptTest `json:"kept,omitempty"`
	Excluded int        `json:"excluded"`
}

// KeptTest is one kept test of a unit decided by its records.
type KeptTest struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// SelectionExclusion is the entry every left-out test refers to
// (GLS-V0-014, LPCV-V0-952).
type SelectionExclusion struct {
	Record       string   `json:"record"`
	Changed      []string `json:"changed"`
	Invalidation string   `json:"invalidation"`
}

// SelectionCounts restates the decisions (GLS-V0-015).
type SelectionCounts struct {
	Units          int            `json:"units"`
	UnitsWhole     int            `json:"unitsWhole"`
	UnitsTests     int            `json:"unitsTests"`
	UnitsCompile   int            `json:"unitsCompile"`
	UnitsExcluded  int            `json:"unitsExcluded"`
	TestsDeclared  int            `json:"testsDeclared"`
	TestsCoverage  int            `json:"testsKeptByCoverage"`
	TestsFallback  int            `json:"testsKeptByFallback"`
	TestsExcluded  int            `json:"testsExcluded"`
	FallbackByKind map[string]int `json:"testsKeptByFallbackCategory,omitempty"`
}

// Fallback returns a FALLBACK selection for reason; it decides no unit.
func Fallback(reason, detail string) Selection {
	return Selection{Profile: SelectionProfile, State: StateFallback, Reason: reason, Detail: detail, Paths: []SelectionPath{}, Units: []UnitSelection{}}
}

// Unit is one Go unit of the static plan: its id, module-relative
// directory, and the changed path the static plan reached it from. MustRun
// marks a declared must-run unit, StaticRead one a reader rule selects; both
// run whole.
type Unit struct {
	ID, Dir, Witness    string
	StaticRead, MustRun bool
}

// PlanInput is what Plan decides from. Drift is the diff from the index
// commit to Base read by ReadDrift, Change the diff from Base to Head read by
// ReadChange. Units are the static plan's Go units. Heads lists each unit
// directory's tests at Head (HeadTestNames). StaticOf is the static plan of
// a set of fallback paths: whether its scope is unknown, and each reached
// unit directory with its witnessing path. Environ is the environment the
// selected tests will run under, whose values every excluded test's
// bindings must hold (GLS-V0-018).
type PlanInput struct {
	Base, Head            string
	Drift, Change         []FileDiff
	ReadDrift, ReadChange func(string) ([]byte, error)
	Units                 []Unit
	Heads                 func(dirs []string) (map[string][]string, error)
	StaticOf              func(paths []string) (unknown bool, reach map[string]string, err error)
	Environ               []string
}

// Plan decides every unit of in.Units (GLS-V0-012..015). Its own time is
// SelectMillis and the StaticOf walk's StaticMillis; the head reads count as
// GitMillis. A StaticOf that fails or is unknown is the
// fallback-scope-unknown FALLBACK.
func (ix *Index) Plan(in PlanInput) Selection {
	began := time.Now()
	var staticTime, readTime time.Duration
	sel := Selection{Profile: SelectionProfile, State: StateNarrowed, Base: in.Base, Head: in.Head, Paths: []SelectionPath{}, Units: []UnitSelection{}}
	sel.Index = &SelectionIndex{Format: Format, Commit: ix.Commit, Runner: ix.Runner, Digest: ix.Digest()}
	if !ix.EnvBound {
		return ix.timed(withPaths(Fallback(ReasonEnvironmentUnbound, "the index records no environment bindings; collect it again"), sel), began, 0, 0)
	}
	values := EnvValues(in.Environ)
	driftRes, drift := ix.AnalyzePaths(in.Drift, in.ReadDrift)
	changeRes, change := ix.AnalyzePaths(in.Change, in.ReadChange)
	class := map[string]string{}
	var fallbackPaths []string
	var changed []string
	add := func(side string, results []PathResult) {
		for _, r := range results {
			sel.Paths = append(sel.Paths, SelectionPath{Path: r.Path, Side: side, Class: r.Class, Detail: r.Detail})
			if r.Mapped() {
				for _, h := range r.Hunks {
					changed = append(changed, fmt.Sprintf("%s %s:%d-%d", side, r.Path, h.Start, h.End))
				}
				continue
			}
			if _, seen := class[r.Path]; !seen {
				fallbackPaths = append(fallbackPaths, r.Path)
			}
			class[r.Path] = r.Class // the change's class wins over the drift's
		}
	}
	add(SideDrift, driftRes)
	add(SideChange, changeRes)
	reach := map[string]string{}
	if everyChangeFallback(changeRes) && len(changeRes) > 0 {
		// Every unit is reached by a changed path, so every unit runs whole
		// without a walk; the witness is a fallback path of the change.
		for _, u := range in.Units {
			reach[u.Dir] = u.Witness
			if _, ok := class[u.Witness]; !ok {
				reach[u.Dir] = firstFallback(changeRes)
			}
		}
	} else if len(fallbackPaths) > 0 {
		s := time.Now()
		unknown, reached, err := in.StaticOf(fallbackPaths)
		staticTime = time.Since(s)
		switch {
		case err != nil:
			return ix.timed(withPaths(Fallback(ReasonFallbackScopeUnknown, err.Error()), sel), began, staticTime, readTime)
		case unknown:
			return ix.timed(withPaths(Fallback(ReasonFallbackScopeUnknown, "the static plan of the fallback paths is UNKNOWN"), sel), began, staticTime, readTime)
		}
		reach = reached
	}
	unordered := ix.unorderedSet()
	units := append([]Unit(nil), in.Units...)
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	dirs := make([]string, 0, len(units))
	for _, u := range units {
		dirs = append(dirs, u.Dir)
	}
	s := time.Now()
	heads, err := in.Heads(dirs)
	readTime = time.Since(s)
	if err != nil {
		return ix.timed(withPaths(Fallback(ReasonGitUnavailable, "head tests: "+err.Error()), sel), began, staticTime, readTime)
	}
	// A unit reached through its in-order record keeps every test.
	orderReached := map[string]bool{}
	for id := range change.Tests {
		if t := ix.Tests[id]; t.Name == InOrder {
			orderReached[t.Package] = true
		}
	}
	for id := range drift.Tests {
		if t := ix.Tests[id]; t.Name == InOrder {
			orderReached[t.Package] = true
		}
	}
	sel.Counts.FallbackByKind = map[string]int{}
	excludedAny := false
	for _, u := range units {
		us := UnitSelection{Unit: u.ID}
		names := heads[u.Dir]
		if u.MustRun {
			us.Mode, us.Category, us.Witness = Whole, CategoryMustRun, u.Witness
		} else if u.StaticRead {
			us.Mode, us.Category, us.Witness = Whole, CategoryStaticRead, u.Witness
		} else if witness, whole := reach[u.Dir]; whole {
			us.Mode, us.Category, us.Witness = Whole, class[witness], witness
			if us.Category == "" {
				us.Category = ClassDeclaration // a witness outside the fallback set: keep whole
			}
		} else if unordered[u.Dir] || orderReached[u.Dir] {
			us.Mode, us.Category = Whole, CategoryUnordered
			if !unordered[u.Dir] {
				us.Category = CategoryOrderSensitive
			}
		} else if id, ok := ix.byName[u.Dir+"\x00"+InOrder]; ok && EnvUnbound(ix.envBindings(id), values) != "" {
			// The order was observed under another environment: what it
			// reaches now is unknown (GLS-V0-018).
			us.Mode, us.Category, us.Witness = Whole, CategoryEnvironment, EnvUnbound(ix.envBindings(id), values)
		} else if ix.suiteUnrecorded(u.Dir, names) {
			// A TestMain-only binary without a suite record: no test record
			// can say the change misses it (GLS-V0-008).
			us.Mode, us.Category = Whole, CategorySuiteUnrecorded
		}
		us.Declared = len(names)
		if us.Mode == Whole {
			sel.Counts.TestsFallback += len(names)
			sel.Counts.FallbackByKind[us.Category] += len(names)
		} else {
			for _, name := range names {
				id, ok := ix.byName[u.Dir+"\x00"+name]
				why := ""
				switch {
				case !ok || ix.Tests[id].Incomplete:
					why = WhyUnrecorded
				case change.Direct[id]:
					why = WhyDirect
				case change.Tests[id]:
					why = WhySelected
				case drift.Tests[id]:
					why = WhyStale
				case EnvUnbound(ix.envBindings(id), values) != "":
					// Observed under another environment: its record may
					// not say what it reaches now (GLS-V0-018).
					why = WhyEnvironment
					sel.Counts.FallbackByKind[CategoryEnvironment]++
				default:
					continue
				}
				us.Kept = append(us.Kept, KeptTest{Name: name, Why: why})
			}
			switch {
			case len(us.Kept) > 0 && len(us.Kept) == len(names):
				us.Mode = Whole
			case len(us.Kept) > 0:
				us.Mode = Tests
				kept := make([]string, len(us.Kept))
				for i, k := range us.Kept {
					kept[i] = k.Name
				}
				us.Run = RunPattern(kept)
			case change.Touched[u.Dir]:
				us.Mode, us.Run = Compile, "^$"
			default:
				us.Mode = Exclude
			}
			us.Excluded = len(names) - len(us.Kept)
			sel.Counts.TestsCoverage += len(us.Kept)
			sel.Counts.TestsExcluded += us.Excluded
			if us.Excluded > 0 {
				excludedAny = true
			}
		}
		sel.Counts.TestsDeclared += len(names)
		sel.Counts.Units++
		switch us.Mode {
		case Whole:
			sel.Counts.UnitsWhole++
		case Tests:
			sel.Counts.UnitsTests++
		case Compile:
			sel.Counts.UnitsCompile++
		case Exclude:
			sel.Counts.UnitsExcluded++
		}
		sel.Units = append(sel.Units, us)
	}
	if len(sel.Counts.FallbackByKind) == 0 {
		sel.Counts.FallbackByKind = nil
	}
	if excludedAny {
		sort.Strings(changed)
		sel.Exclusion = &SelectionExclusion{Record: ix.Digest(), Changed: changed, Invalidation: Invalidation}
	}
	return ix.timed(sel, began, staticTime, readTime)
}

// withPaths carries the analysed paths and identity of sel into fb.
func withPaths(fb, sel Selection) Selection {
	fb.Index, fb.Base, fb.Head, fb.Paths = sel.Index, sel.Base, sel.Head, sel.Paths
	return fb
}

// timed sets the selection's own, static and read times.
func (ix *Index) timed(sel Selection, began time.Time, static, read time.Duration) Selection {
	total := time.Since(began)
	sel.SelectMillis = Millis(total - static - read)
	sel.StaticMillis = Millis(static)
	sel.GitMillis += Millis(read)
	return sel
}

// Millis is d in milliseconds, kept to the microsecond.
func Millis(d time.Duration) float64 {
	return math.Round(float64(d.Microseconds())) / 1000
}

// RunPattern is the anchored go test -run pattern of names.
func RunPattern(names []string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

// UnitDir is the module-relative directory of a Go unit id "go:IMPORTPATH",
// and false for a unit outside module.
func UnitDir(module, id string) (string, bool) {
	importPath, ok := strings.CutPrefix(id, "go:")
	if !ok {
		return "", false
	}
	if importPath == module {
		return ".", true
	}
	rest, ok := strings.CutPrefix(importPath, module+"/")
	if !ok || rest == "" || path.Clean(rest) != rest {
		return "", false
	}
	return rest, true
}

// everyChangeFallback reports whether no changed path was mapped by line;
// the static plan's units are then exactly the fallback paths' reach.
func everyChangeFallback(results []PathResult) bool {
	for _, r := range results {
		if r.Mapped() {
			return false
		}
	}
	return true
}

// firstFallback is the lexically first fallback path of results.
func firstFallback(results []PathResult) string {
	first := ""
	for _, r := range results {
		if !r.Mapped() && (first == "" || r.Path < first) {
			first = r.Path
		}
	}
	return first
}
