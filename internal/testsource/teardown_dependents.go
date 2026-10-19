package testsource

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// `corvint tests teardown` (TSA-V0-110..119): for each resource an example creates and deletes in
// teardown, the actions it performs on the resource, and a TEARDOWN-DEPENDENT-LEFTOVER finding when
// a declared action leaves a dependent the teardown does not remove before the parent delete.

const (
	// TeardownSchema names the report.
	TeardownSchema = "corvint-test-teardown-dependents/0"
	// TeardownRule names the finding.
	TeardownRule = "TEARDOWN-DEPENDENT-LEFTOVER"
	// MaxTeardownReceipts bounds the receipts one report reads.
	MaxTeardownReceipts = 16

	basisDeclared = "declared"
	basisAdvisory = "advisory"

	causeNotRemoved      = "dependent-not-removed"
	causeParentFirst     = "parent-deleted-before-dependent"
	causeRecordedFailure = "recorded-teardown-conflict"

	reasonActionNotDeclared   = "action-not-declared"
	reasonNotBound            = "resource-not-bound"
	reasonHookNotRead         = "hook-not-read"
	reasonTeardownNotDeclared = "teardown-call-not-declared"
	reasonOrderUnknown        = "teardown-order-unknown"
	reasonCleanupConditional  = "cleanup-conditional"
	reasonFileUncertain       = "file-uncertain"

	tdVerdictLeftover = "leftover"
	tdVerdictUnknown  = "unknown"
	tdVerdictNone     = "no-leftover-declared"
)

// TeardownOptions selects the revision ("" reads HEAD), the repository-relative test files, the
// committed side-effect declaration and the receipts to join.
type TeardownOptions struct {
	Revision    string
	Files       []string
	Declaration string
	Receipts    []TeardownReceipt
}

// TeardownReceipt is one named receipt; JS is nil for a provider input that is not a JS receipt.
type TeardownReceipt struct {
	Name string
	JS   *jstestprovider.Receipt
}

// TeardownReport is the corvint-test-teardown-dependents/0 report.
type TeardownReport struct {
	Schema      string                   `json:"schema"`
	Revision    string                   `json:"revision"`
	GraphSource string                   `json:"graphSource"`
	Declaration TeardownDeclaration      `json:"declaration"`
	Files       []TeardownFile           `json:"files"`
	Findings    []TeardownFinding        `json:"findings"`
	Receipts    []TeardownReceiptBinding `json:"receipts"`
}

// TeardownDeclaration identifies the declaration the report used.
type TeardownDeclaration struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Version  int    `json:"version"`
	Revision string `json:"revision"`
}

// TeardownFile is one read test file and its examples that have a resource.
type TeardownFile struct {
	Path      string         `json:"path"`
	Framework string         `json:"framework,omitempty"`
	Uncertain bool           `json:"uncertain"`
	Reason    string         `json:"reason,omitempty"`
	Examples  int            `json:"examples"`
	Tests     []TeardownTest `json:"tests"`
}

// TeardownTest is one example and its resources.
type TeardownTest struct {
	Test      string             `json:"test"`
	Line      int                `json:"line"`
	Resources []TeardownResource `json:"resources"`
}

// TeardownResource is one resource kind an example creates and deletes in teardown. Verdict is
// leftover, unknown or no-leftover-declared, never safe (TSA-V0-118).
type TeardownResource struct {
	Kind       string           `json:"kind"`
	Variable   string           `json:"variable,omitempty"`
	CreateLine int              `json:"createLine"`
	DeleteLine int              `json:"deleteLine"`
	Actions    []TeardownAction `json:"actions"`
	Verdict    string           `json:"verdict"`
	Reasons    []string         `json:"reasons,omitempty"`
}

// TeardownAction is one action an example performs on a resource after creating it.
type TeardownAction struct {
	Call      string `json:"call"`
	Line      int    `json:"line"`
	Phase     string `json:"phase"`
	Declared  bool   `json:"declared"`
	Dependent string `json:"dependent,omitempty"`
	Cascade   bool   `json:"cascade,omitempty"`
}

