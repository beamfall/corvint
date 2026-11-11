// SPDX-License-Identifier: Apache-2.0

package pathbrief

import (
	"slices"
	"testing"
)

const (
	linkBlob  = "3333333333333333333333333333333333333333"
	linkBlob2 = "4444444444444444444444444444444444444444"
)

const goodLinks = `{"schema":"corvint-requirement-links/1","inputCommit":"` + c1 + `",
 "commits":[{"sha":"` + c1 + `","committerTime":10},{"sha":"` + c2 + `","committerTime":20}],
 "evidence":[{"file":"s.md","start":1,"end":2,"blobOid":"` + linkBlob + `","requirements":["A-V1-001"]},
             {"file":"s.md","start":3,"end":4,"blobOid":"` + linkBlob + `","requirements":[]}],
 "paths":[{"path":"a.go","records":[{"commit":0,"blob":"` + linkBlob + `","evidence":[0,1]},{"commit":1,"blob":"` + linkBlob2 + `","evidence":[0]}]},
          {"path":"b.go","records":[{"commit":1,"blob":"","evidence":[1]}]}]}`

// TBC-V1-012: a damaged link file is refused whole.
func TestParseLinksRefusals(t *testing.T) {
	const head = `{"schema":"corvint-requirement-links/1",`
	commit := `"commits":[{"sha":"` + c1 + `","committerTime":1}],`
	evidence := `"evidence":[{"file":"f","start":0,"end":1,"blobOid":"` + linkBlob + `"}],`
	cases := map[string]string{
		"bad JSON":           "{",
		"wrong schema":       `{"schema":"other/1"}`,
		"no schema":          `{}`,
		"repeated path":      head + `"commits":[],"evidence":[],"paths":[{"path":"a"},{"path":"a"}]}`,
		"empty path":         head + `"commits":[],"evidence":[],"paths":[{"path":""}]}`,
		"commit too large":   head + commit + `"evidence":[],"paths":[{"path":"a","records":[{"commit":1,"evidence":[]}]}]}`,
		"commit negative":    head + commit + `"evidence":[],"paths":[{"path":"a","records":[{"commit":-1,"evidence":[]}]}]}`,
		"evidence too large": head + commit + `"evidence":[],"paths":[{"path":"a","records":[{"commit":0,"evidence":[0]}]}]}`,
		"evidence negative":  head + commit + evidence + `"paths":[{"path":"a","records":[{"commit":0,"evidence":[-1]}]}]}`,
		"commit without sha": head + `"commits":[{"committerTime":1}],"evidence":[],"paths":[]}`,
		"commit short sha":   head + `"commits":[{"sha":"abc","committerTime":1}],"evidence":[],"paths":[]}`,
		"commit time":        head + `"commits":[{"sha":"` + c1 + `","committerTime":-1}],"evidence":[],"paths":[]}`,
		"path blob":          head + commit + evidence + `"paths":[{"path":"a","records":[{"commit":0,"blob":"zz","evidence":[0]}]}]}`,
	}
	// A malformed evidence row is refused even when no path names it.
	for name, row := range map[string]string{
		"evidence without file":   `{"start":0,"end":1,"blobOid":"` + linkBlob + `"}`,
		"evidence reversed span":  `{"file":"f","start":5,"end":1,"blobOid":"` + linkBlob + `"}`,
		"evidence negative start": `{"file":"f","start":-1,"end":1,"blobOid":"` + linkBlob + `"}`,
		"evidence without blob":   `{"file":"f","start":0,"end":1}`,
		"evidence malformed blob": `{"file":"f","start":0,"end":1,"blobOid":"b"}`,
		"evidence empty id":       `{"file":"f","start":0,"end":1,"blobOid":"` + linkBlob + `","requirements":[""]}`,
	} {
		cases[name] = head + commit + `"evidence":[` + row + `],"paths":[]}`
	}
	for name, data := range cases {
		if file, err := ParseLinks([]byte(data)); err == nil || file != nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ParseLinks([]byte(head + commit + evidence + `"paths":[{"path":"a","records":[{"commit":0,"evidence":[0]}]}]}`)); err != nil {
		t.Errorf("well-formed file refused: %v", err)
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
	if first.Commit != c1 || first.CommitterTime != 10 || first.PathBlob != linkBlob || first.Path != "a.go" || !slices.Equal(first.Evidence.Requirements, []string{"A-V1-001"}) {
		t.Errorf("first %+v", first)
	}
	if a[1].Evidence.Start != 3 || last.Commit != c2 || last.PathBlob != linkBlob2 || last.CommitterTime != 20 {
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
