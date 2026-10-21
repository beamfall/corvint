package intent

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Value-screen classes (KHN-V0-029). DENY_VALUE is the class of an
// adopter-declared value; the others are opt-in personal-data shapes.
const (
	ScreenClassDenyValue = "DENY_VALUE"
	ScreenClassEmail     = "EMAIL"
	ScreenClassPhone     = "PHONE"
	ScreenClassCard      = "CARD"
)

// ValueScreenPolicy is the KHN-V0-028 knowHow.screen declaration. The
// policy holds only the values file's repository-relative path, never a
// value; Classes is the sorted opt-in subset of EMAIL, PHONE and CARD.
type ValueScreenPolicy struct {
	DenyValuesFile *string
	Classes        []string
}

// ValueScreen returns the declared knowHow.screen, or nil.
func (p *Policy) ValueScreen() *ValueScreenPolicy {
	if p == nil || p.KnowHow == nil {
		return nil
	}
	return p.KnowHow.Screen
}

func readValueScreen(r *wire.Reader) *ValueScreenPolicy {
	r.Closed("denyValuesFile", "classes")
	s := &ValueScreenPolicy{}
	f := r.Field("denyValuesFile")
	if !f.IsNull() {
		path := f.Path()
		s.DenyValuesFile = &path
	}
	s.Classes = r.Field("classes").Strings(3, false, func(x *wire.Reader) string {
		return x.Enum(ScreenClassCard, ScreenClassEmail, ScreenClassPhone)
	})
	if r.Err() == nil && s.DenyValuesFile == nil && len(s.Classes) == 0 {
		r.Fail(wire.CodeMalformed, "screen declares neither a denyValuesFile nor a class")
	}
	return s
}

// WorktreePolicy is the TWT-V0-001 per-ticket worktree declaration.
// Repository is "." (the primary worktree), an absolute path, or a path
// relative to the primary worktree; Base is the ref new branches start
// from; BranchPattern holds {ticket} exactly once.
type WorktreePolicy struct {
	Repository    string
	Base          string
	BranchPattern string
}

// WorktreeTicketToken is the one placeholder a branch pattern holds.
const WorktreeTicketToken = "{ticket}"

var (
	worktreeRefText     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,254}$`)
	worktreePatternText = regexp.MustCompile(`^[A-Za-z0-9._/{}-]{1,200}$`)
)

func readWorktrees(r *wire.Reader) *WorktreePolicy {
	r.Closed("repository", "base", "branchPattern")
	w := &WorktreePolicy{}
	repo := r.Field("repository")
	switch s := repo.String(); {
	case r.Err() != nil:
	case s == ".":
		w.Repository = s
	case strings.HasPrefix(s, "/"):
		w.Repository = repo.PathText()
	default:
		w.Repository = repo.Path()
	}
	base := r.Field("base")
	w.Base = base.String()
	if r.Err() == nil && (!worktreeRefText.MatchString(w.Base) || strings.Contains(w.Base, "..")) {
		base.Fail(wire.CodeMalformed, "base must be a Git ref name of 1..255 bytes")
	}
	pattern := r.Field("branchPattern")
	w.BranchPattern = pattern.String()
	if r.Err() == nil && (!worktreePatternText.MatchString(w.BranchPattern) || strings.Count(w.BranchPattern, WorktreeTicketToken) != 1 ||
		strings.Count(w.BranchPattern, "{") != 1 || strings.Contains(w.BranchPattern, "..") || strings.HasPrefix(w.BranchPattern, "-")) {
		pattern.Fail(wire.CodeMalformed, "branchPattern must hold %s exactly once and otherwise only [A-Za-z0-9._/-]", WorktreeTicketToken)
	}
	return w
}

// DefaultDefectPrefix is the defect-id prefix while policy declares none
// (TDR-V0-002).
const DefaultDefectPrefix = "DEF"

var defectPrefixText = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,15}$`)

// DefectPolicy is the TDR-V0-001 defect-register declaration. A nil Path
// keeps the private register under the Git common directory; a declared
// repository-relative Path places it in the working tree (TDR-V0-009).
type DefectPolicy struct {
	Prefix string
	Path   *string
}

// DefectPrefix returns the declared defect-id prefix or the default.
func (p *Policy) DefectPrefix() string {
	if p != nil && p.Defects != nil && p.Defects.Prefix != "" {
		return p.Defects.Prefix
	}
	return DefaultDefectPrefix
}

func readDefects(r *wire.Reader) *DefectPolicy {
	r.Closed(wire.OptionalKeys(r.Value(), nil, "prefix", "path")...)
	d := &DefectPolicy{}
	if r.Err() == nil && wire.Has(r.Value(), "prefix") {
		pr := r.Field("prefix")
		d.Prefix = pr.String()
		if r.Err() == nil && !defectPrefixText.MatchString(d.Prefix) {
			pr.Fail(wire.CodeMalformed, "prefix must match [A-Z][A-Z0-9]{0,15}")
		}
	}
	if r.Err() == nil && wire.Has(r.Value(), "path") {
		if f := r.Field("path"); !f.IsNull() {
			path := f.Path()
			d.Path = &path
		}
	}
	return d
}

// MaxSharedReservationPaths bounds reservations.sharedPaths (NTP-V0-026),
// matching the `plan batches` --shared-path bound.
const MaxSharedReservationPaths = 64

// ReservationPolicy is the NTP-V0-026 reservation declaration. SharedPaths
// are canonical-sorted PATH keys (a trailing "/" names a directory prefix)
// of derived files every lane regenerates: two live attempts whose
// overlapping PATH keys are both covered by one of them do not collide.
type ReservationPolicy struct {
	SharedPaths []string
}

// SharedReservationPaths returns the declared reservations.sharedPaths, or
// nil when the policy declares none.
func (p *Policy) SharedReservationPaths() []string {
	if p == nil || p.Reservations == nil {
		return nil
	}
	return p.Reservations.SharedPaths
}

func readReservations(r *wire.Reader) *ReservationPolicy {
	r.Closed("sharedPaths")
	paths := r.Field("sharedPaths")
	out := &ReservationPolicy{SharedPaths: paths.Strings(MaxSharedReservationPaths, false, func(x *wire.Reader) string { return x.Path() })}
	if r.Err() == nil && len(out.SharedPaths) == 0 {
		paths.Fail(wire.CodeMalformed, "sharedPaths must name at least one path")
	}
	return out
}
