package capcatalog

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const syntheticRoot = `Tool

Usage:
  tool COMMAND

Commands:
  Core (decision 1; only what FRZ-V1 lists is frozen):
    alpha  first verb
  Experimental, no stability promise; owning spec in parentheses:
    beta thing  second verb (BET-V0)

Global options:
  --root PATH  Root.
  --version    Version (--json: the tool-version/1 identity).

Support boundary:
  Local-only. alpha and beta read without mutating repository
  state. beta best-effort append a bounded row to the ledger. gamma write local
  state (COMMAND --help).
`

func syntheticRegistry() Registry {
	return Registry{
		RootHelp: syntheticRoot,
		Commands: []string{"alpha", "beta", "gamma"},
		Topics: map[string]string{
			"alpha": "Alpha.\n\nUsage:\n  corvint [--root PATH] alpha run --mode M [--json]\n\nWrites alpha-out/2 or path/alpha-out/3. It does not mutate state. Exit 0 or 2 on error.\n",
			"beta":  "Beta.\n\nUsage:\n  tool beta --x X\n",
			"gamma": "Gamma.\n",
		},
		Operations:       map[string][]string{"alpha": {"stop"}},
		NestedOperations: map[string]map[string][]string{"alpha": {"run": {"b", "a"}}},
		NestedHelpTopics: []string{"alpha run"},
	}
}

func TestCAPV0020BuildIsDeterministicAndDerivedFromItsArgument(t *testing.T) {
	t.Parallel()
	first, err := Encode(Build(syntheticRegistry()))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Encode(Build(syntheticRegistry()))
	if !bytes.Equal(first, second) || !bytes.HasSuffix(first, []byte("}\n")) {
		t.Fatal("encoding is not deterministic or lacks its terminal LF")
	}
	catalog := Build(syntheticRegistry())
	alpha, beta := catalog.Commands[0], catalog.Commands[1]
	if *alpha.Purpose != "first verb" || *alpha.Maturity != "core" || *alpha.FreezeContract != "FRZ-V1" || alpha.OwningSpec != nil {
		t.Fatalf("alpha root entry = %+v", alpha)
	}
	if strings.Join(alpha.Operations, ",") != "run,stop" || strings.Join(alpha.NestedOperations["run"], ",") != "a,b" {
		t.Fatalf("alpha operations = %v %v", alpha.Operations, alpha.NestedOperations)
	}
	if strings.Join(alpha.Flags, ",") != "--json,--mode" || strings.Join(alpha.SchemaIDs, ",") != "alpha-out/2" {
		t.Fatalf("alpha flags %v schemas %v", alpha.Flags, alpha.SchemaIDs)
	}
	if strings.Join(intStrings(alpha.ExitCodesMentioned), ",") != "0,2" || *alpha.Writes != "none" || len(alpha.WriteStatements) != 2 {
		t.Fatalf("alpha exit %v writes %v statements %q", alpha.ExitCodesMentioned, alpha.Writes, alpha.WriteStatements)
	}
	if *beta.OwningSpec != "BET-V0" || *beta.Maturity != "experimental" || beta.SelfObservationLedger != "best-effort-append" {
		t.Fatalf("beta = %+v", beta)
	}
	if *catalog.Commands[2].Writes != "local-state" {
		t.Fatalf("gamma writes = %v", catalog.Commands[2].Writes)
	}
	if strings.Join(catalog.GlobalFlags, ",") != "--json,--root,--version" || strings.Join(catalog.GlobalSchemaIDs, ",") != "tool-version/1" {
		t.Fatalf("global flags %v schemas %v", catalog.GlobalFlags, catalog.GlobalSchemaIDs)
	}
}

func TestCAPV0016UnknownFieldsAreNullWithReasons(t *testing.T) {
	t.Parallel()
	gamma := Build(syntheticRegistry()).Commands[2]
	want := []string{"purpose", "maturity", "owningSpec", "inputs", "outputs", "exitCodesMentioned"}
	var got []string
	for _, unknown := range gamma.Unknowns {
		if unknown.Reason == "" {
			t.Fatalf("unknown %s has no reason", unknown.Field)
		}
		got = append(got, unknown.Field)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") || gamma.Purpose != nil || gamma.Inputs != nil || gamma.Outputs != nil {
		t.Fatalf("gamma unknowns = %v (%+v)", got, gamma)
	}
	registry := syntheticRegistry()
	registry.RootHelp = strings.Replace(registry.RootHelp, "gamma write local", "delta write local", 1)
	if gamma := Build(registry).Commands[2]; gamma.Writes != nil || gamma.Unknowns[len(gamma.Unknowns)-1].Field != "writes" {
		t.Fatalf("unclassified writes must be unknown: %+v", gamma)
	}
}

