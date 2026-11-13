// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The servers a table row names (MTT-V1-001, MTT-V1-004, MTT-V1-005).
const (
	// ServerCore is `corvint mcp`: the testing and flow-report tools.
	ServerCore = "corvint mcp"
	// ServerDocs is `corvint-docs mcp`: the documentation tools.
	ServerDocs = "corvint-docs mcp"
	// ServerTestValidity is the retiring standalone corvint-test-validity-mcp
	// binary, the only server left that answers corvint.test_validity; it
	// leaves the release list in its removal batch (MTT-V1-010).
	ServerTestValidity = "corvint-test-validity-mcp"
	// ServerNone marks a "no MCP tool" row: an answer deliberately served
	// only by a command line (MTT-V1-006).
	ServerNone = "none"
)

// The availability values a row may carry. A planned row names a tool no
// server registers yet; every other row on a real server is registered.
const (
	AvailabilityStable       = "stable"
	AvailabilityExperimental = "experimental"
	AvailabilityPreview      = "preview"
	AvailabilityDeprecated   = "deprecated"
	AvailabilityPlanned      = "planned"
)

// The typed tool-error codes every served tool lists (change docs-mcp-hardening, MTT-V1-001).
const (
	ErrorServerBusy = "server-busy"
	ErrorCancelled  = "cancelled"
)

// MCPTool is one row of the MCP tool table (MTT-V1-001), which also serves as
// the offline discovery inventory (CAP-V0-8800). Availability describes the
// row's maturity; it does not claim a configured server currently offers the
// tool. Query tools/list there for the authoritative descriptor and schema.
type MCPTool struct {
	Name   string `json:"name"`
	Server string `json:"server"`
	// AlsoOn lists the other servers that register the same tool from the
	// same code; the name stays one row (MTT-V1-001).
	AlsoOn       []string `json:"alsoOn,omitempty"`
	Availability string   `json:"availability"`
	// Summary is the row's description: what task the tool serves.
	Summary string `json:"summary"`
	// OptIn is the server argument that adds the tool, empty when the tool is
	// on the server's default surface.
	OptIn string `json:"optIn,omitempty"`
	// InputSchema is the source file that defines the registered input schema.
	InputSchema string `json:"inputSchema,omitempty"`
	// Inputs describes every input property of the registered schema.
	Inputs        []MCPInput `json:"inputs,omitempty"`
	OutputProfile string     `json:"outputProfile,omitempty"`
	// Errors is the closed list of tool-error codes the tool answers with
	// isError true; malformed parameters and an unknown tool are JSON-RPC
	// invalid-params errors instead.
	Errors []string `json:"errors,omitempty"`
	// CLIEquivalent is the command that returns the same records
	// (PROD-DOCS-010, PROD-TEST-009, MTT-V1-009); empty when none exists.
	CLIEquivalent string `json:"cliEquivalent,omitempty"`
	// ParityTest names the test that proves the tool and CLIEquivalent
	// return the same records, as "BINARY TestName", BINARY being the
	// binary's cmd/ directory name (MTT-V1-009). It names no repository path,
	// so the table reads no package it cites.
	ParityTest string `json:"parityTest,omitempty"`
	// Replaces lists the tool's earlier names (MTT-V1-007).
	Replaces []string `json:"replaces,omitempty"`
	// Note says why a row has no CLI equivalent, no server, or a planned or
	// deprecated availability.
	Note string `json:"note,omitempty"`
}

// MCPInput is one input property of a row's schema and its description.
type MCPInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Registered reports whether a server must register the row's tool: a row on
// a real server that is not planned (MTT-V1-003).
func (tool MCPTool) Registered() bool {
	return tool.Server != ServerNone && tool.Availability != AvailabilityPlanned
}

