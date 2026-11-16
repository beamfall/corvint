// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cliapp"
)

// CAP-V0-976 (V1-5635): `corvint flows status --help` carries the result-source rule the flows
// topic states, on both the committed-snapshot route and the authored dispatch, and a node with
// no description names its parent help topic instead.
func TestCAPV0976FlowsStatusNodePageCarriesResultSourceRule(t *testing.T) {
	t.Parallel()
	var snapshot, stderr bytes.Buffer
	if code, handled := cliapp.Answer("corvint", []string{"flows", "status", "--help"}, &snapshot, &stderr); !handled || code != 0 || stderr.Len() != 0 {
		t.Fatalf("snapshot route: handled=%t code=%d stderr=%q", handled, code, &stderr)
	}
	for name, got := range map[string]string{
		"snapshot":   snapshot.String(),
		"--help":     runCatalogHelp(t, "flows", "status", "--help"),
		"help flows": runCatalogHelp(t, "help", "flows", "status"),
	} {
		for _, want := range []string{"run evidence store", "--no-store", "--evidence FILE", "test-run-evidence/0",
			"NOT_OBSERVED", "result_sources", "\n\nUsage:\n"} {
			if !strings.Contains(got, want) {
				t.Fatalf("%s: flows status help lacks %q:\n%s", name, want, got)
			}
		}
		if strings.Contains(got, "Full topic:") {
			t.Fatalf("%s: described node also names a parent topic:\n%s", name, got)
		}
	}
	if got := runCatalogHelp(t, "affected", "audit", "--help"); !strings.HasSuffix(got, "Full topic: corvint help affected\n") {
		t.Fatalf("undescribed node names no parent topic:\n%s", got)
	}
}
