package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestStableRootPrecedenceBeforeReads(t *testing.T) {
	for _, args := range [][]string{
		{"--root", "/other", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"},
		{"--root", "relative", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"},
		{"--root", "/missing", "--root", "/missing", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"},
		{"--root", "/missing", "cem", "verify-stable", "--map", "/missing-map"},
		{"--unknown", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"},
		{"cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map", "--target", "a", "--target", "b"},
	} {
		var out bytes.Buffer
		exit, handled := RunStableInvocation(context.Background(), args, &out)
		if !handled || exit != 2 {
			t.Fatalf("dispatch %v %d", handled, exit)
		}
		var r map[string]any
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		if len(r) != 21 || r["stage"] != "arguments" || r["mapSha256"] != nil || r["code"] != "invalid-arguments" {
			t.Fatalf("reads occurred before root refusal: %s", out.Bytes())
		}
	}
}
func TestStableMatchingRootDoesNotReplaceMapRead(t *testing.T) {
	var out bytes.Buffer
	exit, ok := RunStableInvocation(context.Background(), []string{"--root", "/missing", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"}, &out)
	var r map[string]any
	_ = json.Unmarshal(out.Bytes(), &r)
	if !ok || exit != 2 || r["stage"] != "input" || r["code"] != "map-unavailable" {
		t.Fatal(out.String())
	}
}
func TestStableDispatchDoesNotCaptureOldCEM(t *testing.T) {
	var out bytes.Buffer
	if _, ok := RunStableInvocation(context.Background(), []string{"cem", "verify"}, &out); ok {
		t.Fatal("legacy captured")
	}
}
