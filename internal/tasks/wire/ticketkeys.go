package wire

// TicketRecordKeys is the closed taskman-ticket/0 record key set (SPEC §3.1),
// in record order. Corvint Core's read-only planner imports it instead of
// keeping a copy.
var TicketRecordKeys = []string{
	"profile", "ticketId", "revision", "acceptanceRevision", "previousRecordSha256", "status",
	"archivedFrom", "title", "body", "kind", "owner", "milestone", "priority", "order", "labels",
	"dependencies", "acceptanceCriteria", "requirementRefs", "source", "effects", "capabilities",
	"requiredGates", "holds", "executionClass", "approvals", "completion", "dueDate",
	"estimateMinutes", "supersedes", "supersededBy", "shadowOverlay", "createdAt", "updatedAt",
	"updatedBy",
}

// TicketRecordOptionalKeys are the taskman-ticket/0 record keys a record may
// omit. The native codec and Core's read-only planner both admit exactly
// these, so a new optional key cannot reach one reader and not the other.
var TicketRecordOptionalKeys = []string{"requiresPool", "requiredRoles", "escalations", "operatorNote", "externalReviews", "executionPrerequisites", "attachedEvidence"}

// EscalationMaxCurrentOpen bounds the OPEN questions of a ticket's current
// acceptance revision (ESC-V0-002). Stale OPEN questions do not count toward
// it. The native codec and Core's read-only planner both enforce it.
const EscalationMaxCurrentOpen = 16

// Attached-evidence bounds (TEA-V0-001): at most AttachedEvidenceMaxEntries
// entries per ticket, 1..AttachedEvidenceMaxDigests digests per entry, and a
// nonblank reason of at most AttachedEvidenceMaxReasonBytes. The native codec
// and Core's read-only planner both enforce them.
const (
	AttachedEvidenceMaxEntries     = 32
	AttachedEvidenceMaxDigests     = 16
	AttachedEvidenceMaxReasonBytes = 512
)
