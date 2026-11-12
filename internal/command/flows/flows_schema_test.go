// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// FSV-V1-005 (V1-5410): `flows gaps`, `flows status` and `flows impact`, the
// reports that join flows with test evidence or results, exit non-zero with
// the shared reader's reason and write no report when it refuses the flows
// declaration; with no declaration or a version 0 one they print their
// report.
func TestFSVV1005FlowReportsRefuseWithoutPartialOutput_V15410(t *testing.T) {
	fx := newShopFixture(t)
	reports := map[string][]string{
		"gaps":   {"gaps", "--flows", "flows", "--evidence", fx.evidence},
		"status": {"status", "--flows", "flows", "--evidence", fx.evidence},
		"impact": {"impact", "--flows", "flows", "--base", fx.b},
	}
	declaration := filepath.Join(fx.root, filepath.FromSlash(appflows.FlowsDeclarationPath))
	write := func(body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(declaration), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(declaration, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plain := map[string]string{}
	for name, args := range reports {
		code, out, diagnostic := runFlowsCLI(fx.root, args...)
		if code != 0 || out == "" {
			t.Fatalf("%s without a declaration exited %d: %s", name, code, diagnostic)
		}
		plain[name] = out
	}
	write(`{"profile":"corvint-flows/0","flows":"flows"}`)
	for name, args := range reports {
		// impact counts the untracked declaration as a changed path, so only
		// gaps and status must print the very same bytes.
		if code, out, diagnostic := runFlowsCLI(fx.root, args...); code != 0 || out == "" || name != "impact" && out != plain[name] {
			t.Fatalf("%s with a version 0 declaration exited %d or changed its report: %s", name, code, diagnostic)
		}
	}
	for _, tc := range []struct{ body, code, reason, detail string }{
		{`{"profile":"corvint-flows/7","flows":"flows","variants":[]}`, "flows-schema-unsupported", "FLOWS_SCHEMA_UNSUPPORTED", "schema version 7"},
		{`{"profile":"flows/1","flows":"flows"}`, "flows-declaration-invalid", "FLOWS_DECLARATION_INVALID", `profile "flows/1"`},
		{`{"profile":"corvint-flows/0","flows":"flows"`, "flows-declaration-invalid", "FLOWS_DECLARATION_INVALID", appflows.FlowsDeclarationPath},
	} {
		write(tc.body)
		for name, args := range reports {
			code, out, diagnostic := runFlowsCLI(fx.root, args...)
			if code == 0 || out != "" {
				t.Fatalf("%s on %s exited %d with output %q", name, tc.body, code, out)
			}
			var envelope struct{ Code, Error string }
			if err := json.Unmarshal([]byte(diagnostic), &envelope); err != nil {
				t.Fatalf("%s on %s: diagnostic %q: %v", name, tc.body, diagnostic, err)
			}
			if envelope.Code != tc.code || !strings.HasPrefix(envelope.Error, tc.reason+": "+appflows.FlowsDeclarationPath+": ") || !strings.Contains(envelope.Error, tc.detail) {
				t.Fatalf("%s on %s: %+v", name, tc.body, envelope)
			}
		}
	}
}
