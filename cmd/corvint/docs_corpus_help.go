package main

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/workspace"
)

// corpusOperation is one `docs corpus` operation: the options it admits and requires, in usage
// order, and its one-line purpose. parseCorpusInvocation and the corpus help topics read the same
// table (DCP-V1-056), so help cannot name an option the parser refuses.
type corpusOperation struct {
	name, purpose string
	required      []string
	optional      []string
}

// corpusOperations lists every `docs corpus` operation in help order.
var corpusOperations = buildCorpusOperations()

func buildCorpusOperations() []corpusOperation {
	reads := []string{"--limit", "--offset", "--retirement", "--caller-revision", "--verify-rederive", "--timings"}
	read := func(name, purpose string, required []string, optional ...string) corpusOperation {
		return corpusOperation{name: name, purpose: purpose, required: append([]string{"--artifact"}, required...), optional: append(optional, reads...)}
	}
	typedWith := func(name, purpose string, optional ...string) corpusOperation {
		return read(name, purpose, []string{"--" + doccorpus.OperationInput(name)}, optional...)
	}
	typed := func(name, purpose string) corpusOperation { return typedWith(name, purpose) }
	operations := []corpusOperation{
		{name: "manifest", purpose: "inventory one or more scopes at a full commit as a corpus manifest", required: []string{"--revision", "--scope", "--timestamp"}, optional: []string{"--schema"}},
		{name: "build", purpose: "build the immutable corpus artifact from a manifest, or rebuild an artifact from the manifest it embeds", optional: []string{"--manifest", "--artifact", "--max-unresolved-mention-rate", "--max-pattern-rate"}},
		{name: "render", purpose: "emit a deterministic filename-to-content documentation plan; writes nothing", required: []string{"--artifact"}},
		{name: "maintain", purpose: "preview, or with --apply write, generated blocks of one human page", required: []string{"--artifact", "--page"}, optional: []string{"--apply"}},
		{name: "cem", purpose: "emit CEM citation arguments and a corpus/CEM binding sidecar", required: []string{"--artifact", "--cem"}, optional: []string{"--id"}},
		{name: "cem-stable", purpose: "project claims through a verified cem/1.0 map; read-only, refuses any other outcome", required: []string{"--artifact", "--cem", "--expected-base", "--target", "--stable-artifact-root"}, optional: []string{"--id"}},
		{name: "behavior-adapter", purpose: "map a behavior-adapter request to a provider bundle, or --check it", required: []string{"--input"}, optional: []string{"--previous", "--check"}},
		{name: "behavior-provider", purpose: "build a corvint-corpus-behavior-provider/2 declaration from a request", required: []string{"--input"}},
		{name: "estate", purpose: "reconcile the repository inventories of several verified corpora by exact name (corvint-corpus-estate/1)", required: []string{"--input"}},
		{name: "compact-provider", purpose: "rewrite a corvint-corpus-adoption-provider/1 record or shard as its compact /2 encoding", required: []string{"--input"}},
		{name: "discover-playwright", purpose: "produce Playwright test discovery from a listing and migration", required: []string{"--migration", "--config", "--playwright-list"}, optional: []string{"--receipt"}},
		{name: "witness-playwright", purpose: "import qualified Playwright witnesses from a committed receipt and report", required: []string{"--input", "--receipt-input", "--report"}},
		{name: "discover-legacy", purpose: "inventory Jasmine or Mocha/Cypress legacy cases statically from Git blobs", required: []string{"--runner", "--revision", "--path"}, optional: []string{"--input"}},
		{name: "discover-tests", purpose: "read Playwright or RSpec examples, expectations, navigations, reaches and hooks statically from Git blobs", required: []string{"--runner", "--revision", "--path"}},
		{name: "extract", purpose: "extract generic subjects and relations with a repository-declared, or digest-pinned caller-owned, extractor profile from Git blobs", required: []string{"--profile", "--revision"}, optional: []string{"--profile-sha256", "--schema", "--repository"}},
		{name: "import-cases", purpose: "import manual legacy cases from a caller-exported TestRail JSON or CSV file", required: []string{"--format", "--input"}, optional: []string{"--content"}},
		{name: "search-cases", purpose: "search a caller-exported TestRail JSON or CSV file for legacy cases", required: []string{"--format", "--input"}, optional: []string{"--query", "--id", "--match", "--limit"}},
		read("info", "artifact identity, capabilities and freshness", nil),
		read("validate", "recheck the artifact against its original inputs", nil),
		read("inventory", "page through every subject, claim and relation ID", nil),
		read("search", "rank subjects by text, optionally of some kinds or with claim hits", []string{"--query"}, "--include", "--kind"),
		read("get", "one subject by ID", []string{"--id"}),
		read("trace", "source citations of one subject, by --id or by the file --path (in --repository)", nil, "--id", "--path", "--repository"),
		read("related", "subjects related to one subject", []string{"--id"}),
		read("journey", "one recorded journey and its observations", []string{"--id"}),
		read("stability", "repeated-run stability evidence of one subject", []string{"--id"}),
		read("locate", "subjects that cite one path", []string{"--path"}, "--repository"),
		read("coverage", "scoped coverage with its denominator", nil),
		read("gaps", "reported gaps, optionally for one subject, one file --path (in --repository) or some kinds", nil, "--id", "--path", "--repository", "--kind"),
		typed("concept", "typed subjects matching a concept"),
		read("claims", "typed claims about one path, claims ranked by --query text, or supplied status claims in one --state (exactly one), optionally only some --kind or without some --exclude-kind claim kinds", nil, "--path", "--query", "--state", "--qualifier", "--run", "--kind", "--exclude-kind"),
		read("flow", "one typed flow, optionally only some sections or record kinds", []string{"--id"}, "--section", "--include"),
		typed("dependencies", "typed dependency relations of one subject"),
		typedWith("recommend-tests", "tests related to one path; grants no test-selection authority", "--repository"),
		read("navigation", "typed navigation records of one flow --id, or of the flows a --query selects (exactly one)", nil, "--id", "--query"),
		typed("vocabulary", "typed vocabulary matching a term"),
		read("intent", "typed intent comparisons of one subject, or every intent comparison without --id", nil, "--id"),
		{name: "delta", purpose: "page a cross-revision worklist: carry-forward, moved, changed, removed, unresolved and uncovered; records-provider subjects, claims and relations, adoption kinds such as flows and paragraphs included, can carry forward", required: []string{"--previous"}, optional: []string{"--artifact", "--manifest", "--limit", "--offset", "--uncovered", "--anchorless", "--format", "--verify-rederive", "--timings"}},
		{name: "carry-forward", purpose: "emit a provider record of one provider's records whose span hashes still hold; adoption kinds keep their typed details in an adoption provider record", required: []string{"--previous", "--provider"}, optional: []string{"--artifact", "--manifest"}},
		{name: "eval", purpose: "score caller-labelled cases through the corpus MCP tool path; --baseline ratchets", required: []string{"--artifact", "--cases"}, optional: []string{"--baseline", "--format"}},
		{name: "audit-sample", purpose: "draw a reproducible claim sample to audit; also spelled audit sample", required: []string{"--artifact", "--seed", "--size"}, optional: []string{"--stratify", "--scope"}},
		{name: "audit-report", purpose: "measure sampled-claim accuracy from independent verdicts; also spelled audit report", required: []string{"--artifact", "--sample"}, optional: []string{"--format"}},
		{name: "patterns", purpose: "report templated text, anchor clusters, broad anchors, token-disjoint claims and uniform shapes", required: []string{"--artifact"}, optional: []string{"--format"}},
	}
	// DCP-V1-604: every operation takes the shared workspace declaration (WSD-V0) or its
	// --repo flag form.
	for i := range operations {
		operations[i].optional = append(operations[i].optional, corpusWorkspaceFlags...)
	}
	return operations
}

