package playwrightruns

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
)

// Baseline profiles (PWR-V0-041..045, GitHub #1033).
const (
	BaselineProfile     = "corvint-run-baseline/0"
	BaselineDiffProfile = "corvint-runs-baseline-diff/0"
	// MaxBaselineEntries bounds the known and flaky lists of one baseline.
	MaxBaselineEntries = 4096
	// MaxBaselineBytes bounds a baseline document.
	MaxBaselineBytes = 16 << 20
	// DefaultBaselineMaxAge is the default staleness bound in commits between the baseline
	// revision and the branch point (PWR-V0-045).
	DefaultBaselineMaxAge = 50
)

// Baseline-diff reasons (PWR-V0-042..044).
const (
	ReasonNotInBaseline   = "NOT_IN_BASELINE"
	ReasonKnownVerified   = "KNOWN_VERIFIED"
	ReasonOwnFile         = "OWN_FILE"
	ReasonEvidenceMissing = "EVIDENCE_MISSING"
	ReasonEvidenceChanged = "EVIDENCE_CHANGED"
	ReasonEvidencePassing = "EVIDENCE_PASSING"
	ReasonSuspectedFlake  = "SUSPECTED_FLAKE"
	// ReasonUnobservedAtBaseline is a candidate failure of a test the baseline's full run never
	// observed completing (PWR-V0-061).
	ReasonUnobservedAtBaseline = "UNOBSERVED_AT_BASELINE"
	VerdictBaselinePass        = "PASS"
	VerdictBaselineBlocked     = "BLOCKED"
)

// Evidence names one retained report by the path it was read from and its sha256.
type Evidence struct {
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
	// IdentitySource is present only when the caller gave a sidecar option (PWR-V0-067).
	IdentitySource string `json:"identitySource,omitempty"`
}

// evidenceOf is the evidence statement of one named report.
func evidenceOf(nr NamedRun) Evidence {
	return Evidence{Path: nr.Path, Sha256: nr.Run.Sha256, IdentitySource: nr.Run.IdentitySource}
}

// validIdentitySource reports whether every evidence states no identity source or a known one
// (PWR-V0-067).
func validIdentitySource(evidence ...*Evidence) bool {
	for _, ev := range evidence {
		if ev == nil {
			continue
		}
		switch ev.IdentitySource {
		case "", IdentityEmbedded, IdentitySidecar, IdentityNotDeclared:
		default:
			return false
		}
	}
	return true
}

// BaselineEntry is one failing test of a baseline. A known entry carries the full-run and the
// isolated re-run report it failed in; a suspected flake carries the full-run report only.
type BaselineEntry struct {
	Identity
	Full     Evidence  `json:"full"`
	Isolated *Evidence `json:"isolated"`
}

// UnobservedEntry is a test the baseline's full run listed but never observed completing: every
// occurrence was interrupted (RUN_INTERRUPTED) or did not run (NOT_RUN). Full is the first full-run
// report that lists it so (PWR-V0-060).
type UnobservedEntry struct {
	Identity
	Reason string   `json:"reason"`
	Full   Evidence `json:"full"`
}

// Baseline is a revision-pinned record of the failures a full run and an isolated re-run share
// (PWR-V0-041). Unobserved, present only when the full run left tests unobserved and the caller
// accepted that, lists those tests; a baseline never states them passing (PWR-V0-060).
type Baseline struct {
	Profile    string            `json:"profile"`
	Revision   string            `json:"revision"`
	Known      []BaselineEntry   `json:"known"`
	Flaky      []BaselineEntry   `json:"flaky"`
	Unobserved []UnobservedEntry `json:"unobserved,omitempty"`
}

// ErrBaselineShape is returned for a document that is not a well-formed baseline.
var ErrBaselineShape = errors.New("BASELINE_NOT_WELL_FORMED")

