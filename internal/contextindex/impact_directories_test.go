package contextindex

import (
	"reflect"
	"testing"
)

// GPK-V0-883: expansion keeps operand order, reads only direct children with
// an admitted suffix, and names the first directory that expands to nothing.
func TestGPKV0883ExpandImpactDirectories(t *testing.T) {
	t.Parallel()
	index := &Index{Sources: map[string]Source{
		"pkg/b.go": {}, "pkg/a.go": {}, "pkg/Makefile": {}, "pkg/go.mod": {}, "pkg/sub/c.go": {},
		"docs/guide.md": {}, "only/sub/d.go": {},
	}}
	paths := []string{"docs/guide.md", "pkg", "pkg/sub"}
	expanded, empty := ExpandImpactDirectories(index, paths, map[string]bool{"pkg": true, "pkg/sub": true})
	if want := []string{"docs/guide.md", "pkg/a.go", "pkg/b.go", "pkg/go.mod", "pkg/sub/c.go"}; empty != "" || !reflect.DeepEqual(expanded, want) {
		t.Fatalf("expanded = %v, empty = %q, want %v", expanded, empty, want)
	}
	if expanded, empty := ExpandImpactDirectories(index, []string{"pkg", "only", "absent"}, map[string]bool{"pkg": true, "only": true, "absent": true}); expanded != nil || empty != "only" {
		t.Fatalf("expanded = %v, empty = %q, want the first empty directory", expanded, empty)
	}
	if expanded, empty := ExpandImpactDirectories(index, paths, nil); empty != "" || !reflect.DeepEqual(expanded, paths) {
		t.Fatalf("without directories: expanded = %v, empty = %q", expanded, empty)
	}
}
