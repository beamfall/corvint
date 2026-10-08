package native

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// V1-0600 (TRE-V0-024): contradictory, aliased or unknown native failure
// evidence in a VSTest or MTP TRX report must refuse or stay incomplete for
// every qualified framework, while the unmodified captured report stays complete.
func TestTRXConflictingEvidence(t *testing.T) {
	const teamTest = `xmlns="http://microsoft.com/schemas/VisualStudio/TeamTest/2010"`
	passedRow := regexp.MustCompile(`<UnitTestResult [^>]*outcome="Passed"[^>]*/>`)
	firstPassedRow := func(b []byte, edit func([]byte) []byte) []byte {
		loc := passedRow.FindIndex(b)
		if loc == nil {
			return b
		}
		return append(append(bytes.Clone(b[:loc[0]]), edit(bytes.Clone(b[loc[0]:loc[1]]))...), b[loc[1]:]...)
	}
	withChildren := func(children string) func([]byte) []byte {
		return func(b []byte) []byte {
			return firstPassedRow(b, func(row []byte) []byte {
				return append(bytes.TrimSuffix(bytes.TrimSpace(row), []byte("/>")), []byte(">"+children+"</UnitTestResult>")...)
			})
		}
	}
	passedAttr := func(replacement string) func([]byte) []byte {
		return func(b []byte) []byte {
			return firstPassedRow(b, func(row []byte) []byte {
				return bytes.Replace(row, []byte(`outcome="Passed"`), []byte(replacement), 1)
			})
		}
	}
	summaryError := func(b []byte) []byte {
		// Insert native summary ErrorInfo into the existing summary Output when
		// the runner wrote one, else into a new Output; both are real TRX shapes.
		i := bytes.Index(b, []byte("<ResultSummary"))
		if i < 0 {
			return b
		}
		tail := b[i:]
		info := []byte(`<ErrorInfo><Message>fixture unavailable</Message></ErrorInfo>`)
		if j := bytes.Index(tail, []byte("</StdOut>")); j >= 0 {
			j += i + len("</StdOut>")
			return append(append(bytes.Clone(b[:j]), info...), b[j:]...)
		}
		return bytes.Replace(b, []byte(`</ResultSummary>`), append(append([]byte(`<Output>`), info...), []byte(`</Output></ResultSummary>`)...), 1)
	}
	cases := []struct {
		name     string
		mutate   func([]byte) []byte
		wantCode string // problem code suffix required when the parser does not refuse
	}{
		{"namespaced-outcome-overwrites-failure", passedAttr(`outcome="Failed" xmlns:p="urn:other" p:outcome="Passed"`), ""},
		{"same-namespace-prefixed-outcome-alias", passedAttr(`outcome="Failed" xmlns:t="http://microsoft.com/schemas/VisualStudio/TeamTest/2010" t:outcome="Passed"`), ""},
		{"undeclared-prefix-outcome", passedAttr(`outcome="Failed" p:outcome="Passed"`), ""},
		{"duplicate-outcome-attribute", passedAttr(`outcome="Failed" outcome="Passed"`), ""},
		{"xml-reserved-attribute", passedAttr(`outcome="Passed" xml:space="preserve"`), ""},
		{"child-default-namespace-redeclaration", passedAttr(`outcome="Passed" ` + teamTest), ""},
		{"prefixed-element-alias", func(b []byte) []byte {
			b = bytes.Replace(b, []byte(teamTest), []byte(teamTest+` xmlns:t="http://microsoft.com/schemas/VisualStudio/TeamTest/2010"`), 1)
			return withChildren(`<t:Output><t:ErrorInfo><t:Message>setup unavailable</t:Message></t:ErrorInfo></t:Output>`)(b)
		}, ""},
		{"foreign-namespace-error-subtree", withChildren(`<Output xmlns="urn:other"><ErrorInfo><Message>setup unavailable</Message></ErrorInfo></Output>`), ""},
		{"case-variant-outcome", passedAttr(`outcome="passed"`), "OUTCOME"},
		{"padded-outcome", passedAttr(`outcome="Passed "`), "OUTCOME"},
		{"passed-row-native-error-info", withChildren(`<Output><ErrorInfo><Message>setup unavailable</Message></ErrorInfo></Output>`), "CONTRADICTORY_ERROR"},
		{"passed-row-empty-error-info", withChildren(`<Output><ErrorInfo/></Output>`), "CONTRADICTORY_ERROR"},
		{"passed-row-stack-only-error-info", withChildren(`<Output><ErrorInfo><StackTrace>at Setup()</StackTrace></ErrorInfo></Output>`), "CONTRADICTORY_ERROR"},
		{"unknown-summary-fatal-error", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`</ResultSummary>`), []byte(`<FatalError message="discovery lost"/></ResultSummary>`), 1)
		}, ""},
		{"unknown-result-child", withChildren(`<InnerResults/>`), ""},
		{"summary-native-error-info", summaryError, "SUMMARY_ERROR"},
		{"second-summary-output", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`</ResultSummary>`), []byte(`<Output><ErrorInfo><Message>fixture unavailable</Message></ErrorInfo></Output></ResultSummary>`), 1)
		}, ""},
	}
	for _, f := range []struct {
		runner, fixture string
		exit            int
	}{
		{"dotnet-vstest-nunit", "nunit-pass.trx", 0}, {"dotnet-vstest-nunit", "nunit.trx", 1}, {"dotnet-vstest-mstest", "mstest.trx", 1}, {"dotnet-vstest-xunit", "xunit.trx", 1},
		{"dotnet-mtp-nunit", "mtp/nunit/pass.trx", 0}, {"dotnet-mtp-mstest", "mtp/mstest/pass.trx", 0}, {"dotnet-mtp-xunit", "mtp/xunit/pass.trx", 0},
		{"dotnet-mtp-mstest", "mtp/mstest/mixed2.trx", 2},
	} {
		b, err := readFixture("testdata", f.fixture)
		if err != nil {
			t.Fatal(err)
		}
		if o, e := Parse(tr.Input{Runner: f.runner, ExitCode: f.exit, Reports: map[string][]byte{"results.trx": b}}); e != nil || !o.Complete {
			t.Fatalf("%s %s: unmodified native report must stay complete: %v %+v", f.runner, f.fixture, e, o)
		}
		for _, c := range cases {
			t.Run(f.runner+"/"+f.fixture+"/"+c.name, func(t *testing.T) {
				data := c.mutate(b)
				if bytes.Equal(data, b) {
					t.Fatal("mutation did not apply to the captured report")
				}
				o, e := Parse(tr.Input{Runner: f.runner, ExitCode: f.exit, Reports: map[string][]byte{"results.trx": data}})
				if e != nil {
					return
				}
				if o.Complete {
					t.Fatalf("contradictory or unknown failure evidence accepted: %+v", o)
				}
				if c.wantCode != "" {
					for _, p := range o.Problems {
						if strings.HasSuffix(p.Code, c.wantCode) {
							return
						}
					}
					t.Fatalf("want problem *%s, got %+v", c.wantCode, o.Problems)
				}
			})
		}
	}
}
