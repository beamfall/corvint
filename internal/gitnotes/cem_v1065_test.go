package gitnotes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// stableFixture is the public receipt-authentication stable map.
const stableFixture = "../../protocol/cem-1.0/stable/receipt-authentication/stable.json"

// CEM-V1-065: anchor routes a committed cem/1.0 map to the stable parser and
// records spec cem/1.0 and the sha256 of the exact map bytes; provenance then
// reads the pointer back as verified without moving a ref or the worktree.
func TestCEMV1065AnchorStableMap(t *testing.T) {
	t.Parallel()
	stable, err := os.ReadFile(stableFixture)
	if err != nil {
		t.Fatal(err)
	}
	root := fixtureRepo(t)
	writeFile(t, root, mapPath, string(stable))
	fixtureGit(t, root, nil, "commit", "-qam", "stable map")
	ctx := context.Background()
	receipt, err := Anchor(ctx, root, mapPath, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	var stored Pointer
	if err := json.Unmarshal([]byte(fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head)), &stored); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(stable)
	if receipt["ok"] != true || receipt["verification"] != "verified" || stored.MapSpec != "cem/1.0" ||
		stored.MapSha256 != hex.EncodeToString(digest[:]) || stored.Schema != PointerSchema || stored.CEMCommit != head {
		t.Fatalf("receipt %v, pointer %+v", receipt, stored)
	}
	before := refState(t, root)
	read, err := Provenance(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	rows := read["evidence"].([]map[string]any)
	if refState(t, root) != before || read["mutates"] != false || len(rows) != 1 || rows[0]["state"] != "verified" {
		t.Fatalf("provenance: %v", read)
	}
	// A map that declares cem/1.0 but is not a valid stable map is refused,
	// and no note is written.
	broken := fixtureRepo(t)
	writeFile(t, broken, mapPath, `{"spec":"cem/1.0"}`)
	fixtureGit(t, broken, nil, "commit", "-qam", "broken stable map")
	if _, err := Anchor(ctx, broken, mapPath, "HEAD"); err == nil {
		t.Fatal("anchored an invalid stable map")
	}
	if fixtureGit(t, broken, nil, "for-each-ref", Ref) != "" {
		t.Fatal("a refused stable anchor wrote a note")
	}
}

// CEM-V1-065: a 0.x map keeps its pointer bytes exactly, and a spec neither
// parser accepts keeps the 0.x refusal code.
func TestCEMV1065AnchorLegacyUnchanged(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	if _, err := Anchor(ctx, root, mapPath, "HEAD"); err != nil {
		t.Fatal(err)
	}
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	blob := fixtureGit(t, root, nil, "rev-parse", "HEAD:"+mapPath)
	digest := sha256.Sum256([]byte(fixtureMap))
	want := `{"schema":"corvint-cem-anchor/0","cem_commit":"` + head + `","map_path":"` + mapPath +
		`","map_blob":"` + blob + `","map_sha256":"` + hex.EncodeToString(digest[:]) + `","map_spec":"cem/0.2"}`
	if got := fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head); got != want {
		t.Fatalf("0.x pointer bytes changed:\n got %s\nwant %s", got, want)
	}
	other := fixtureRepo(t)
	writeFile(t, other, mapPath, `{"spec":"cem/1.0-experimental.1"}`)
	fixtureGit(t, other, nil, "commit", "-qam", "candidate map")
	if _, err := Anchor(ctx, other, mapPath, "HEAD"); cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("candidate map: code %q (%v)", cemcode.CodeOf(err), err)
	}
}
