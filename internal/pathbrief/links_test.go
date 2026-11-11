package pathbrief

import (
	"slices"
	"testing"
)

const goodLinks = `{"schema":"corvint-requirement-links/1","inputCommit":"x",
 "commits":[{"sha":"c1","committerTime":10},{"sha":"c2","committerTime":20}],
 "evidence":[{"file":"s.md","start":1,"end":2,"blobOid":"b","requirements":["A-V1-001"]},
             {"file":"s.md","start":3,"end":4,"blobOid":"b","requirements":[]}],
 "paths":[{"path":"a.go","records":[{"commit":0,"blob":"o1","evidence":[0,1]},{"commit":1,"blob":"o2","evidence":[0]}]},
          {"path":"b.go","records":[{"commit":1,"blob":"","evidence":[1]}]}]}`

// TBC-V1-012: a damaged link file is refused whole.
func TestParseLinksRefusals(t *testing.T) {
	for name, data := range map[string]string{
		"bad JSON":           "{",
		"wrong schema":       `{"schema":"other/1"}`,
		"no schema":          `{}`,
		"repeated path":      `{"schema":"corvint-requirement-links/1","commits":[],"evidence":[],"paths":[{"path":"a"},{"path":"a"}]}`,
		"empty path":         `{"schema":"corvint-requirement-links/1","commits":[],"evidence":[],"paths":[{"path":""}]}`,
		"commit too large":   `{"schema":"corvint-requirement-links/1","commits":[{"sha":"c","committerTime":1}],"evidence":[],"paths":[{"path":"a","records":[{"commit":1,"evidence":[]}]}]}`,
		"commit negative":    `{"schema":"corvint-requirement-links/1","commits":[{"sha":"c","committerTime":1}],"evidence":[],"paths":[{"path":"a","records":[{"commit":-1,"evidence":[]}]}]}`,
		"evidence too large": `{"schema":"corvint-requirement-links/1","commits":[{"sha":"c","committerTime":1}],"evidence":[],"paths":[{"path":"a","records":[{"commit":0,"evidence":[0]}]}]}`,
		"evidence negative":  `{"schema":"corvint-requirement-links/1","commits":[{"sha":"c","committerTime":1}],"evidence":[{"file":"f","start":0,"end":1,"blobOid":"b"}],"paths":[{"path":"a","records":[{"commit":0,"evidence":[-1]}]}]}`,
	} {
		if file, err := ParseLinks([]byte(data)); err == nil || file != nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TBC-V1-012: entries are joined to their tables and matched by exact path.
func TestLinkFileForPathAndEntries(t *testing.T) {
	file, err := ParseLinks([]byte(goodLinks))
	if err != nil {
		t.Fatal(err)
	}
	if file.Entries() != 4 {
		t.Errorf("entries = %d", file.Entries())
	}
	a := file.ForPath("a.go")
	if len(a) != 3 {
		t.Fatalf("a.go links = %d", len(a))
	}
	first, last := a[0], a[2]
	if first.Commit != "c1" || first.CommitterTime != 10 || first.PathBlob != "o1" || first.Path != "a.go" || !slices.Equal(first.Evidence.Requirements, []string{"A-V1-001"}) {
		t.Errorf("first %+v", first)
	}
	if a[1].Evidence.Start != 3 || last.Commit != "c2" || last.PathBlob != "o2" || last.CommitterTime != 20 {
		t.Errorf("file order broken: %+v", a)
	}
	if b := file.ForPath("b.go"); len(b) != 1 || b[0].PathBlob != "" {
		t.Errorf("b.go %+v", b)
	}
	for _, name := range []string{"c.go", "", "a.go/", "A.go", "dir/a.go"} {
		if got := file.ForPath(name); got != nil {
			t.Errorf("ForPath(%q) = %v, want none", name, got)
		}
	}
	if empty, err := ParseLinks([]byte(`{"schema":"corvint-requirement-links/1"}`)); err != nil || empty.Entries() != 0 || empty.ForPath("a") != nil {
		t.Errorf("empty file: %v", err)
	}
}
