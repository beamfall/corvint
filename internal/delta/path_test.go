package delta

import (
	"context"
	"strings"
	"testing"
)

func TestDeltaSchemaPathCorpus(t *testing.T) {
	root, _, head := fixtureMerge(t)
	base, err := Compile(context.Background(), root, Options{Base: head, Head: head, Build: "163"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a", "dir/file.go", "space name.txt", "café/雪.js", "a//b", "a/", "/a", "../a", "a/./b", "a/../b", "a\tb", "a\nb", "a\rb", "a\x7fb", "a\\b"} {
		t.Run(strings.ReplaceAll(p, "/", "_"), func(t *testing.T) {
			r := base
			r.ChangedPaths = []string{p}
			raw, err := r.Canonical()
			want := oneOf(p, "a", "dir/file.go", "space name.txt", "café/雪.js")
			if (err == nil) != want {
				t.Fatalf("path %q admitted %t", p, err == nil)
			}
			if err == nil {
				t.Logf("SCHEMA_RECORD=%s", raw)
			}
		})
	}
}
func TestDeltaRefusesControlCharacterGitPaths(t *testing.T) {
	for _, p := range []string{"tab\tfile", "line\nfile"} {
		t.Run("path", func(t *testing.T) {
			root, _, base := fixtureMerge(t)
			put(t, root, p, "data")
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "control")
			head := git(t, root, "rev-parse", "HEAD")
			if _, err := Compile(context.Background(), root, Options{Base: base, Head: head, Build: "163"}); err == nil {
				t.Fatalf("control path admitted %q", p)
			}
		})
	}
}
