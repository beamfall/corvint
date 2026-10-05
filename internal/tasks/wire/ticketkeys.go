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
var TicketRecordOptionalKeys = []string{"requiresPool", "requiredRoles", "escalations", "operatorNote", "externalReviews", "executionPrerequisites"}

// EscalationMaxCurrentOpen bounds the OPEN questions of a ticket's current
// acceptance revision (ESC-V0-002). Stale OPEN questions do not count toward
// it. The native codec and Core's read-only planner both enforce it.
const EscalationMaxCurrentOpen = 16

// EscalationMaxGuidanceBytes bounds the encoded answer array delivered to a
// claim (ESC-V0-005). The writer refuses a proposal past it and the journal
// audit restates that refusal (ESC-V0-010).
const EscalationMaxGuidanceBytes = 256 * 1024
