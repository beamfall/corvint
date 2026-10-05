package snapshot_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0071_ProgramRepositoryRecords(t *testing.T) {
	base := strings.Repeat("a", 40)
	program := func(repos []snapshot.RepositoryRecord, worktrees ...snapshot.WorktreeRecord) snapshot.Programs {
		return snapshot.Programs{Profile: "taskman-programs/0", QueueID: "queue:acme:main", Entries: []snapshot.Program{{ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 1, OwnerStarted: "start", Epoch: 1, ConfigSHA256: string(wire.Sum(nil)), Phase: "ADMITTED", Repositories: repos, Worktrees: worktrees}}}
	}
	raw, e := program(nil, snapshot.WorktreeRecord{Path: "/w/implement-1", Commit: base}).Encode()
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte(`"repositories"`)) || bytes.Contains(raw, []byte(`"repository"`)) {
		t.Fatalf("single-repository bytes changed: %s", raw)
	}
	docs := snapshot.RepositoryRecord{Name: "docs", Checkout: "/work/docs", CommonIdentity: "dev:1", Base: base}
	site := snapshot.RepositoryRecord{Name: "site", Checkout: "/work/site", CommonIdentity: "dev:2", Base: base, Candidate: base}
	raw, e = program([]snapshot.RepositoryRecord{docs, site}, snapshot.WorktreeRecord{Path: "/w/implement-1@docs", Commit: base, Repository: "docs"}).Encode()
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := snapshot.DecodePrograms(raw)
	if e != nil || len(decoded.Entries[0].Repositories) != 2 || decoded.Entries[0].Worktrees[0].Repository != "docs" {
		t.Fatalf("round trip %+v %v", decoded, e)
	}
	nine := []snapshot.RepositoryRecord{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		r := docs
		r.Name = name
		nine = append(nine, r)
	}
	bad := func(edit func(*snapshot.RepositoryRecord)) []snapshot.RepositoryRecord {
		r := docs
		edit(&r)
		return []snapshot.RepositoryRecord{r}
	}
	for name, p := range map[string]snapshot.Programs{
		"unsorted":            program([]snapshot.RepositoryRecord{site, docs}),
		"duplicate":           program([]snapshot.RepositoryRecord{docs, docs}),
		"over limit":          program(nine),
		"invalid name":        program(bad(func(r *snapshot.RepositoryRecord) { r.Name = "Docs" })),
		"no checkout":         program(bad(func(r *snapshot.RepositoryRecord) { r.Checkout = "" })),
		"no identity":         program(bad(func(r *snapshot.RepositoryRecord) { r.CommonIdentity = "" })),
		"no base":             program(bad(func(r *snapshot.RepositoryRecord) { r.Base = "" })),
		"undeclared worktree": program([]snapshot.RepositoryRecord{docs}, snapshot.WorktreeRecord{Path: "/w/x@site", Commit: base, Repository: "site"}),
	} {
		if _, e := p.Encode(); e == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
