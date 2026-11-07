// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package sessiontest

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-9923 (Greptile P1 on PR #125): when the graph fails, the FULL
// fallback runs the declared Go suite, `go test ./...`, which skips dot and
// underscore directories. A failing test there must keep the run from
// passing; with no hidden Go source the fallback still runs complete. Another
// declared command that runs `go` keeps its own discovery and still runs. The
// real Go toolchain runs, so a skipped directory is observed, not assumed.
func TestNoPlanFullConfirmsHiddenGo_AZU_V0_9923(t *testing.T) {
	const failing = "import \"testing\"\nfunc TestHidden(t *testing.T) { t.Fatal(\"hidden test ran\") }\n"
	invalid := `{"profile":"corvint-non-source-paths/0","entries":[{"path":"_out/","reason":"r"},{"path":"_out/","reason":"r"}]}`
	valid := `{"profile":"corvint-non-source-paths/0","entries":[{"path":"_out/","reason":"generated output"}]}`
	for _, tc := range []struct {
		name, declaration string
		hidden            map[string]string
		want              string // "" is PASS
	}{
		{name: "underscore", declaration: invalid, hidden: map[string]string{"_private/private_test.go": "package private\n" + failing}, want: "_private/private_test.go"},
		{name: "github", declaration: invalid, hidden: map[string]string{".github/helper/helper.go": "package helper\n", ".github/helper/helper_test.go": "package helper\n" + failing}, want: ".github/helper/helper.go"},
		{name: "nested hidden", declaration: invalid, hidden: map[string]string{"tools/.gen/gen_test.go": "package gen\n" + failing}, want: "tools/.gen/gen_test.go"},
		// An invalid declaration holds nothing, so even declared output is
		// unconfirmed; a valid one keeps it output, as the Go plugin does.
		{name: "invalid declaration holds nothing", declaration: invalid, hidden: map[string]string{"_out/dump.go": "package dump\n"}, want: "_out/dump.go"},
		{name: "declared output", declaration: valid, hidden: map[string]string{"_out/dump.go": "package dump\n"}},
		{name: "testdata fixture", declaration: invalid, hidden: map[string]string{"ordinary/testdata/_fixture/fixture.go": "package fixture\n"}},
		{name: "no hidden source", declaration: invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                    "module example.test/m\n\ngo 1.27.3\n",
				"ordinary/ordinary.go":      "package ordinary\n",
				"ordinary/ordinary_test.go": "package ordinary\nimport \"testing\"\nfunc TestOrdinary(t *testing.T) {}\n",
				affected.NonSourcePathsPath: tc.declaration,
			}
			for name, body := range tc.hidden {
				files[name] = body
			}
			for name, body := range files {
				full := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			planError := "base unresolved"
			if tc.declaration == invalid {
				_, err := affected.Build(context.Background(), root, golang.New())
				if err == nil || !strings.Contains(err.Error(), affected.NonSourcePathsPath) {
					t.Fatalf("the graph accepted an invalid declaration: %v", err)
				}
				planError = err.Error()
			}
			source := SourceIdentity{Head: strings.Repeat("b", 40), Paths: []PathDigest{}, Tree: strings.Repeat("c", 40)}
			declared := parsedDeclared()
			declared.Languages = []LanguageCommand{{Language: "vet", Argv: []string{"go", "vet", "./ordinary"}}}
			m, err := Build(Input{
				Base: strings.Repeat("a", 40), Declared: declared, PlanError: planError,
				RunnerProfile: ProfileLocal, ShardCount: 1, Source: source,
			})
			if err != nil || m.Scope != ScopeFull || len(m.Units) != 2 || m.Units[0].Unit != "declared:go" || m.Units[1].Unit != "declared:vet" {
				t.Fatalf("no-plan manifest %+v: %v", m, err)
			}
			digest, _ := Digest(m)
			runner := &Runner{
				Declared: declared, Environ: append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local"),
				Errors: io.Discard, Output: io.Discard, Root: root, Tool: "corvint test",
				Recheck: func(context.Context) (SourceIdentity, error) { return source, nil },
				// No plan, no Go universe: cmd/corvint leaves it nil.
			}
			receipt := runner.Execute(context.Background(), m, digest, 0)
			if tc.want == "" {
				if receipt.Result != ResultPass || len(receipt.Invocations) != 2 || receipt.Invocations[0].Exit != 0 || receipt.Invocations[1].Exit != 0 {
					t.Fatalf("a complete fallback did not pass: %s %v %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
				}
				return
			}
			if receipt.Result == ResultPass {
				t.Fatalf("FULL fallback passed over hidden Go source %s: %+v", tc.want, receipt.Invocations)
			}
			reasons := strings.Join(receipt.Incomplete, "\n")
			if receipt.Result != ResultIncomplete || !strings.Contains(reasons, "declared:go: no plan gave a Go universe") || !strings.Contains(reasons, tc.want) {
				t.Fatalf("receipt %s, incomplete %q; want INCOMPLETE naming %s", receipt.Result, reasons, tc.want)
			}
			// Only the suite is withdrawn: the declared vet command runs.
			if len(receipt.Invocations) != 1 || receipt.Invocations[0].Outcomes[0].Unit != "declared:vet" || receipt.Invocations[0].Exit != 0 {
				t.Fatalf("invocations %+v; want only declared:vet", receipt.Invocations)
			}
		})
	}
}

// AZU-V0-9923: with a planned universe the declared Go suite shares the root
// Go adapter's hidden-package helper: discovery lists and validates ./...
// plus the hidden packages, and the suite's argv names them too. A listing
// that differs from the universe, including packages under an empty one,
// launches nothing.
func TestDeclaredGoSuiteSharesHiddenUniverse_AZU_V0_9923(t *testing.T) {
	const declaredGo = `#!/bin/sh
case "$1" in
env) echo go1.27.3; exit 0;;
list)
  echo "$*" > "$FAKE_DIR/list-argv"
  [ "$FAKE_MODE" = listfail ] && exit 1
  echo m/a
  [ "$FAKE_MODE" = extra ] && echo m/unplanned
  for arg in "$@"; do
    case "$arg" in m/_private) echo "$arg";; esac
  done
  exit 0;;
esac
echo "$*" > "$FAKE_DIR/argv"
`
	for _, tc := range []struct {
		mode     string
		universe []string
		list     string
		want     string // "" is PASS
	}{
		{mode: "pass", universe: []string{"m/a", "m/_private"}, list: "list ./... m/_private"},
		{mode: "extra", universe: []string{"m/a", "m/_private"}, list: "list ./... m/_private", want: "declared:go: go list ./... differs from the planned Go universe: 1 unplanned [m/unplanned], 0 unlisted []"},
		{mode: "listfail", universe: []string{"m/a", "m/_private"}, list: "list ./... m/_private", want: "declared:go: go list ./... failed; the Go universe cannot be confirmed"},
		{mode: "empty", universe: []string{}, list: "list ./...", want: "declared:go: go list ./... differs from the planned Go universe: 1 unplanned [m/a], 0 unlisted []"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			runner, m, _, dir := runnerFixture(t, tc.mode)
			if err := execfile.WriteFile(filepath.Join(dir, "go"), []byte(declaredGo), 0o755); err != nil {
				t.Fatal(err)
			}
			runner.Universe = tc.universe
			m.Scope, m.Units = ScopeFull, []Unit{{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:go"}}
			digest, _ := Digest(m)
			receipt := runner.Execute(context.Background(), m, digest, 0)
			if probe, err := os.ReadFile(filepath.Join(dir, "list-argv")); err != nil || string(probe) != tc.list+"\n" {
				t.Fatalf("universe probe %q: %v", probe, err)
			}
			argv, err := os.ReadFile(filepath.Join(dir, "argv"))
			if tc.want != "" {
				if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 0 || !errors.Is(err, os.ErrNotExist) || receipt.Incomplete[0] != tc.want {
					t.Fatalf("unconfirmed suite ran: %s %v %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
				}
				return
			}
			if err != nil || string(argv) != "test -p 1 -timeout 30m -count=1 ./... m/_private\n" {
				t.Fatalf("suite argv %q: %v", argv, err)
			}
			if receipt.Result != ResultPass || len(receipt.Invocations) != 1 || strings.Join(receipt.Invocations[0].Argv, " ") != "go test -p 1 -timeout 30m -count=1 ./... m/_private" {
				t.Fatalf("receipt %s %+v", receipt.Result, receipt.Invocations)
			}
		})
	}
}
