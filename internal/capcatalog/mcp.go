// SPDX-License-Identifier: Apache-2.0
package capcatalog

// MCPTool is one offline discovery entry. Availability describes the server and
// opt-ins; it does not claim the configured server currently offers the tool.
// Query tools/list there for the authoritative descriptor and input schema.
type MCPTool struct {
	Name         string `json:"name"`
	Availability string `json:"availability"`
	Summary      string `json:"summary"`
}

// MCPTools returns the authored discovery inventory (CAP-V0-8800). The capability
// checker compares its names with the independently scanned MCP implementations.
func MCPTools() []MCPTool {
	return []MCPTool{
		{"corvint.cem.report", "corvint-mcp task-review, task-review-lsp", "Read a CEM report"},
		{"corvint.consolidate_tests", "corvint-corpus-mcp with --consolidation", "Test consolidation plan"},
		{"corvint.context", "corvint-mcp task-review, task-review-lsp", "Same packet as corvint context"},
		{"corvint.docs_claims", "corvint-corpus-mcp", "Claims about a path, claims ranked by text, or supplied status claims by state, qualifier and run; each claim carries a claim_kind, filterable by kind / exclude_kind, and run records rank below behavioural claims ([DCS-V1-591..593](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_concept", "corvint-corpus-mcp", "One concept's record"},
		{"corvint.docs_consume", "corvint-docs-mcp", "Recheck a draft's exact bytes against source"},
		{"corvint.docs_coverage", "corvint-corpus-mcp", "Coverage counts per outcome"},
		{"corvint.docs_dependencies", "corvint-corpus-mcp", "Dependencies of a subject"},
		{"corvint.docs_draft", "corvint-docs-mcp", "Source-pinned documentation draft of a module directory in any language the native index extracts declarations from; dry_run previews an untracked or inline owner as NOT_ADMISSIBLE (SDD-V0-019)"},
		{"corvint.docs_find_related", "corvint-corpus-mcp", "Related subjects"},
		{"corvint.docs_flow", "corvint-corpus-mcp", "One documented flow"},
		{"corvint.docs_gaps", "corvint-corpus-mcp", "Producer gap claims"},
		{"corvint.docs_get", "corvint-corpus-mcp", "One subject and its links, by ID or legacy case ID"},
		{"corvint.docs_get_journey", "corvint-corpus-mcp", "One user journey"},
		{"corvint.docs_get_stability", "corvint-corpus-mcp", "Stability records"},
		{"corvint.docs_info", "corvint-corpus-mcp", "Corpus header, revision and capabilities"},
		{"corvint.docs_intent", "corvint-corpus-mcp", "Intent records of one subject, or every intent comparison"},
		{"corvint.docs_inventory", "corvint-corpus-mcp (/2 artifacts)", "Page through the corpus inventory"},
		{"corvint.docs_locate", "corvint-corpus-mcp", "Where a path is documented, including the subjects documenting it through one documents hop, or module and symbol subjects ranked by a query, with line anchors ([DCS-V1-594](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_navigation", "corvint-corpus-mcp", "Navigation structure of one flow id, or of the flows a query selects by name or linked test ([DCS-V1-596..597](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_recommend_tests", "corvint-corpus-mcp", "Tests the corpus links to a subject, plus the subjects documenting the path and their test links ([DCS-V1-595](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_search", "corvint-corpus-mcp", "Search the corpus; compact receipt by default, receipt: \"full\" for the full one"},
		{"corvint.docs_search_cases", "corvint-corpus-mcp with --legacy-cases", "Search imported legacy test cases"},
		{"corvint.docs_trace", "corvint-corpus-mcp", "Walk relations to a depth"},
		{"corvint.docs_validate", "corvint-corpus-mcp", "Validate the corpus artifact"},
		{"corvint.docs_vocabulary", "corvint-corpus-mcp", "Corpus vocabulary"},
		{"corvint.flows.coverage", "corvint-mcp flows", "Flow variation coverage"},
		{"corvint.flows.gaps", "corvint-mcp flows", "Flow gaps"},
		{"corvint.flows.impact", "corvint-mcp flows", "Flows a change reaches"},
		{"corvint.flows.map", "corvint-mcp flows", "Application flow map"},
		{"corvint.flows.navigate", "corvint-mcp flows", "Navigation within the flow map"},
		{"corvint.graph.imports", "corvint-mcp --tool-profile import-graph", "Pinned whole-tree import JSONL pages; required full revision, optional kinds and offset; concatenate jsonl following nextOffset until done"},
		{"corvint.help", "corvint-mcp (all profiles), corvint-docs-mcp, corvint-corpus-mcp, corvint-test-validity-mcp", "Same bytes as corvint help guide (default), corvint help --json COMMAND (command) or corvint help --json (topic catalog)"},
		{"corvint.impact", "corvint-mcp (all profiles)", "Same receipt as corvint impact, path requests for Go, Ruby, JS and TS"},
		{"corvint.map_knowledge", "corvint-corpus-mcp with --map", "Site knowledge (page objects, steps, setup, static links, candidate flows) mined from committed E2E sources"},
		{"corvint.map_plan", "corvint-corpus-mcp with --map", "Scenario plan over an application map"},
		{"corvint.query", "corvint-mcp (all profiles)", "Same receipt as corvint query"},
		{"corvint.status", "corvint-mcp (all profiles)", "Repository status receipt"},
		{"corvint.tasks.queue_status", "corvint-mcp --tool-profile tasks", "Same envelope as corvint-tasks queue status"},
		{"corvint.tasks.receipt_audit", "corvint-mcp --tool-profile tasks", "Same envelope as corvint-tasks receipt audit"},
		{"corvint.tasks.roadmap", "corvint-mcp --tool-profile tasks", "One offset/limit page of corvint-tasks roadmap"},
		{"corvint.tasks.ticket_search", "corvint-mcp --tool-profile tasks", "One page of corvint-tasks ticket search by status, kind, priority, owner, milestone, label or text"},
		{"corvint.tasks.ticket_show", "corvint-mcp --tool-profile tasks", "Same envelope as corvint-tasks ticket show ID"},
		{"corvint.test_validity", "corvint-test-validity-mcp", "Five-axis projection of retained test evidence"},
	}
}
