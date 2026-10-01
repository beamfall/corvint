package cli_test

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
	"time"
)

func TestCALV0051_CreateHelpAndMeaningfulIDs(t *testing.T) {
	t.Run("CAL-V0-051 meaningful IDs create help", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		h := handoffCLI(t, root, "ticket", "create", "--help")
		if h.code != 0 || strings.Contains(field(h.res.Items[0], "usage").Str, "--target") || strings.Contains(field(h.res.Items[0], "usage").Str, "--expected-revision") || field(h.res.Items[0], "localToken").Str == "" {
			t.Fatalf("create help: %s", h.stdout)
		}
		for _, local := range []string{"BT-002", "F0-5", "FL-016.matrix"} {
			p, e := wire.Parse([]byte(createPayloadJSON + "\n"))
			if e != nil {
				t.Fatal(e)
			}
			p.Obj.Set("localToken", wire.String(local))
			x := handoffCLI(t, root, "ticket", "create", "--request-id", "create-"+local, "--payload", string(wire.Encode(p)))
			if x.code != 0 || field(x.res.Items[0], "ticketId").Str != "ticket:acme:main:"+local {
				t.Fatalf("explicit ID: %s", x.stdout)
			}
			show := handoffCLI(t, root, "ticket", "show", local)
			if show.code != 0 {
				t.Fatalf("local show: %s", show.stdout)
			}
			duplicate := handoffCLI(t, root, "ticket", "create", "--request-id", "duplicate-"+local, "--payload", string(wire.Encode(p)))
			if duplicate.code == 0 {
				t.Fatal("duplicate accepted")
			}
			claim := handoffCLI(t, root, "claim", local, "--holder", "board-agent", "--scope", "src/"+local, "--request-id", "claim-"+local)
			if claim.code != 0 {
				t.Fatalf("claim explicit ID: %s", claim.stdout)
			}
		}
		for _, local := range []string{"bt-002", "bad token"} {
			p, e := wire.Parse([]byte(createPayloadJSON + "\n"))
			if e != nil {
				t.Fatal(e)
			}
			p.Obj.Set("localToken", wire.String(local))
			refused := handoffCLI(t, root, "ticket", "create", "--request-id", strings.ReplaceAll("invalid-"+local, " ", "-"), "--payload", string(wire.Encode(p)))
			if refused.code == 0 {
				t.Fatalf("invalid or colliding token accepted: %s", local)
			}
		}
		preview := handoffCLI(t, root, "plan", "preview")
		if preview.code != 0 {
			t.Fatal(preview.stdout)
		}
		ids := map[string]bool{}
		for _, e := range field(preview.res.Items[0], "entries").Arr {
			ids[field(e, "ticketId").Str] = true
		}
		for _, local := range []string{"BT-002", "F0-5", "FL-016.matrix"} {
			if !ids["ticket:acme:main:"+local] {
				t.Fatalf("preview lost identity %s: %s", local, preview.stdout)
			}
		}
	})
}
