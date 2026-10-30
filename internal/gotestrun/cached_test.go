package gotestrun

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// cachedShape is go test -json output for a package whose result go test
// replayed from its build cache: the test events are the recorded ones, and
// the package's final output line says (cached).
const cachedShape = `{"Time":"2026-10-28T10:00:00.000000Z","Action":"start","Package":"example.test/run/lib"}
{"Time":"2026-10-28T10:00:00.000100Z","Action":"run","Package":"example.test/run/lib","Test":"TestPass"}
{"Time":"2026-10-28T10:00:00.000200Z","Action":"output","Package":"example.test/run/lib","Test":"TestPass","Output":"=== RUN   TestPass\n"}
{"Time":"2026-10-28T10:00:00.000300Z","Action":"pass","Package":"example.test/run/lib","Test":"TestPass","Elapsed":0}
{"Time":"2026-10-28T10:00:00.000400Z","Action":"output","Package":"example.test/run/lib","Output":"PASS\n"}
{"Time":"2026-10-28T10:00:00.000500Z","Action":"output","Package":"example.test/run/lib","Output":"ok  \texample.test/run/lib\t(cached)\n"}
{"Time":"2026-10-28T10:00:00.000600Z","Action":"pass","Package":"example.test/run/lib","Elapsed":0}
`

// TestAGWV0040CachedPackagesAreNotExecutedPasses pins AGW-V0-040 (V1-3004):
// a package whose result go test replayed from its cache yields no
// executed-pass observation. The stream names the cached package, and a run
// over it records cached-result infrastructure, which the witness never binds.
func TestAGWV0040CachedPackagesAreNotExecutedPasses(t *testing.T) {
	tests, cached, malformed := parseRun([]byte(cachedShape))
	if malformed != "" || len(tests) != 1 || !slices.Equal(cached, []string{"example.test/run/lib"}) {
		t.Fatalf("tests=%v cached=%v malformed=%q", tests, cached, malformed)
	}
	if _, cached, _ := parseRun([]byte(realShape)); len(cached) != 0 {
		t.Fatalf("an executed run names cached packages %v", cached)
	}
	if failure := cachedFailure([]string{"example.test/run/lib"}); failure == nil || failure.Reason != "cached-result" {
		t.Fatalf("cached packages recorded %+v", failure)
	}
}

// TestAGWV0040DefaultRunExecutesAndCachedRunIsInfrastructure runs the real
// go test twice. The default command carries -count=1 and executes the tests
// every time; a configured command without it replays the second run from the
// cache, and that document records cached-result rather than passes.
func TestAGWV0040DefaultRunExecutesAndCachedRunIsInfrastructure(t *testing.T) {
	for _, tool := range []string{"go", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not available")
		}
	}
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	write(t, root, "mod/go.mod", "module example.test/run\n\ngo 1.21\n")
	write(t, root, "mod/lib/lib_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.test", "commit", "-q", "-m", "init")
	dir := filepath.Join(root, "mod")
	for run := range 2 {
		document, err := Run(context.Background(), Config{Dir: dir})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(document)
		if document.Infrastructure != nil || len(document.Tests) != 1 || !slices.Contains(document.Command, "-count=1") {
			t.Fatalf("default run %d = %s", run, encoded)
		}
	}
	cachedCommand := Config{Dir: dir, Go: []string{"go", "test", "-json"}}
	if _, err := Run(context.Background(), cachedCommand); err != nil {
		t.Fatal(err)
	}
	document, err := Run(context.Background(), cachedCommand)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(document)
	if document.Infrastructure == nil || document.Infrastructure.Reason != "cached-result" {
		t.Fatalf("cached run = %s", encoded)
	}
	if _, err := Decode(encoded); err != nil {
		t.Fatal(err)
	}
}
