package companionrelease

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// stableAcceptResult is the public neutral-criterion ACCEPT result line, an
// exact cem-stable-verification/1 result whose reference integrity passes.
const stableAcceptResult = "../../protocol/cem-1.0/stable/neutral-criterion/results/accept.json"

func stableResultMembersOf(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(stableAcceptResult)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		t.Fatal(err)
	}
	return members
}

// emitStableResult writes members in order; reseal recomputes verdictSha256 so
// a mutation is refused for its own class rather than for a stale verdict.
func emitStableResult(t *testing.T, members map[string]json.RawMessage, order []string, reseal bool) []byte {
	t.Helper()
	encode := func() []byte {
		var b bytes.Buffer
		b.WriteByte('{')
		for i, name := range order {
			if i > 0 {
				b.WriteByte(',')
			}
			key, _ := json.Marshal(name)
			b.Write(key)
			b.WriteByte(':')
			b.Write(members[name])
		}
		b.WriteString("}\n")
		return b.Bytes()
	}
	if reseal {
		decoder := json.NewDecoder(bytes.NewReader(encode()))
		decoder.UseNumber()
		var r verify.StableResult
		if err := decoder.Decode(&r); err != nil {
			t.Fatal(err)
		}
		digest, err := r.VerdictDigest()
		if err != nil {
			t.Fatal(err)
		}
		members["verdictSha256"], _ = json.Marshal(digest)
	}
	return encode()
}

func completeStableOutcomesFixture() StableReleaseOutcomes {
	return StableReleaseOutcomes{Required: []string{"go-test"}, Observed: map[string]string{"go-test": "PASSED"}}
}

