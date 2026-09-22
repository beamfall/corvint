package touchsurprise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const fixtureTask = "update the existing helper in internal/new"

// fixture builds a two-commit repository: the base commit holds the packet's
// subject, and the target commit changes it beside three paths no packet term
// names, which are therefore the misses under test.
func fixture(t *testing.T) (string, string, string, *contextindex.Index) {
	t.Helper()
	root := t.TempDir()
	base := map[string]string{
		"go.mod":                        "module example.test/fixture\n\ngo 1.27.0\n",
		"internal/new/existing.go":      "package newpkg\n\nfunc Existing() string { return \"existing\" }\n",
		"internal/new/existing_test.go": "package newpkg\n\nimport \"testing\"\n\nfunc TestExisting(t *testing.T) { _ = Existing() }\n",
		"internal/caller/caller.go":     "package caller\n\nimport \"example.test/fixture/internal/new\"\n\nvar Value = newpkg.Existing\n",
	}
	for name, content := range base {
		writeFixtureFile(t, root, name, content)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "corvint@example.test")
	runGit(t, root, "config", "user.name", "Corvint Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "base")
	baseRevision := headRevision(t, root)
	target := map[string]string{
		"internal/new/existing.go":       "package newpkg\n\nfunc Existing() string { return \"existing value\" }\n",
		"internal/zebra/quokka.go":       "package zebra\n\nfunc Quokka() int { return 7 }\n",
		"internal/zebra/quokka_test.go":  "package zebra\n\nimport \"testing\"\n\nfunc TestQuokka(t *testing.T) { _ = Quokka() }\n",
		"docs/zebra-quokka-marsupial.md": "# Quokka\n\nUnrelated note.\n",
	}
	for name, content := range target {
		writeFixtureFile(t, root, name, content)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "target")
	targetRevision := headRevision(t, root)
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, baseRevision, targetRevision, index
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func headRevision(t *testing.T, root string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(output))
}

func render(t *testing.T, options Options, index *contextindex.Index) map[string]any {
	t.Helper()
	var stdout bytes.Buffer
	if err := Render(context.Background(), index, options, &stdout); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func fixtureOptions(root, base, target string) Options {
	return Options{Root: root, Task: fixtureTask, Base: base, Target: target, Limit: DefaultLimit}
}

func stringList(t *testing.T, value any) []string {
	t.Helper()
	raw, isList := value.([]any)
	if !isList {
		t.Fatalf("expected a list, got %T", value)
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		values = append(values, item.(string))
	}
	return values
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// TSS-V0-004: the pure arithmetic, independent of any repository.
func TestComputeReportsSetArithmetic(t *testing.T) {
	report := Compute([]string{"b.go", "a.go", "a.go", ""}, []string{"a.go", "c.go", "d.go"})
	if !reflect.DeepEqual(report.Predicted, []string{"a.go", "b.go"}) {
		t.Fatalf("predicted=%v", report.Predicted)
	}
	if !reflect.DeepEqual(report.PredictedOnly, []string{"b.go"}) {
		t.Fatalf("predicted_only=%v", report.PredictedOnly)
	}
	if !reflect.DeepEqual(report.ActualOnly, []string{"c.go", "d.go"}) {
		t.Fatalf("actual_only=%v", report.ActualOnly)
	}
	if !reflect.DeepEqual(report.Intersection, []string{"a.go"}) {
		t.Fatalf("intersection=%v", report.Intersection)
	}
	if report.SymmetricDifference != 3 {
		t.Fatalf("symmetric_difference=%d", report.SymmetricDifference)
	}
	if report.Surprise != 0.6667 || report.EmptyActual {
		t.Fatalf("surprise=%v empty_actual=%v", report.Surprise, report.EmptyActual)
	}
}

// TSS-V0-004, TSS-V0-005: a path with edge whitespace keeps its literal
// spelling, so its miss row still finds it in the tracked set.
func TestComputeKeepsEdgeWhitespacePaths(t *testing.T) {
	literal := "docs/trailing.md "
	report := Compute([]string{"docs/trailing.md"}, []string{literal})
	if !reflect.DeepEqual(report.ActualOnly, []string{literal}) {
		t.Fatalf("actual_only=%q, want %q", report.ActualOnly, literal)
	}
	row := missRow(report.ActualOnly[0], map[string]struct{}{literal: {}})
	if row["tracked_at_target"] != true {
		t.Fatalf("miss row=%v, want tracked_at_target true", row)
	}
}

// TSS-V0-004: an empty actual set scores 0 and says so rather than dividing.
func TestComputeAndRenderReportEmptyActualRange(t *testing.T) {
	if report := Compute([]string{"a.go"}, nil); report.Surprise != 0 || !report.EmptyActual {
		t.Fatalf("surprise=%v empty_actual=%v", report.Surprise, report.EmptyActual)
	}
	root, _, target, index := fixture(t)
	rendered := render(t, fixtureOptions(root, target, target), index)
	surprise := rendered["surprise"].(map[string]any)
	if surprise["empty_actual"] != true || surprise["surprise"] != float64(0) {
		t.Fatalf("surprise member=%v", surprise)
	}
	if len(stringList(t, rendered["actual"].(map[string]any)["paths"])) != 0 {
		t.Fatalf("actual paths=%v", rendered["actual"])
	}
}

// TSS-V0-002: the predicted set separates the packet's own paths from the ones
// only the impact expansion of those paths reached.
func TestSurpriseSeparatesPacketAndImpactPredictions(t *testing.T) {
	root, base, target, index := fixture(t)
	rendered := render(t, fixtureOptions(root, base, target), index)
	predicted := rendered["predicted"].(map[string]any)
	fromPacket := stringList(t, predicted["from_packet"])
	fromImpact := stringList(t, predicted["from_impact"])
	all := stringList(t, predicted["paths"])
	if len(fromPacket) == 0 {
		t.Fatalf("expected packet paths, got %v", predicted)
	}
	if !contains(fromPacket, "internal/new/existing.go") {
		t.Fatalf("packet never named the subject: %v", fromPacket)
	}
	if len(all) != len(fromPacket)+len(fromImpact) {
		t.Fatalf("union %v is not packet %v plus impact %v", all, fromPacket, fromImpact)
	}
	for _, value := range fromImpact {
		if contains(fromPacket, value) {
			t.Fatalf("impact path %q is already a packet path", value)
		}
	}
	if rendered["revision"] != index.Revision || rendered["range"].(map[string]any)["target"] != target {
		t.Fatalf("receipt is not bound to the committed range: %v", rendered["range"])
	}
}

// TSS-V0-002: an impact row that names a ledger record rather than a tracked
// path (a feature a changed path carries) never enters the predicted paths.
func TestSurprisePredictsOnlyTrackedImpactPaths(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                   "module example.test/fixture\n\ngo 1.27.0\n",
		"internal/new/existing.go": "package newpkg\n\n// feature:stream-token\nfunc Existing() string { return \"existing\" }\n",
		"testing/features.yaml":    "features:\n  - id: stream-token\n    area: auth\n    summary: Mint a stream token.\n    status: shipped\n",
		"testing/scenarios.yaml":   "scenarios:\n  - id: revoked-token\n    area: auth\n    summary: Revoked tokens are denied.\n    features: [stream-token]\n    status: shipped\n",
	}
	for name, content := range files {
		writeFixtureFile(t, root, name, content)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "corvint@example.test")
	runGit(t, root, "config", "user.name", "Corvint Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "base")
	revision := headRevision(t, root)
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	predicted := render(t, fixtureOptions(root, revision, revision), index)["predicted"].(map[string]any)
	if !contains(stringList(t, predicted["from_packet"]), "internal/new/existing.go") {
		t.Fatalf("packet never named the marked subject: %v", predicted)
	}
	for _, value := range stringList(t, predicted["paths"]) {
		if _, tracked := index.Sources[value]; !tracked {
			t.Fatalf("predicted path %q is not a tracked source: %v", value, predicted)
		}
	}
}

// TSS-V0-005: a path the packet never named is reported as a miss and carries
// its heuristic classification.
func TestSurpriseReportsActualOnlyPathsAsMisses(t *testing.T) {
	root, base, target, index := fixture(t)
	rendered := render(t, fixtureOptions(root, base, target), index)
	surprise := rendered["surprise"].(map[string]any)
	actualOnly := stringList(t, surprise["actual_only"])
	if !contains(actualOnly, "docs/zebra-quokka-marsupial.md") {
		t.Fatalf("expected the unrelated doc among the misses: %v", actualOnly)
	}
	if surprise["surprise"].(float64) <= 0 {
		t.Fatalf("expected a positive surprise, got %v", surprise["surprise"])
	}
	rows := rendered["misses"].([]any)
	if len(rows) != len(actualOnly) {
		t.Fatalf("misses=%d actual_only=%d", len(rows), len(actualOnly))
	}
	byPath := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		byPath[row["path"].(string)] = row
	}
	doc := byPath["docs/zebra-quokka-marsupial.md"]
	if doc["doc_path"] != true || doc["test_path"] != false || doc["tracked_at_target"] != true {
		t.Fatalf("doc miss row=%v", doc)
	}
	if row, present := byPath["internal/zebra/quokka_test.go"]; present && row["test_path"] != true {
		t.Fatalf("test miss row=%v", row)
	}
}

// TSS-V0-003: both revisions must resolve to a commit.
func TestSurpriseRefusesUnresolvableRevision(t *testing.T) {
	root, base, target, index := fixture(t)
	options := fixtureOptions(root, base, target)
	options.Target = "no-such-revision"
	err := Render(context.Background(), index, options, &bytes.Buffer{})
	requireCode(t, err, "unsupported-surprise-revision")
}

// TSS-V0-003: the actual set is committed evidence, so a dirty worktree is
// refused rather than silently compared.
func TestSurpriseRefusesDirtyWorktree(t *testing.T) {
	root, base, target, index := fixture(t)
	writeFixtureFile(t, root, "internal/new/existing.go", "package newpkg\n\nfunc Existing() string { return \"dirty\" }\n")
	err := Render(context.Background(), index, fixtureOptions(root, base, target), &bytes.Buffer{})
	requireCode(t, err, "unsupported-surprise-dirty-worktree")
}

// TSS-V0-006: the verb writes nothing; `git status` and `.corvint/` are untouched.
func TestSurpriseWritesNothing(t *testing.T) {
	root, base, target, index := fixture(t)
	render(t, fixtureOptions(root, base, target), index)
	command := exec.Command("git", "status", "--porcelain")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.TrimSpace(output)) != 0 {
		t.Fatalf("worktree changed: %s", output)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !os.IsNotExist(err) {
		t.Fatalf(".corvint exists after a read-only verb: %v", err)
	}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *gokernel.Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error=%v, want code %q", err, code)
	}
}

// TSS-V0-003, TSS-V0-005: newline output would C-quote a non-ASCII or
// quote-bearing path and trimming would drop a leading space, so the actual and
// tracked sets must read the literal spelling the index records.
func TestSurpriseReadsLiteralNonASCIIPaths(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.test/fixture\n")
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "corvint@example.test")
	runGit(t, root, "config", "user.name", "Corvint Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "base")
	base := headRevision(t, root)
	literal := []string{"docs/café.md", "docs/say \"hi\".md", "docs/trailing .md"}
	for _, name := range literal {
		writeFixtureFile(t, root, name, "# note\n")
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "target")
	target := headRevision(t, root)
	changed, err := changedPaths(context.Background(), root, base, target)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docs/café.md", "docs/say \"hi\".md", "docs/trailing .md"}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed=%q, want %q", changed, want)
	}
	tracked, err := trackedPaths(context.Background(), root, target)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range want {
		if _, found := tracked[name]; !found {
			t.Fatalf("tracked=%q lacks %q", tracked, name)
		}
	}
}
