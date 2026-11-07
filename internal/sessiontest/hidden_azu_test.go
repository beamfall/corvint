// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package sessiontest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// AZU-V0-005: hidden packages execute only after the wildcard plus explicit
// package probe confirms the entire universe; extra or absent packages refuse.
func TestAZUV0005RunnerExecutesHiddenPackages(t *testing.T) {
	for _, mode := range []string{"pass", "extra", "missing", "listfail"} {
		t.Run(mode, func(t *testing.T) {
			runner, m, _, dir := runnerFixture(t, mode)
			const hiddenGo = `#!/bin/sh
case "$1" in
env) echo go1.27.3; exit 0;;
list)
  echo "$*" > "$FAKE_DIR/list-argv"
  [ "$FAKE_MODE" = listfail ] && exit 1
  echo m/a
  [ "$FAKE_MODE" = extra ] && echo m/unplanned
  [ "$FAKE_MODE" = missing ] && exit 0
  for arg in "$@"; do
    case "$arg" in m/.github/pkg|m/_private) echo "$arg";; esac
  done
  exit 0;;
esac
echo "$*" > "$FAKE_DIR/argv"
for arg in "$@"; do
  case "$arg" in m/*) echo "{\"Action\":\"pass\",\"Package\":\"$arg\"}";; esac
done
`
			if err := execfile.WriteFile(filepath.Join(dir, "go"), []byte(hiddenGo), 0o755); err != nil {
				t.Fatal(err)
			}
			runner.Universe = []string{"m/a", "m/.github/pkg", "m/_private"}
			for _, pkg := range runner.Universe[1:] {
				m.Units = append(m.Units, Unit{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:" + pkg})
			}
			digest, _ := Digest(m)
			receipt := runner.Execute(context.Background(), m, digest, 0)
			probe, err := os.ReadFile(filepath.Join(dir, "list-argv"))
			if err != nil || string(probe) != "list ./... m/.github/pkg m/_private\n" {
				t.Fatalf("universe probe %q: %v", probe, err)
			}
			if mode != "pass" {
				if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 0 || !strings.Contains(strings.Join(receipt.Incomplete, "\n"), "Go universe") {
					t.Fatalf("unconfirmed universe executed: %+v", receipt)
				}
				return
			}
			if receipt.Result != ResultPass || len(receipt.Invocations) != 1 || len(receipt.Invocations[0].Outcomes) != 3 {
				t.Fatalf("hidden packages did not execute: %+v", receipt)
			}
			argv, err := os.ReadFile(filepath.Join(dir, "argv"))
			if err != nil || !strings.HasSuffix(string(argv), "m/a m/.github/pkg m/_private\n") {
				t.Fatalf("execution argv %q: %v", argv, err)
			}
		})
	}
}