// MCPTools returns the authored MCP tool table (MTT-V1-001), sorted by name;
// each call returns a fresh copy. The capability checker compares its names
// with offline descriptors and the source inventory (CAP-V0-8800), and the
// drift tests compare each server's registered tools with its rows (MTT-V1-003).
func MCPTools() []MCPTool {
	rows := mcpToolRows()
	for index := range rows {
		// Rows share package-level slices; a caller must not reach them.
		rows[index].Inputs = slices.Clone(rows[index].Inputs)
		rows[index].Errors = slices.Clone(rows[index].Errors)
		rows[index].Replaces = slices.Clone(rows[index].Replaces)
		rows[index].AlsoOn = slices.Clone(rows[index].AlsoOn)
	}
	slices.SortFunc(rows, func(left, right MCPTool) int {
		switch {
		case left.Name < right.Name:
			return -1
		case left.Name > right.Name:
			return 1
		}
		return 0
	})
	return rows
}

// MCPToolsFor returns the rows one server must register, its own and those
// it also serves, sorted by name.
func MCPToolsFor(server string) []MCPTool {
	var rows []MCPTool
	for _, row := range MCPTools() {
		if (row.Server == server || slices.Contains(row.AlsoOn, server)) && row.Registered() {
			rows = append(rows, row)
		}
	}
	return rows
}

// toolName is the one naming convention, corvint.<area>_<verb> with exactly
// one dot (MTT-V1-007): a lower-case area word, then lower-case words joined
// by underscores.
var toolName = regexp.MustCompile(`^corvint\.[a-z]+(_[a-z]+)*$`)

// parityTest is a ParityTest value: a binary's cmd/ directory name and one
// top-level test function name.
var parityTest = regexp.MustCompile(`^corvint(-[a-z0-9]+)* Test[A-Za-z0-9_]+$`)

var (
	knownServers      = []string{ServerCore, ServerDocs, ServerTestValidity, ServerNone}
	knownAvailability = []string{AvailabilityStable, AvailabilityExperimental, AvailabilityPreview, AvailabilityDeprecated, AvailabilityPlanned}
)

