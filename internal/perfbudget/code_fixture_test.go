package perfbudget

import (
	"bufio"
	"context"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// codeScenarioIDs are the code-repository scenarios PRB-V0-057 adds after unchanged-delta.
var codeScenarioIDs = []string{"index-cold-build", "index-warm-build", "context-query", "lifecycle-hook", "affected-go", "affected-ts"}

// PRB-V0-057: the scenario set ends with the cold and warm index build, the context query, the
// lifecycle hook, and Go and TypeScript affected plans, after the seven earlier scenarios.
func TestPRBV0057CodeScenarios(t *testing.T) {
	if !slices.Equal(ScenarioIDs[7:], codeScenarioIDs) || len(ScenarioIDs) != 13 {
		t.Fatalf("scenario order %v", ScenarioIDs)
	}
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	commands := map[string][]string{}
	for _, m := range b.Baseline.Scenarios {
		commands[m.ID] = m.Command
	}
	want := map[string][]string{
		"index-cold-build": {"corvint", "--root", "<fixture>/code-app", "index"},
		"index-warm-build": {"corvint", "--root", "<fixture>/code-app", "index", "--if-stale"},
		"context-query":    {"corvint", "--root", "<fixture>/code-app", "context", "--task", codeContextTask, "--subject", codeGoPath, "--lsp", "off"},
		"lifecycle-hook":   {"corvint", "adapter", "claude-code", "user-prompt"},
		"affected-go":      {"corvint", "--root", "<fixture>/code-app", "affected", "--path", codeGoPath},
		"affected-ts":      {"corvint", "--root", "<fixture>/code-app", "affected", "--path", codeTSPath},
	}
	for id, argv := range want {
		if !slices.Equal(commands[id], argv) {
			t.Errorf("%s baseline command %v, want %v", id, commands[id], argv)
		}
	}
}

// PRB-V0-058: the code repository holds the scaled Go and TypeScript import trees, test files
// for both, and single-line Go and TypeScript declarations of at least LongLineBytes that the
// root package and module import, and its commit is deterministic.
func TestPRBV0058CodeFixtureShape(t *testing.T) {
	if testing.Short() {
		t.Skip("generates two Git repositories")
	}
	ctx := context.Background()
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := generateCode(ctx, dir, Smoke); err != nil {
			t.Fatal(err)
		}
	}
	a, errA := gitOutput(ctx, first, "rev-parse", "HEAD")
	b, errB := gitOutput(ctx, second, "rev-parse", "HEAD")
	if errA != nil || errB != nil || a != b {
		t.Fatalf("code HEAD differs between identical generations: %s vs %s (%v %v)", a, b, errA, errB)
	}
	count := map[string]int{}
	_ = filepath.WalkDir(first, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			name := d.Name()
			switch {
			case strings.HasSuffix(name, "_test.go"):
				count["go-test"]++
			case strings.HasSuffix(name, ".go"):
				count["go"]++
			case strings.HasSuffix(name, ".test.ts"):
				count["ts-test"]++
			case strings.HasSuffix(name, ".ts"):
				count["ts"]++
			}
		}
		return nil
	})
	if count["go"] != Smoke.GoPackages+1 || count["go-test"] != Smoke.GoPackages || count["ts"] != Smoke.TSModules+1 || count["ts-test"] < 1 {
		t.Fatalf("code repository shape %v", count)
	}
	for _, path := range []string{"pkg/table/table.go", "web/src/table.ts"} {
		data, err := os.ReadFile(filepath.Join(first, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		longest := 0
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(nil, math.MaxInt32)
		for scanner.Scan() {
			longest = max(longest, len(scanner.Bytes()))
		}
		if longest < Smoke.LongLineBytes {
			t.Fatalf("%s longest line %d bytes, want at least %d", path, longest, Smoke.LongLineBytes)
		}
	}
	for path, importLine := range map[string]string{"pkg/p0001/p0001.go": `import "example.com/codeapp/pkg/table"`, "web/src/m00001.ts": `from "./table"`, codeGoPath: `import "example.com/codeapp/pkg/p0001"`, codeTSPath: `from "./m00001"`} {
		data, err := os.ReadFile(filepath.Join(first, filepath.FromSlash(path)))
		if err != nil || !strings.Contains(string(data), importLine) {
			t.Fatalf("%s lacks %q: %v", path, importLine, err)
		}
	}
	if Full.GoPackages < 1000 || Full.TSModules < 1000 || Full.LongLineBytes < 1<<20 {
		t.Fatalf("full code scale %+v is not realistic", Full)
	}
}

// PRB-V0-059: every code-repository scenario is bounded by enforced ceilings derived from its
// recorded baseline (3x p95 rounded up to 500 ms, 1.5x peak resident set rounded up to 64 MiB).
func TestPRBV0059CodeScenarioCeilings(t *testing.T) {
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]ScenarioMeasured{}
	for _, m := range b.Baseline.Scenarios {
		baseline[m.ID] = m
	}
	const mib = 1 << 20
	for _, s := range b.Scenarios {
		if !slices.Contains(codeScenarioIDs, s.ID) {
			continue
		}
		m := baseline[s.ID]
		if m.P95WallMS == nil || m.PeakRSSBytes == nil || m.Provenance == nil {
			t.Fatalf("%s baseline unmeasured: %+v", s.ID, m)
		}
		want := map[string]float64{MetricP95Wall: math.Ceil(3**m.P95WallMS/500) * 500, MetricPeakRSS: math.Ceil(1.5*float64(*m.PeakRSSBytes)/(64*mib)) * 64 * mib}
		for _, c := range s.Ceilings {
			if c.Status != StatusEnforced || c.Limit != want[c.Metric] || !strings.Contains(c.Reason, "PRB-V0-059") {
				t.Errorf("%s ceiling %+v, want %v enforced", s.ID, c, want[c.Metric])
			}
		}
	}
}
