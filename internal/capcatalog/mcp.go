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
// checker compares its names with offline descriptors and the source inventory.
func MCPTools() []MCPTool {
	return []MCPTool{
		{"corvint.cem.report", "corvint-mcp task-review, task-review-lsp", "Read a CEM report"},
		{"corvint.context", "corvint-mcp task-review, task-review-lsp", "Same packet as corvint context"},
		{"corvint.docs_claims", "corvint-mcp --tool-profile corpus", "Claims about a path, claims ranked by text, or supplied status claims by state, qualifier and run; each claim carries a claim_kind, filterable by kind / exclude_kind, and run records rank below behavioural claims ([DCS-V1-591..593](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_concept", "corvint-mcp --tool-profile corpus", "One concept's record"},
		{"corvint.docs_consume", "corvint-mcp --tool-profile docs", "Recheck a draft's exact bytes against source"},
		{"corvint.docs_coverage", "corvint-mcp --tool-profile corpus", "Coverage counts per outcome"},
		{"corvint.docs_dependencies", "corvint-mcp --tool-profile corpus", "Dependencies of a subject"},
		{"corvint.docs_draft", "corvint-mcp --tool-profile docs", "Source-pinned documentation draft of a module directory in any language the native index extracts declarations from; dry_run previews an untracked or inline owner as NOT_ADMISSIBLE (SDD-V0-019)"},
		{"corvint.docs_find_related", "corvint-mcp --tool-profile corpus", "Related subjects"},
		{"corvint.docs_flow", "corvint-mcp --tool-profile corpus", "One documented flow"},
		{"corvint.docs_gaps", "corvint-mcp --tool-profile corpus", "Producer gap claims"},
		{"corvint.docs_get", "corvint-mcp --tool-profile corpus", "One subject and its links, by ID or legacy case ID"},
		{"corvint.docs_get_journey", "corvint-mcp --tool-profile corpus", "One user journey"},
		{"corvint.docs_get_stability", "corvint-mcp --tool-profile corpus", "Stability records"},
		{"corvint.docs_info", "corvint-mcp --tool-profile corpus", "Corpus header, revision and capabilities"},
		{"corvint.docs_intent", "corvint-mcp --tool-profile corpus", "Intent records of one subject, or every intent comparison"},
		{"corvint.docs_inventory", "corvint-mcp --tool-profile corpus (/2 artifacts)", "Page through the corpus inventory"},
		{"corvint.docs_locate", "corvint-mcp --tool-profile corpus", "Where a path is documented, including the subjects documenting it through one documents hop, or module and symbol subjects ranked by a query, with line anchors ([DCS-V1-594](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_navigation", "corvint-mcp --tool-profile corpus", "Navigation structure of one flow id, or of the flows a query selects by name or linked test ([DCS-V1-596..597](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_recommend_tests", "corvint-mcp --tool-profile corpus", "Tests the corpus links to a subject, plus the subjects documenting the path and their test links ([DCS-V1-595](specs/documentation-corpus-search-ranking-v1.md))"},
		{"corvint.docs_search", "corvint-mcp --tool-profile corpus", "Search the corpus; compact receipt by default, receipt: \"full\" for the full one"},
		{"corvint.docs_trace", "corvint-mcp --tool-profile corpus", "Walk relations to a depth"},
		{"corvint.docs_validate", "corvint-mcp --tool-profile corpus", "Validate the corpus artifact"},
		{"corvint.docs_vocabulary", "corvint-mcp --tool-profile corpus", "Corpus vocabulary"},
		{"corvint.flows.coverage", "corvint-mcp flows", "Flow variation coverage"},
		{"corvint.flows.gaps", "corvint-mcp flows", "Flow gaps"},
		{"corvint.flows.impact", "corvint-mcp flows", "Flows a change reaches"},
		{"corvint.flows.map", "corvint-mcp flows", "Application flow map"},
		{"corvint.flows.navigate", "corvint-mcp flows", "Navigation within the flow map"},
		{"corvint.graph.imports", "corvint-mcp --tool-profile import-graph", "Pinned whole-tree import JSONL pages; required full revision, optional kinds and offset; concatenate jsonl following nextOffset until done"},
		{"corvint.help", "corvint-mcp (all profiles), corvint-test-validity-mcp", "Same bytes as corvint help guide (default), corvint help --json COMMAND (command) or corvint help --json (topic catalog)"},
		{"corvint.impact", "corvint-mcp (all profiles)", "Same receipt as corvint impact, path requests for Go, Ruby, JS and TS"},
		{"corvint.query", "corvint-mcp (all profiles)", "Same receipt as corvint query"},
		{"corvint.status", "corvint-mcp (all profiles)", "Repository status receipt"},
		{"corvint.test_evidence", "corvint-mcp --tool-profile test-evidence", "Same read-only path, range, test and explanation projection as corvint test-evidence"},
		{"corvint.test_validity", "corvint-test-validity-mcp", "Five-axis projection of retained test evidence"},
	}
}