func lookupCorpusOperation(name string) (corpusOperation, bool) {
	for _, operation := range corpusOperations {
		if operation.name == name {
			return operation, true
		}
	}
	return corpusOperation{}, false
}

// admits reports whether the operation accepts flag.
func (operation corpusOperation) admits(flag string) bool {
	for _, known := range operation.required {
		if known == flag {
			return true
		}
	}
	for _, known := range operation.optional {
		if known == flag {
			return true
		}
	}
	return false
}

// corpusLocalInputFlags are the options whose value is a local input file: a canonical path
// relative to --root that sits under it (DCP-V1-054).
var corpusLocalInputFlags = []string{"--manifest", "--artifact", "--input", "--previous", "--cem", "--retirement", "--migration", "--playwright-list", "--receipt", "--report", "--cases", "--baseline", "--sample"}

var corpusFlagHelp = map[string][2]string{
	"--revision":                    {"FULL_COMMIT", "source revision the manifest pins"},
	"--scope":                       {"[ALIAS:]PATH", "repository path the manifest inventories; repeat for more disjoint scopes; ALIAS: inventories it in that workspace repository"},
	"--schema":                      {"1|2", "manifest schema: 1 (default, " + strconv.Itoa(doccorpus.MaxRecords) + " files) or 2 (" + strconv.Itoa(doccorpus.MaxCorpusRecords) + " files); for extract, 2 emits an adoption provider record for a manifest /2 (" + strconv.Itoa(doccorpus.MaxCorpusRecords) + " sites)"},
	"--timestamp":                   {"RFC3339", "manifest timestamp"},
	"--manifest":                    {"INPUT.json", "manifest from docs corpus manifest"},
	"--artifact":                    {"CORPUS.json", "corpus artifact from docs corpus build"},
	"--page":                        {"PAGE.md", "human page holding generated blocks"},
	"--apply":                       {"", "write the page; without it the change is only previewed"},
	"--cem":                         {"CEM.json", "change evidence map"},
	"--id":                          {"ID", "subject or claim ID"},
	"--input":                       {"REQUEST.json", "request file"},
	"--previous":                    {"RESULT.json", "previous result; the new result reports the delta from it"},
	"--check":                       {"", "validate the request and print a check report; emits no result"},
	"--migration":                   {"MIGRATION.json", "migration input"},
	"--config":                      {"PATH", "Playwright config file"},
	"--playwright-list":             {"LIST.json", "Playwright --list JSON output"},
	"--receipt":                     {"RECEIPT.json", "qualified Playwright receipt, read unchanged"},
	"--receipt-input":               {"ID", "REQUEST.json input naming the committed receipt; not a file"},
	"--report":                      {"REPORT.json", "caller-run Playwright JSON report"},
	"--format":                      {"testrail-json|csv", "test-management export format"},
	"--content":                     {"ids-paths-titles|full", "case text kept; full is local_only (default ids-paths-titles)"},
	"--query":                       {"TEXT", "search text"},
	"--path":                        {"PATH", "repository path"},
	"--repository":                  {"ROOT_COMMIT|ALIAS", "declared repository's root-commit identity or workspace alias; keeps only records anchored at the path there (extract: the workspace alias whose checkout is read)"},
	"--workspace":                   {"FILE", "workspace declaration (" + workspace.Profile + ") of the repositories: manifest inventories each alias beside --root; other operations check the corpus declares them"},
	"--no-workspace":                {"", "do not discover " + workspace.DiscoveryPath + " under --root"},
	"--repo":                        {"ALIAS=ROOT", "a further repository checkout, as a workspace alias would declare it; repeat for more"},
	"--state":                       {strings.Join(doccorpus.SuppliedStatusStates, "|"), "supplied status state; without --qualifier only unqualified claims"},
	"--qualifier":                   {strings.Join(doccorpus.SuppliedStatusQualifiers, "|"), "supplied status qualifier (requires --state)"},
	"--run":                         {"RUN", "keep claims whose subject a migration_mapping of this run names (requires --state)"},
	"--limit":                       {"N", "1.." + strconv.Itoa(doccorpus.MaxResults) + " results (default 20)"},
	"--offset":                      {"N", "results to skip"},
	"--caller-revision":             {"FULL_COMMIT", "caller's checkout commit; the trust envelope relates it to the source revision"},
	"--section":                     {"NAME[,NAME]", "flow sections whose paragraphs to return"},
	"--include":                     {"KIND[,KIND]", "flow record kinds: claims, relations, test_knowledge, paragraphs; search admits only claims, which adds claim hits to the subject hits, ranked together and labelled in result_kinds (the claims operation returns claims only)"},
	"--kind":                        {"KIND[,KIND]", "gaps: gap kinds to keep, each totalled before paging; search: subject kinds to keep"},
	"--retirement":                  {"POLICY.json", "retirement policy"},
	"--verify-rederive":             {"", "rederive the artifact from its pinned inputs before answering, as validate does"},
	"--timings":                     {"", "write the read phase durations to stderr; stdout is unchanged"},
	"--runner":                      {"jasmine|mocha", "legacy test runner"},
	"--expected-base":               {"FULL_COMMIT", "base the cem/1.0 map must bind"},
	"--target":                      {"FULL_COMMIT", "target commit holding the map as its sidecar"},
	"--stable-artifact-root":        {"DIR", "directory holding the map's referenced artifacts"},
	"--cases":                       {"CASES.json", "caller-owned " + corpusbridge.EvalCasesSchema + " case file"},
	"--baseline":                    {"RECEIPT.json", "earlier eval receipt; every regressed case is listed and exits 1"},
	"--seed":                        {"N", "sample seed, 0.." + strconv.FormatUint(doccorpus.MaxAuditSeed, 10) + "; the same seed, artifact and options give the same sample"},
	"--size":                        {"N", "claims to sample, 1.." + strconv.Itoa(doccorpus.MaxAuditSample)},
	"--stratify":                    {"provider|kind|flow|trust|patterns", "allocate the sample across strata: proportionally, or patterns over-samples flagged claims 4x"},
	"--max-unresolved-mention-rate": {"R", "refuse the build when claims with an unresolved code-like mention / claims with one exceeds R in [0, 1]"},
	"--max-pattern-rate":            {"R|NAME=R[,NAME=R]", "refuse the build when a corpus pattern's rate exceeds R in [0, 1]; bare R applies to every pattern"},
	"--sample":                      {"SAMPLE.json", doccorpus.AuditSampleSchema + " sample from docs corpus audit sample"},
	"--provider":                    {"PROVIDER", "records provider ID declared in the previous corpus manifest"},
	"--profile":                     {"PATH", "repository path of the " + doccorpus.ExtractorProfileSchema + " profile, read from Git at --revision; with --profile-sha256, the absolute path of a caller-owned profile outside the repository"},
	"--profile-sha256":              {"HEX", "SHA-256 of the caller-owned profile's exact bytes; a mismatch is refused, and the digest and canonical profile are recorded in the inventory"},
	"--uncovered":                   {"changed|all", "delta uncovered entries: changed regions only, or also uncited subjects (default all)"},
	"--anchorless":                  {"summary|list", "delta anchorless records: counted by provider and kind, or listed (default summary)"},
}