func TestCAPV0018GuideProblemsRejectsDrift(t *testing.T) {
	t.Parallel()
	catalog := Build(syntheticRegistry())
	clean := "Goals\n  corvint alpha run --mode M > out.json\n  corvint --root /repo alpha run --mode=M\n  corvint --version --json\n  `corvint help guide`\n  `corvint help alpha run`\nOutput: alpha-out/2 and tool-version/1.\n\nNon-capabilities\n\n- No network. By design (NET-V0-001).\n- No beta mode. Tracking\n  #12.\n"
	if problems := GuideProblems(catalog, clean); len(problems) != 0 {
		t.Fatalf("clean guide problems: %v", problems)
	}
	for _, drift := range []string{
		"  corvint omega\n",
		"  corvint alpha jump\n",
		"  corvint alpha run --mode M --speed 2\n",
		"  corvint alpha run c --mode M\n",
		"Output: alpha-out/9.\n",
		"  `corvint help omega`\n",
		"  corvint --root /repo omega\n",
		"  corvint --root=/repo alpha jump --mode M\n",
		"  corvint --root\n",
		"  `corvint help alpha nope`\n",
		"  `corvint help alpha run extra`\n",
		"  corvint alpha run\n",
	} {
		if problems := GuideProblems(catalog, drift+clean); len(problems) != 1 {
			t.Fatalf("drift %q problems = %v", drift, problems)
		}
	}
	if problems := GuideProblems(catalog, clean+"- No gamma.\n"); len(problems) != 1 {
		t.Fatalf("untracked non-capability problems = %v", problems)
	}
}

func TestCAPV0017RequiredFlagsAreDerivedFromUsageForms(t *testing.T) {
	t.Parallel()
	registry := syntheticRegistry()
	registry.Commands = append(registry.Commands, "delta")
	registry.Operations["delta"] = []string{"show"}
	registry.Topics["delta"] = "Delta.\n\nUsage:\n" +
		"  corvint [--root PATH] delta show --a A --b B\n    --c C [--d D] (--e E | --f F)\n" +
		"  corvint [--root PATH] delta show --a A --c C --g G\n" +
		"  corvint delta --x X | --y Y\n\nExamples:\n  corvint delta show --z Z\n"
	catalog := Build(registry)
	delta := catalog.Commands[3]
	if len(delta.RequiredFlags) != 1 || strings.Join(delta.RequiredFlags["delta show"], ",") != "--a,--c" {
		t.Fatalf("delta required flags = %v", delta.RequiredFlags)
	}
	if strings.Join(catalog.Commands[0].RequiredFlags["alpha run"], ",") != "--mode" || strings.Join(catalog.NestedHelpTopics, ",") != "alpha run" {
		t.Fatalf("alpha required flags %v, nested help topics %v", catalog.Commands[0].RequiredFlags, catalog.NestedHelpTopics)
	}
}

// The shipped catalog requires the verification options of cem verify, so the
// guide's example must carry them (gh#1590 review).
func TestCAPV0017GuideExamplesCarryRequiredOptions(t *testing.T) {
	t.Parallel()
	var catalog Catalog
	if err := json.Unmarshal(Snapshot(), &catalog); err != nil {
		t.Fatal(err)
	}
	if problems := GuideProblems(catalog, Guide()); len(problems) != 0 {
		t.Fatalf("shipped guide problems: %v", problems)
	}
	problems := GuideProblems(catalog, "  corvint cem verify --map .corvint/change.cem.json\n"+Guide())
	if len(problems) != 3 || !strings.Contains(strings.Join(problems, "\n"), "required option --expected-base of cem verify") {
		t.Fatalf("a cem verify example with only --map: problems = %v", problems)
	}
	// CAP-V0-018 against the shipped catalog: the --root preamble and nested
	// help topics are checked, not skipped.
	for _, drift := range []string{"  corvint --root /repo nonexistent-command\n", "  `corvint help docs nonexistent-operation`\n"} {
		if problems := GuideProblems(catalog, drift+Guide()); len(problems) != 1 {
			t.Fatalf("drift %q problems = %v", drift, problems)
		}
	}
	if problems := GuideProblems(catalog, "  `corvint help docs corpus`\n  `corvint help cem verify`\n  corvint --root /repo witness --base FULL\n"+Guide()); len(problems) != 0 {
		t.Fatalf("catalogued nested topics and a --root preamble: problems = %v", problems)
	}
}

