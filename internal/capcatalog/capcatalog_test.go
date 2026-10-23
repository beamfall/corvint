package capcatalog

import (
	"bytes"
	"encoding/json"
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
	clean := "Goals\n  corvint alpha run --mode M > out.json\n  corvint --root /repo alpha run --mode=M\n  corvint --version --json\n  `corvint help guide`\n  `corvint help alpha run`\nOutput: alpha-out/2 and tool-version/1.\n\nNon-capabilities\n\n- No network. By design.\n- No beta mode. Tracking\n  #12.\n"
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