var (
	objectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ReadBaseline reads a baseline document through one bounded read and returns it with the sha256
// of its bytes. Unknown members, another profile, a revision that is not a full object id, an
// entry without its evidence, a known entry without an isolated report, a duplicate identity or
// more than MaxBaselineEntries entries refuse (PWR-V0-041). Like ReadFile, it opens without
// blocking on a FIFO and reads only a regular file.
func ReadBaseline(name string) (Baseline, string, error) {
	f, err := openNonblocking(name)
	if err != nil {
		return Baseline{}, "", err
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return Baseline{}, "", ErrUnreadable
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxBaselineBytes+1))
	if err != nil {
		return Baseline{}, "", err
	}
	if len(raw) > MaxBaselineBytes {
		return Baseline{}, "", ErrTooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var b Baseline
	if dec.Decode(&b) != nil || dec.More() || b.Profile != BaselineProfile || !objectID.MatchString(b.Revision) ||
		b.Known == nil || b.Flaky == nil || len(b.Known)+len(b.Flaky)+len(b.Unobserved) > MaxBaselineEntries {
		return Baseline{}, "", ErrBaselineShape
	}
	seen := map[string]bool{}
	for i, list := range [][]BaselineEntry{b.Known, b.Flaky} {
		for _, e := range list {
			if e.File == "" || len(e.TitlePath) == 0 || seen[e.Key()] || e.Full.Path == "" || !sha256Re.MatchString(e.Full.Sha256) ||
				(i == 0) != (e.Isolated != nil) || (e.Isolated != nil && (e.Isolated.Path == "" || !sha256Re.MatchString(e.Isolated.Sha256))) ||
				!validIdentitySource(&e.Full, e.Isolated) {
				return Baseline{}, "", ErrBaselineShape
			}
			seen[e.Key()] = true
		}
	}
	for _, e := range b.Unobserved {
		if e.File == "" || len(e.TitlePath) == 0 || seen[e.Key()] || e.Full.Path == "" || !sha256Re.MatchString(e.Full.Sha256) ||
			(e.Reason != ReasonInterrupted && e.Reason != ReasonNotRun) || !validIdentitySource(&e.Full) {
			return Baseline{}, "", ErrBaselineShape
		}
		seen[e.Key()] = true
	}
	sum := sha256.Sum256(raw)
	return b, hex.EncodeToString(sum[:]), nil
}

// NamedRun is an admitted report with the path it was read from.
type NamedRun struct {
	Path string
	Run  *Run
}

// failing reports whether a test ran and failed: Playwright's unexpected status. A flaky test
// (failed, then passed on retry) is not failing here; RecordBaseline lists one seen in the full
// run as a suspected flake.
func failing(t Test) bool { return t.Executed && t.Status == "unexpected" }

func passing(t Test) bool { return t.Executed && t.Status == "expected" }

// firstFailure returns the first run, in order, where the identity failed unambiguously. An
// occurrence with an interrupted result is not failure evidence (PWR-V0-060).
func firstFailure(runs []NamedRun, key string, flakyToo bool) (Evidence, bool) {
	for _, nr := range runs {
		if nr.Run.Ambiguous[key] {
			continue
		}
		for _, t := range nr.Run.Tests {
			if t.Key() == key && !anyInterrupted(t) && (failing(t) || (flakyToo && t.Executed && t.Status == "flaky")) {
				return evidenceOf(nr), true
			}
		}
	}
	return Evidence{}, false
}

// RecordBaseline builds the baseline at revision from a full run and an isolated re-run of its
// failures (PWR-V0-041). A test is known only when it failed in the full run and in the isolated
// re-run; a test that failed (or was flaky) in the full run only is a suspected flake. Identities
// a report holds ambiguously are left out, so they are never excused. A known entry's full-run
// evidence is the first report that holds the test failing, so it does not depend on whether a
// report holding it flaky comes earlier.
func RecordBaseline(revision string, full, isolated []NamedRun) Baseline {
	b := Baseline{Profile: BaselineProfile, Revision: revision, Known: []BaselineEntry{}, Flaky: []BaselineEntry{}}
	seen := map[string]bool{}
	for _, nr := range full {
		for _, t := range nr.Run.Tests {
			key := t.Key()
			if seen[key] || anyInterrupted(t) || !(failing(t) || (t.Executed && t.Status == "flaky")) {
				continue
			}
			ev, ok := firstFailure(full, key, true)
			if !ok {
				continue // every occurrence is ambiguous
			}
			seen[key] = true
			e := BaselineEntry{Identity: t.Identity.presented(), Full: ev}
			if hardEv, hard := firstFailure(full, key, false); hard {
				if iso, ok := firstFailure(isolated, key, false); ok {
					e.Full = hardEv
					e.Isolated = &iso
					b.Known = append(b.Known, e)
					continue
				}
			}
			b.Flaky = append(b.Flaky, e)
		}
	}
	sortEntries(b.Known)
	sortEntries(b.Flaky)
	b.Unobserved = unobservedInFull(full, seen)
	return b
}

// unobservedInFull lists the unambiguous tests of the full run that no full-run report observed
// completing, skipping those already listed (PWR-V0-060). A test counts as observed when one
// occurrence executed with no interrupted result; nil when there is none.
func unobservedInFull(full []NamedRun, listed map[string]bool) []UnobservedEntry {
	observed := map[string]bool{}
	for _, nr := range full {
		for _, t := range nr.Run.Tests {
			if !nr.Run.Ambiguous[t.Key()] && t.Executed && !anyInterrupted(t) {
				observed[t.Key()] = true
			}
		}
	}
	var out []UnobservedEntry
	seen := map[string]bool{}
	for _, nr := range full {
		for _, t := range nr.Run.Tests {
			key := t.Key()
			if nr.Run.Ambiguous[key] || observed[key] || listed[key] || seen[key] {
				continue
			}
			seen[key] = true
			reason := ReasonInterrupted
			if !t.Executed {
				reason = ReasonNotRun
			}
			out = append(out, UnobservedEntry{Identity: t.Identity.presented(), Reason: reason, Full: evidenceOf(nr)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

func anyInterrupted(t Test) bool {
	for _, r := range t.Results {
		if r.Status == "interrupted" {
			return true
		}
	}
	return false
}

func sortEntries(list []BaselineEntry) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Key() < list[j].Key() })
}

// DiffTest is one test of the baseline diff.
type DiffTest struct {
	Identity
	Excused bool   `json:"excused"`
	Reason  string `json:"reason"`
}

// BaselineCounts are the set sizes of a baseline diff.
type BaselineCounts struct {
	New        int `json:"new"`
	Known      int `json:"known"`
	Excused    int `json:"excused"`
	Fixed      int `json:"fixed"`
	Flaky      int `json:"flaky"`
	Unobserved int `json:"unobserved"`
	Blocking   int `json:"blocking"`
	// UnobservedAtBaseline counts the baseline's unobserved tests (PWR-V0-061).
	UnobservedAtBaseline int `json:"unobservedAtBaseline,omitempty"`
}

// Staleness is the baseline's age from the branch point (PWR-V0-045).
type Staleness struct {
	Status      string `json:"status"` // FRESH | STALE | NOT_OBSERVED
	BranchPoint string `json:"branchPoint"`
	Commits     *int   `json:"commits"`
	MaxAge      int    `json:"maxAge"`
	Reason      string `json:"reason"`
	// MergeBase and the two side counts are present on BASELINE_NOT_ANCESTOR when Git names a merge
	// base of the baseline revision and the branch point: the commits from it to the baseline
	// revision and to the branch point (PWR-V0-062). Absent means not observed.
	MergeBase           string `json:"mergeBase,omitempty"`
	BaselineSideCommits *int   `json:"baselineSideCommits,omitempty"`
	BranchSideCommits   *int   `json:"branchSideCommits,omitempty"`
}

// BaselineDiff is the run-gate receipt (PWR-V0-044).
type BaselineDiff struct {
	Profile          string         `json:"profile"`
	Verdict          string         `json:"verdict"`
	Reasons          []string       `json:"reasons"`
	BaselineRevision string         `json:"baselineRevision"`
	BaselineSha256   string         `json:"baselineSha256"`
	Staleness        Staleness      `json:"staleness"`
	Runs             []Evidence     `json:"runs"`
	Counts           BaselineCounts `json:"counts"`
	New              []DiffTest     `json:"new"`
	Known            []DiffTest     `json:"known"`
	Flaky            []DiffTest     `json:"flaky"`
	Fixed            []Identity     `json:"fixed"`
	Unobserved       []Identity     `json:"unobserved"`
	// UnobservedAtBaseline lists the baseline's own unobserved tests: its full run never observed
	// them completing, so a candidate failure of one is new (UNOBSERVED_AT_BASELINE) (PWR-V0-061).
	UnobservedAtBaseline []Identity `json:"unobservedAtBaseline,omitempty"`
}

// EvidenceLoader re-reads a retained report by its recorded path; an error means it is missing
// or unreadable.
type EvidenceLoader func(path string) (*Run, error)

// verifyKnown re-reads both reports of a known entry: each must still exist with its recorded
// digest and still hold the test failing unambiguously (PWR-V0-042).
func verifyKnown(e BaselineEntry, load EvidenceLoader) string {
	if e.Isolated == nil {
		return ReasonEvidenceMissing
	}
	for _, ev := range []Evidence{e.Full, *e.Isolated} {
		run, err := load(ev.Path)
		if err != nil {
			return ReasonEvidenceMissing
		}
		if run.Sha256 != ev.Sha256 {
			return ReasonEvidenceChanged
		}
		if _, ok := firstFailure([]NamedRun{{Path: ev.Path, Run: run}}, e.Key(), false); !ok {
			return ReasonEvidencePassing
		}
	}
	return ""
}

// DiffBaseline gates candidate runs on a baseline (PWR-V0-042..044). Every unambiguous failing
// test is new, known or a suspected flake. A known failure is excused only when its evidence
// re-reads and own names no reason to block it (own returns "" for a file it does not hold, else
// the blocking reason, such as OWN_FILE); a suspected flake is never excused. Baseline
// entries that pass in the candidate are fixed, and those the candidate did not run are
// unobserved. A candidate report with top-level errors blocks.
func DiffBaseline(b Baseline, baselineSha string, candidate []NamedRun, own func(file string) string, load EvidenceLoader, staleness Staleness) BaselineDiff {
	d := BaselineDiff{Profile: BaselineDiffProfile, BaselineRevision: b.Revision, BaselineSha256: baselineSha, Staleness: staleness,
		Reasons: []string{}, Runs: []Evidence{}, New: []DiffTest{}, Known: []DiffTest{}, Flaky: []DiffTest{}, Fixed: []Identity{}, Unobserved: []Identity{}}
	known, flaky := map[string]BaselineEntry{}, map[string]bool{}
	for _, e := range b.Known {
		known[e.Key()] = e
	}
	for _, e := range b.Flaky {
		flaky[e.Key()] = true
	}
	unobservedAtBaseline := map[string]bool{}
	for _, e := range b.Unobserved {
		unobservedAtBaseline[e.Key()] = true
		d.UnobservedAtBaseline = append(d.UnobservedAtBaseline, e.Identity)
	}
	errorsSeen := false
	fail, pass := map[string]Identity{}, map[string]bool{}
	ambiguous := map[string]Identity{}
	for _, nr := range candidate {
		d.Runs = append(d.Runs, evidenceOf(nr))
		if nr.Run.ReportErrors > 0 {
			errorsSeen = true
		}
		for _, t := range nr.Run.Tests {
			key := t.Key()
			switch {
			case nr.Run.Ambiguous[key]:
				if failing(t) {
					ambiguous[key] = t.Identity.presented()
				}
			case failing(t):
				fail[key] = t.Identity.presented()
			case passing(t):
				pass[key] = true
			}
		}
	}
	// An identity any candidate report holds ambiguously while failing is blocking, even when
	// another report holds it failing unambiguously (PWR-V0-043).
	for key, id := range ambiguous {
		delete(fail, key)
		d.New = append(d.New, DiffTest{Identity: id, Reason: ReasonAmbiguous})
	}
	for key, id := range fail {
		switch e, isKnown := known[key]; {
		case isKnown:
			reason := verifyKnown(e, load)
			if reason == "" && own != nil {
				reason = own(id.File)
			}
			if reason == "" {
				d.Known = append(d.Known, DiffTest{Identity: id, Excused: true, Reason: ReasonKnownVerified})
			} else {
				d.Known = append(d.Known, DiffTest{Identity: id, Reason: reason})
			}
		case flaky[key]:
			d.Flaky = append(d.Flaky, DiffTest{Identity: id, Reason: ReasonSuspectedFlake})
		case unobservedAtBaseline[key]:
			d.New = append(d.New, DiffTest{Identity: id, Reason: ReasonUnobservedAtBaseline})
		default:
			d.New = append(d.New, DiffTest{Identity: id, Reason: ReasonNotInBaseline})
		}
	}
	for _, list := range [][]BaselineEntry{b.Known, b.Flaky} {
		for _, e := range list {
			key := e.Key()
			_, failed := fail[key]
			if _, amb := ambiguous[key]; failed || amb {
				continue
			}
			if pass[key] {
				d.Fixed = append(d.Fixed, e.Identity)
			} else {
				d.Unobserved = append(d.Unobserved, e.Identity)
			}
		}
	}
	byKey := func(list []DiffTest) {
		sort.Slice(list, func(i, j int) bool { return list[i].Key() < list[j].Key() })
	}
	byKey(d.New)
	byKey(d.Known)
	byKey(d.Flaky)
	sort.Slice(d.Fixed, func(i, j int) bool { return d.Fixed[i].Key() < d.Fixed[j].Key() })
	sort.Slice(d.Unobserved, func(i, j int) bool { return d.Unobserved[i].Key() < d.Unobserved[j].Key() })
	d.Counts = BaselineCounts{New: len(d.New), Known: len(d.Known), Fixed: len(d.Fixed), Flaky: len(d.Flaky), Unobserved: len(d.Unobserved),
		UnobservedAtBaseline: len(d.UnobservedAtBaseline)}
	for _, k := range d.Known {
		if k.Excused {
			d.Counts.Excused++
		}
	}
	d.Counts.Blocking = d.Counts.New + d.Counts.Flaky + d.Counts.Known - d.Counts.Excused
	d.Verdict = VerdictBaselinePass
	if errorsSeen {
		d.Reasons = append(d.Reasons, ReasonReportHasErrors)
	}
	if d.Counts.Blocking > 0 {
		d.Reasons = append(d.Reasons, "BLOCKING_FAILURES")
	}
	if len(d.Reasons) > 0 {
		d.Verdict = VerdictBaselineBlocked
	}
	return d
}
