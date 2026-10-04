package intent

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func TestPSRSafeReuseBounds(t *testing.T) {
	for _, test := range []struct {
		timeout, attempts, literal string
		valid                      bool
	}{{"1", "1", "literal.*", true}, {"900", "2", "yes", true}, {"0", "1", "yes", false}, {"901", "1", "yes", false}, {"1", "3", "yes", false}, {"1", "1", strings.Repeat("x", 4097), false}} {
		raw := `{"argv":["/bin/true"],"envKeys":[],"maxAttempts":"` + test.attempts + `","timeoutSeconds":"` + test.timeout + `","verify":{"argv":["/bin/true"],"envKeys":[],"expectExit":"0","expectStdout":"` + test.literal + `"}}`
		v, e := wire.Parse([]byte(raw + "\n"))
		if e != nil {
			t.Fatal(e)
		}
		r := wire.NewReader(v, "safeReuse")
		s := readSafeReuse(r, nil)
		if (r.Err() == nil) != test.valid {
			t.Fatalf("%+v: %v", test, r.Err())
		}
		if test.valid && s.ExpectStdout != test.literal {
			t.Fatal("literal changed")
		}
	}
}

func TestPSRPolicyLegacyAndOccupiedDefinition(t *testing.T) {
	legacy := []byte("{\"pools\":[{\"id\":\"db\",\"members\":[\"a\"]}]}\n")
	v, e := wire.Parse(legacy)
	if e != nil {
		t.Fatal(e)
	}
	pools, _ := v.Obj.Get("pools")
	r := wire.NewReader(pools, "pools")
	decoded := readPools(r, nil)
	if r.Err() != nil || decoded[0].MemberConfig["a"].SafeReuse != nil || string(wire.EncodeFile(v)) != string(legacy) {
		t.Fatal("legacy bytes/config changed", r.Err())
	}
	p := &Policy{Raw: legacy, Pools: decoded}
	want := wire.Sum([]byte("{\"member\":\"a\",\"memberConfig\":null,\"pool\":\"db\",\"reservedFor\":null}\n"))
	if p.MemberDefinition("db", "a") != want {
		t.Fatal("legacy member definition changed")
	}
	valid := `{"argv":["/bin/true"],"envKeys":[],"maxAttempts":"2","timeoutSeconds":"900","verify":{"argv":["/bin/true"],"envKeys":[],"expectExit":"255","expectStdout":"literal.*"}}`
	for _, tc := range []struct {
		name, raw string
		ok        bool
	}{
		{"maximum", valid, true}, {"minimum", strings.ReplaceAll(strings.ReplaceAll(valid, `"900"`, `"1"`), `"255"`, `"0"`), true},
		{"null", "null", false}, {"unknown", strings.Replace(valid, `"argv":`, `"extra":true,"argv":`, 1), false},
		{"missing-timeout", strings.Replace(valid, `"timeoutSeconds":"900",`, "", 1), false},
		{"null-verify", strings.Replace(valid, `{"argv":["/bin/true"],"envKeys":[],"expectExit":"255","expectStdout":"literal.*"}`, "null", 1), false},
		{"exit-overflow", strings.Replace(valid, `"255"`, `"256"`, 1), false},
		{"timeout-overflow", strings.Replace(valid, `"900"`, `"901"`, 1), false},
		{"attempt-overflow", strings.Replace(valid, `"maxAttempts":"2"`, `"maxAttempts":"3"`, 1), false},
		{"undeclared-env", strings.Replace(valid, `"envKeys":[]`, `"envKeys":["TOKEN"]`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var shape any
			if e := json.Unmarshal([]byte(`[{"id":"db","members":["a"],"memberConfig":{"a":{"safeReuse":`+tc.raw+`}}}]`), &shape); e != nil {
				t.Fatal(e)
			}
			encoded, e := json.Marshal(shape)
			if e != nil {
				t.Fatal(e)
			}
			value, e := wire.Parse(append(encoded, '\n'))
			if e != nil {
				t.Fatal(e)
			}
			reader := wire.NewReader(value, "pools")
			got := readPools(reader, nil)
			if (reader.Err() == nil) != tc.ok {
				t.Fatal(reader.Err())
			}
			if tc.ok {
				raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("pools", value)))
				next := &Policy{Raw: raw, Pools: got}
				if next.MemberDefinition("db", "a") == want {
					t.Fatal("safeReuse did not bind occupied definition")
				}
			}
		})
	}
}
