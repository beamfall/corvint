package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

func runCatalogHelp(t *testing.T, args ...string) string {
	t.Helper()
	reader := &forbiddenHelpReader{}
	var stdout, stderr bytes.Buffer
	args = append([]string{"--root", "/definitely/not/a/repository"}, args...)
	if exit := run(args, reader, &stdout, &stderr); exit != 0 || stderr.Len() != 0 || reader.reads != 0 {
		t.Fatalf("exit=%d stderr=%q stdin reads=%d", exit, &stderr, reader.reads)
	}
	if !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("output lacks terminal LF")
	}
	return stdout.String()
}

func TestCAPV0015HelpJSONListsEveryDispatchedCommand(t *testing.T) {
	t.Parallel()
	output := runCatalogHelp(t, "help", "--json")
	if output != runCatalogHelp(t, "help", "--json") {
		t.Fatal("help --json is not deterministic")
	}
	var catalog capcatalog.Catalog
	if err := json.Unmarshal([]byte(output), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != capcatalog.Schema || len(catalog.Commands) != len(topLevelCommands) {
		t.Fatalf("schema=%q commands=%d want %d", catalog.Schema, len(catalog.Commands), len(topLevelCommands))
	}
	for index, command := range catalog.Commands {
		if command.Name != topLevelCommands[index] || command.Purpose == nil || command.Maturity == nil {
			t.Fatalf("command %d = %+v, want %q with purpose and maturity from root help", index, command, topLevelCommands[index])
		}
	}
	byName := map[string]capcatalog.Command{}
	for _, command := range catalog.Commands {
		byName[command.Name] = command
	}
	if affected := byName["affected"]; !strings.Contains(strings.Join(affected.Operations, " "), "discovery") ||
		!strings.Contains(strings.Join(affected.Flags, " "), "--playwright-discovery") ||
		!strings.Contains(strings.Join(affected.SchemaIDs, " "), "affected-plan/1") {
		t.Fatalf("affected entry is not derived from its help: %+v", affected)
	}
	if docs := byName["docs"]; len(docs.NestedOperations["corpus"]) != len(corpusOperations) || docs.Writes == nil || *docs.Writes != "mode-dependent" {
		t.Fatalf("docs entry lacks the corpus operation table or its write class: %+v", docs)
	}
	if impact := byName["impact"]; impact.Writes == nil || *impact.Writes != "none" || impact.SelfObservationLedger != "best-effort-append" {
		t.Fatalf("impact write class is not the root support boundary's: %+v", impact)
	}
}

// CAP-V0-022: the binary's own catalog marks requiredFlags partial.
func TestCAPV0022BinaryCatalogMarksRequiredFlagsPartial(t *testing.T) {
	t.Parallel()
	var catalog capcatalog.Catalog
	if err := json.Unmarshal([]byte(runCatalogHelp(t, "help", "--json")), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.RequiredFlagsCoverage != capcatalog.RequiredFlagsPartial {
		t.Fatalf("requiredFlagsCoverage = %q, want %q", catalog.RequiredFlagsCoverage, capcatalog.RequiredFlagsPartial)
	}
}

func TestCAPV0016UnderivableFieldsAreUnknownWithReasons(t *testing.T) {
	t.Parallel()
	catalog := capcatalog.Build(capabilityRegistry())
	for _, command := range catalog.Commands {
		unknown := map[string]bool{}
		for _, entry := range command.Unknowns {
			if entry.Reason == "" {
				t.Fatalf("%s unknown %s has no reason", command.Name, entry.Field)
			}
			unknown[entry.Field] = true
		}
		if command.Inputs != nil || command.Outputs != nil || !unknown["inputs"] || !unknown["outputs"] {
			t.Fatalf("%s states inputs or outputs the registry cannot separate", command.Name)
		}
		if (command.OwningSpec == nil) != unknown["owningSpec"] || (command.Writes == nil) != unknown["writes"] {
			t.Fatalf("%s null field without an unknown, or an unknown with a value: %+v", command.Name, command)
		}
		if *command.Maturity == "core" && (command.OwningSpec != nil || command.FreezeContract == nil || *command.FreezeContract != "CCF-V1") {
			t.Fatalf("core %s must name the CCF-V1 freeze, not a per-verb spec: %+v", command.Name, command)
		}
	}
}

func TestCAPV0017HelpGuideServesTheShippedGuide(t *testing.T) {
	t.Parallel()
	if got := runCatalogHelp(t, "help", "guide"); got != capcatalog.Guide() {
		t.Fatal("help guide does not print the embedded guide")
	}
	if got := capcatalog.Build(capabilityRegistry()).Guide; got.Topic != "corvint help guide" || len(got.SHA256) != 64 {
		t.Fatalf("catalog guide binding = %+v", got)
	}
}

func TestCAPV0018ShippedGuideMatchesTheLiveCatalog(t *testing.T) {
	t.Parallel()
	if problems := capcatalog.GuideProblems(capcatalog.Build(capabilityRegistry()), capcatalog.Guide()); len(problems) != 0 {
		t.Fatalf("internal/capcatalog/AGENT-GUIDE.md drifted from the binary:\n%s", strings.Join(problems, "\n"))
	}
}

func TestCAPV0019CommittedCatalogMatchesTheBinary(t *testing.T) {
	t.Parallel()
	if !bytes.Equal(capcatalog.Snapshot(), []byte(capabilityCatalogJSON())) {
		t.Fatal("internal/capcatalog/catalog.json differs from `corvint help --json`; run `make capability-catalog` and commit the result")
	}
}

func TestCAPV0021RootHelpNamesTheCatalogTopicsWithinItsBound(t *testing.T) {
	t.Parallel()
	if !strings.Contains(rootHelp, " | corvint help --json|guide\n") || len(rootHelp) > rootHelpMaxBytes {
		t.Fatalf("root help lacks the catalog topics or exceeds %d bytes (%d)", rootHelpMaxBytes, len(rootHelp))
	}
	err := unknownHelpTopic([]string{"nope"})
	if strings.Contains(err.Error(), "--json") || strings.Contains(err.Error(), "'guide'") {
		t.Fatalf("the unknown-topic refusal must keep listing commands and nested topics only: %v", err)
	}
}

// catalogDriftHint names the recovery when the embedded catalog no longer
// matches the CLI (V1-3659): the snapshot the help tool serves is regenerated,
// not edited.
const catalogDriftHint = "; the embedded catalog is stale: run `make capability-catalog` and commit internal/capcatalog/"

// MCPV0-105: the corvint.help answer that corvint-docs-mcp, corvint-corpus-mcp
// and corvint-test-validity-mcp return through bridge.HelpToolResult carries the
// CLI's own stdout byte for byte, for both topics, in structuredContent. Each
// server's wire test holds its answer equal to bridge.HelpToolResult's.
func TestMCPV0105CompanionHelpMatchesTheCLI(t *testing.T) {
	t.Parallel()
	for arguments, cli := range map[string][]string{`{}`: {"help", "--json"}, `{"topic":"guide"}`: {"help", "guide"}} {
		answer, callErr := bridge.HelpToolResult(context.Background(), []byte(arguments))
		if callErr != nil {
			t.Fatal(callErr)
		}
		structured, _ := answer["structuredContent"].(map[string]any)
		receipt, _ := structured["receipt"].(map[string]any)
		if text, _ := receipt["text"].(string); text != runCatalogHelp(t, cli...) {
			t.Fatalf("companion corvint.help %s differs from corvint %s%s", arguments, strings.Join(cli, " "), catalogDriftHint)
		}
	}
}

// MCPV0-100: the corvint.help MCP tool returns the CLI's own stdout byte for
// byte, for both topics.
func TestMCPV0100HelpToolMatchesTheCLI(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry, bridgeErr := bridge.New(root)
	if bridgeErr != nil {
		t.Fatal(bridgeErr)
	}
	for arguments, cli := range map[string][]string{`{}`: {"help", "--json"}, `{"topic":"guide"}`: {"help", "guide"}} {
		result, callErr := registry.Call(context.Background(), bridge.ToolHelp, []byte(arguments))
		if callErr != nil {
			t.Fatal(callErr)
		}
		if text, _ := result.Receipt["text"].(string); text != runCatalogHelp(t, cli...) {
			t.Fatalf("corvint.help %s differs from corvint %s%s", arguments, strings.Join(cli, " "), catalogDriftHint)
		}
	}
}

// TestMCPV0100DriftFailureNamesTheRecovery (V1-3659): the MCPV0-100 and
// MCPV0-105 drift failures name the regeneration command, not only the diff.
func TestMCPV0100DriftFailureNamesTheRecovery(t *testing.T) {
	t.Parallel()
	if !strings.Contains(catalogDriftHint, "make capability-catalog") {
		t.Fatalf("drift hint %q names no recovery command", catalogDriftHint)
	}
}
