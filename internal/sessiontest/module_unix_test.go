//go:build darwin || linux

package sessiontest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// fakeModuleGo is a `go` that answers from the directory it runs in: the
// repository root lists and passes m/a; the nested module sub/mod lists its
// own packages and passes, fails, or cannot be listed as FAKE_MODE says. Each
// launch records its working directory, GOWORK and argv.
const fakeModuleGo = `#!/bin/sh
here=$(pwd -P)
case "$here" in */sub/mod) where=module;; *) where=root;; esac
case "$1" in
env) echo go1.27.1; exit 0;;
list)
  if [ "$where" = module ]; then
    if [ "$FAKE_MODE" = listfail ]; then exit 1; fi
    echo example.test/nested; echo example.test/nested/deep
  else
    echo m/a
  fi
  exit 0;;
esac
echo "$here GOWORK=$GOWORK $*" >> "$FAKE_DIR/argv"
if [ "$where" = root ]; then echo '{"Action":"pass","Package":"m/a"}'; exit 0; fi
echo '{"Action":"pass","Package":"example.test/nested"}'
if [ "$FAKE_MODE" = modfail ]; then echo '{"Action":"fail","Package":"example.test/nested/deep"}'; exit 1; fi
echo '{"Action":"skip","Package":"example.test/nested/deep"}'
`

func moduleRunnerFixture(t *testing.T, mode string) (*Runner, Manifest, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(dir, "go"), []byte(fakeModuleGo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub", "mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+string(os.PathListSeparator)+"/usr/bin")
	source := SourceIdentity{Head: "h", Paths: []PathDigest{}, Tree: "t"}
	m := Manifest{
		Checks: []string{}, Incomplete: []string{}, Profile: ManifestProfile, RunnerProfile: ProfileLocal,
		Scope: ScopeNarrowed, ShardCount: 1, Source: source,
		Units: []Unit{
			{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:m/a"},
			{Adapter: AdapterGo, Dir: "sub/mod", Reason: moduleReason, Shard: 0, Slice: SliceWhole, Unit: ModuleUnitPrefix + "sub/mod"},
		},
	}
	digest, _ := Digest(m)
	runner := &Runner{
		Declared: parsedDeclared(), Environ: []string{"PATH=" + os.Getenv("PATH"), "FAKE_MODE=" + mode, "FAKE_DIR=" + dir},
		Errors: io.Discard, Output: io.Discard, Root: dir, Tool: "corvint test", Universe: []string{"m/a"},
		Recheck: func(context.Context) (SourceIdentity, error) { return source, nil },
	}
	return runner, m, digest, dir
}

// STR-V0-064: `corvint test` executes a nested-module unit: its own go test
// ./... from the module directory with GOWORK=off, confirmed by its own
// go list, its outcome folded from the module's packages; a module that
// cannot be listed launches nothing for it and is INCOMPLETE.
func TestSTRV0064RunnerExecutesNestedModule(t *testing.T) {
	runner, m, digest, dir := moduleRunnerFixture(t, "pass")
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Invocations) != 2 {
		t.Fatalf("pass receipt %s %+v %v", receipt.Result, receipt.Invocations, receipt.Incomplete)
	}
	module := receipt.Invocations[1]
	if module.Dir != "sub/mod" || len(module.Outcomes) != 1 || module.Outcomes[0].Unit != ModuleUnitPrefix+"sub/mod" || module.Outcomes[0].Outcome != OutcomePass {
		t.Fatalf("module invocation %+v", module)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	argv, _ := os.ReadFile(filepath.Join(dir, "argv"))
	want := real + " GOWORK= test -json -p 1 -timeout 30m -count=1 m/a\n" +
		filepath.Join(real, "sub", "mod") + " GOWORK=off test -json -p 1 -timeout 30m -count=1 ./...\n"
	if string(argv) != want {
		t.Fatalf("argv\n%s\nwant\n%s", argv, want)
	}

	runner, m, digest, _ = moduleRunnerFixture(t, "modfail")
	failed := runner.Execute(context.Background(), m, digest, 0)
	if failed.Result != ResultFail || len(failed.Invocations) != 2 || failed.Invocations[1].Outcomes[0].Outcome != OutcomeFail {
		t.Fatalf("fail receipt %s %+v", failed.Result, failed.Invocations)
	}

	runner, m, digest, dir = moduleRunnerFixture(t, "listfail")
	refused := runner.Execute(context.Background(), m, digest, 0)
	if refused.Result != ResultIncomplete || !strings.Contains(strings.Join(refused.Incomplete, "\n"), "go list ./... failed in sub/mod; the nested module cannot be confirmed") {
		t.Fatalf("listfail receipt %s %v", refused.Result, refused.Incomplete)
	}
	argv, _ = os.ReadFile(filepath.Join(dir, "argv"))
	if strings.Contains(string(argv), "./...") {
		t.Fatalf("the unconfirmed module launched: %q", argv)
	}
}
