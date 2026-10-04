package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestCALV0071_RepositoryBindingImmutable proves a program's extra repository
// set, checkouts and Git identities never change, a base moves only on
// reassignment (which clears the candidate), and a new program starts with no
// candidate.
func TestCALV0071_RepositoryBindingImmutable(t *testing.T) {
	old := []snapshot.RepositoryRecord{{Name: "docs", Checkout: "/work/docs", CommonIdentity: "dev:1", Base: "base-1", Candidate: "cand-1"}}
	edit := func(f func(*snapshot.RepositoryRecord)) []snapshot.RepositoryRecord {
		r := old[0]
		f(&r)
		return []snapshot.RepositoryRecord{r}
	}
	for _, tc := range []struct {
		name     string
		next     []snapshot.RepositoryRecord
		reassign bool
		ok       bool
	}{
		{"unchanged", old, false, true},
		{"candidate preserved", edit(func(r *snapshot.RepositoryRecord) { r.Candidate = "cand-2" }), false, true},
		{"base moved without reassignment", edit(func(r *snapshot.RepositoryRecord) { r.Base = "base-2" }), false, false},
		{"reassignment moves base and clears candidate", edit(func(r *snapshot.RepositoryRecord) { r.Base = "base-2"; r.Candidate = "" }), true, true},
		{"reassignment keeps candidate", edit(func(r *snapshot.RepositoryRecord) { r.Base = "base-2" }), true, false},
		{"renamed", edit(func(r *snapshot.RepositoryRecord) { r.Name = "site" }), false, false},
		{"moved checkout", edit(func(r *snapshot.RepositoryRecord) { r.Checkout = "/work/other" }), false, false},
		{"other Git directory", edit(func(r *snapshot.RepositoryRecord) { r.CommonIdentity = "dev:2" }), false, false},
		{"dropped", nil, false, false},
		{"added", append(append([]snapshot.RepositoryRecord{}, old...), snapshot.RepositoryRecord{Name: "site"}), false, false},
	} {
		if got := sameRepositories(old, tc.next, tc.reassign); got != tc.ok {
			t.Fatalf("%s: same=%v", tc.name, got)
		}
	}
	if !sameRepositories(nil, nil, false) || !freshRepositories(nil) || freshRepositories(old) || !freshRepositories(edit(func(r *snapshot.RepositoryRecord) { r.Candidate = "" })) {
		t.Fatal("single-repository or fresh admission binding")
	}
}