// corpusOperationFlagHelp overrides a shared option's help for one operation: eval's --format
// selects the report view, not a test-management export format.
var corpusOperationFlagHelp = map[string]map[string][2]string{
	"eval":           {"--format": {"json|table", "report view (default json)"}},
	"discover-tests": {"--runner": {"playwright|rspec", "test runner whose committed examples are read"}},
	"audit-sample":   {"--scope": {"PATH", "sample only claims citing this path or directory"}},
	"audit-report":   {"--format": {"json|table", "report view (default json)"}},
	"delta": {"--previous": {"CORPUS.json", "previous corpus artifact the worklist compares from"},
		"--artifact":   {"CORPUS.json", "current corpus artifact (or --manifest; exactly one)"},
		"--manifest":   {"INPUT.json", "current manifest, built in memory (or --artifact; exactly one)"},
		"--limit":      {"N", "1.." + strconv.Itoa(doccorpus.MaxResults) + " entries (default 20)"},
		"--uncovered":  {"changed|all", "all (default) also lists every uncited subject; changed lists changed regions only and counts the subjects"},
		"--anchorless": {"summary|list", "summary (default) counts anchorless records by provider and kind; list gives each its own entry"},
		"--format":     {"json|jsonl", "json (default) pages one receipt; jsonl streams a header line and every entry once, without --limit or --offset"}},
	"carry-forward": {"--previous": {"CORPUS.json", "previous corpus artifact whose records are carried"},
		"--artifact": {"CORPUS.json", "current corpus artifact (or --manifest; exactly one)"},
		"--manifest": {"INPUT.json", "current manifest, built in memory (or --artifact; exactly one)"}},
	"patterns": {"--format": {"json|table", "report view (default json)"}},
	"build": {"--manifest": {"INPUT.json", "manifest from docs corpus manifest (or --artifact; exactly one)"},
		"--artifact": {"CORPUS.json", "artifact to rebuild from its embedded manifest (or --manifest; exactly one)"}},
}

