package intent

import (
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func waitPools(t *testing.T, fields string) ([]Pool, error) {
	t.Helper()
	var shape any
	if e := json.Unmarshal([]byte(`[{"id":"db","members":["a"]`+fields+`}]`), &shape); e != nil {
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

// PSR-V0-131, PSR-V0-136: maxWaitSeconds is 1..600 and handoffGraceSeconds
// 0..600; both are optional and default to 0 (no waitlist, no grace).
func TestPSRV0131_PoolWaitPolicyBounds(t *testing.T) {
	got, e := waitPools(t, "")
	if e != nil || got[0].MaxWaitSeconds != 0 || got[0].HandoffGraceSeconds != 0 {
		t.Fatalf("defaults %+v %v", got, e)
	}
	got, e = waitPools(t, `,"maxWaitSeconds":"600","handoffGraceSeconds":"0"`)
	if e != nil || got[0].MaxWaitSeconds != MaxPoolWaitSeconds || got[0].HandoffGraceSeconds != 0 {
		t.Fatalf("bounds %+v %v", got, e)
	}
	for _, bad := range []string{`,"maxWaitSeconds":"0"`, `,"maxWaitSeconds":"601"`, `,"handoffGraceSeconds":"601"`} {
		if _, e := waitPools(t, bad); wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("%s: %v", bad, e)
		}
	}
}
