package main

import (
	"bytes"
	"context"
	"testing"
)

func TestCandidateExplicitClosedInvocation(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"verify"}, {"assemble", "--experimental"}, {"verify", "--experimental", "--experimental"}, {"verify", "--experimental", "--patch", "input.patch"}, {"assemble", "--experimental", "--request", "/missing", "--out-dir", "/new", "extra"}} {
		var out, errout bytes.Buffer
		if code := command(context.Background(), args, &out, &errout); code != 2 {
			t.Fatalf("%v: code %d stdout=%s stderr=%s", args, code, out.String(), errout.String())
		}
	}
}
