package cli

import (
	"os/user"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0825_ActorSeamKeepsEnvPrecedence pins V1-4187: with Env.Actor
// empty, as cmd/corvint-tasks leaves it, the actor is CORVINT_TASKS_ACTOR,
// then ATM_ACTOR, then the OS user, and the two variables refuse MALFORMED
// when they disagree. A set Env.Actor wins over both and is validated as a
// label, and a child started for it inherits it as CORVINT_TASKS_ACTOR.
func TestTCLV0825_ActorSeamKeepsEnvPrecedence(t *testing.T) {
	id := func(env Env) (string, string) {
		b, err := initActor(env, "OWNER")
		if err != nil {
			return "", wire.CodeOf(err)
		}
		if b.Role != "OWNER" {
			t.Fatalf("role %q", b.Role)
		}
		return string(b.ID), ""
	}
	u, err := user.Current()
	if err != nil {
		t.Skipf("no OS user: %v", err)
	}
	for _, c := range []struct {
		primary, legacy, seam, want, code string
	}{
		{"", "", "", u.Username, ""},
		{"primary", "", "", "primary", ""},
		{"", "legacy", "", "legacy", ""},
		{"same", "same", "", "same", ""},
		{"primary", "legacy", "", "", wire.CodeMalformed},
		{"primary", "legacy", "seam", "seam", ""},
		{"", "", "seam", "seam", ""},
		{"primary", "", "bad actor\n", "", wire.CodeMalformed},
	} {
		t.Setenv("CORVINT_TASKS_ACTOR", c.primary)
		t.Setenv("ATM_ACTOR", c.legacy)
		got, code := id(Env{Actor: c.seam})
		if got != c.want || code != c.code {
			t.Fatalf("primary=%q legacy=%q seam=%q: got %q %q, want %q %q", c.primary, c.legacy, c.seam, got, code, c.want, c.code)
		}
	}
	if childActorEnv(Env{}) != nil {
		t.Fatal("an unset seam changed the child environment")
	}
	t.Setenv("ATM_ACTOR", "legacy")
	env := childActorEnv(Env{Actor: "seam"})
	if !slices.Contains(env, "CORVINT_TASKS_ACTOR=seam") || slices.ContainsFunc(env, func(kv string) bool { return kv == "ATM_ACTOR=legacy" }) {
		t.Fatalf("child env: %v", env)
	}
}
