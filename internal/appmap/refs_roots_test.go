package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// AMAP-V0-350: issue #1940. The refs check over --root sources of a map whose tests live in an
// aliased root used to pass with every test file unread and no references when no --repo bound
// that root; it now refuses naming the alias and the flag, and answers once the root is bound.
func TestAMAPV0350RefsUnboundTestRoot(t *testing.T) {
	ctx := context.Background()
	app, appRev, e2e, _ := twoRoots(t, false)
	m, err := BuildRoots(ctx, app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil || len(m.Files) == 0 {
		t.Fatalf("two-root map: %v", err)
	}
	_, err = CheckRefs(ctx, m, []string{"app"}, Options{Root: app, Full: true})
	if codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "root e2e holds") || !strings.Contains(err.Error(), "--repo e2e=DIR") {
		t.Fatalf("unbound test root: %v", err)
	}
	r, err := CheckRefs(ctx, m, []string{"app"}, Options{Root: app, Full: true, Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	var doc refsDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil || doc.TestFilesRead != len(m.Files) || doc.TestFilesUnread != 0 || doc.References == 0 {
		t.Fatalf("bound test root: %v %+v", err, doc)
	}
}
