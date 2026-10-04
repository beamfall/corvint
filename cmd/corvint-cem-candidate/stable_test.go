package main

import (
	"bytes"
	"context"
	"testing"
)

func TestStableAssemblyExplicitClosedInvocation(t *testing.T) {
	for _, args := range [][]string{{"assemble-stable"}, {"assemble-stable", "--request", "/missing", "--out-dir", "/new"}, {"assemble-stable", "--experimental", "--experimental"}, {"assemble-stable", "--experimental", "--request", "/missing", "--request", "/other", "--out-dir", "/new"}, {"assemble-stable", "--experimental", "--request", "/missing", "--out-dir", "/new", "--authority", "yes"}, {"assemble-stable", "--experimental", "--request", "/missing", "--out-dir", "/new", "extra"}} {
		var out, errout bytes.Buffer
		if code := command(context.Background(), args, &out, &errout); code != 2 || out.Len() != 0 {
			t.Fatalf("%v code=%d out=%s err=%s", args, code, out.String(), errout.String())
		}
	}
}