func TestCAPV0017ShippedGuideDocumentsThePlaywrightTwoStep(t *testing.T) {
	t.Parallel()
	guide := Guide()
	discovery := strings.Index(guide, "corvint affected discovery --playwright-config")
	plan := strings.Index(guide, "corvint affected --playwright-config playwright.config.ts --playwright-discovery")
	if discovery < 0 || plan < discovery || !strings.Contains(guide, "playwright test --list --reporter=json") || !strings.Contains(guide, "\nNon-capabilities\n") {
		t.Fatal("the guide must document affected discovery --playwright-list before affected --playwright-discovery, and list non-capabilities")
	}
}

// AFP-V0-431: the guide's test-selection goal tells callers to batch changed paths into one call
// and to pass --cache DIR on a large tree, since every uncached call builds the graph again.
// AFP-V0-436 amends the goal: the cache is on by default.
func TestAFPV0431ShippedGuideAdvisesBatchingAndTheAffectedCache(t *testing.T) {
	t.Parallel()
	guide := Guide()
	from := strings.Index(guide, "\n3. Select the tests to run for a branch\n")
	to := strings.Index(guide, "\n4. Select Playwright tests")
	if from < 0 || to < from {
		t.Fatal("the guide must keep its test-selection goal before the Playwright goal")
	}
	goal := guide[from:to]
	for _, want := range []string{"--path", "--paths-from FILE", "one call builds the graph", "--cache DIR", "outside\n   the repository", "unchanged checkout", "under the user cache directory", "--no-cache", "corvint affected cache"} {
		if !strings.Contains(goal, want) {
			t.Errorf("the test-selection goal does not say %q:\n%s", want, goal)
		}
	}
}

func intStrings(values []int) []string {
	texts := make([]string, len(values))
	for index, value := range values {
		texts[index] = string(rune('0' + value))
	}
	return texts
}

// CAP-V0-022: every catalog states, right after derivation, that requiredFlags
// is partial, and the derivation map says why.
func TestCAPV0022RequiredFlagsCoverageIsPartial(t *testing.T) {
	t.Parallel()
	catalog := Build(syntheticRegistry())
	if catalog.RequiredFlagsCoverage != RequiredFlagsPartial || !strings.Contains(catalog.Derivation["requiredFlagsCoverage"], "lower bound") {
		t.Fatalf("requiredFlagsCoverage = %q, derivation %q", catalog.RequiredFlagsCoverage, catalog.Derivation["requiredFlagsCoverage"])
	}
	encoded, err := Encode(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte("\n  },\n  \"requiredFlagsCoverage\": \"partial\",\n  \"globalFlags\": [")) {
		t.Fatal("requiredFlagsCoverage is not encoded between derivation and globalFlags")
	}
}

// CAP-V0-023: an option the help requires only in prose, or through
// alternative usage lines, is never derived: the form stays omitted and the
// catalog still says requiredFlags is partial.
func TestCAPV0023ProseRequirementsAreNotDerived(t *testing.T) {
	t.Parallel()
	registry := syntheticRegistry()
	registry.Topics["gamma"] = "Gamma.\n\nUsage:\n  corvint gamma --dry-run\n  corvint gamma --apply --digest D\n\n" +
		"Exactly one of --dry-run and --apply is required. --digest is required with --apply; --mode is required.\n"
	catalog := Build(registry)
	if gamma := catalog.Commands[2]; len(gamma.RequiredFlags) != 0 || catalog.RequiredFlagsCoverage != RequiredFlagsPartial {
		t.Fatalf("gamma requiredFlags = %v, coverage %q", gamma.RequiredFlags, catalog.RequiredFlagsCoverage)
	}
}

