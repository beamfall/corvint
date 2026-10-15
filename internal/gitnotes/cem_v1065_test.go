package gitnotes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stableFixture is the public receipt-authentication stable map.
const stableFixture = "../../protocol/cem-1.0/stable/receipt-authentication/stable.json"

// CEM-V1-065: anchor routes a committed cem/1.0 map to the stable parser and
// records spec cem/1.0 and the sha256 of the exact map bytes; provenance then
// reads the pointer back as verified without moving a ref or the worktree.
func TestCEMV1065AnchorStableMap(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t) // commits stableFixture, rebound to its change, as the map
	stable := committedMap(t, root)
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

// CEM-V1-210: a committed retired cem/0.x map refuses with unsupported-spec
// and the migration fix line, and no note is written; the exact pointer bytes
// of a cem/1.0 map stay the anchor's wire.
func TestCEMV1210AnchorRefusesARetiredMap(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	if _, err := Anchor(ctx, root, mapPath, "HEAD"); err != nil {
		t.Fatal(err)
	}
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	blob := fixtureGit(t, root, nil, "rev-parse", "HEAD:"+mapPath)
	committed := committedMap(t, root)
	digest := sha256.Sum256(committed)
	parsed, err := wire.ParseStable(committed)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema":"corvint-cem-anchor/1","cem_commit":"` + head + `","map_path":"` + mapPath +
		`","map_blob":"` + blob + `","map_sha256":"` + hex.EncodeToString(digest[:]) + `","map_spec":"cem/1.0"` +
		`,"base":"` + parsed.Change.BaseRevision + `","patch_sha256":"` + parsed.Change.PatchSha256 +
		`","target_tree":"` + fixtureGit(t, root, nil, "rev-parse", "HEAD^{tree}") + `"}`
	if got := fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head); got != want {
		t.Fatalf("pointer bytes:\n got %s\nwant %s", got, want)
	}
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3", "cem/1.0-experimental.1"} {
		other := fixtureRepo(t)
		writeFile(t, other, mapPath, strings.Replace(fixtureMap, `"cem/1.0"`, strconv.Quote(spec), 1))
		fixtureGit(t, other, nil, "commit", "-qam", "retired map")
		_, err := Anchor(ctx, other, mapPath, "HEAD")
		if cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
			t.Fatalf("%s: code %q (%v)", spec, cemcode.CodeOf(err), err)
		}
		if strings.HasPrefix(spec, "cem/0.") && !strings.Contains(err.Error(), "fix: "+wire.RetiredSpecFix) {
			t.Fatalf("%s: no fix line: %v", spec, err)
		}
		if fixtureGit(t, other, nil, "for-each-ref", Ref) != "" {
			t.Fatalf("%s: a refused anchor wrote a note", spec)
		}
	}
}