// corpusFlagHelpFor is the help entry of flag for operation.
func corpusFlagHelpFor(operation, flag string) [2]string {
	if entry, ok := corpusOperationFlagHelp[operation][flag]; ok {
		return entry
	}
	return corpusFlagHelp[flag]
}

func corpusOperationFlagUsage(operation, flag string) string {
	if metavar := corpusFlagHelpFor(operation, flag)[0]; metavar != "" {
		return flag + " " + metavar
	}
	return flag
}

const docsCorpusHelpTrailer = "Generated evidence grants no task or test-selection authority. Read operations print bounded\n" +
	"JSON and never collect tests.\n" +
	"Guide: docs/DOCUMENTATION-CORPUS.md; contract: docs/specs/documentation-corpus-v1.md\n"

// docsCorpusHelp is the `help docs corpus` topic: every operation with its one-line purpose.
func docsCorpusHelp() string {
	var text strings.Builder
	text.WriteString("Experimental documentation corpus profile\n\nUsage:\n  corvint [--root PATH] docs corpus OPERATION [OPTIONS]\n  corvint docs corpus OPERATION --help\n\nOperations:\n")
	for _, operation := range corpusOperations {
		text.WriteString("  " + operation.name + "  " + operation.purpose + "\n")
	}
	text.WriteString("\nLocal input files are canonical paths relative to --root that sit under it; absolute\n" +
		"paths are refused: " + strings.Join(corpusLocalInputFlags, ", ") + ".\n")
	text.WriteString(docsCorpusHelpTrailer)
	return text.String()
}

