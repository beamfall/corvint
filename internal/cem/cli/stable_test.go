package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
		exit, handled := RunStableInvocation(context.Background(), args, &out, io.Discard)
		if !handled || exit != 2 {
			t.Fatalf("dispatch %v %d", handled, exit)
		}
		var r map[string]any
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		if len(r) != 23 || r["stage"] != "arguments" || r["mapSha256"] != nil || r["code"] != "invalid-arguments" {
			t.Fatalf("reads occurred before root refusal: %s", out.Bytes())
		}
	}
}
func TestStableMatchingRootDoesNotReplaceMapRead(t *testing.T) {
	var out bytes.Buffer
	exit, ok := RunStableInvocation(context.Background(), []string{"--root", "/missing", "cem", "verify-stable", "--repository", "/missing", "--map", "/missing-map"}, &out, io.Discard)
	var r map[string]any
	_ = json.Unmarshal(out.Bytes(), &r)
	if !ok || exit != 2 || r["stage"] != "input" || r["code"] != "map-unavailable" {
		t.Fatal(out.String())
	}
}
func TestStableDispatchDoesNotCaptureOldCEM(t *testing.T) {
	var out bytes.Buffer
	if _, ok := RunStableInvocation(context.Background(), []string{"cem", "verify"}, &out, io.Discard); ok {
		t.Fatal("legacy captured")
	}
}

// CEM-V1-204: the converter is retired, so `cem convert` is no longer a stable
// entry; the ordinary cem parser refuses it as an unknown action.
func TestCEMV1204StableDispatchNoLongerCapturesConvert(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{
		{"cem", "convert", "--map", "/missing-map", "--output", "/missing-out"},
		{"--root", "/missing", "cem", "convert", "--map", "/missing-map", "--in-place"},
	} {
		if _, ok := RunStableInvocation(context.Background(), args, &out, io.Discard); ok || out.Len() != 0 {
			t.Fatalf("%q captured by the stable entry: %s", args, out.String())
		}
	}
}

func TestStableDispatchDoesNotCaptureOtherCommandValues(t *testing.T) {
	for _, args := range [][]string{
		{"query", "--task", "cem", "verify-stable"},
		{"context", "--task", "cem", "verify-stable"},
		{"query", "x", "--", "cem", "verify-stable"},
		{"cem", "verify", "--map", "cem", "verify-stable"},
		{"--root", "/missing", "query", "--task", "cem", "verify-stable"},
	} {
		var out bytes.Buffer
		if _, ok := RunStableInvocation(context.Background(), args, &out, io.Discard); ok || out.Len() != 0 {
			t.Fatalf("captured %v: %s", args, out.Bytes())
		}
	}
}
