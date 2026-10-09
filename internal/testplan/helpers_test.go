package testplan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const (
	exampleSpec   = "tests/e2e/teesheet.spec.ts"
	exampleScreen = "screen:admin:club.teesheet"
	fixedRevision = "0123456789abcdef0123456789abcdef01234567"
)

// variation is one complete, valid input row of the worked example's context; edits adjust it.
func variation(id string, edits ...func(map[string]any)) map[string]any {
	v := map[string]any{
		"variation_id": id, "spec": exampleSpec, "app": "admin", "setup": "scenarios/club.ts", "screen": exampleScreen,
		"user": "club-admin", "org": "club-a", "action": []any{"element:" + id + ".act"}, "assertion": []any{"element:" + id + ".see"},
		"requires": []any{}, "changes": []any{}, "destructive": false,
	}
	for _, edit := range edits {
		edit(v)
	}
	return v
}

func set(member string, value any) func(map[string]any) {
	return func(v map[string]any) { v[member] = value }
}

func drop(member string) func(map[string]any) {
	return func(v map[string]any) { delete(v, member) }
}

func inputJSON(t testing.TB, variations ...map[string]any) []byte {
	t.Helper()
	rows := make([]any, len(variations))
	for i, v := range variations {
		rows[i] = v
	}
	data, err := json.Marshal(map[string]any{"schema": InputSchema, "variations": rows})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decode(t testing.TB, data []byte) *Input {
	t.Helper()
	in, err := DecodeInput(data)
	if err != nil {
		t.Fatalf("DecodeInput: %v", err)
	}
	return in
}

func build(t testing.TB, evidence Evidence, maxSteps int, variations ...map[string]any) *Plan {
	t.Helper()
	p, err := Build(decode(t, inputJSON(t, variations...)), evidence, maxSteps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return p
}

// rows returns the table lines after the header, column names and rule.
func rows(p *Plan) []string {
	lines := strings.Split(strings.TrimSuffix(string(p.Table()), "\n"), "\n")
	return lines[3:]
}

func code(err error) string {
	if coded, ok := err.(*gokernel.Error); ok {
		return coded.Code
	}
	return ""
}

func passing() testvalidity.Projection {
	return testvalidity.Projection{
		Association: testvalidity.Axis{State: testvalidity.AssociationAssociated},
		Hygiene:     testvalidity.Axis{State: testvalidity.HygieneEligible},
		Freshness:   testvalidity.Axis{State: testvalidity.FreshnessCurrent},
		Execution:   testvalidity.Axis{State: testvalidity.ExecutionPassed},
		Strength:    testvalidity.Axis{State: "NOT_MEASURED", Reason: "no-mutation-run"},
	}
}

// e2eDocument is an already-bound JavaScript e2e document holding one test.
func e2eDocument(id, project string, projection testvalidity.Projection) testvaliditydoc.Document {
	test := testvaliditydoc.Test{ID: id, Name: id, State: "passed", Projection: projection}
	if project != "" {
		test.Project = &jstestprovider.ProjectIdentity{Name: project}
	}
	return testvaliditydoc.Document{Schema: testvaliditydoc.Schema, Source: jsProviderSource, Kind: jsEndToEndKind, Tests: []testvaliditydoc.Test{test}}
}

// gitRepo creates a repository whose files are committed; it returns the root.
func gitRepo(t testing.TB, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	writeFiles(t, root, files)
	commit(t, root)
	return root
}

func writeFiles(t testing.TB, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func commit(t testing.TB, root string) string {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-qm", "fixture")
	return strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
}

func git(t testing.TB, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir, c.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}