// CheckMCPTools returns one problem per broken table rule (MTT-V1-001,
// MTT-V1-006, MTT-V1-007), each naming its row: a duplicate name across the
// servers, an unknown server or availability, a missing column, a summary two
// rows share, an input property without a description, a row without the
// shared error codes, a name off the convention, or a table without its
// "no MCP tool" row for Tasks.
func CheckMCPTools(rows []MCPTool) []string {
	var problems []string
	names, summaries := map[string]bool{}, map[string]string{}
	tasksRow := false
	for _, row := range rows {
		if names[row.Name] {
			problems = append(problems, fmt.Sprintf("%s: duplicate tool name", row.Name))
		}
		names[row.Name] = true
		if other, shared := summaries[row.Summary]; shared {
			problems = append(problems, fmt.Sprintf("%s: shares its description with %s", row.Name, other))
		} else {
			summaries[row.Summary] = row.Name
		}
		if !slices.Contains(knownServers, row.Server) {
			problems = append(problems, fmt.Sprintf("%s: unknown server %q", row.Name, row.Server))
		}
		for _, also := range row.AlsoOn {
			if also == row.Server || also == ServerNone || !slices.Contains(knownServers, also) {
				problems = append(problems, fmt.Sprintf("%s: also-on server %q is unknown, none or its own", row.Name, also))
			}
		}
		if !slices.Contains(knownAvailability, row.Availability) {
			problems = append(problems, fmt.Sprintf("%s: unknown availability %q", row.Name, row.Availability))
		}
		if strings.TrimSpace(row.Summary) == "" {
			problems = append(problems, row.Name+": missing description")
		}
		if row.Server == ServerNone {
			if strings.TrimSpace(row.Note) == "" || strings.TrimSpace(row.CLIEquivalent) == "" {
				problems = append(problems, row.Name+": a no-MCP-tool row needs its command and a note")
			}
			tasksRow = tasksRow || strings.Contains(row.Name, "tasks")
			continue
		}
		if strings.Contains(row.Name, "tasks") {
			problems = append(problems, row.Name+": a Tasks tool on an MCP server (MTT-V1-006)")
		}
		if !toolName.MatchString(row.Name) {
			problems = append(problems, row.Name+": name is not corvint.<area>_<verb> with one dot")
		}
		for _, code := range []string{ErrorServerBusy, ErrorCancelled} {
			if !slices.Contains(row.Errors, code) {
				problems = append(problems, fmt.Sprintf("%s: typed errors lack %s", row.Name, code))
			}
		}
		if strings.TrimSpace(row.OutputProfile) == "" {
			problems = append(problems, row.Name+": missing output profile")
		}
		if strings.TrimSpace(row.CLIEquivalent) == "" && strings.TrimSpace(row.Note) == "" {
			problems = append(problems, row.Name+": no CLI equivalent and no note saying why")
		}
		if row.Availability == AvailabilityPlanned || row.Availability == AvailabilityDeprecated {
			if strings.TrimSpace(row.Note) == "" {
				problems = append(problems, row.Name+": a "+row.Availability+" row needs a note")
			}
		}
		if row.Availability == AvailabilityPlanned {
			continue
		}
		switch {
		case row.CLIEquivalent != "" && row.ParityTest == "":
			problems = append(problems, row.Name+": has a CLI equivalent but no parity test (MTT-V1-009)")
		case row.ParityTest != "" && (row.CLIEquivalent == "" || !parityTest.MatchString(row.ParityTest)):
			problems = append(problems, fmt.Sprintf("%s: parity test %q needs a CLI equivalent and the form BINARY TestName", row.Name, row.ParityTest))
		}
		if strings.TrimSpace(row.InputSchema) == "" {
			problems = append(problems, row.Name+": missing input schema reference")
		}
		inputs := map[string]bool{}
		for _, input := range row.Inputs {
			if inputs[input.Name] || input.Name == "" {
				problems = append(problems, fmt.Sprintf("%s: input property %q is empty or repeated", row.Name, input.Name))
			}
			inputs[input.Name] = true
			if strings.TrimSpace(input.Description) == "" {
				problems = append(problems, fmt.Sprintf("%s: input property %s has no description", row.Name, input.Name))
			}
		}
	}
	for _, row := range rows {
		for _, old := range row.Replaces {
			if names[old] {
				problems = append(problems, fmt.Sprintf("%s: replaces %s, which is still a row", row.Name, old))
			}
		}
	}
	if !tasksRow {
		problems = append(problems, "the table has no no-MCP-tool row for Tasks (MTT-V1-006)")
	}
	return problems
}

func errorsOf(codes ...string) []string {
	return append([]string{ErrorCancelled, ErrorServerBusy}, codes...)
}

func in(name, description string) MCPInput { return MCPInput{Name: name, Description: description} }

const (
	collision     = "corvint-envelope-terminator-collision"
	bridgeSource  = "internal/mcp/bridge/bridge.go"
	flowsSource   = "internal/mcp/bridge/flows.go"
	corpusSource  = "internal/mcp/corpusbridge/corpusbridge.go"
	corpusProfile = "corvint-corpus-receipt/2"
	corpusOptIn   = "--corpus-artifact PATH"
	flowsOptIn    = "--tool-profile flows"
)

var (
	bridgeErrors = errorsOf("internal-error", "repository-unavailable", collision)
	flowsErrors  = errorsOf("flows-refused", "internal-error", "repository-unavailable", collision)
	corpusErrors = errorsOf("corpus-input-unavailable", "corpus-refused", "corpus-unavailable", "deadline-exceeded", "internal-error", collision)
	docsErrors   = errorsOf("documentation-limit-exceeded", "internal-error", "not-installed", "repository-unavailable", "stale-documentation-draft", "unsupported-documentation-source", collision)
)

