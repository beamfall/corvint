package frontiernextrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

func countAdapterGit(t *testing.T) func() int {
	t.Helper()
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	// exec leaves Git as the sole child under the existing contained runner.
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\nprintf x >>"+quote(log)+"\nexec "+quote(binary)+" \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		raw, err := os.ReadFile(log)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return len(raw)
	}
}

func TestRequestAdapterPreservesFixtureUniverseAndChecks(t *testing.T) {
	t.Run("PLE-V0-009 memo adapter rechecks canonical inputs with fixture authority NONE", func(t *testing.T) {
		source, err := os.ReadFile("../wp3codec/codec.go")
		if err != nil {
			t.Fatal(err)
		}
		driver, err := os.ReadFile("../../tools/local-authority/driver.go")
		if err != nil {
			t.Fatal(err)
		}
		root, request, policy := fixtureRequest(t, source, driver, false)
		newMemo := func() *gitauth.RequestReadMemo {
			view, err := gitauth.Open(root, gitrun.NewDefaultBudget())
			if err != nil {
				t.Fatal(err)
			}
			if err = view.LoadObjectFormat(context.Background()); err != nil {
				t.Fatal(err)
			}
			m, err := gitauth.NewRequestReadMemo(view)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(m.Release)
			return m
		}
		count := countAdapterGit(t)
		for _, status := range []string{"PASS", "FAIL"} {
			sign(t, &request, &policy, status)
			start := count()
			want, err := frontiernext.ComputeFixture(context.Background(), request, policy, time.Unix(1200, 0), Adapter{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			ordinaryCalls := count() - start
			memo := newMemo()
			start = count()
			got, err := frontiernext.ComputeFixture(context.Background(), request, policy, time.Unix(1200, 0), NewRequestAdapter(memo))
			if err != nil {
				t.Fatal(err)
			}
			memoCalls := count() - start
			wantRaw, _ := localauthority.Canonical(want)
			gotRaw, _ := localauthority.Canonical(got)
			if !reflect.DeepEqual(want, got) || string(wantRaw) != string(gotRaw) || got.Authority != "NONE" || memoCalls >= ordinaryCalls {
				t.Fatalf("fixture parity/calls status=%s ordinary=%d memo=%d got=%+v want=%+v", status, ordinaryCalls, memoCalls, got, want)
			}
			t.Logf("component only status=%s ordinary Git calls=%d memo Git calls=%d", status, ordinaryCalls, memoCalls)
			// Even a warmed session cannot reuse a verdict after a canonical
			// input changes. The real verifier must still reject the bad map.
			bad := request
			bad.CEM = []byte("{}")
			var selection frontiernext.Selection
			if err := localauthority.Decode(request.Selection, &selection); err != nil {
				t.Fatal(err)
			}
			if _, err := NewRequestAdapter(memo).Recompute(context.Background(), bad, selection); err == nil {
				t.Fatal("memo bypassed canonical verification")
			}
			memo.Release()
		}
		memo := newMemo()
		reader, err := memo.Open(gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		want, err := lrfrepo.VerifyUniverse(context.Background(), root, request.CEM, request.OCM, request.Enrollment.Binding.Base, request.Enrollment.Binding.Target)
		if err != nil {
			t.Fatal(err)
		}
		got, err := lrfrepo.VerifyUniverseWithRepository(context.Background(), reader, request.CEM, request.OCM, request.Enrollment.Binding.Base, request.Enrollment.Binding.Target)
		if err != nil || !reflect.DeepEqual(want, got) {
			t.Fatalf("repository entry parity: %v", err)
		}
	})
}
