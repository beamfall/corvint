package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func runTestValidityCLI(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"test-validity"}, arguments...), strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeTestValidityInput(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "provider.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// LPCV-V0-051: the main binary recomputes each test's projection from the
// receipt; a carried projection claiming strength is ignored, and a passing
// test keeps strength NOT_MEASURED (LPCV-V0-048).
func TestTestValidityProjectsReceiptWithoutTrustingCarriedProjections(t *testing.T) {
	t.Parallel()
	path := writeTestValidityInput(t, `{"receipt":{"kind":"unit","tests":[{"name":"adds","state":"passed"}]},
"testProjections":[{"projection":{"strength":{"state":"KILLED"}}}],"runProjection":{}}`)
	code, stdout, stderr := runTestValidityCLI(t, "--receipt", path)
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var document testValidityDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	if document.Schema != "corvint-test-validity/0" || document.Kind != "unit" || len(document.Tests) != 1 {
		t.Fatalf("document=%+v", document)
	}
	projection := document.Tests[0].Projection
	if projection.Execution.State != testvalidity.ExecutionPassed || projection.Strength.State != testvalidity.StrengthNotMeasured {
		t.Fatalf("projection=%+v", projection)
	}
}

// LPCV-V0-051/LPCV-V0-049: with no receipt there is no evidence, so every run
// axis is UNSUPPORTED rather than a default pass.
func TestTestValidityWithoutReceiptAbstainsOnEveryAxis(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runTestValidityCLI(t)
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	var document testValidityDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	run := document.Run
	for _, axis := range []testvalidity.Axis{run.Association, run.Hygiene, run.Freshness, run.Execution, run.Strength} {
		if axis.State != testvalidity.StateUnsupported || axis.Reason != "no-input-supplied" {
			t.Fatalf("axis=%+v document=%s", axis, stdout)
		}
	}
	if document.Source != "none" || document.Tests == nil || len(document.Tests) != 0 {
		t.Fatalf("document=%s", stdout)
	}
}

// LPCV-V0-051: an input that is not a closed provider receipt is refused with
// no projection on stdout.
func TestTestValidityRefusesNonReceiptInput(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"no receipt member":  `{"testProjections":[],"runProjection":{}}`,
		"unknown field":      `{"receipt":{"kind":"unit","tests":[],"valid":true}}`,
		"unknown kind":       `{"receipt":{"kind":"lint","tests":[]}}`,
		"unknown profile":    `{"profile":"other-provider/0"}`,
		"ambiguous provider": `{"profile":"corvint-go-live-session-event/0","receipt":{"kind":"unit","tests":[]}}`,
		"trailing data":      `{"receipt":{"kind":"unit","tests":[]}} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runTestValidityCLI(t, "--receipt", writeTestValidityInput(t, body))
			if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-test-validity-receipt"`) {
				t.Fatalf("exit=%d stdout=%q stderr=%s", code, stdout, stderr)
			}
		})
	}
}
