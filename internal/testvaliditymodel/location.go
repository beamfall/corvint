package testvaliditymodel

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/rubytestreceipt"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Location is where one projected test is declared (LPCV-V0-145): its repository-relative file,
// its declaring line and its title path, outermost group first. Each member is present only when
// the provider or a bound source reading knows it; otherwise Absent names it with one closed
// reason (LPCV-V0-146), so an absent member is never silent. IncludedAt lists an RSpec shared
// example's inclusion sites, innermost first, as the provider recorded them (LPCV-V0-148).
type Location struct {
	File       string            `json:"file,omitempty"`
	Line       int               `json:"line,omitempty"`
	TitlePath  []string          `json:"titlePath,omitempty"`
	IncludedAt []string          `json:"includedAt,omitempty"`
	Absent     map[string]string `json:"absent,omitempty"`
}

// The closed location absence reasons (LPCV-V0-146).
const (
	// LocationNotRecorded: the provider records no such member for this test.
	LocationNotRecorded = "not-recorded-by-provider"
	// LocationRootUnbound: the file is an absolute provider path and no repository root was given.
	LocationRootUnbound = "repository-root-unbound"
	// LocationFileUnbound: the anchor binds no single retained test file inside the root.
	LocationFileUnbound = "anchor-file-unbound"
	// LocationNotDeclaration: the provider's anchor may be a failure frame, not the declaration.
	LocationNotDeclaration = "anchor-not-declaration"
	// LocationSourceNotRead: no bound reading of the test's source was made.
	LocationSourceNotRead = "source-not-read"
	// LocationSourceUnplaced: the source was read but the test's declaration was not placed in it.
	LocationSourceUnplaced = "source-unplaced"
	// LocationTitleNotLiteral: a title on the path is not one literal string.
	LocationTitleNotLiteral = "title-not-literal"
	// LocationSharedExample: the test is a shared example; its title path depends on the inclusion.
	LocationSharedExample = "shared-example-definition"
	// LocationLineAmbiguous: more than one test is declared on the line, so the reading cannot say
	// which title path is this test's.
	LocationLineAmbiguous = "declaration-line-ambiguous"
	// LocationDeclarationAmbiguous: discovery found more than one bound declaration whose literal
	// title path is the test's full name (LPCV-V0-291).
	LocationDeclarationAmbiguous = "declaration-ambiguous"
)

// The location member names an Absent key may take.
const (
	LocationFile      = "file"
	LocationLine      = "line"
	LocationTitlePath = "titlePath"
)

func NewLocation() *Location { return &Location{Absent: map[string]string{}} }

func (l *Location) SetFile(file string) {
	l.File = file
	delete(l.Absent, LocationFile)
}

func (l *Location) setLine(line int) {
	l.Line = line
	delete(l.Absent, LocationLine)
}

// setTitlePath records a literal title path, or title-not-literal when there is none.
func (l *Location) setTitlePath(path []string) {
	if len(path) == 0 {
		l.TitlePath = nil
		l.Absent[LocationTitlePath] = LocationTitleNotLiteral
		return
	}
	l.TitlePath = append([]string{}, path...)
	delete(l.Absent, LocationTitlePath)
}

// PlaceSource records a bound source reading's declaration of one test: the file, the declaring
// line and, unless the test is a shared example or shares its line with another test, its title
// path.
func (l *Location) PlaceSource(file string, line int, source testvalidity.ExampleSource) {
	if l == nil {
		return
	}
	l.SetFile(file)
	l.setLine(line)
	switch {
	case l.Absent[LocationTitlePath] == LocationSharedExample:
	case source.Read == testvalidity.HygieneReasonJSLineAmbiguous:
		l.TitlePath = nil
		l.Absent[LocationTitlePath] = LocationLineAmbiguous
	default:
		l.setTitlePath(source.TitlePath)
	}
}

// goLocation is a Go test's location: the session event records no file or line, and a Go test
// has no title path beyond its name.
func goLocation() *Location {
	l := NewLocation()
	l.Absent[LocationFile] = LocationNotRecorded
	l.Absent[LocationLine] = LocationNotRecorded
	l.Absent[LocationTitlePath] = LocationNotRecorded
	return l
}

// jsDeclarationAnchor reports whether the outcome's anchor is the test's declaration: a profiled
// receipt's reporter takes test.location, and stock Playwright's is the spec's own location unless
// it failed, when the anchor is the failure frame. Vitest anchors are failure frames.
func JSDeclarationAnchor(receipt jstestreceipt.Receipt, outcome jstestreceipt.TestOutcome) bool {
	if outcome.Anchor == nil || outcome.Anchor.Line <= 0 {
		return false
	}
	return receipt.Profile != "" || receipt.Identity.RunnerName == "playwright" && outcome.FailureMessage == ""
}

// jsLocation is a JS/TS outcome's location before a root is known: the declaring line when the
// anchor is the declaration, the file pending a root, and no title path until source is read.
func jsLocation(receipt jstestreceipt.Receipt, outcome jstestreceipt.TestOutcome) *Location {
	l := NewLocation()
	l.Absent[LocationTitlePath] = LocationSourceNotRead
	switch {
	case outcome.Anchor == nil || outcome.Anchor.Line <= 0:
		l.Absent[LocationFile] = LocationNotRecorded
		l.Absent[LocationLine] = LocationNotRecorded
	case !JSDeclarationAnchor(receipt, outcome):
		l.Absent[LocationFile] = LocationNotDeclaration
		l.Absent[LocationLine] = LocationNotDeclaration
	default:
		l.Absent[LocationFile] = LocationRootUnbound
		l.setLine(outcome.Anchor.Line)
	}
	return l
}

// RubyLocation is an RSpec example's location: the provider's worktree-relative file and line,
// which for a shared example are its definition, with the inclusion sites the provider recorded.
func RubyLocation(document rubytestreceipt.Document, example rubytestreceipt.Example) *Location {
	l := NewLocation()
	l.Absent[LocationTitlePath] = LocationSourceNotRead
	if example.File != "" {
		l.SetFile(example.File)
	} else {
		l.Absent[LocationFile] = LocationNotRecorded
	}
	if example.Line > 0 {
		l.setLine(example.Line)
	} else {
		l.Absent[LocationLine] = LocationNotRecorded
	}
	_, _, through := InclusionSite(document, example)
	if example.SharedExample != nil || through {
		l.Absent[LocationTitlePath] = LocationSharedExample
	}
	if example.SharedExample != nil {
		l.IncludedAt = append([]string{}, example.SharedExample.IncludedAt...)
	}
	return l
}
