//go:build darwin || linux

package cli_test

import (
	"regexp"
	"testing"
)

var (
	equivalenceAttemptID = regexp.MustCompile(`attempt:[a-z]+:[a-z]+:[0-9a-f]{32}`)
	equivalenceTime      = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`)
	equivalenceDigest    = regexp.MustCompile(`[0-9a-f]{64}`)
)

// equivalentOutcome is a claim's exit code and complete envelope with what
// differs between two stores built the same way removed: the random part of
// an attempt ID, the wall-clock instants and the digests of records that
// carry them.
func equivalentOutcome(x run) string {
	out := equivalenceAttemptID.ReplaceAllString(string(x.stdout), "attempt:<id>")
	out = equivalenceTime.ReplaceAllString(out, "<time>")
	out = equivalenceDigest.ReplaceAllString(out, "<sha256>")
	return string(rune('0'+x.code)) + " " + out
}

// detachedScenario builds a store, gives holder "agent" a live attempt on
// the first ticket under request ID r1, and then runs each probe claim, in
// the foreground or, with detached, as the same argv plus --detach --wait 30.
func detachedScenario(t *testing.T, detached bool) map[string]string {
	t.Helper()
	root, _, first := detachClaimStore(t)
	second := planTicket(t, root, "second", "P1", `["docs/"]`)
	if x := atm(t, root, nil, "claim", first, "--holder", "agent", "--request-id", "r1"); x.code != 0 {
		t.Fatalf("setup claim: %s", x.stdout)
	}
	claim := func(args ...string) run {
		if detached {
			return cliRun(t, root, append(append([]string{"claim"}, args...), "--detach", "--wait", "30")...)
		}
		return cliRun(t, root, append([]string{"claim"}, args...)...)
	}
	return map[string]string{
		// A live attempt under another request ID on another ticket.
		"other ticket, other request ID": equivalentOutcome(claim(second, "--holder", "agent", "--request-id", "r2")),
		// On the ticket the holder's live attempt already holds.
		"same ticket, other request ID": equivalentOutcome(claim(first, "--holder", "agent", "--request-id", "r3")),
		// The request ID of the live attempt itself: the store replays it.
		"same ticket, same request ID": equivalentOutcome(claim(first, "--holder", "agent", "--request-id", "r1")),
	}
}

// ATR-V0-312 (V1-2173, V1-1557): `--detach` only moves where a claim runs,
// so a detached claim by a holder whose live attempt has another request ID
// has exactly the outcome of the foreground claim: the same exit code and the
// same envelope, a second attempt, the store's ATTEMPT_LIVE refusal or the
// idempotent replay, never a detach-specific answer. Two stores built the
// same way receive the same claims, one in the foreground and one detached.
func TestATRV0312_DetachedClaimOutcomeEqualsForeground(t *testing.T) {
	foreground := detachedScenario(t, false)
	detached := detachedScenario(t, true)
	for name, want := range foreground {
		if got := detached[name]; got != want {
			t.Errorf("%s:\ndetached   %s\nforeground %s", name, got, want)
		}
	}
	// The probes are not all one outcome: the comparison is not vacuous.
	if foreground["other ticket, other request ID"] == foreground["same ticket, other request ID"] || foreground["same ticket, same request ID"] == foreground["same ticket, other request ID"] {
		t.Fatalf("the probes do not differ: %v", foreground)
	}
}