// TeardownFinding is one TEARDOWN-DEPENDENT-LEFTOVER finding.
type TeardownFinding struct {
	Rule                string             `json:"rule"`
	Basis               string             `json:"basis"`
	Cause               string             `json:"cause"`
	Path                string             `json:"path"`
	Test                string             `json:"test"`
	Kind                string             `json:"kind"`
	Dependent           string             `json:"dependent,omitempty"`
	ActionLine          int                `json:"actionLine,omitempty"`
	DeleteLine          int                `json:"deleteLine"`
	DependentDeleteLine int                `json:"dependentDeleteLine,omitempty"`
	Reasons             []string           `json:"reasons,omitempty"`
	Recorded            []TeardownRecorded `json:"recorded,omitempty"`
	Message             string             `json:"message"`
}

// Declared counts the findings whose side-effect graph is complete.
func (r TeardownReport) Declared() int {
	n := 0
	for _, f := range r.Findings {
		if f.Basis == basisDeclared {
			n++
		}
	}
	return n
}

// TeardownDependents reads each named committed test file and the declaration at the revision and
// reports teardown dependents (TSA-V0-110..119). It reads committed Git content and the given
// receipts only.
func TeardownDependents(r Repo, options TeardownOptions) (TeardownReport, error) {
	files := []string{}
	seen := map[string]bool{}
	for _, name := range append([]string{options.Declaration}, options.Files...) {
		if !fs.ValidPath(name) || name == "." || name == ".git" || strings.HasPrefix(name, ".git/") || strings.ContainsAny(name, "\x00\n\r\\") {
			return TeardownReport{}, refuse("tests-invalid-arguments", "%q is not a repository-relative file path", name)
		}
	}
	for _, name := range options.Files {
		if !seen[name] {
			seen[name] = true
			files = append(files, name)
		}
	}
	switch {
	case len(files) == 0:
		return TeardownReport{}, refuse("tests-invalid-arguments", "no test file was named")
	case len(files) > MaxValidityFiles:
		return TeardownReport{}, refuse("tests-bound-exceeded", "more than %d files", MaxValidityFiles)
	case len(options.Receipts) > MaxTeardownReceipts:
		return TeardownReport{}, refuse("tests-bound-exceeded", "more than %d receipts", MaxTeardownReceipts)
	}
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return TeardownReport{}, err
	}
	data, found, err := r.readOne(commit, options.Declaration, MaxFileBytes)
	if err != nil {
		return TeardownReport{}, err
	}
	if !found {
		return TeardownReport{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", options.Declaration)
	}
	declaration, effects, err := decodeSideEffects(options.Declaration, data)
	if err != nil {
		return TeardownReport{}, err
	}
	report := TeardownReport{Schema: TeardownSchema, Revision: commit, GraphSource: "declaration",
		Declaration: TeardownDeclaration{Path: options.Declaration, SHA256: sha256Hex(data), Version: declaration.Version, Revision: declaration.Revision},
		Files:       []TeardownFile{}, Findings: []TeardownFinding{}, Receipts: []TeardownReceiptBinding{}}
	for _, name := range files {
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		if err != nil {
			return TeardownReport{}, err
		}
		if !found {
			return TeardownReport{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
		}
		var read tdFile
		switch {
		case strings.HasSuffix(name, "_spec.rb"):
			read = readRSpecTeardown(string(data), effects)
		case IsTestFile(name):
			read = readJSTeardown(name, string(data))
		default:
			report.Files = append(report.Files, TeardownFile{Path: name, Uncertain: true, Reason: "not-a-test-file", Tests: []TeardownTest{}})
			continue
		}
		rows, bindings := receiptRows(options.Receipts, name, sha256Hex(data))
		report.Receipts = append(report.Receipts, bindings...)
		file, findings := analyzeTeardown(name, read, effects, rows)
		report.Files = append(report.Files, file)
		report.Findings = append(report.Findings, findings...)
	}
	return report, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// analyzeTeardown reads every example of one file.
func analyzeTeardown(path string, read tdFile, effects sideEffects, rows []receiptRow) (TeardownFile, []TeardownFinding) {
	file := TeardownFile{Path: path, Framework: read.framework, Uncertain: read.uncertain, Reason: read.reason, Examples: len(read.examples), Tests: []TeardownTest{}}
	titles := map[string]int{}
	for _, ex := range read.examples {
		if ex.titled {
			titles[ex.title]++
		}
	}
	findings := []TeardownFinding{}
	for _, ex := range read.examples {
		joinable := ex.titled && titles[ex.title] == 1
		test, found := analyzeExample(path, read, ex, effects, rows, joinable)
		if len(test.Resources) > 0 {
			file.Tests = append(file.Tests, test)
		}
		findings = append(findings, found...)
	}
	return file, findings
}

// analyzeExample builds the resources of one example and their findings (TSA-V0-113..118).
func analyzeExample(path string, read tdFile, ex tdExample, effects sideEffects, rows []receiptRow, joinable bool) (TeardownTest, []TeardownFinding) {
	test := TeardownTest{Test: ex.title, Line: ex.line, Resources: []TeardownResource{}}
	findings := []TeardownFinding{}
	creates, parents := map[string]int{}, map[string][]tdCall{}
	kinds := []string{}
	for k, c := range ex.calls {
		if kind, ok := effects.creates[c.name]; ok && c.phase != phaseTeardown {
			if _, seen := creates[kind]; !seen {
				creates[kind] = k
				kinds = append(kinds, kind)
			}
		}
		if kind, ok := effects.deletes[c.name]; ok && c.phase == phaseTeardown {
			parents[kind] = append(parents[kind], c)
		}
	}
	hookUnread := ex.opaque
	for _, c := range ex.unread {
		hookUnread = hookUnread || effects.declared(c.name)
	}
	for _, kind := range kinds {
		deletes, ok := parents[kind]
		if !ok {
			continue
		}
		create, parent := ex.calls[creates[kind]], deletes[0]
		res := TeardownResource{Kind: kind, Variable: create.assigned, CreateLine: create.line, DeleteLine: parent.line, Actions: []TeardownAction{}}
		reasons := map[string]bool{}
		if create.assigned == "" {
			reasons[reasonNotBound] = true
		}
		if hookUnread {
			reasons[reasonHookNotRead] = true
		}
		if read.uncertain {
			reasons[reasonFileUncertain] = true
		}
		declared := []tdCall{}
		for _, c := range ex.calls {
			// A call that runs after every parent delete does not act on the live resource.
			live := runsBeforeAny(c, deletes)
			if c.phase == phaseTeardown && live && !effects.declared(c.name) {
				reasons[reasonTeardownNotDeclared] = true
				continue
			}
			if !live || c.seq <= create.seq || effects.creates[c.name] != "" || effects.deletes[c.name] != "" {
				continue
			}
			action, isAction := effects.actions[c.name]
			named := create.assigned != "" && c.mentions[create.assigned]
			switch {
			case isAction && action.Parent == kind:
				res.Actions = append(res.Actions, TeardownAction{Call: c.name, Line: c.line, Phase: c.phase, Declared: true, Dependent: action.Dependent, Cascade: action.Cascade})
				if action.Dependent != "" && !action.Cascade {
					declared = append(declared, c)
				}
			case isAction && action.Parent == "" && named:
				res.Actions = append(res.Actions, TeardownAction{Call: c.name, Line: c.line, Phase: c.phase, Declared: true})
			case !isAction && named && c.phase != phaseTeardown:
				res.Actions = append(res.Actions, TeardownAction{Call: c.name, Line: c.line, Phase: c.phase})
				reasons[reasonActionNotDeclared] = true
			}
		}
		mine := []TeardownFinding{}
		for _, action := range declared {
			dependent := effects.actions[action.name].Dependent
			// Every parent delete the action may precede is checked: choosing one would discard the
			// order uncertainty between same-group hooks.
			for _, p := range deletes {
				before, known := runsBefore(action, p)
				if known && !before {
					continue
				}
				cause, leftover, conditional := dependentLeftover(ex, action, p, dependent, effects)
				if !leftover {
					continue
				}
				if conditional {
					reasons[reasonCleanupConditional] = true
				}
				f := TeardownFinding{Rule: TeardownRule, Cause: cause, Path: path, Test: ex.title, Kind: kind, Dependent: dependent,
					ActionLine: action.line, DeleteLine: p.line}
				switch {
				case cause == reasonOrderUnknown || !known:
					reasons[reasonOrderUnknown] = true
					if cause == reasonOrderUnknown {
						f.Cause = causeParentFirst
					}
				case cause == causeParentFirst:
					f.DependentDeleteLine = firstDependentDelete(ex, action, dependent, effects)
				}
				mine = append(mine, f)
				break
			}
		}
		var recorded []TeardownRecorded
		if joinable {
			lines := map[int]bool{}
			for _, p := range deletes {
				if !lines[p.line] {
					lines[p.line] = true
					recorded = append(recorded, recordedAt(rows, ex.title, p.line, soleDependent(res.Actions))...)
				}
			}
		}
		if len(mine) == 0 && hasStatus(recorded) {
			mine = append(mine, TeardownFinding{Rule: TeardownRule, Cause: causeRecordedFailure, Path: path, Test: ex.title, Kind: kind,
				Dependent: soleDependent(res.Actions), DeleteLine: parent.line, Reasons: []string{}})
		}
		res.Reasons = sortedKeys(reasons)
		for k := range mine {
			f := &mine[k]
			f.Basis, f.Reasons, f.Recorded = basisDeclared, res.Reasons, recorded
			if len(res.Reasons) > 0 || f.Cause == causeRecordedFailure {
				f.Basis = basisAdvisory
			}
			f.Message = findingMessage(*f)
		}
		switch {
		case len(mine) > 0:
			res.Verdict = tdVerdictLeftover
		case len(res.Reasons) > 0:
			res.Verdict = tdVerdictUnknown
		default:
			res.Verdict = tdVerdictNone
		}
		test.Resources = append(test.Resources, res)
		findings = append(findings, mine...)
	}
	return test, findings
}

// runsBeforeAny reports whether c runs before, or in unknown order with, one of the parent deletes.
func runsBeforeAny(c tdCall, deletes []tdCall) bool {
	for _, p := range deletes {
		if before, known := runsBefore(c, p); before || !known {
			return true
		}
	}
	return false
}

// dependentLeftover reports the cause when a declared action's dependent is still present at the
// parent delete (TSA-V0-115, TSA-V0-116). Only a dependent delete known to run after the action and
// before the parent delete removes it, and only when it runs unconditionally; a conditional one
// leaves the finding with conditional set. An unknown order returns reasonOrderUnknown.
func dependentLeftover(ex tdExample, action, parent tdCall, dependent string, effects sideEffects) (cause string, leftover, conditional bool) {
	after, unknown := false, false
	for _, c := range ex.calls {
		if effects.deletes[c.name] != dependent || c.phase == phaseSetup {
			continue
		}
		follows, known := runsBefore(action, c)
		switch {
		case !known:
			unknown = true
			continue
		case !follows:
			continue
		}
		before, known := runsBefore(c, parent)
		switch {
		case !known:
			unknown = true
		case !before:
			after = true
		case c.conditional:
			conditional = true
		default:
			return "", false, false
		}
	}
	switch {
	case unknown:
		return reasonOrderUnknown, true, conditional
	case after:
		return causeParentFirst, true, conditional
	}
	return causeNotRemoved, true, conditional
}

// firstDependentDelete is the line of the first dependent delete that runs after the action.
func firstDependentDelete(ex tdExample, action tdCall, dependent string, effects sideEffects) int {
	for _, c := range ex.calls {
		if follows, known := runsBefore(action, c); known && follows && c.phase != phaseSetup && effects.deletes[c.name] == dependent {
			return c.line
		}
	}
	return 0
}

// soleDependent is the resource's one declared non-cascading dependent kind, or "".
func soleDependent(actions []TeardownAction) string {
	kinds := map[string]bool{}
	for _, a := range actions {
		if a.Dependent != "" && !a.Cascade {
			kinds[a.Dependent] = true
		}
	}
	if len(kinds) != 1 {
		return ""
	}
	return sortedKeys(kinds)[0]
}

func findingMessage(f TeardownFinding) string {
	var b strings.Builder
	switch f.Cause {
	case causeNotRemoved:
		fmt.Fprintf(&b, "line %d produces a %s dependent of the %s and no cleanup removes it before the %s delete at line %d", f.ActionLine, f.Dependent, f.Kind, f.Kind, f.DeleteLine)
	case causeParentFirst:
		fmt.Fprintf(&b, "line %d produces a %s dependent of the %s, but the %s delete at line %d may run before the dependent cleanup", f.ActionLine, f.Dependent, f.Kind, f.Kind, f.DeleteLine)
		if f.DependentDeleteLine > 0 {
			fmt.Fprintf(&b, " at line %d", f.DependentDeleteLine)
		}
	default:
		fmt.Fprintf(&b, "a recorded run refused the %s delete at line %d", f.Kind, f.DeleteLine)
	}
	if len(f.Reasons) > 0 {
		b.WriteString("; advisory: the side-effect graph is incomplete (" + strings.Join(f.Reasons, ", ") + ")")
	}
	return b.String()
}
