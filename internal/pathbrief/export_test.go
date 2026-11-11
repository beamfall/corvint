// SPDX-License-Identifier: Apache-2.0

package pathbrief

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const exportSpec = "# Spec\n\nABC-V1-001. The first requirement MUST hold.\n\nABC-V1-002. The second requirement MUST hold.\n\nProse that defines nothing.\n"

type exportFixture struct {
	root           string
	c1, c2, c3     string // commits named by two sealed records and one damaged record
	final          string // commit that adds the records and deletes src/b.go
	specBlob       string
	first, prose   [2]int // byte spans: ABC-V1-001 definition, the prose line
	outputRevision string
}

func span(text, from, to string) [2]int {
	start := strings.Index(text, from)
	return [2]int{start, start + len(to) + strings.Index(text[start:], to)}
}

// spanDigest is the digest a sealed record states for a span of the fixture spec.
func spanDigest(span [2]int) string {
	sum := sha256.Sum256([]byte(exportSpec[span[0]:span[1]]))
	return hex.EncodeToString(sum[:])
}

func record(t *testing.T, blob string, first, prose [2]int, hunks string) string {
	return fmt.Sprintf(`{"spec":"cem/1.0","baseRevision":%q,"patchSha256":%q,"evidence":[
	 {"id":"e1","path":"docs/spec.md","blobOid":%q,"spanSha256":%q,"span":{"start":%d,"end":%d}},
	 {"id":"e2","path":"docs/spec.md","blobOid":%q,"spanSha256":%q,"span":{"start":%d,"end":%d}}],
	 "hunks":[%s]}`, c1, strings.Repeat("d", 64), blob, spanDigest(first), first[0], first[1], blob, spanDigest(prose), prose[0], prose[1], hunks)
}

// newExportFixture builds a repository with three commits, then one commit that adds two sealed
// records (for the first two commits), a damaged record for the third, a stray file in the record
// directory, and deletes src/b.go.
func newExportFixture(t *testing.T) exportFixture {
	t.Helper()
	f := exportFixture{}
	f.root = newRepo(t, map[string]string{})
	f.c1 = commitAll(t, f.root, map[string]string{"docs/spec.md": exportSpec, "src/a.go": "package a\n", "src/b.go": "package b\n"})
	f.c2 = commitAll(t, f.root, map[string]string{"src/a.go": "package a // changed\n"})
	f.c3 = commitAll(t, f.root, map[string]string{"src/c.go": "package c\n"})
	f.specBlob = blobAt(t, f.root, "HEAD:docs/spec.md")
	f.first = span(exportSpec, "ABC-V1-001.", "hold.")
	f.prose = span(exportSpec, "Prose", "nothing.")
	hunk := func(path, basis string) string {
		return fmt.Sprintf(`{"path":%q,"basis":[%s]}`, path, basis)
	}
	spec := func(id string) string { return fmt.Sprintf(`{"evidenceId":%q,"relation":"specification"}`, id) }
	r1 := record(t, f.specBlob, f.first, f.prose, strings.Join([]string{
		hunk("src/a.go", spec("e1")+`,{"evidenceId":"e2","relation":"decision"}`),
		hunk("src/b.go", spec("e1")),
		hunk("src/a.go", `{"evidenceId":"e2","relation":"test"}`), // another relation: not a link
	}, ","))
	r2 := record(t, f.specBlob, f.first, f.prose, hunk("src/a.go", spec("e1")+","+spec("missing"))) // one dangling basis
	f.final = commitAll(t, f.root, map[string]string{
		recordDirectory + "/" + f.c1 + ".cem.json": r1,
		recordDirectory + "/" + f.c2 + ".cem.json": r2,
		recordDirectory + "/" + f.c3 + ".cem.json": "not json",
		recordDirectory + "/README.md":             "stray\n",
		"src/b.go":                                 deleted,
	})
	return f
}

