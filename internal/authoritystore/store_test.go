package authoritystore

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/localauthority"
)

func rootFixture() (RootDocument, GenerationFloor) {
	d := strings.Repeat("a", 64)
	r := RootDocument{Profile: RootProfile, Admission: "OPERATOR_ACCEPTED", KeyClass: "PRODUCTION", RootID: "test-only-parser", PublicKey: d, Epoch: "1", Generation: "2", RepositoryID: d, RepositoryRoot: "/fixture", PolicySHA256: d, Audience: "stop", AuthorityUID: "600", AuthorityGID: "600", ReaderUID: "501", Checks: []localauthority.Check{{ID: "test-only"}}}
	return r, GenerationFloor{Profile: FloorProfile, RootID: r.RootID, Epoch: r.Epoch, Generation: "2"}
}

func encode(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := localauthority.Canonical(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRootParserRejectsRollbackFixtureAndUnknownFields(t *testing.T) {
	t.Run("PLE-V0-007 root parser refuses rollback and revocation", func(t *testing.T) {
		root, floor := rootFixture()
		if _, _, _, err := decodeRoot(encode(t, root), encode(t, floor)); err != nil {
			t.Fatal(err)
		}
		for _, change := range []func(*RootDocument){func(r *RootDocument) { r.Generation = "1" }, func(r *RootDocument) { r.KeyClass = "FIXTURE" }, func(r *RootDocument) { r.Admission = "SELF_ACCEPTED" }, func(r *RootDocument) { r.Revoked = true }, func(r *RootDocument) { r.RootID = "fixture-unadmitted" }, func(r *RootDocument) { r.AuthorityUID = "0" }, func(r *RootDocument) { r.RepositoryRoot = "relative" }} {
			changed := root
			change(&changed)
			if _, _, _, err := decodeRoot(encode(t, changed), encode(t, floor)); err == nil {
				t.Fatalf("accepted %+v", changed)
			}
		}
		raw := encode(t, root)
		raw = append(raw[:len(raw)-1], []byte(`,"callerRoot":true}`)...)
		if _, _, _, err := decodeRoot(raw, encode(t, floor)); err == nil {
			t.Fatal("unknown field accepted")
		}

	})
}

func TestLiveResolveDoesNotAcceptFixtureRootPaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Resolve(ctx, strings.Repeat("a", 64))
	if err == nil || result.RootCurrent {
		t.Fatal("cancelled/default route admitted")
	}
	if _, err = Resolve(context.Background(), "../../tmp/fixture"); err == nil {
		t.Fatal("path accepted")
	}
}

func TestSignerReadPathAllowlist(t *testing.T) {
	h := strings.Repeat("a", 64)
	for _, path := range []string{"private/signing-key", "private/enrollments/" + h + "/objects.json", "private/journal/" + h + ".terminal", "versions/" + h + "/go/bin/go"} {
		if !executionPath(path) {
			t.Fatalf("rejected %s", path)
		}
	}
	for _, path := range []string{"../accepted-root.json", "/etc/passwd", "private/enrollments/" + h + "/../../signing-key", "private/other-key", "versions/" + h + "/go/../key", "private/journal/" + h + ".terminal/extra"} {
		if executionPath(path) {
			t.Fatalf("accepted %s", path)
		}
	}
}
