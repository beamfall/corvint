package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

// PSR-V0-024: memberConfig.prepare is an optional pool command with the
// health/cleanup shape and 1..3600 bound; it requires safeReuse, and an
// absent prepare leaves the decoded configuration without one.
func TestPSRV0024PrepareConfig(t *testing.T) {
	const reuse = `{"argv":["/usr/bin/true"],"envKeys":[],"maxAttempts":"1","timeoutSeconds":"3","verify":{"argv":["/usr/bin/true"],"envKeys":[],"expectExit":"0","expectStdout":"x"}}`
	command := func(timeout string) string {
		return `{"argv":["/usr/bin/true"],"cwd":"REPOSITORY","env":[],"timeoutSeconds":"` + timeout + `"}`
	}
	got, e := gh628Pools(t, `{"prepare":`+command("3600")+`,"safeReuse":`+reuse+`}`)
	if e != nil || got[0].MemberConfig["a"].Prepare == nil || got[0].MemberConfig["a"].Prepare.TimeoutSeconds != "3600" {
		t.Fatalf("prepare not decoded: %+v %v", got, e)
	}
	if got, e = gh628Pools(t, `{"safeReuse":`+reuse+`}`); e != nil || got[0].MemberConfig["a"].Prepare != nil {
		t.Fatalf("absent prepare: %+v %v", got, e)
	}
	if _, e = gh628Pools(t, `{"prepare":`+command("3")+`}`); wire.CodeOf(e) != wire.CodeMalformed {
		t.Fatalf("prepare without safeReuse: %v", e)
	}
	for _, timeout := range []string{"0", "3601"} {
		if _, e = gh628Pools(t, `{"prepare":`+command(timeout)+`,"safeReuse":`+reuse+`}`); wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("prepare timeout %s: %v", timeout, e)
		}
	}
	if _, e = gh628Pools(t, `{"prepare":{"argv":["/usr/bin/true"],"cwd":"REPOSITORY","env":["SECRET"],"timeoutSeconds":"3"},"safeReuse":`+reuse+`}`); e == nil {
		t.Fatal("prepare env outside the allowlist accepted")
	}
}
