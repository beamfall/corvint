// Package roadmap joins the human-owned roadmap ticket store (`atm`) to
// current source evidence (docs/specs/REQUIREMENTS.tsv), current test
// receipts (a configured testvalidity.Projection receipts directory), and
// generated-doc state (a configured docs-state file) for the local dashboard
// snapshot (IPR-10, docs/plans/integrated-product-roadmap-2026-09-12.md).
//
// This join never mutates the planning store: it runs only `atm roadmap`
// and `atm ticket show`, both listed read verbs (AGENTS.md invariant 4).
// Every missing or refused input is an explicit NOT_OBSERVED with a reason;
// nothing here invents a passing receipt, a resolved requirement, or a ready
// doc (AGENTS.md invariant 2). Ticket completion still requires the owning
// accepted criteria/gate workflow — this package only reports what it
// observed.
package roadmap

import (
	"time"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Schema is this join's own wire profile. It is deliberately separate from
// corvint-dashboard-snapshot/0 (internal/dashboard/model): the roadmap join
// composes a different, less strictly typed input (a planning-store CLI)
// than the snapshot's git-repository/adapter pipeline, and folding it into
// that hashed, invariant-checked schema would require touching the snapshot
// compiler and its acceptance tests, which sit outside IPR-10's owned files.
const Schema = "corvint-dashboard-roadmap/0"

// ErrorProfile is this join's error envelope profile, in the same style as
// corvint-dashboard-error/0 (cmd/corvint-dashboard-snapshot/main.go).
const ErrorProfile = "corvint-dashboard-roadmap-error/0"

const notObserved = "NOT_OBSERVED"

// Blocker mirrors one `atm ticket show` blocker entry verbatim.
type Blocker struct {
	Code     string  `json:"code"`
	Detail   string  `json:"detail"`
	TicketID *string `json:"ticketId,omitempty"`
}

// EvidenceLink is one requirementRefs entry resolved against
// docs/specs/REQUIREMENTS.tsv. Resolved is false, and File/Line/Title stay
// empty, when the id has no row in that snapshot of the file — "unresolved"
// is reported explicitly rather than silently dropped. Reason is set when
// the table itself could not be read, so an unobserved table never reads
// as an ID the table lacks.
type EvidenceLink struct {
	RequirementID string `json:"requirementId"`
	File          string `json:"file,omitempty"`
	Line          string `json:"line,omitempty"`
	Title         string `json:"title,omitempty"`
	Resolved      bool   `json:"resolved"`
	Reason        string `json:"reason,omitempty"`
}

// requirementsNotObserved is an EvidenceLink's Reason when REQUIREMENTS.tsv
// could not be loaded.
const requirementsNotObserved = "requirements table not observed"

// TestReceipt is one requirement's current test-validity projection, read
// from the configured receipts directory. State is CURRENT when the
// receipt's inputIdentity matches the caller-supplied current tree digest,
// STALE when it does not, and NOT_OBSERVED (with Reason set) when no
// receipt could be read at all.
type TestReceipt struct {
	RequirementID string                   `json:"requirementId"`
	State         string                   `json:"state"`
	Reason        string                   `json:"reason,omitempty"`
	InputIdentity string                   `json:"inputIdentity,omitempty"`
	Projection    *testvalidity.Projection `json:"projection,omitempty"`
}

// DocState is one ticket's generated-doc state, read from the configured
// docs-state file (internal/mcp/docsbridge's READY / SOURCE_REDERIVED
// consume states plus the page's content hash). State is NOT_OBSERVED (with
// Reason set) when no docs-state file is configured or no entry exists.
type DocState struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// Ticket is one roadmap ticket joined to its evidence, receipts and doc
// state.
type Ticket struct {
	TicketID     string         `json:"ticketId"`
	LocalID      string         `json:"localId"`
	Title        string         `json:"title"`
	Milestone    string         `json:"milestone"`
	Status       string         `json:"status"`
	Eligibility  string         `json:"eligibility"`
	Owner        string         `json:"owner"`
	Priority     string         `json:"priority"`
	Blockers     []Blocker      `json:"blockers"`
	Evidence     []EvidenceLink `json:"evidence"`
	TestReceipts []TestReceipt  `json:"testReceipts"`
	DocState     DocState       `json:"docState"`
}

// MilestoneGroup is every ticket sharing one milestone, in first-observed
// order from `atm roadmap`.
type MilestoneGroup struct {
	Milestone string   `json:"milestone"`
	Tickets   []Ticket `json:"tickets"`
}

// Snapshot is the roadmap join's whole rendered result. Outcome is "OK" or
// NOT_OBSERVED (with Reason set) for the whole join — for example, `atm`
// refused or could not be run at all. A partial per-ticket failure does not
// abort the join; it shows up as that ticket's own NOT_OBSERVED blocker.
type Snapshot struct {
	Schema        string           `json:"schema"`
	GeneratedAt   string           `json:"generatedAt"`
	Outcome       string           `json:"outcome"`
	Reason        string           `json:"reason,omitempty"`
	WorktreeDirty bool             `json:"worktreeDirty"`
	Milestones    []MilestoneGroup `json:"milestones"`
}

// Options configures one Compile call.
type Options struct {
	AtmBinary string
	StoreRoot string
	// RepoRoot is the repository whose tree TreeDigest names; the dirty
	// check runs there, never in the planning store.
	RepoRoot        string
	RequirementsTSV string
	ReceiptsDir     string
	DocsStatePath   string
	TreeDigest      string
	GeneratedAt     string
	Timeout         time.Duration
}
