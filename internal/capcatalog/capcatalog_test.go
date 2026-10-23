package capcatalog

import (
	"bytes"
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
	clean := "Goals\n  corvint alpha run --mode M > out.json\n  corvint --version --json\n  `corvint help guide`\nOutput: alpha-out/2 and tool-version/1.\n\nNon-capabilities\n\n- No network. By design.\n- No beta mode. Tracking\n  #12.\n"
	if problems := GuideProblems(catalog, clean); len(problems) != 0 {
		t.Fatalf("clean guide problems: %v", problems)
	}
	for _, drift := range []string{
		"  corvint omega\n",
		"  corvint alpha jump\n",
		"  corvint alpha run --speed 2\n",
		"  corvint alpha run c\n",
		"Output: alpha-out/9.\n",
		"  `corvint help omega`\n",
	} {
		if problems := GuideProblems(catalog, drift+clean); len(problems) != 1 {
			t.Fatalf("drift %q problems = %v", drift, problems)
		}
	}
	if problems := GuideProblems(catalog, clean+"- No gamma.\n"); len(problems) != 1 {
		t.Fatalf("untracked non-capability problems = %v", problems)
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
