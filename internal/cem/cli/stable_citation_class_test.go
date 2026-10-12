package cli

import (
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEM-V1-106, DCW-V0-049: `cem status --citation` names a stable map's
// citation class beside the verifier result: a map with no role, or role
// change, is class change, and a bind-chore or seal-chore map is class chore.
// The result stays exactly what verify-stable reports and never echoes the
// role, and the status without --citation keeps its envelope unchanged.
func TestCEMV1106StatusCitationClassGroupsChoreMaps(t *testing.T) {
	f := materializeStableRead(t)
	status := func(citation bool) (int, map[string]any) {
		t.Helper()
		arguments := coreStableArguments("status", f, f.artifacts)
		if citation {
			arguments = append(arguments, "--citation")
		}
		exit, envelope, refusal := runCoreRead(t, f.repo, arguments...)
		if refusal != nil {
			t.Fatalf("status refused: %v", refusal)
		}
		wantExit, want := runVerifyStable(t, f, f.artifacts)
		if got, _ := envelope["stableVerification"].(map[string]any); exit != wantExit || !reflect.DeepEqual(got, want) {
			t.Errorf("status --citation=%v differs from verify-stable: exit %d/%d", citation, exit, wantExit)
		}
		if result, _ := envelope["stableVerification"].(map[string]any); result != nil {
			if _, echoed := result["role"]; echoed {
				t.Error("the stable result echoes the role")
			}
		}
		return exit, envelope
	}
	if _, envelope := status(false); envelope["citationClass"] != nil {
		t.Errorf("status without --citation carries citationClass %v", envelope["citationClass"])
	}
	_, plain := status(true)
	if got := plain["citationClass"]; !reflect.DeepEqual(got, map[string]any{"class": "change", "role": "change"}) {
		t.Errorf("a map without role: citationClass = %v; want class change", got)
	}
	data, err := os.ReadFile(f.mapPath)
	if err != nil {
		t.Fatal(err)
	}
	var reference map[string]any
	for _, role := range []string{wire.StableRoleChange, wire.StableRoleBindChore, wire.StableRoleSealChore} {
		document, err := wire.ParseStable(data)
		if err != nil {
			t.Fatal(err)
		}
		document.Role = role
		encoded, err := wire.EncodeStable(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.mapPath, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		_, envelope := status(true)
		class := "chore"
		if role == wire.StableRoleChange {
			class = "change"
		}
		if got := envelope["citationClass"]; !reflect.DeepEqual(got, map[string]any{"class": class, "role": role}) {
			t.Errorf("role %s: citationClass = %v; want class %s", role, got, class)
		}
		// Re-encoding changes the sealed map's bytes, so its artifacts reject it;
		// the role must leave that outcome and its issues unchanged (CEM-V1-095).
		result, _ := envelope["stableVerification"].(map[string]any)
		if reference == nil {
			reference = result
			continue
		}
		if result["outcome"] != reference["outcome"] || !reflect.DeepEqual(result["issueCodes"], reference["issueCodes"]) || !reflect.DeepEqual(result["citation"], reference["citation"]) {
			t.Errorf("role %s: outcome %v issues %v; want the change role's %v %v", role, result["outcome"], result["issueCodes"], reference["outcome"], reference["issueCodes"])
		}
	}
}
