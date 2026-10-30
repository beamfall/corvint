package changee2e

import (
	"errors"
	"strings"
	"testing"
)

// CCE-V0-119: a manifest that does not build refuses e2e-map-unavailable, unchanged, and the
// refusal names the command that checks the manifest and the manifest's schema and spec, at
// both the catalog revision and the obligations base.
func TestCCEV0119MapUnavailableNamesProducer(t *testing.T) {
	root, good := fixtureRepo(t)
	broken := commitFiles(t, root, map[string]*string{"appmap.json": text(`{"schema": "application-map-manifest/0"}`)}, "broken manifest")
	assertHint := func(t *testing.T, err error, side string) {
		t.Helper()
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != "e2e-map-unavailable" {
			t.Fatalf("want e2e-map-unavailable, got %v", err)
		}
		if !strings.HasPrefix(refusal.Detail, side) {
			t.Fatalf("detail %q does not start with %q", refusal.Detail, side)
		}
		for _, want := range []string{"`corvint flows appmap build --manifest FILE --revision REV`",
			"application-map-manifest/0", "not a built application-map/0", "AMAP-V0-001", "docs/specs/application-map-v0.md", "docs/E2E-ADOPTION.md"} {
			if !strings.Contains(refusal.Detail, want) {
				t.Fatalf("detail %q does not name %q", refusal.Detail, want)
			}
		}
	}
	t.Run("catalog", func(t *testing.T) {
		_, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: broken, Manifest: "appmap.json"})
		assertHint(t, err, "")
	})
	t.Run("missing manifest", func(t *testing.T) {
		_, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: good, Manifest: "absent.json"})
		assertHint(t, err, "")
	})
	t.Run("obligations base", func(t *testing.T) {
		head := commitFiles(t, root, map[string]*string{"appmap.json": blobTextAt(t, root, good, "appmap.json")}, "restore manifest")
		_, err := BuildObligations(bg, diffOpts(root, broken, head))
		assertHint(t, err, "base: ")
	})
}

// blobTextAt reads path at rev in the fixture repository.
func blobTextAt(t *testing.T, root, rev, path string) *string {
	t.Helper()
	data, _, ok, err := blobAt(bg, root, rev, path)
	if err != nil || !ok {
		t.Fatalf("read %s at %s: ok=%v err=%v", path, rev, ok, err)
	}
	s := string(data)
	return &s
}
