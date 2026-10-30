package harnesseventv0

import (
	_ "embed"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

//go:embed common-logical-interaction.json
var logicalInteraction []byte

func TestAHI011EveryEmbeddedHostHasAGoldenFixture(t *testing.T) {
	var fixture struct {
		Events []struct {
			Event    string                     `json:"event"`
			Expected map[string]json.RawMessage `json:"expected"`
		} `json:"events"`
	}
	if err := json.Unmarshal(logicalInteraction, &fixture); err != nil {
		t.Fatal(err)
	}
	goldenHosts := []string(nil)
	for _, event := range fixture.Events {
		for host := range event.Expected {
			if !gokernel.KnownHarnessHost(host) {
				t.Fatalf("golden event %q names host %q absent from the embedded schema", event.Event, host)
			}
		}
		if event.Event == "session-start" {
			goldenHosts = make([]string, 0, len(event.Expected))
			for host := range event.Expected {
				goldenHosts = append(goldenHosts, host)
			}
			sort.Strings(goldenHosts)
		}
	}
	if got := gokernel.HarnessHosts(); !slices.Equal(goldenHosts, got) {
		t.Fatalf("session-start golden hosts = %v, embedded hosts = %v", goldenHosts, got)
	}
}

// TestAHI014EventExpectationsAreHostConsistent pins the per-event host set and
// requires every present host's golden `expected` object to be byte-identical
// (after canonicalization) to the others, since the fixture is one common
// logical interaction observed through different host normalizations
// (AHI-014). Before this test, `docs/reviews/corpus-mutation-audit-2026-09-13.md`
// found seven per-host field mutations (sessionIdSha256, task, paths, an added
// codex host on file-change, verification, stopHookActive, openedPaths) stayed
// green because no consumer read these values.
func TestAHI014EventExpectationsAreHostConsistent(t *testing.T) {
	var fixture struct {
		Events []struct {
			Event    string                     `json:"event"`
			Expected map[string]json.RawMessage `json:"expected"`
		} `json:"events"`
	}
	if err := json.Unmarshal(logicalInteraction, &fixture); err != nil {
		t.Fatal(err)
	}
	expectedHosts := map[string][]string{
		"session-start": {"claude-code", "codex", "gemini-cli", "opencode", "pi"},
		"user-prompt":   {"claude-code", "codex", "gemini-cli", "opencode", "pi"},
		"file-change":   {"claude-code", "gemini-cli", "opencode"},
		"post-tool":     {"claude-code", "opencode", "pi"},
		"stop":          {"claude-code", "codex", "gemini-cli", "opencode", "pi"},
		"session-end":   {"claude-code", "codex", "gemini-cli", "opencode", "pi"},
	}
	seen := map[string]bool{}
	for _, event := range fixture.Events {
		seen[event.Event] = true
		want, ok := expectedHosts[event.Event]
		if !ok {
			t.Fatalf("event %q has no pinned host list in this test", event.Event)
		}
		gotHosts := make([]string, 0, len(event.Expected))
		for host := range event.Expected {
			gotHosts = append(gotHosts, host)
		}
		sort.Strings(gotHosts)
		wantHosts := append([]string(nil), want...)
		sort.Strings(wantHosts)
		if !slices.Equal(gotHosts, wantHosts) {
			t.Fatalf("%s hosts = %v, want %v", event.Event, gotHosts, wantHosts)
		}
		firstHost := want[0]
		firstCanonical := canonicalJSON(t, event.Expected[firstHost])
		for _, host := range want[1:] {
			if canonical := canonicalJSON(t, event.Expected[host]); canonical != firstCanonical {
				t.Fatalf("%s %s expected = %s, want same as %s = %s", event.Event, host, canonical, firstHost, firstCanonical)
			}
		}
	}
	for event := range expectedHosts {
		if !seen[event] {
			t.Fatalf("pinned event %q is absent from the fixture", event)
		}
	}
}

func canonicalJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(canonical)
}

func TestDecision0059HostForcedRuntimeIsSingularAndUncapped(t *testing.T) {
	raw, err := os.ReadFile("../../integrations/compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct {
		Entries []struct {
			Host              string         `json:"host"`
			HostForcedRuntime map[string]any `json:"hostForcedRuntime"`
			Status            string         `json:"status"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	packageRaw, err := os.ReadFile("../../integrations/opencode/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var packageManifest struct {
		Engines map[string]string `json:"engines"`
	}
	if err := json.Unmarshal(packageRaw, &packageManifest); err != nil {
		t.Fatal(err)
	}
	declared := 0
	for _, entry := range matrix.Entries {
		if entry.Status != "FALLBACK" {
			t.Fatalf("host %q status = %q without a passing plugin performance row", entry.Host, entry.Status)
		}
		if entry.HostForcedRuntime == nil {
			continue
		}
		declared++
		if entry.Host != "opencode" {
			t.Fatalf("host %q declares an unexpected forced runtime", entry.Host)
		}
		want := map[string]any{"name": "node", "versionRange": packageManifest.Engines["node"]}
		if !maps.Equal(entry.HostForcedRuntime, want) {
			t.Fatalf("host-forced runtime = %v, want uncapped declaration %v", entry.HostForcedRuntime, want)
		}
	}
	if declared != 1 {
		t.Fatalf("host-forced runtime declarations = %d, want exactly OpenCode's one forced runtime", declared)
	}
}
