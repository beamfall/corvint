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