// CAP-V0-024: a non-capability must name an issue or a requirement ID; the
// words "By design" alone no longer pass.
func TestCAPV0024NonCapabilitiesNameAnIssueOrRequirement(t *testing.T) {
	t.Parallel()
	catalog := Build(syntheticRegistry())
	base := "Goals\n  corvint --version\n\nNon-capabilities\n\n"
	for item, want := range map[string]int{
		"- No network. By design.\n":                  1,
		"- No gamma mode.\n":                          1,
		"- No network. By design (NET-V0-001).\n":     0,
		"- No gamma mode. Tracking beamfall/x#7.\n":   0,
		"- No gamma mode. Tracking MCPV0-100.\n":      0,
		"- No gamma mode. Task V1-2458 is not one.\n": 1,
	} {
		if problems := GuideProblems(catalog, base+item); len(problems) != want {
			t.Fatalf("item %q problems = %v, want %d", item, problems, want)
		}
	}
	for _, item := range shippedNonCapabilities(t) {
		if len(nonCapabilityIDs(item)) == 0 {
			t.Fatalf("shipped non-capability names no issue or requirement: %s", item)
		}
	}
}

// CAP-V0-025: the checked-in status table parses, and malformed rows are
// refused rather than read as some state.
func TestCAPV0025StatusTableIsWellFormed(t *testing.T) {
	t.Parallel()
	if _, problems := parseNonCapabilityStatus(NonCapabilityStatus()); len(problems) != 0 {
		t.Fatalf("NONCAPABILITY-STATUS.tsv problems: %v", problems)
	}
	header := "id\tstate\tevidence\n"
	for _, bad := range []string{
		"id\tstate\n#1\topen\tx\n",
		header + "#1\tdelivered\tx\n",
		header + "CAP-V0-001\topen\tx\n",
		header + "V1-2458\tdelivered\tx\n",
		header + "#1\topen\n",
		header + "#1\topen\t \n",
		header + "#1\topen\tx\n#1\tclosed\ty\n",
		header + "\topen\tx\n",
	} {
		if _, problems := parseNonCapabilityStatus(bad); len(problems) != 1 {
			t.Fatalf("status %q problems = %v", bad, problems)
		}
	}
	states, problems := parseNonCapabilityStatus(header + "#1\topen\tx\n#2\tclosed\tx\nCAP-V0-001\tundelivered\tx\nCAP-V0-002\tdelivered\tx\nCAP-V0-003\texcludes\tx\n")
	if len(problems) != 0 || len(states) != 5 || states["CAP-V0-003"] != "excludes" {
		t.Fatalf("states %v problems %v", states, problems)
	}
}

// CAP-V0-026: an item naming a closed issue, a delivered requirement or an
// unrecorded ID fails, and so does a By design item without an excludes
// requirement or another item without an open issue or undelivered requirement.
func TestCAPV0026NonCapabilityProblemsRejectsStaleItems(t *testing.T) {
	t.Parallel()
	status := "id\tstate\tevidence\n#1\topen\tx\n#2\tclosed\tx\nCAP-V0-001\tundelivered\tx\nCAP-V0-002\tdelivered\tx\nCAP-V0-003\texcludes\tx\n"
	base := "Goals\n\nNon-capabilities\n\n"
	for item, want := range map[string]int{
		"- A. Tracking #1.\n":                                  0,
		"- A. Not yet (CAP-V0-001).\n":                         0,
		"- A. By design\n  (CAP-V0-003).\n":                    0,
		"- A. Tracking #2.\n":                                  2,
		"- A. Residual of #2, tracking #1.\n":                  1,
		"- A. Not yet (CAP-V0-002).\n":                         2,
		"- A. Tracking #3.\n":                                  2,
		"- A. Not yet (CAP-V0-009).\n":                         2,
		"- A. By design (CAP-V0-001).\n":                       1,
		"- A. By design, tracking #1.\n":                       1,
		"- A. Out of scope (CAP-V0-003).\n":                    1,
		"- A. By design (CAP-V0-003), delivered CAP-V0-002.\n": 1,
	} {
		if problems := NonCapabilityProblems(base+item, status); len(problems) != want {
			t.Fatalf("item %q problems = %v, want %d", item, problems, want)
		}
	}
	if problems := NonCapabilityProblems(base+"- A. Tracking #1.\n", "id\tstate\n"); len(problems) != 3 {
		t.Fatalf("a malformed status table must fail: %v", problems)
	}
}

