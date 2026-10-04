package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCEMStableDispatchBeforeRootReads(t *testing.T) {
	var out, errout bytes.Buffer
	exit := runContext(context.Background(), []string{"--root", "/incompatible-root", "cem", "verify-stable", "--repository", "/unavailable-repository", "--map", "/unavailable-map"}, nil, &out, &errout)
	var result map[string]any
	if e := json.Unmarshal(out.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if exit != 2 || errout.Len() != 0 || len(result) != 21 || result["stage"] != "arguments" || result["code"] != "invalid-arguments" || result["mapSha256"] != nil {
		t.Fatalf("wrong actual dispatch: %d %s %s", exit, out.String(), errout.String())
	}
}
