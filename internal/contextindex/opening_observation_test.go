package contextindex

import (
	"context"
	"errors"
	"testing"
)

// TestBuildRefusesCarriedOpeningObservationErrors pins that a carried opening
// observation whose identity or status read failed never yields an index: the
// bracket's opening observation is the evidence the index is pinned to, and a
// failed observation is a refusal, not an empty one (GPK-V0-007). The carried
// branch reaches this through loadSnapshot when the engine identity is
// unavailable, which hands the observation on with its errors intact.
func TestBuildRefusesCarriedOpeningObservationErrors(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "go.mod", "module example.test/carried\n\ngo 1.27.0\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "one source")
	ctx := context.Background()
	identity, err := readIdentity(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	statusFailure := &Error{Message: "carried status failure"}
	identityFailure := &Error{Message: "carried identity failure"}
	for _, tc := range []struct {
		name    string
		opening repositoryObservation
		want    error
	}{
		{"status", repositoryObservation{identity: identity, statusErr: statusFailure}, statusFailure},
		{"identity", repositoryObservation{identity: identity, identityErr: identityFailure}, identityFailure},
		{"both", repositoryObservation{identity: identity, identityErr: identityFailure, statusErr: statusFailure}, identityFailure},
	} {
		opening := tc.opening
		index, err := buildStableFrom(ctx, root, &opening, (*Index).compile)
		if !errors.Is(err, tc.want) || index != nil {
			t.Fatalf("%s: index=%v err=%v, want the carried error %v", tc.name, index != nil, err, tc.want)
		}
	}
}
