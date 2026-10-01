package native

import (
	"bytes"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"regexp"
	"testing"
)

func TestMTPConflictingEvidence(t *testing.T) {
	b, e := os.ReadFile("testdata/mtp/mstest/pass.trx")
	if e != nil {
		t.Fatal(e)
	}
	result := regexp.MustCompile(`<UnitTestResult[^>]+/>`)
	cases := map[string][]byte{
		"namespaced-outcome-overwrites-failure": bytes.Replace(b, []byte(`outcome="Passed"`), []byte(`outcome="Failed" xmlns:p="urn:other" p:outcome="Passed"`), 1),
		"passed-row-native-error-info": result.ReplaceAllFunc(b, func(x []byte) []byte {
			return append(bytes.Clone(bytes.TrimSuffix(x, []byte("/>"))), []byte(`><Output><ErrorInfo><Message>setup unavailable</Message></ErrorInfo></Output></UnitTestResult>`)...)
		}),
		"unknown-summary-fatal-error": bytes.Replace(b, []byte(`</ResultSummary>`), []byte(`<FatalError message="discovery lost"/></ResultSummary>`), 1),
		"summary-native-error-info":   bytes.Replace(b, []byte(`</ResultSummary>`), []byte(`<Output><ErrorInfo><Message>fixture unavailable</Message></ErrorInfo></Output></ResultSummary>`), 1),
	}
	for n, data := range cases {
		t.Run(n, func(t *testing.T) {
			o, e := Parse(tr.Input{Runner: "dotnet-mtp-mstest", ExitCode: 0, Reports: map[string][]byte{"r": data}})
			t.Logf("error=%v observation=%+v", e, o)
			if e == nil && o.Complete {
				t.Fatalf("contradictory or unknown failure evidence accepted")
			}
		})
	}
}
