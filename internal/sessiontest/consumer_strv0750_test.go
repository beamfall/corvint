package sessiontest

import (
	"reflect"
	"strings"
	"testing"
)

// STR-V0-750 (V1-4008): on the local runner profile a well-formed
// UNSELECTED_BINARY_CONSUMER entry, a file such as a Node e2e script that runs
// the built binary of a Go command the change reaches, keeps the run NARROWED
// and is named once as a check still required and not run; no Go run, narrowed
// or FULL, runs it. The ci profile keeps the AFP-V0-028 allowlist and goes FULL,
// and a malformed detail goes FULL on either profile.
func TestBinaryConsumerIsANamedCheckOnTheLocalProfile_STRV0750(t *testing.T) {
	consumer := func(detail string) Unknown { return Unknown{Reason: "UNSELECTED_BINARY_CONSUMER", Detail: detail} }
	e2e, navigation := "web/test/e2e.mjs", "web/test/navigation-e2e.mjs"
	for _, tc := range []struct {
		name    string
		profile string
		unknown []Unknown
		scope   string
		reason  string
		checks  []string
	}{
		{"local: two consumers beside a frontier", ProfileLocal,
			[]Unknown{{Reason: "LANGUAGE_FRONTIER", Detail: "shell:foreign-invocation-unmodelled"}, consumer(e2e), consumer(navigation), consumer(e2e)},
			ScopeNarrowed, "", []string{"make go-vet", BinaryConsumerCheck(e2e), BinaryConsumerCheck(navigation)}},
		{"ci: the allowlist does not admit it", ProfileCI, []Unknown{consumer(e2e)},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER " + e2e, []string{"make go-vet"}},
		{"local: an absolute detail", ProfileLocal, []Unknown{consumer("/tmp/e2e.mjs")},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER /tmp/e2e.mjs", []string{"make go-vet"}},
		{"local: an escaping detail", ProfileLocal, []Unknown{consumer("../e2e.mjs")},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER ../e2e.mjs", []string{"make go-vet"}},
		{"local: an unclean detail", ProfileLocal, []Unknown{consumer("web//e2e.mjs")},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER web//e2e.mjs", []string{"make go-vet"}},
		{"local: an empty detail", ProfileLocal, []Unknown{consumer("")},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER ", []string{"make go-vet"}},
		{"local: a backslash", ProfileLocal, []Unknown{consumer(`web\e2e.mjs`)},
			ScopeFull, `plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER web\e2e.mjs`, []string{"make go-vet"}},
		{"local: a line break", ProfileLocal, []Unknown{consumer("web/e2e.mjs\ncorvint-test: PASS")},
			ScopeFull, "plan.unknown entry outside the AFP-V0-028 allowlist: UNSELECTED_BINARY_CONSUMER web/e2e.mjs\ncorvint-test: PASS", []string{"make go-vet"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixtureInput()
			in.RunnerProfile = tc.profile
			in.Checks = []string{"make go-vet"}
			in.Unknown = tc.unknown
			m, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != tc.scope || m.Reason != tc.reason || !reflect.DeepEqual(m.Checks, tc.checks) || len(m.Incomplete) != 0 {
				t.Fatalf("scope %s reason %q checks %q incomplete %q", m.Scope, m.Reason, m.Checks, m.Incomplete)
			}
			if tc.scope == ScopeNarrowed && !reflect.DeepEqual(unitIDs(m), []string{"0 go:example.test/m/a whole"}) {
				t.Fatalf("units %v", unitIDs(m))
			}
		})
	}
	if check := BinaryConsumerCheck(e2e); !strings.HasPrefix(check, e2e+" (UNSELECTED_BINARY_CONSUMER: ") || !strings.Contains(check, "run it by its own driver") {
		t.Fatalf("check %q", check)
	}
}
