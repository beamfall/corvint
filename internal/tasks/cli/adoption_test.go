package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestExternalAgentTemplatesRequireQualification(t *testing.T) {
	t.Run("CTS-V0-005 fresh external-agent templates retain qualification", func(t *testing.T) {
		r := fixture.TempRepo(t)
		git(t, r.Root, "init", "-q", "-b", "main")
		git(t, r.Root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "initial")
		for _, name := range []string{"queue.json", "policy.json"} {
			raw, err := os.ReadFile(filepath.Join("testdata", "external-agents", name))
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, filepath.Join(r.IntentDir, name), raw)
		}
		if x := atm(t, r.Root, nil, "init"); x.code != 0 {
			t.Fatalf("init: %s", x.stdout)
		}
		st, err := intent.Load(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		if st.Queue.Fixture || st.Queue.CanonicalWriter != "NATIVE" || len(st.Policy.RequireEnforcedFields) != 0 || st.Policy.SerialFallback != "WHOLE_REPOSITORY" {
			t.Fatal("template changed its non-fixture, conservative external-agent contract")
		}
		payload, err := os.ReadFile(filepath.Join("testdata", "external-agents", "ticket-create.json"))
		if err != nil {
			t.Fatal(err)
		}
		if x := atm(t, r.Root, payload, "ticket", "create", "--request-id", "first", "--payload-stdin"); x.code != 0 {
			t.Fatalf("create: %s", x.stdout)
		}
		x := atm(t, r.Root, nil, "claim", "--next", "--holder", "agent", "--request-id", "unqualified")
		if !hasCode(x.res, wire.CodeCutoverMissing) {
			t.Fatalf("unqualified claim: %s", x.stdout)
		}
		if x := atm(t, r.Root, nil, "receipt", "audit"); x.code != 0 {
			t.Fatalf("audit: %s", x.stdout)
		}
	})
}

func TestExternalAgentHelpNamesAdmissionAndPayloadRules(t *testing.T) {
	r := fixture.TempRepo(t)
	x := atm(t, r.Root, nil, "help")
	for _, want := range []string{"claim --next", "cutover --execution", "plan preview", "submit --attempt", "gate run --attempt", "complete --attempt"} {
		if !strings.Contains(string(x.stdout), want) {
			t.Errorf("missing help for %s", want)
		}
	}
	if got := field(x.res.Items[0], "releaseReasonCodes"); len(got.Arr) != len(wire.Codes) {
		t.Fatal("release reasons do not match closed codes")
	}
	x = atm(t, r.Root, nil, "ticket", "create", "--help", "--verbose")
	for _, want := range []string{"UTF-8", "touchPaths", "ordered arrays"} {
		if !strings.Contains(string(x.stdout), want) {
			t.Errorf("missing payload rule %s", want)
		}
	}
}
