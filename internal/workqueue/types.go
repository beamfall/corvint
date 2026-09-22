// Package workqueue validates Work Queue Observation V0 wire artifacts and
// computes deterministic, non-operative wave proposals.
package workqueue

import (
	"fmt"
)

const (
	SnapshotProfile = "work-queue-snapshot/0"
	EnvelopeProfile = "work-capacity-envelope/0"
	ProposalProfile = "work-wave-proposal/0"

	StateValidated  = "VALIDATED_AT"
	StatePartial    = "PARTIAL"
	StateConflicted = "CONFLICTED"
	StateStale      = "STALE"
	StateUnknown    = "UNKNOWN"

	UnknownAccessIncomplete               = "ACCESS_INCOMPLETE"
	UnknownAdapterInvalid                 = "ADAPTER_INVALID"
	UnknownCheckpointChanged              = "CHECKPOINT_CHANGED"
	UnknownCollisionClosureIncomplete     = "COLLISION_CLOSURE_INCOMPLETE"
	UnknownContainmentUnqualified         = "CONTAINMENT_UNQUALIFIED"
	UnknownDetailMissing                  = "DETAIL_MISSING"
	UnknownExecutableIdentityUnqualified  = "EXECUTABLE_IDENTITY_UNQUALIFIED"
	UnknownHostileInput                   = "HOSTILE_INPUT"
	UnknownInputLimit                     = "INPUT_LIMIT"
	UnknownMultiRepoUnsupported           = "MULTI_REPO_UNSUPPORTED"
	UnknownMutationDetected               = "MUTATION_DETECTED"
	UnknownMutationEnforcementUnqualified = "MUTATION_ENFORCEMENT_UNQUALIFIED"
	UnknownNetworkUnobserved              = "NETWORK_UNOBSERVED"
	UnknownProcessResidue                 = "PROCESS_RESIDUE"
	UnknownRepositoryDirty                = "REPOSITORY_DIRTY"
	UnknownRouteUnknown                   = "ROUTE_UNKNOWN"
	UnknownSourceUnqualified              = "SOURCE_UNQUALIFIED"
	UnknownReference                      = "UNKNOWN_REFERENCE"
)

var unknownCodeSet = map[string]struct{}{
	UnknownAccessIncomplete: {}, UnknownAdapterInvalid: {}, UnknownCheckpointChanged: {},
	UnknownCollisionClosureIncomplete: {}, UnknownContainmentUnqualified: {}, UnknownDetailMissing: {},
	UnknownExecutableIdentityUnqualified: {}, UnknownHostileInput: {}, UnknownInputLimit: {},
	UnknownMultiRepoUnsupported: {}, UnknownMutationDetected: {}, UnknownMutationEnforcementUnqualified: {},
	UnknownNetworkUnobserved: {}, UnknownProcessResidue: {}, UnknownRepositoryDirty: {},
	UnknownRouteUnknown: {}, UnknownSourceUnqualified: {}, UnknownReference: {},
}

const (
	CodeMalformedInput = "MALFORMED_INPUT"
	CodeInputLimit     = "INPUT_LIMIT"
	CodeHostileInput   = "HOSTILE_INPUT"
	CodeConflicted     = "CONFLICTED"
)

// Error is a bounded WQO structural or compatibility failure.
type Error struct {
	Code    string
	Message string
}

func (err *Error) Error() string {
	if err.Message == "" {
		return err.Code
	}
	return err.Code + ": " + err.Message
}

func fail(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Count is the decoded value of a WQO Count string.
type Count uint32

func (count Count) String() string { return fmt.Sprintf("%d", count) }

// Rank is the decoded value of a WQO Rank string.
type Rank uint32

func (rank Rank) String() string { return fmt.Sprintf("%d", rank) }

type RepositorySource struct {
	Commit                string
	ID                    string
	MaterializationSHA256 string
	ObjectFormat          string
	StatusSHA256          string
	Tree                  string
}

type Checkpoint struct {
	ID      string
	Version string
}

type Scope struct {
	Complete    bool
	ID          string
	TicketCount Count
}

type CapacityClass struct {
	AvailableUnits Count
	ID             string
}

type CapacityUse struct {
	ClassID string
	Units   Count
}

type RouteAlternative struct {
	ID       string
	Requires []string
}

type SelectionFacts struct {
	Approvals    string
	Dependencies string
	Holds        string
	Lease        string
}

type TicketSummary struct {
	AtomicRepositoryAuthorityIDs []string
	Authority                    string
	CapacityUses                 []CapacityUse
	CollisionGroupIDs            []string
	DeclaredVersion              string
	DependencyTicketIDs          []string
	DetailPayloadSHA256          *string
	Lifecycle                    string
	QueueAuthorityID             string
	Rank                         Rank
	RepositoryAuthorityID        string
	RouteAlternatives            []RouteAlternative
	SelectionFacts               SelectionFacts
	TicketContentSHA256          string
	TicketID                     string
	TicketVersionID              string
	TouchPaths                   []string
}

type LeaseSummary struct {
	BlocksSelection       bool
	CapacityUses          []CapacityUse
	CollisionGroupIDs     []string
	HolderID              string
	LeaseID               string
	LeaseVersionID        string
	Lifecycle             string
	QueueAuthorityID      string
	RepositoryAuthorityID string
	TicketID              string
	TicketVersionID       string
}

type CollisionGroup struct {
	ID              string
	MemberTicketIDs []string
	Path            *string
	Source          string
}

type ProposalEntry struct {
	CollisionGroupIDs  []string
	Reason             string
	RouteAlternativeID *string
	State              string
	TicketID           string
	TicketVersionID    string
}

// Snapshot is one parsed work-queue-snapshot/0. The proposal binding fields
// are caller-supplied observation metadata and are not snapshot wire members.
type Snapshot struct {
	AccessContextID               string
	CapacityClasses               []CapacityClass
	Checkpoint                    Checkpoint
	DetailRequestTicketVersionIDs []string
	ID                            string
	Leases                        []LeaseSummary
	PolicyID                      string
	Profile                       string
	QueueAuthorityID              string
	RepositoryAuthorityID         string
	RepositorySource              RepositorySource
	Scope                         Scope
	Tickets                       []TicketSummary

	ObservationID       string
	QueueSourceID       string
	ObservationState    string
	ObservationUnknowns []string
}

type CapacityEnvelope struct {
	Available             []CapacityClass
	Capabilities          []string
	ID                    string
	Profile               string
	RepositoryAuthorityID string
}

type Proposal struct {
	CapacityEnvelopeID string
	CollisionClosure   []CollisionGroup
	Entries            []ProposalEntry
	ID                 string
	MutationAuthority  bool
	ObservationID      string
	Profile            string
	QueueSourceID      string
	State              string
	Unknowns           []string
	WaveLimit          Count
	WaveOptimality     string
}

type ValidationResult struct {
	State    string
	Unknowns []string
}

type CollisionClosure struct {
	Groups         []CollisionGroup
	TicketGroupIDs map[string][]string
	Complete       bool
	State          string
	Unknowns       []string
}

// CollisionSource expands declared paths at one immutable repository commit.
type CollisionSource interface {
	Closure(paths []string) (closure []string, complete bool)
}
