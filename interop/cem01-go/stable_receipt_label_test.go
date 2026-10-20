package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestCEMV1312PortableOpaqueReceiptLabel is the portable side of CEM-V1-312:
// the public neutral case with its runner receipt labelled opaque/0 ACCEPTs
// with the public case's axes, and any other unknown label is refused.
func TestCEMV1312PortableOpaqueReceiptLabel(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := neutralCaseFiles(t)
	var want neutralExpected
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	repo, base, target := neutralRepository(t, dir)
	mapBytes, err := os.ReadFile(filepath.Join(dir, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`"profile":"corvint-test-runner-receipt/0"`)
	if bytes.Count(mapBytes, legacy) != 1 {
		t.Fatal("public case does not carry exactly one legacy runner label")
	}
	opaque := bytes.Replace(mapBytes, legacy, []byte(`"profile":"opaque/0"`), 1)
	r, exit := runStableCanonical(context.Background(), repo, opaque, base, target, dir, nil)
	if got := neutralResultMap(t, r); exit != 0 || r.Outcome != "ACCEPT" || !reflect.DeepEqual(got["axes"], want.Accept.Result["axes"]) {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonical exit=%d result=%s, want ACCEPT with the public case axes", exit, b)
	}
	other := bytes.Replace(mapBytes, legacy, []byte(`"profile":"opaque/1"`), 1)
	if r, exit := runStableCanonical(context.Background(), repo, other, base, target, dir, nil); exit == 0 || r.Outcome == "ACCEPT" {
		t.Fatalf("an unknown runner label was accepted: exit=%d %s", exit, r.Outcome)
	}
}