// corpusInputs are the input properties every corpus tool takes, followed by
// the tool's own.
func corpusInputs(own ...MCPInput) []MCPInput {
	return append([]MCPInput{
		in("caller_revision", "Optional full commit id the caller works at; the receipt says whether the corpus was built at it."),
		in("limit", "Most records to return in this page, 1 to the corpus result bound."),
		in("offset", "Records to skip before this page, for a schema /2 artifact."),
		in("retirement", "Which retired records to include: the corpus retirement filter."),
	}, own...)
}

func corpusRow(name, summary, cli string, own ...MCPInput) MCPTool {
	return MCPTool{
		Name: name, Server: ServerDocs, Availability: AvailabilityExperimental, Summary: summary,
		OptIn: corpusOptIn, InputSchema: corpusSource, Inputs: corpusInputs(own...),
		OutputProfile: corpusProfile, Errors: corpusErrors,
		CLIEquivalent: "corvint-docs docs corpus " + cli + " --artifact PATH",
		ParityTest:    "corvint-docs TestCorpusTypedCLIMCPEndToEnd",
	}
}

var (
	corpusID    = in("id", "Subject id, or legacy case id, to read.")
	corpusQuery = in("query", "Text to rank records against.")
	corpusPath  = in("path", "One repository path or symbol to look up; give path or paths, not both.")
	corpusPaths = in("paths", "A batch of repository paths or symbols to look up; give path or paths, not both.")
)

// flowsRow is one flow-report row; name is the tool's earlier
// corvint.flows.<verb> name, which the row replaces (MTT-V1-007).
func flowsRow(name, verb, summary, profile string, inputs ...MCPInput) MCPTool {
	return MCPTool{
		Name: "corvint.flows_" + verb, Server: ServerCore, Availability: AvailabilityExperimental, Summary: summary,
		OptIn: flowsOptIn, InputSchema: flowsSource, Inputs: inputs, OutputProfile: profile, Errors: flowsErrors,
		CLIEquivalent: "corvint-docs flows " + verb, Replaces: []string{name},
		ParityTest: "corvint-docs TestMTTV1009FlowsToolsMatchCLI",
	}
}

var (
	flowsDir      = in("flows", "Repository-relative directory holding the flow intent files.")
	flowsEvidence = in("evidence", "Repository-relative test evidence files to join to the flows.")
	flowsRegistry = in("registry", "Repository-relative E2E test registry file.")
	flowsBase     = in("base", "Full commit id of the base; the change is base..HEAD.")
)

