package transaction

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0020_QualificationRequiresCompletePassingRun(t *testing.T) {
	var b bytes.Buffer
	event := func(action, test string) {
		raw, err := json.Marshal(testEvent{Action: action, Package: QualificationPackage, Test: test})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	event("start", "")
	for _, test := range QualificationSuite {
		event("run", test)
		event("pass", test)
	}
	partial := b.String()
	event("pass", "")
	complete := b.String()
	fail := `{"Action":"fail","Package":"other/package","Test":"TestOther"}` + "\n"
	for _, tc := range []struct {
		name, run, code string
	}{
		{"complete", complete, ""},
		{"go 1.27 output", strings.Replace(complete, `{"Action":"run"`, `{"Action":"output","Package":"`+QualificationPackage+`","Output":"=== RUN   test\n","OutputType":"frame"}`+"\n"+`{"Action":"run"`, 1), ""},
		{"go build output", `{"Action":"build-output","ImportPath":"other/package","Output":"compiler output\n"}` + "\n" + complete, ""},
		{"go build failure", `{"Action":"build-fail","ImportPath":"other/package"}` + "\n" + complete, wire.CodeGateFailed},
		{"empty", "", wire.CodeMissingEvidence},
		{"truncated package", partial, wire.CodeMissingEvidence},
		{"other package", strings.ReplaceAll(complete, QualificationPackage, "other/package"), wire.CodeMissingEvidence},
		{"pass without run", strings.ReplaceAll(complete, `"Action":"run"`, `"Action":"output"`), wire.CodeMissingEvidence},
		{"pass then fail", complete + fail, wire.CodeGateFailed},
		{"fail then pass", fail + complete, wire.CodeGateFailed},
		{"build failed", complete + `{"Action":"build-fail","Package":"other/package"}`, wire.CodeGateFailed},
		{"skip after pass", strings.TrimSuffix(complete, "\n") + "\n" + `{"Action":"skip","Package":"` + QualificationPackage + `"}`, wire.CodeMissingEvidence},
		{"array", complete + `[]`, wire.CodeMalformed},
		{"null", complete + `null`, wire.CodeMalformed},
		{"empty object", complete + `{}`, wire.CodeMalformed},
		{"unknown action", complete + `{"Action":"success","Package":"other/package"}`, wire.CodeMalformed},
		{"wrong action type", complete + `{"Action":42,"Package":"other/package"}`, wire.CodeMalformed},
		{"duplicate action", complete + `{"Action":"fail","Action":"pass","Package":"other/package"}`, wire.CodeMalformed},
		{"case folded action", complete + `{"Action":"fail","action":"pass","Package":"other/package"}`, wire.CodeMalformed},
		{"nested output", complete + `{"Action":"output","Package":"other/package","Output":{}}`, wire.CodeMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runVerdict("decision", []byte(tc.run))
			if tc.code == "" {
				if got != nil {
					t.Fatalf("passing run refused: %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("accepted %s", tc.name)
			}
			for _, code := range got.Outcome.Codes {
				if code == tc.code {
					return
				}
			}
			t.Fatalf("want %s, got %+v", tc.code, got)
		})
	}
}