// TBC-V1-012: the export reads sealed records, counts what it skips and drops, and is deterministic.
func TestExport(t *testing.T) {
	f := newExportFixture(t)
	ctx := context.Background()
	data, stats, err := Export(ctx, f.root, f.final)
	if err != nil {
		t.Fatal(err)
	}
	want := ExportStats{InputCommit: f.final, RecordsRead: 2, RecordsSkipped: 2, Links: 4, LinksWithoutRequirement: 1,
		LinksPathAbsentAtInput: 1, LinksDropped: 1, Paths: 2, Evidence: 2}
	sum := sha256.Sum256(data)
	want.OutputSHA256, want.OutputBytes = hex.EncodeToString(sum[:]), len(data)
	if stats != want {
		t.Errorf("stats = %+v\nwant   %+v", stats, want)
	}
	// Deterministic: a second run, by branch name this time, gives the same bytes and stats.
	again, stats2, err := Export(ctx, f.root, "HEAD")
	if err != nil || !bytes.Equal(data, again) || stats2 != stats {
		t.Errorf("second run differs: %v", err)
	}

	file, err := ParseLinks(data)
	if err != nil {
		t.Fatalf("output does not parse: %v", err)
	}
	if file.InputCommit != f.final || file.Entries() != 4 || len(file.Commits) != 2 {
		t.Errorf("file %+v", file)
	}
	times := map[string]int64{}
	for _, c := range file.Commits {
		times[c.SHA] = c.CommitterTime
	}
	for _, sha := range []string{f.c1, f.c2} {
		got, _ := strconv.ParseInt(git(t, f.root, "show", "-s", "--format=%ct", sha), 10, 64)
		if times[sha] != got || got == 0 {
			t.Errorf("committer time of %s = %d, want %d", sha, times[sha], got)
		}
	}
	a := file.ForPath("src/a.go")
	if len(a) != 3 {
		t.Fatalf("src/a.go links %d, entries of different records are kept apart", len(a))
	}
	withID := 0
	for _, link := range a {
		if slices.Equal(link.Evidence.Requirements, []string{"ABC-V1-001"}) {
			withID++
			if link.PathBlob != blobAt(t, f.root, link.Commit+":src/a.go") {
				t.Errorf("blob at %s = %s", link.Commit, link.PathBlob)
			}
		} else if len(link.Evidence.Requirements) != 0 {
			t.Errorf("prose span carries %v", link.Evidence.Requirements)
		}
	}
	equal(t, "links with an ID", withID, 2)
	// A path deleted at the input commit keeps its link, with the blob it had at the record commit.
	if b := file.ForPath("src/b.go"); len(b) != 1 || b[0].PathBlob != blobAt(t, f.root, f.c1+":src/b.go") {
		t.Errorf("b.go %+v", b)
	}

	// The output feeds Build: one requirement item for ABC-V1-001 with both records behind it.
	root := f.root
	commitAll(t, root, map[string]string{LinksPath: string(data)})
	brief := build(t, root, Sources{}, "src/a.go")
	s := pick(t, brief, 0, "requirements")
	if s.State != StateIncomplete || len(s.Items) != 2 || s.Items[0].Requirement != "ABC-V1-001" || s.Items[0].Entries != 2 {
		t.Fatalf("requirements %+v", s)
	}
	// The reported entry has the later committer time, ties to the lower SHA.
	reported := f.c1
	if times[f.c2] > times[f.c1] || times[f.c2] == times[f.c1] && f.c2 < f.c1 {
		reported = f.c2
	}
	changed := blobAt(t, root, reported+":src/a.go") != blobAt(t, root, "HEAD:src/a.go")
	if it := s.Items[0]; it.Commit != reported || it.ChangedSince == nil || *it.ChangedSince != changed {
		t.Errorf("reported %s changed %v; item %+v", reported, changed, it)
	}
	if span := s.Items[1]; span.Requirement != "" || span.Evidence == nil || span.Evidence.Start != f.prose[0] || span.Evidence.End != f.prose[1] {
		t.Errorf("prose item %+v", span)
	}
	// The path src/b.go was deleted: no link follows it anywhere.
	if got := pick(t, build(t, root, Sources{}, "src/b.go"), 0, "requirements"); got.Reason != "path-not-at-head" {
		t.Errorf("deleted path %+v", got)
	}
}