// CAP-V0-026: the Non-capabilities parse fails closed. Every non-blank line
// after the one intro paragraph must be a "- " item or its two-space
// continuation, so another bullet style, an indented or numbered item, or
// stray prose is reported instead of bypassing the status checks.
func TestCAPV0026UnrecognisedNonCapabilityLinesFailClosed(t *testing.T) {
	t.Parallel()
	status := "id\tstate\tevidence\n#1\topen\tx\n#2\tclosed\tx\n"
	catalog := Build(syntheticRegistry())
	base := "Goals\n  corvint --version\n\nNon-capabilities\n\nEach item is a capability\nCorvint lacks.\n\n- A. Tracking\n  #1.\n"
	if problems := NonCapabilityProblems(base, status); len(problems) != 0 {
		t.Fatalf("intro, item and continuation problems = %v", problems)
	}
	if problems := GuideProblems(catalog, base); len(problems) != 0 {
		t.Fatalf("intro, item and continuation guide problems = %v", problems)
	}
	for _, line := range []string{
		"* No delta worklist. Tracking #2.",
		"+ No delta worklist. Tracking #2.",
		"  - No delta worklist. Tracking #2.",
		"\t- No delta worklist. Tracking #2.",
		"1. No delta worklist. Tracking #2.",
		"2) No delta worklist. Tracking #2.",
		"No delta worklist. Tracking #2.",
		"-No delta worklist. Tracking #2.",
		"-  No delta worklist. Tracking #2.",
		"   No delta worklist. Tracking #2.",
		"Non-capabilities",
	} {
		for _, guide := range []string{base + line + "\n", base + "\n" + line + "\n", strings.Replace(base, "- A.", line+"\n- A.", 1)} {
			problems := NonCapabilityProblems(guide, status)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "not a recognised") }) {
				t.Fatalf("guide %q: problems = %v, want an unrecognised-line problem", guide, problems)
			}
			if guideProblems := GuideProblems(catalog, guide); !slices.ContainsFunc(guideProblems, func(p string) bool { return strings.Contains(p, "not a recognised") }) {
				t.Fatalf("guide %q: guide problems = %v, want an unrecognised-line problem", guide, guideProblems)
			}
		}
	}
	// A continuation after a blank line, or before any item, is not one.
	for _, guide := range []string{
		base + "\n  #1 again.\n",
		"Goals\n\nNon-capabilities\n\n  Tracking #1.\n- A. Tracking #1.\n",
		"Goals\n\nNon-capabilities\n\nIntro.\n\nSecond intro paragraph.\n- A. Tracking #1.\n",
	} {
		if problems := NonCapabilityProblems(guide, status); len(problems) != 1 || !strings.Contains(problems[0], "not a recognised") {
			t.Fatalf("guide %q: problems = %v", guide, problems)
		}
	}
}

func TestCAPV0026ShippedNonCapabilitiesAreCurrent(t *testing.T) {
	t.Parallel()
	if problems := NonCapabilityProblems(Guide(), NonCapabilityStatus()); len(problems) != 0 {
		t.Fatalf("internal/capcatalog/AGENT-GUIDE.md Non-capabilities disagree with NONCAPABILITY-STATUS.tsv:\n%s", strings.Join(problems, "\n"))
	}
	// Closing #1604 in the status table without removing its item must fail.
	closed := strings.Replace(NonCapabilityStatus(), "#1604\topen\t", "#1604\tclosed\t", 1)
	if problems := NonCapabilityProblems(Guide(), closed); len(problems) != 2 || !strings.Contains(problems[0], "#1604, recorded as closed") {
		t.Fatalf("a closed #1604 problems = %v", problems)
	}
}

