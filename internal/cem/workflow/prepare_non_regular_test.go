package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// nonRegularRange commits, over the calc fixture, a regular file and a
// symbolic link pointing outside the tree, and returns the range.
func nonRegularRange(t *testing.T) (root, base, target string) {
	t.Helper()
	root, base, _ = makeCalcRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../outside", filepath.Join(root, "web", "escape")); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	writeFile(t, root, "web/page.txt", "page\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "link")
	return root, base, gitCmd(t, root, "rev-parse", "HEAD")
}

// TestCAXV0020PrepareMapsRegularHunksAndRecordsLinks checks CAX-V0-020,
// CAX-V0-022 and CAX-V0-023 (V1-2974): under --non-regular unknown, prepare
// over a range that adds a symlink maps every regular-file hunk, records the
// link once as an explicit unknown row without its target, and cite and
// status work on the map, status counting the row as unknown. The default
// stays the cem/1.0 refusal, an invalid value is refused, and a range whose
// every changed path is non-regular is refused.
func TestCAXV0020PrepareMapsRegularHunksAndRecordsLinks(t *testing.T) {
	root, base, target := nonRegularRange(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); cemcode.CodeOf(err) != cemcode.UnsupportedTreeMode {
		t.Fatalf("default prepare over an added link: %v", err)
	}
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, NonRegular: "follow"}); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("--non-regular follow: %v", err)
	}
	result, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target, NonRegular: NonRegularUnknown})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	rows, _ := json.Marshal(result["nonRegular"])
	if result["profile"] != "cem-non-regular/1" ||
		string(rows) != `[{"baseMode":"absent","disposition":"unknown","path":"web/escape","targetMode":"120000"}]` ||
		strings.Contains(string(encoded), "outside") {
		t.Fatalf("prepare result %s", encoded)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	page := ""
	for _, hunk := range document.Hunks {
		if hunk.Path == "web/escape" {
			t.Fatalf("the link was mapped as a hunk: %+v", hunk)
		}
		if hunk.Path == "web/page.txt" {
			page = hunk.ID
		}
	}
	if page == "" || len(document.Hunks) < 2 {
		t.Fatalf("mapped hunks %+v, want every regular-file hunk including web/page.txt", document.Hunks)
	}
	if _, err := openSession(t, root).Cite(ctx(), CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: page, EvidencePath: "pkg/calc/calc.go", Lines: "1:1", Relation: "specification",
	}); err != nil {
		t.Fatalf("cite on a cem-non-regular/1 map: %v", err)
	}
	status := func(nonRegular bool) map[string]any {
		t.Helper()
		result, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
			MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target,
			ArtifactRoot: root, ArtifactsGiven: true, NonRegular: nonRegular,
		})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result)
		var generic map[string]any
		if err := json.Unmarshal(encoded, &generic); err != nil {
			t.Fatal(err)
		}
		return generic
	}
	profiled := status(true)
	counts, _ := json.Marshal(profiled["counts"])
	outcome := profiled["stableVerification"].(map[string]any)["outcome"]
	if outcome != "ACCEPT" || profiled["nonRegularProfile"] != "cem-non-regular/1" ||
		string(counts) != fmt.Sprintf(`{"mechanical":0,"nonRegular":1,"supported":1,"total":%d,"unknown":%d}`, len(document.Hunks)+1, len(document.Hunks)) {
		encoded, _ := json.Marshal(profiled)
		t.Fatalf("profiled status %s", encoded)
	}
	plain := status(false)
	verification := plain["stableVerification"].(map[string]any)
	if verification["outcome"] != "REJECT" || !strings.Contains(string(mustJSON(t, verification["issueCodes"])), "patch-digest-mismatch") {
		t.Fatalf("a plain cem/1.0 status accepted the profile map: %s", mustJSON(t, plain))
	}

	// A range whose only change is a link leaves nothing to map.
	if err := os.Symlink("page.txt", filepath.Join(root, "web", "alias")); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "alias")
	_, err = openSession(t, root).Prepare(ctx(), PrepareOptions{Base: target, Target: gitCmd(t, root, "rev-parse", "HEAD"), NonRegular: NonRegularUnknown, Replace: true})
	if cemcode.CodeOf(err) != cemcode.UnsupportedTreeMode || !strings.Contains(cemcode.MessageOf(err), "every changed path is a non-regular entry") {
		t.Fatalf("an all-link range: %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