// TBC-V1-012: a record that lacks a required field, or holds a malformed one, is skipped whole
// and gives no link.
func TestExportSkipsDamagedRecords(t *testing.T) {
	f := newExportFixture(t)
	_, before, err := Export(context.Background(), f.root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	good := record(t, f.specBlob, f.first, f.prose, `{"path":"src/c.go","basis":[{"evidenceId":"e1","relation":"specification"}]}`)
	digest := strings.Repeat("d", 64)
	for name, damage := range map[string][2]string{
		"no base revision":      {`"baseRevision":"` + c1 + `",`, ""},
		"short base revision":   {c1, c1[:39]},
		"no patch digest":       {`"patchSha256":"` + digest + `",`, ""},
		"no span digest":        {`"spanSha256":"` + spanDigest(f.first) + `",`, ""},
		"malformed span digest": {`"spanSha256":"` + spanDigest(f.first), `"spanSha256":"` + spanDigest(f.first)[:63] + "X"},
		"no blob":               {`"blobOid":"` + f.specBlob + `",`, ""},
		"no evidence id":        {`"id":"e2",`, ""},
		"no evidence path":      {`"path":"docs/spec.md",`, ""},
		"no span":               {fmt.Sprintf(`,"span":{"start":%d,"end":%d}`, f.prose[0], f.prose[1]), ""},
		"reversed span":         {fmt.Sprintf(`"span":{"start":%d,"end":%d}`, f.prose[0], f.prose[1]), `"span":{"start":9,"end":3}`},
		"no hunk path":          {`"path":"src/c.go",`, ""},
	} {
		damaged := strings.Replace(good, damage[0], damage[1], 1)
		if damaged == good {
			t.Fatalf("%s: fixture not changed", name)
		}
		commitAll(t, f.root, map[string]string{recordDirectory + "/" + f.c3 + ".cem.json": damaged})
		_, stats, err := Export(context.Background(), f.root, "HEAD")
		if err != nil || stats.RecordsRead != before.RecordsRead || stats.RecordsSkipped != before.RecordsSkipped || stats.Links != before.Links {
			t.Errorf("%s: stats %+v %v", name, stats, err)
		}
	}
	// A well-formed record whose span bytes do not have the stated digest is read, and that
	// evidence gives no link: the record no longer supports it.
	wrong := strings.Replace(good, spanDigest(f.first), spanDigest(f.prose), 1)
	commitAll(t, f.root, map[string]string{recordDirectory + "/" + f.c3 + ".cem.json": wrong})
	_, stats, err := Export(context.Background(), f.root, "HEAD")
	if err != nil || stats.RecordsRead != before.RecordsRead+1 || stats.Links != before.Links || stats.LinksDropped != before.LinksDropped+1 {
		t.Errorf("wrong span digest: stats %+v %v", stats, err)
	}
	// The undamaged record is read and adds its link.
	commitAll(t, f.root, map[string]string{recordDirectory + "/" + f.c3 + ".cem.json": good})
	_, stats, err = Export(context.Background(), f.root, "HEAD")
	if err != nil || stats.RecordsRead != before.RecordsRead+1 || stats.Links != before.Links+1 {
		t.Errorf("good record: stats %+v %v", stats, err)
	}
}

// TBC-V1-012: a repository with no records exports an empty, valid file.
func TestExportWithoutRecords(t *testing.T) {
	root := newRepo(t, map[string]string{"a.go": "x\n"})
	data, stats, err := Export(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	file, perr := ParseLinks(data)
	if perr != nil || file.Entries() != 0 || stats.Links != 0 || stats.RecordsRead != 0 || !bytes.HasSuffix(data, []byte("\n")) {
		t.Errorf("empty export: %v %+v %s", perr, stats, data)
	}
}

// TBC-V1-012: a record whose commit the repository lacks is skipped; an input that is not a
// commit is an error.
func TestExportRefusals(t *testing.T) {
	f := newExportFixture(t)
	ghost := strings.Repeat("c", 40)
	commitAll(t, f.root, map[string]string{
		recordDirectory + "/" + ghost + ".cem.json": `{"spec":"cem/1.0","evidence":[],"hunks":[]}`,
		recordDirectory + "/" + f.c3 + ".cem.json":  `{"spec":"cem/0.9"}`,
	})
	_, stats, err := Export(context.Background(), f.root, "HEAD")
	if err != nil || stats.RecordsRead != 2 || stats.RecordsSkipped != 3 {
		t.Errorf("stats %+v %v", stats, err)
	}
	for _, revision := range []string{"", "no-such-ref", "-x", "HEAD:docs/spec.md", f.specBlob} {
		if data, _, err := Export(context.Background(), f.root, revision); err == nil || data != nil {
			t.Errorf("revision %q accepted", revision)
		}
	}
	// Export writes nothing.
	if out := git(t, f.root, "status", "--porcelain=v1"); out != "" {
		t.Errorf("worktree changed: %s", out)
	}
	if _, err := os.Stat(filepath.Join(f.root, LinksPath)); err == nil {
		t.Error("export wrote the link file")
	}
}