func TestCEMV1569StableReleaseReadiness(t *testing.T) {
	order := append([]string(nil), stableResultMembers...)
	type mutation struct {
		members func(map[string]json.RawMessage)
		order   []string
		reseal  bool
		release func(*StableReleaseEvidence)
	}
	setAxis := func(axis, value string) func(map[string]json.RawMessage) {
		return func(m map[string]json.RawMessage) {
			var axes map[string]string
			_ = json.Unmarshal(m["axes"], &axes)
			axes[axis] = value
			m["axes"], _ = json.Marshal(axes)
		}
	}
	set := func(name, value string) func(map[string]json.RawMessage) {
		return func(m map[string]json.RawMessage) { m[name] = json.RawMessage(value) }
	}
	swapped := append([]string(nil), order...)
	swapped[0], swapped[1] = swapped[1], swapped[0]
	cases := []struct {
		name  string
		m     mutation
		class string
		// integrity marks cases whose result keeps passing reference integrity.
		integrity bool
	}{
		{name: "accepted exact stable result", integrity: true},
		{name: "unknown member", m: mutation{members: set("extra", `true`), order: append(append([]string(nil), order...), "extra"), reseal: true}, class: StableRefusalSchema},
		{name: "missing member", m: mutation{order: order[:len(order)-2]}, class: StableRefusalSchema},
		{name: "member order", m: mutation{order: swapped}, class: StableRefusalSchema},
		{name: "stale verdict", m: mutation{members: set("stage", `"drift"`)}, class: StableRefusalSchema},
		{name: "unknown axis", m: mutation{members: setAxis("releaseApproval", "NOT_OBSERVED"), reseal: true}, class: StableRefusalSchema},
		{name: "wrong profile", m: mutation{members: set("profile", `"cem-stable-verification/2"`), reseal: true}, class: StableRefusalMode},
		{name: "retired spec", m: mutation{members: set("spec", `"cem/0.3"`), reseal: true}, class: StableRefusalMode},
		{name: "other verification mode", m: mutation{members: set("verificationMode", `"structural-only"`), reseal: true}, class: StableRefusalMode},
		{name: "native authority raised", m: mutation{members: setAxis("nativeAuthority", "VERIFIED"), reseal: true}, class: StableRefusalAuthority, integrity: true},
		{name: "runner execution raised", m: mutation{members: setAxis("runnerExecution", "PASSED"), reseal: true}, class: StableRefusalAuthority, integrity: true},
		{name: "experimental structural tuple", m: mutation{members: set("runtime", `{"goVersion":"go1.27.1","structuralQualification":"EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS"}`)}, class: StableRefusalAdmission, integrity: true},
		{name: "unsupported structural runtime", m: mutation{members: set("runtime", `{"goVersion":"go1.26.0","structuralQualification":"UNSUPPORTED"}`)}, class: StableRefusalAdmission, integrity: true},
		{name: "unadmitted repository envelope", m: mutation{members: set("repositoryEnvelope", `"linked-worktree/0"`)}, class: StableRefusalAdmission, integrity: true},
		{name: "unsupported outcome", m: mutation{members: func(m map[string]json.RawMessage) {
			m["outcome"], m["accept"], m["stage"], m["code"] = json.RawMessage(`"UNSUPPORTED"`), json.RawMessage(`null`), json.RawMessage(`"runtime"`), json.RawMessage(`"unsupported-structural-runtime"`)
		}, reseal: true}, class: StableRefusalAdmission, integrity: true},
		{name: "rejected result", m: mutation{members: func(m map[string]json.RawMessage) {
			m["outcome"], m["accept"], m["stage"] = json.RawMessage(`"REJECT"`), json.RawMessage(`false`), json.RawMessage(`"artifacts"`)
			setAxis("referenceIntegrity", "FAILED")(m)
		}, reseal: true}, class: StableRefusalNotAccept},
		{name: "runner row failed", m: mutation{release: func(e *StableReleaseEvidence) { e.Runner.Observed = map[string]string{"go-test": "FAILED"} }}, class: StableRefusalRunner, integrity: true},
		{name: "runner row skipped", m: mutation{release: func(e *StableReleaseEvidence) { e.Runner.Observed = map[string]string{"go-test": "SKIPPED"} }}, class: StableRefusalRunner, integrity: true},
		{name: "runner row missing", m: mutation{release: func(e *StableReleaseEvidence) { e.Runner.Required = []string{"go-test", "vitest"} }}, class: StableRefusalRunner, integrity: true},
		{name: "platform not observed", m: mutation{release: func(e *StableReleaseEvidence) {
			e.Platform = StableReleaseOutcomes{Required: []string{"linux-amd64", "windows-amd64"}, Observed: map[string]string{"linux-amd64": "PASSED", "windows-amd64": "NOT_OBSERVED"}}
		}}, class: StableRefusalPlatform, integrity: true},
		{name: "external undeclared", m: mutation{release: func(e *StableReleaseEvidence) { e.External = StableReleaseOutcomes{} }}, class: StableRefusalExternal, integrity: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			members := stableResultMembersOf(t)
			if c.m.members != nil {
				c.m.members(members)
			}
			o := order
			if c.m.order != nil {
				o = c.m.order
			}
			e := StableReleaseEvidence{Result: emitStableResult(t, members, o, c.m.reseal), Runner: completeStableOutcomesFixture(), Platform: completeStableOutcomesFixture(), External: completeStableOutcomesFixture()}
			if c.m.release != nil {
				c.m.release(&e)
			}
			if c.integrity && !bytes.Contains(e.Result, []byte(`"referenceIntegrity":"REFERENCE_INTEGRITY_ONLY"`)) {
				t.Fatal("fixture no longer passes reference integrity")
			}
			err := ValidateStableReleaseEvidence(e)
			if c.class == "" {
				if err != nil {
					t.Fatalf("exact accepted result refused: %v", err)
				}
				return
			}
			var refusal *StableReleaseRefusal
			if !errors.As(err, &refusal) || refusal.Class != c.class {
				t.Fatalf("got %v, want class %s", err, c.class)
			}
			if !strings.HasPrefix(err.Error(), c.class+": ") {
				t.Fatalf("refusal text %q does not lead with its class", err)
			}
		})
	}
}
