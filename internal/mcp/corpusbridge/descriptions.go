// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"maps"

	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
)

// TestingServerNote points every test-selection question away from the docs tools to the testing
// server (DMF-V1-009, R42 B2): no docs tool answers which tests to run. It is docsbridge's, so the
// two docs tool families say it in the same words.
const TestingServerNote = docsbridge.TestingServerNote

// corpusTrust is the trust statement every corpus tool carries.
const corpusTrust = "Read-only, source-revalidated experimental corpus evidence; generated authority and explicit limitations remain."

// toolDescriptions says, per corpus operation, what task the tool serves, what it returns and which
// other tool serves the nearest different task (DMF-V1-008). Each one is distinct; descriptions
// appends the trust statement and TestingServerNote.
var toolDescriptions = map[string]string{
	"info":            "Identify the loaded corpus: its schema, source revision and declared capabilities. Returns the corpus header and each capability's state. To verify the artifact against its sources, use corvint.docs_validate.",
	"validate":        "Verify the configured corpus artifact against its pinned sources. Returns the validation receipt and every failed check. For the header and capabilities alone, use corvint.docs_info.",
	"inventory":       "Page through every record the corpus holds (corpus /2 artifacts). Returns one page of inventory entries; use offset and limit to continue. To find records by meaning, use corvint.docs_search.",
	"search":          "Find documentation subjects by a free-text query. Returns ranked subject hits with anchors. For claims only, use corvint.docs_claims.",
	"get":             "Read one subject by its ID or legacy case ID. Returns the subject record with its links and anchors. To find an ID first, use corvint.docs_search.",
	"concept":         "Look up a concept by a text query. Returns the matching concept records with their anchors. For any kind of subject by ID, use corvint.docs_get.",
	"claims":          "List the claims about a path or paths, the claims a text query ranks, or supplied status claims by state, qualifier and run. Returns claims, each with its claim_kind and anchors, filterable by kind and exclude_kind. For subjects rather than claims, use corvint.docs_search.",
	"flow":            "Read one documented flow by ID, optionally narrowed by section and record kind. Returns the flow's sections and records. For its navigation structure, use corvint.docs_navigation.",
	"navigation":      "Show the navigation structure of one flow ID, or of the flows a query selects by name or linked test. Returns entry points, steps and linked tests. For the flow's documented content, use corvint.docs_flow.",
	"dependencies":    "List what one subject depends on. Returns its direct dependency relations with their evidence. To walk relations several levels deep, use corvint.docs_trace.",
	"trace":           "Walk the relations of one subject, following flow relations downstream to a depth. Returns the reachable subjects and the edges walked. For direct dependencies only, use corvint.docs_dependencies.",
	"related":         "List the subjects related to one subject. Returns related subjects with the relation that links each. For dependency edges only, use corvint.docs_dependencies.",
	"locate":          "Find where a repository path is documented, including subjects documenting it through one documents hop, or module and symbol subjects a query ranks. Returns documenting subjects with line anchors. For the claims about the path, use corvint.docs_claims.",
	"vocabulary":      "Look up corpus vocabulary terms by a text query. Returns the matching terms and the subjects using them. For subjects matching a free-text query, use corvint.docs_search.",
	"intent":          "Read the intent records of one subject, or every intent comparison when no ID is given. Returns intent records and their comparison with the documented behaviour. For producer gap claims, use corvint.docs_gaps.",
	"coverage":        "Count documented outcomes per coverage state across the corpus. Returns coverage counts per outcome. For the gap claims behind missing coverage, use corvint.docs_gaps.",
	"gaps":            "List producer gap claims, optionally for one subject ID and gap kinds. Returns gap claims with their kinds and anchors. For coverage counts, use corvint.docs_coverage.",
	"journey":         "Read one user journey by ID. Returns the journey's steps and the subjects it crosses. For one documented flow, use corvint.docs_flow.",
	"recommend-tests": "Being removed (DMF-V1-009): list the tests the corpus links to one path. For where the path is documented, use corvint.docs_locate.",
	"stability":       "Read the stability records of one subject by ID. Returns its stability records and their evidence. For coverage counts across the corpus, use corvint.docs_coverage.",
}

