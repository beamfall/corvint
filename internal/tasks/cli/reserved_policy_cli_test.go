package cli_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0802_PolicyShowAndUpdateWarnReservedFields: policy show warns
// that a store policy setting cemRequired is reserved and unenforced, a
// policy update that clears it commits without that warning, and policy
// show then reads clean (V1-3948).
func TestTCLV0802_PolicyShowAndUpdateWarnReservedFields(t *testing.T) {
	root, _ := leaseCLIStoreWith(t, 0, time.Now().UTC().Add(-time.Hour).Truncate(time.Second), func(o *wire.Object) {
		o.Set("reviewLane", wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(false))))
		o.Set("cemRequired", wire.Bool(true))
	}, "")
	reserved := func(warnings []string) bool {
		return slices.ContainsFunc(warnings, func(w string) bool { return strings.Contains(w, "TCL-V0-802") })
	}
	show := handoffCLI(t, root, "policy", "show")
	if show.code != 0 || !reserved(show.res.Warnings) || !strings.Contains(strings.Join(show.res.Warnings, "\n"), "cemRequired") {
		t.Fatalf("policy show: %s", show.stdout)
	}
	loaded, e := intent.Load(root)
	if e != nil {
		t.Fatal(e)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(fmt.Sprint(loaded.Policy.PolicyVersion.Uint64()+1)))
	v.Obj.Set("cemRequired", wire.Bool(false))
	path := filepath.Join(t.TempDir(), "policy.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	update := handoffCLI(t, root, "policy", "update", "--request-id", "clear-reserved", "--expected-policy-version", string(loaded.Policy.PolicyVersion), "--file", path)
	if update.res.Outcome != wire.OutcomeOK || reserved(update.res.Warnings) {
		t.Fatalf("policy update: %s", update.stdout)
	}
	if again := handoffCLI(t, root, "policy", "show"); again.code != 0 || reserved(again.res.Warnings) {
		t.Fatalf("policy show after update: %s", again.stdout)
	}
}