func mcpToolRows() []MCPTool {
	return []MCPTool{
		// corvint mcp: the testing and flow-report tools (MTT-V1-005).
		{
			Name: "corvint.help", Server: ServerCore, AlsoOn: []string{ServerDocs}, Availability: AvailabilityExperimental,
			Summary:     "Corvint's agent guide, one command's catalog entry, or the full command catalog, read from the binary; listed on both servers",
			InputSchema: "internal/mcp/bridge/help.go",
			Inputs: []MCPInput{
				in("command", "Command name whose catalog entry to return, with topic command."),
				in("topic", "guide (the default), command or catalog."),
			},
			OutputProfile: "corvint-mcp-help/0", Errors: errorsOf("internal-error"),
			CLIEquivalent: "corvint help guide | corvint help --json COMMAND | corvint help --json",
			ParityTest:    "corvint TestMTTV1009HelpToolMatchesCLI",
		},
		{
			Name: "corvint.impact", Server: ServerCore, Availability: AvailabilityExperimental,
			Summary:     "Revision-bound impact evidence for Go, Ruby, JavaScript and TypeScript paths, without source bodies",
			InputSchema: bridgeSource,
			Inputs: []MCPInput{
				in("budgetBytes", "Optional byte budget for the evidence packet."),
				in("limit", "Most ranked results to return, 1 to 50; default 10."),
				in("paths", "Repository-relative paths whose impact to compile."),
				in("snapshot", "Optional commit and tree binding the answer must match."),
			},
			OutputProfile: "corvint-mcp-bridge-result/0", Errors: bridgeErrors,
			CLIEquivalent: "corvint impact --language-profile non-go-syntax-v0 --limit N PATH...",
			ParityTest:    "corvint TestMTTV1009ImpactToolMatchesCLI",
		},
		{
			Name: "corvint.status", Server: ServerCore, Availability: AvailabilityExperimental,
			Summary:       "Exact commit, tree, worktree state and a privacy-preserving dirty-path digest of the served root",
			InputSchema:   bridgeSource,
			OutputProfile: "corvint-mcp-bridge-result/0", Errors: bridgeErrors,
			Note: "no command prints this receipt; corvint status does not exist",
		},
		{
			Name: "corvint.affected", Server: ServerCore, Availability: AvailabilityExperimental,
			Summary:     "Which tests a change requires: the affected-summary/1 verdict for a base or for the dirty worktree, or a select-all answer with its reason",
			OptIn:       "--tool-profile affected",
			InputSchema: "internal/mcp/toolgroup/affected.go",
			Inputs: []MCPInput{
				in("base", "Optional base revision; with it the plan is base..HEAD plus the dirty worktree, without it the dirty worktree alone."),
			},
			OutputProfile: "affected-summary/1, affected-select-all/1", Errors: errorsOf("affected-failed", "invalid-root", "not-served", "verdict-oversized"),
			CLIEquivalent: "corvint affected --summary [--base REV]",
			ParityTest:    "corvint TestV15254MCPAffectedMatchesCLI",
			Note:          "a planner refusal carries the CLI's own error code; an unknown CORVINT_SELECTION value and a stale binary select all tests where the CLI exits 2 (MTT-V1-008, MTT-V1-011)",
		},
		{
			Name: "corvint.test_evidence", Server: ServerCore, Availability: AvailabilityPreview,
			Summary:     "Retained test evidence for a path, a test, or a test's explanation against a base, read-only",
			OptIn:       "--tool-profile test-evidence",
			InputSchema: "internal/mcp/toolgroup/test_evidence.go",
			Inputs: []MCPInput{
				in("base", "Base commit for explain."),
				in("explain", "Test whose selection to explain against base."),
				in("index", "Optional repository-relative test inventory file."),
				in("lines", "Optional START:END line range within path."),
				in("path", "Repository-relative path whose tests to read."),
				in("revision", "Commit to read at; default HEAD."),
				in("test", "Test whose dependencies to read."),
			},
			OutputProfile: "corvint-test-evidence-query/1", Errors: errorsOf("internal-error", "invalid-root"),
			CLIEquivalent: "corvint test-evidence",
			ParityTest:    "corvint TestMTTV1009TestEvidenceToolMatchesCLI",
		},
		flowsRow("corvint.flows.coverage", "coverage", "Complete accepted flow variation coverage, bound to immutable bytes; the page verdict covers the full inventory", "application-flow-coverage/1",
			in("denominator", "Repository-relative accepted variation denominator file."),
			in("limit", "Most rows in this page, 1 to 100."),
			in("offset", "Rows to skip before this page."),
			in("receipts", "Repository-relative run receipts file."),
		),
		flowsRow("corvint.flows.gaps", "gaps", "Flows at HEAD without passing test evidence and, with a provider, the E2E tests no flow variation links", "application-flow-gaps/1",
			flowsEvidence, flowsDir,
			in("provider", "Repository-relative E2E inventory provider file."),
			flowsRegistry,
		),
		flowsRow("corvint.flows.impact", "impact", "The flows the base..HEAD changed paths reach through links and the impact graph", "application-flow-impact/1",
			flowsBase, flowsDir,
		),
		flowsRow("corvint.flows.map", "map", "Flow intents at HEAD with their links and test evidence, or the flows one path or test key reaches", "application-flow-map/1, application-flow-lookup/1",
			flowsEvidence, flowsDir,
			in("path", "Repository-relative path whose flows to look up."),
			flowsRegistry,
			in("testKey", "Test key whose flows to look up."),
		),
		flowsRow("corvint.flows.navigate", "navigate", "The navigation map of the flow intents, or the ordered packet that reaches one goal flow with each step above maxEffect marked requires-grant", "application-navigation-map/0, application-navigation-packet/0",
			flowsEvidence, flowsDir,
			in("goal", "Flow id of the goal whose packet to compile."),
			in("maxEffect", "Highest effect class a step may have without a grant: read, write-reversible, write-irreversible or external."),
			flowsRegistry,
			in("traffic", "Repository-relative recorded traffic files."),
		),
		flowsRow("corvint.flows.status", "status", "Each flow at HEAD, its variations, attached tests and the last recorded result of each, optionally only the flows a change reaches", "application-flow-status/0",
			in("base", "Optional full commit id of the base; only the flows base..HEAD reaches are reported."),
			flowsEvidence, flowsDir,
		),

		// corvint-docs mcp: the documentation tools (MTT-V1-004).
		corpusRow("corvint.docs_claims", "Claims about a path, claims ranked by text, or supplied status claims by state, qualifier and run, filterable by claim kind", "claims",
			in("exclude_kind", "Claim kind, or list of kinds, to leave out."),
			in("kind", "Claim kind, or list of kinds, to keep."),
			corpusPath, corpusPaths,
			in("qualifier", "Supplied status qualifier to filter by."),
			in("query", "Claim text to rank against, in place of path or paths."),
			in("run", "Supplied status run id to filter by."),
			in("state", "Supplied status state to filter by."),
		),
		corpusRow("corvint.docs_concept", "One concept's record, found by name", "concept", corpusQuery),
		corpusRow("corvint.docs_coverage", "Documentation coverage counts per outcome", "coverage"),
		corpusRow("corvint.docs_dependencies", "What one subject depends on", "dependencies", corpusID),
		corpusRow("corvint.docs_find_related", "Subjects related to one subject", "related", corpusID),
		corpusRow("corvint.docs_flow", "One documented flow, its sections and records", "flow",
			corpusID,
			in("include", "Flow record kinds to include."),
			in("section", "Flow section name, or list of names, to return."),
		),
		corpusRow("corvint.docs_gaps", "Producer gap claims, optionally for one subject or gap kind", "gaps",
			in("id", "Optional subject id whose gaps to return."),
			in("kind", "Gap kind, or list of kinds, to keep."),
		),
		corpusRow("corvint.docs_get", "One subject and its links, by id or legacy case id", "get", corpusID),
		corpusRow("corvint.docs_get_journey", "One user journey of a subject", "journey", corpusID),
		corpusRow("corvint.docs_get_stability", "Stability records of one subject", "stability", corpusID),
		corpusRow("corvint.docs_info", "The corpus header, revision and capabilities", "info"),
		corpusRow("corvint.docs_intent", "Intent records of one subject, or every intent comparison", "intent",
			in("id", "Optional subject id; without it every intent comparison is listed."),
		),
		corpusRow("corvint.docs_inventory", "A page of the corpus inventory of a schema /2 artifact", "inventory"),
		corpusRow("corvint.docs_locate", "Where a path is documented, or module and symbol subjects ranked by a query, with line anchors", "locate",
			in("kind", "With query: module or symbol, or both, to rank."),
			corpusPath, corpusPaths,
			in("query", "Text to rank module and symbol subjects against, in place of path or paths."),
			in("repository", "Optional root-commit identity of a declared repository the paths belong to."),
		),
		corpusRow("corvint.docs_navigation", "Navigation structure of one flow, or of the flows a query selects by name or linked test", "navigation",
			in("id", "Flow id whose navigation to return; give id or query."),
			in("query", "Text matched to flow names and linked tests; give id or query."),
		),
		corpusRow("corvint.docs_search", "Search the corpus: a compact receipt by default, the full one on request", "search",
			in("include", "claims adds claim hits to the subject hits; capabilities adds the corpus capabilities."),
			in("kind", "Subject kind, or list of kinds, to keep."),
			corpusQuery,
			in("receipt", "compact (the default) or full."),
		),
		corpusRow("corvint.docs_trace", "Relations walked from one subject to a depth", "trace",
			in("depth", "How many relation hops to walk."),
			corpusID,
		),
		corpusRow("corvint.docs_validate", "Validation of the corpus artifact itself", "validate"),
		corpusRow("corvint.docs_vocabulary", "Corpus vocabulary entries matching a term", "vocabulary", corpusQuery),
		{
			Name: "corvint.docs_draft", Server: ServerDocs, Availability: AvailabilityExperimental,
			Summary:     "A source-pinned documentation draft of a module directory; dry_run previews an untracked or inline owner as NOT_ADMISSIBLE",
			InputSchema: "internal/mcp/docsbridge/docsbridge.go",
			Inputs: []MCPInput{
				in("dry_run", "Preview over an untracked owner file or inline owner_markdown; the result governs nothing."),
				in("owner_markdown", "Inline owner Markdown, with dry_run."),
				in("package", "Repository-relative module directory to draft."),
				in("source", "Repository-relative owner Markdown source."),
			},
			OutputProfile: "corvint-mcp-docsbridge-result/0", Errors: docsErrors,
			CLIEquivalent: "corvint-docs docs draft --source PATH --package DIR",
			ParityTest:    "corvint-docs TestDocsToolVerbAnswersInOneEnvelope",
		},
		{
			Name: "corvint.docs_consume", Server: ServerDocs, Availability: AvailabilityExperimental,
			Summary:     "Recheck a draft's exact bytes against source and return the bounded task match",
			InputSchema: "internal/mcp/docsbridge/docsbridge.go",
			Inputs: []MCPInput{
				in("draft", "The draft bytes to recheck."),
				in("package", "Repository-relative module directory the draft covers."),
				in("source", "Repository-relative owner Markdown source."),
				in("task", "Task text to match against the draft."),
			},
			OutputProfile: "corvint-mcp-docsbridge-result/0", Errors: docsErrors,
			CLIEquivalent: "corvint-docs docs consume --source PATH --package DIR --task TEXT < DRAFT",
			ParityTest:    "corvint-docs TestMTTV1009DocsConsumeToolMatchesCLI",
		},
		{
			Name: "corvint.docs_how", Server: ServerDocs, Availability: AvailabilityPlanned,
			Summary:       "Answer a how-to question from the application map",
			OutputProfile: "site-knowledge-how/0", Errors: errorsOf(),
			CLIEquivalent: "corvint-docs how --json",
			Note:          "lands with change how-rewrite; no server registers it yet",
		},

		// The retiring standalone server and the deliberate CLI-only answers.
		{
			Name: "corvint.test_validity", Server: ServerTestValidity, Availability: AvailabilityDeprecated,
			Summary:     "Five-axis projection of retained test evidence",
			InputSchema: "internal/mcp/testvaliditybridge/testvaliditybridge.go",
			Inputs: []MCPInput{
				in("discover", "Discover retained receipts instead of naming one."),
				in("receipt", "Retained test receipt to project."),
			},
			OutputProfile: "corvint-mcp-test-validity-result/0", Errors: errorsOf("internal-error", "repository-unavailable"),
			Note: "removed from corvint mcp with the G24 batch (M12); served only by the retiring corvint-test-validity-mcp binary until its removal batch (MTT-V1-010), so no CLI parity is claimed for it",
		},
		{
			Name: "corvint-tasks", Server: ServerNone, Availability: AvailabilityStable,
			Summary:       "Corvint Tasks is used through its command line; neither server serves a Tasks tool",
			CLIEquivalent: "corvint-tasks",
			Note:          "no Tasks MCP server or tool ships in 1.0 (owner decision R17, MTT-V1-006)",
		},
	}
}
