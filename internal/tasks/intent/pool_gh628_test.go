package intent

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func gh628Pools(t *testing.T, member string) ([]Pool, error) {
	t.Helper()
	var shape any
	if e := json.Unmarshal([]byte(`[{"id":"db","members":["a"],"memberConfig":{"a":`+member+`}}]`), &shape); e != nil {
		t.Fatal(e)
	}
	canonical, e := json.Marshal(shape)
	if e != nil {
		t.Fatal(e)
	}
	v, e := wire.Parse(append(canonical, '\n'))
	if e != nil {
		t.Fatal(e)
	}
	r := wire.NewReader(v, "pools")
	got := readPools(r, nil)
	return got, r.Err()
}

// PSR-V0-011: health and cleanup accept timeoutSeconds 1..3600 at policy
// validation; 0 and above the maximum refuse with LIMIT_EXCEEDED.
func TestPSRV0011PoolCommandBound(t *testing.T) {
	for _, tc := range []struct {
		timeout string
		ok      bool
	}{{"1", true}, {"300", true}, {"301", true}, {"3600", true}, {"0", false}, {"3601", false}} {
		for _, kind := range []string{"health", "cleanup"} {
			command := `{"argv":["/bin/true"],"cwd":"REPOSITORY","env":[],"timeoutSeconds":"` + tc.timeout + `"}`
			got, e := gh628Pools(t, `{"`+kind+`":`+command+`}`)
			if (e == nil) != tc.ok {
				t.Fatalf("%s %s: %v", kind, tc.timeout, e)
			}
			if !tc.ok && wire.CodeOf(e) != wire.CodeLimitExceeded {
				t.Fatalf("%s %s: code %v", kind, tc.timeout, e)
			}
			if tc.ok && tc.timeout == "3600" {
				config := got[0].MemberConfig["a"]
				def := config.Health
				if kind == "cleanup" {
					def = config.Cleanup
				}
				if def.TimeoutSeconds.Int() != MaxPoolCommandSeconds || def.Cwd != CwdRepository || def.Pinned != nil {
					t.Fatalf("decoded %+v", def)
				}
			}
		}
	}
}

// PSR-V0-012: cwd is "REPOSITORY" or a closed pinned external worktree with a
// clean absolute path and a full object id; safeReuse cwd applies to reset and
// verify.
func TestPSRV0012PinnedCwdShape(t *testing.T) {
	rev := strings.Repeat("a", 40)
	pin := func(body string) string {
		return `{"health":{"argv":["/bin/true"],"cwd":` + body + `,"env":[],"timeoutSeconds":"3"}}`
	}
	for _, tc := range []struct {
		name, member string
		ok           bool
	}{
		{"pinned", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"` + rev + `"}`), true},
		{"pinned-sha256", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"` + strings.Repeat("b", 64) + `"}`), true},
		{"relative", pin(`{"kind":"PINNED_REPOSITORY","path":"srv/tools","revision":"` + rev + `"}`), false},
		{"unclean", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/../tools","revision":"` + rev + `"}`), false},
		{"trailing-slash", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools/","revision":"` + rev + `"}`), false},
		{"root", pin(`{"kind":"PINNED_REPOSITORY","path":"/","revision":"` + rev + `"}`), false},
		{"short-revision", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"abc123"}`), false},
		{"branch-revision", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"main"}`), false},
		{"missing-revision", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools"}`), false},
		{"wrong-kind", pin(`{"kind":"REPOSITORY","path":"/srv/tools","revision":"` + rev + `"}`), false},
		{"unknown-key", pin(`{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"` + rev + `","blob":"` + rev + `"}`), false},
		{"other-string", pin(`"WORKTREE"`), false},
		{"null", pin(`null`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := gh628Pools(t, tc.member)
			if (e == nil) != tc.ok {
				t.Fatal(e)
			}
			if tc.ok {
				def := got[0].MemberConfig["a"].Health
				if def.Cwd != CwdPinned || def.Pinned == nil || def.Pinned.Path != "/srv/tools" {
					t.Fatalf("decoded %+v", def)
				}
			}
		})
	}
	reuse := `{"safeReuse":{"argv":["/bin/true"],"cwd":{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"` + rev + `"},"envKeys":[],"maxAttempts":"1","timeoutSeconds":"3","verify":{"argv":["/bin/true"],"envKeys":[],"expectExit":"0","expectStdout":"ok"}}}`
	got, e := gh628Pools(t, reuse)
	if e != nil {
		t.Fatal(e)
	}
	s := got[0].MemberConfig["a"].SafeReuse
	if s.Reset.Pinned == nil || s.Verify.Pinned == nil || s.Reset.Pinned.Revision != rev || s.Verify.Cwd != CwdPinned {
		t.Fatalf("safeReuse cwd %+v", s)
	}
	got, e = gh628Pools(t, strings.Replace(reuse, `"cwd":{"kind":"PINNED_REPOSITORY","path":"/srv/tools","revision":"`+rev+`"},`, "", 1))
	if e != nil || got[0].MemberConfig["a"].SafeReuse.Reset.Cwd != CwdRepository || got[0].MemberConfig["a"].SafeReuse.Reset.Pinned != nil {
		t.Fatalf("omitted safeReuse cwd %v", e)
	}
}