// docsCorpusOperationHelp is the `docs corpus OPERATION --help` topic: its usage, options and the
// options whose files must be relative to --root.
func docsCorpusOperationHelp(name string) string {
	operation, ok := lookupCorpusOperation(name)
	if !ok {
		panic("unknown corpus help topic")
	}
	usage := []string{"corvint [--root PATH] docs corpus " + operation.name}
	for _, flag := range operation.required {
		usage = append(usage, corpusOperationFlagUsage(name, flag))
	}
	for _, flag := range operation.optional {
		usage = append(usage, "["+corpusOperationFlagUsage(name, flag)+"]")
	}
	var text strings.Builder
	text.WriteString("Experimental documentation corpus: " + operation.purpose + "\n\nUsage:\n  " + strings.Join(usage, " ") + "\n\nOptions:\n")
	for _, flags := range [][]string{operation.required, operation.optional} {
		for _, flag := range flags {
			text.WriteString("  " + corpusOperationFlagUsage(name, flag) + "  " + corpusFlagHelpFor(name, flag)[1] + "\n")
		}
	}
	var relative []string
	for _, flag := range corpusLocalInputFlags {
		if operation.admits(flag) {
			relative = append(relative, flag)
		}
	}
	if len(relative) > 0 {
		text.WriteString("\nRelative to --root: " + strings.Join(relative, ", ") + ". Each is a canonical path that sits under\n" +
			"--root: no leading /, no .., no ./ or //. Absolute paths are refused (corpus-refused).\n")
	}
	text.WriteString(docsCorpusHelpTrailer)
	if name == "discover-playwright" || name == "witness-playwright" {
		text.WriteString("\n" + playwrightReceiptHelp)
	}
	if name == "witness-playwright" {
		text.WriteString("\n" + behaviorAnnotationHelp)
	}
	if name == "import-cases" || name == "search-cases" {
		text.WriteString("\n" + legacyCasesHelp)
	}
	if name == "build" {
		text.WriteString("\n" + corpusRebuildHelp)
	}
	return text.String()
}

// corpusRebuildHelp is the build topic's note on rebuilding a stale artifact (DCX-V1-495, DCX-V1-497, DCX-V1-532).
const corpusRebuildHelp = "Rebuild: a verified read refuses an artifact whose builder differs (`builder differs:`): the\n" +
	"derivation version (derivation-N in builder.engine), the builder schema, the Go toolchain in the\n" +
	"engine, or an unattested revision. Another attested revision with the same derivation is read as\n" +
	"is. A derivation version changes only with a merge that can alter the records built from the same\n" +
	"pinned inputs. --artifact rebuilds from the artifact's own embedded manifest: its content and\n" +
	"manifest digests are checked, every pinned input is reread from Git and must match its recorded\n" +
	"blob and SHA-256, and the new artifact goes to stdout. When an attested builder rebuilds an\n" +
	"artifact of its own derivation, it reuses the artifact's records instead of rederiving them and\n" +
	"prints the bytes a full build would. Write it to a new file and rename it over\n" +
	"the old one; redirecting onto the artifact being read truncates it first. Reads never rebuild.\n"

// corpusHelpTopic names the help topic of `docs corpus` arguments, or "" when the operation is
// unknown so the corpus parser refuses it.
func corpusHelpTopic(rest []string) string {
	if len(rest) == 0 || rest[0] == "--help" || rest[0] == "-h" {
		return "docs-corpus"
	}
	if op, ok := corpusAuditOperation(rest); ok {
		return "docs-corpus-" + op
	}
	if _, ok := lookupCorpusOperation(rest[0]); ok {
		return "docs-corpus-" + rest[0]
	}
	return ""
}