// toolDescription is the full description of one corpus operation's tool.
func toolDescription(op string) string {
	description := toolDescriptions[op] + " " + corpusTrust
	if op == "search" {
		description += " " + SearchIncludeDescription + " " + SearchReceiptDescription
	}
	return description + " " + TestingServerNote
}

// propertyDescriptions are the input property descriptions shared by every operation; an operation
// whose property means something narrower overrides it in operationPropertyDescriptions.
var propertyDescriptions = map[string]string{
	"limit":           "Maximum results to return (default 20).",
	"offset":          "Results to skip before the first returned one, for paging.",
	"retirement":      "Optional retirement policy: evaluated_at plus age or merge-distance bounds after which evidence counts as retired.",
	"caller_revision": "Optional full lowercase Git object ID of the caller's checkout, so freshness is judged against it.",
	"query":           "Free-text query.",
	"id":              "Subject ID.",
	"path":            "One repository-relative path; give path or paths, not both.",
	"paths":           "A batch of distinct repository-relative paths answered with one verification; give path or paths, not both.",
	"repository":      "Optional root-commit identity of a declared additional repository to restrict the answer to.",
	"kind":            "One kind, or a list of kinds, to keep.",
	"include":         "Extra record kinds to include.",
	"receipt":         "Receipt view: compact (default) or full.",
	"state":           "Supplied status state to filter claims by, instead of a path or query.",
	"qualifier":       "Optional supplied status qualifier; requires state.",
	"run":             "Optional run identifier; requires state.",
	"exclude_kind":    "One claim kind, or a list of kinds, to leave out.",
	"section":         "One flow section name, or a list of them, to keep.",
	"depth":           "How many relation levels to walk downstream (flow subjects).",
}

var operationPropertyDescriptions = map[string]map[string]string{
	"search":     {"include": `"claims" adds claim hits ranked with the subject hits; "capabilities" adds the corpus capabilities to a compact receipt.`, "kind": "One subject kind, or a list of kinds, to keep."},
	"claims":     {"query": "Free-text query ranking claims by their text; give one of path, paths, query or state.", "kind": "One claim kind, or a list of kinds, to keep."},
	"locate":     {"query": "Free-text query ranking module and symbol subjects; give path, paths or query.", "kind": "With query: module, symbol or both."},
	"navigation": {"query": "Free-text query selecting flows by name or linked test; give id or query.", "id": "Flow ID; give id or query."},
	"intent":     {"id": "Optional subject ID; without it every intent comparison is listed."},
	"gaps":       {"id": "Optional subject ID to restrict gap claims to.", "kind": "One gap kind, or a list of kinds, to keep."},
	"flow":       {"id": "Flow ID.", "include": "Record kinds of the flow to keep."},
	"concept":    {"query": "Free-text query naming the concept."},
	"vocabulary": {"query": "Free-text query naming the term."},
}

var retirementMemberDescriptions = map[string]string{
	"evaluated_at":       "RFC 3339 time the policy is evaluated at.",
	"max_age_seconds":    "Evidence older than this many seconds counts as retired.",
	"max_merge_distance": "Evidence more than this many merges behind counts as retired.",
	"merge_distance":     "The caller's merge distance from the evidence revision.",
	"source_revision":    "Revision the evidence was produced at.",
	"target_revision":    "Revision the caller compares against.",
}

// describeProperties sets a description on every input property of op, and on every member of the
// retirement object, copying each schema so no shared map is changed (DMF-V1-008).
func describeProperties(op string, properties map[string]any) {
	for name, raw := range properties {
		schema, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		schema = maps.Clone(schema)
		text := propertyDescriptions[name]
		if override, ok := operationPropertyDescriptions[op][name]; ok {
			text = override
		}
		schema["description"] = text
		if name == "retirement" {
			if members, ok := schema["properties"].(map[string]any); ok {
				described := map[string]any{}
				for member, value := range members {
					if m, ok := value.(map[string]any); ok {
						m = maps.Clone(m)
						m["description"] = retirementMemberDescriptions[member]
						value = m
					}
					described[member] = value
				}
				schema["properties"] = described
			}
		}
		properties[name] = schema
	}
}