// CAP-V0-027: the goals teach the capabilities #1626 found listed as missing,
// each with a runnable example; CAP-V0-018 checks those lines against the
// live catalog.
func TestCAPV0027GuideTeachesDeliveredCapabilities(t *testing.T) {
	t.Parallel()
	guide := Guide()
	goals, _, _ := strings.Cut(guide, "\nNon-capabilities\n")
	for _, example := range []string{
		"\n   corvint docs corpus discover-tests --runner playwright --revision FULL_COMMIT --path ",
		" --coverage-artifact lcov=trace.lcov > run.jsonl\n",
		"\n   corvint flows ingest-coverage --format lcov --from trace.lcov --evidence run.jsonl --revision FULL_COMMIT --test-key \"example.com/upload > TestRetry\"\n",
		"trace.lcov must hold\n   that test's coverage only, and --test-key names the test's key in run.jsonl",
		"\n   corvint docs corpus search --artifact corpus.json --query \"retry policy\" --kind symbol --include claims\n",
		"\n   corvint docs corpus claims --artifact corpus.json --query \"retry policy\"\n",
		"\n   corvint docs corpus delta --previous previous-corpus.json --artifact corpus.json --format jsonl\n",
	} {
		if !strings.Contains(goals, example) {
			t.Fatalf("the guide goals lack the example %q", example)
		}
	}
	var catalog Catalog
	if err := json.Unmarshal(Snapshot(), &catalog); err != nil {
		t.Fatal(err)
	}
	if problems := GuideProblems(catalog, goals+"\nNon-capabilities\n\n- A. Tracking #1.\n"); len(problems) != 0 {
		t.Fatalf("goal examples drifted from the catalog: %v", problems)
	}
	// The quoted go test key's " > " must not cut the checked command line,
	// so an option after it is still checked against the catalog.
	if !slices.Contains(guideCommandLines(goals), "corvint flows ingest-coverage --format lcov --from trace.lcov --evidence run.jsonl --revision FULL_COMMIT --test-key \"example.com/upload > TestRetry\"") {
		t.Fatal("the ingest-coverage example must be checked whole, with its --test-key")
	}
	if problems := GuideProblems(catalog, "  corvint flows ingest-coverage --test-key \"a > b\" --no-such-option\n\nNon-capabilities\n\n- A. Tracking #1.\n"); !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "--no-such-option") }) {
		t.Fatalf("an option after a quoted redirection must be checked: %v", problems)
	}
}

// CAP-V0-028: the seven delivered items left the Non-capabilities section,
// their issues are recorded closed, and the tsconfig residual names open #1604.
func TestCAPV0028DeliveredItemsLeftTheNonCapabilities(t *testing.T) {
	t.Parallel()
	states, _ := parseNonCapabilityStatus(NonCapabilityStatus())
	items := strings.Join(shippedNonCapabilities(t), "\n")
	for _, issue := range []string{"#1571", "#1573", "#1578", "#1579", "#1584", "#1585", "#1587"} {
		if states[issue] != "closed" || strings.Contains(items, issue) {
			t.Fatalf("%s: status %q, still listed %v", issue, states[issue], strings.Contains(items, issue))
		}
	}
	if states["#1604"] != "open" || !strings.Contains(items, "tsconfig paths aliases unresolved") || !strings.Contains(items, "Tracking #1604.") {
		t.Fatal("the tsconfig paths residual must name open #1604")
	}
}

// CAP-V0-029: the guide stays inside the binary; its by-design non-capability
// names this requirement, recorded as excludes.
func TestCAPV0029GuideIsNotAnArchiveMember(t *testing.T) {
	t.Parallel()
	states, _ := parseNonCapabilityStatus(NonCapabilityStatus())
	if states["CAP-V0-029"] != "excludes" || !strings.Contains(strings.Join(shippedNonCapabilities(t), "\n"), "release archive, because a seventh archive member would break deployed updaters. By design (CAP-V0-029).") {
		t.Fatal("the guide's archive non-capability must cite CAP-V0-029, recorded as excludes")
	}
	if Guide() != guide || !strings.HasPrefix(Guide(), "Corvint agent guide (corvint help guide)\n") {
		t.Fatal("Guide must return the embedded guide")
	}
}

func shippedNonCapabilities(t *testing.T) []string {
	t.Helper()
	items, problems := nonCapabilities(Guide())
	if len(problems) != 0 {
		t.Fatalf("shipped Non-capabilities lines are not recognised: %v", problems)
	}
	return items
}
