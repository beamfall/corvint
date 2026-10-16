package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cc "github.com/Beamfall/corvint/internal/cemcandidate"
)

func TestStableAssemblyExplicitClosedInvocation(t *testing.T) {
	for _, args := range [][]string{{"assemble-stable"}, {"assemble-stable", "--request", "/missing", "--out-dir", "/new"}, {"assemble-stable", "--experimental", "--experimental"}, {"assemble-stable", "--experimental", "--request", "/missing", "--request", "/other", "--out-dir", "/new"}, {"assemble-stable", "--experimental", "--request", "/missing", "--out-dir", "/new", "--authority", "yes"}, {"assemble-stable", "--experimental", "--request", "/missing", "--out-dir", "/new", "extra"}} {
		var out, errout bytes.Buffer
		if code := command(context.Background(), args, &out, &errout); code != 2 || out.Len() != 0 {
			t.Fatalf("%v code=%d out=%s err=%s", args, code, out.String(), errout.String())
		}
	}
}

// TestCEMV1240PlainProfileSelectsPlainDecoder proves the plain request
// profile reaches the plain assembler's closed decoder and validation, while
// a plain-shaped request under any other profile is refused by the Tasks-bound
// decoder (CEM-V1-240).
func TestCEMV1240PlainProfileSelectsPlainDecoder(t *testing.T) {
	pin := `{"path":"/missing","sha256":"` + strings.Repeat("0", 64) + `"}`
	body := `,"repository":"relative","expectedBase":"` + strings.Repeat("1", 40) + `","target":"` + strings.Repeat("2", 40) + `","sourceMap":` + pin + `,"runnerPlan":` + pin + `,"runnerReceipt":` + pin + `,"criteria":[]}`
	for profile, want := range map[string]string{cc.PlainAssemblyProfile: "incomplete plain stable assembly request", cc.StableAssemblyProfile: "criteria"} {
		dir, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		request := filepath.Join(dir, "request.json")
		if e := os.WriteFile(request, []byte(`{"profile":"`+profile+`"`+body), 0o600); e != nil {
			t.Fatal(e)
		}
		var out, errout bytes.Buffer
		code := command(context.Background(), []string{"assemble-stable", "--experimental", "--request", request, "--out-dir", filepath.Join(dir, "bundle")}, &out, &errout)
		if code != 1 || out.Len() != 0 || !strings.Contains(errout.String(), "stable assembly refused:") || !strings.Contains(errout.String(), want) {
			t.Fatalf("%s: code=%d out=%s err=%s, want %q", profile, code, out.String(), errout.String(), want)
		}
	}
}
