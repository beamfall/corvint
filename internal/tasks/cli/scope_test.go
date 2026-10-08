package cli_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0022_CLIUsesOptInPack(t *testing.T) {
	t.Run("CAL-V0-022 CLI scope precedence", func(t *testing.T) {
		for _, tc := range []struct {
			name, format, declared, requested, want string
			next                                    bool
		}{
			{name: "default", want: "WHOLE_REPOSITORY"},
			{name: "unselective", format: "pack", want: "WHOLE_REPOSITORY"},
			{name: "pack", format: "pack", want: "DERIVED"},
			{name: "next", format: "pack", want: "DERIVED", next: true},
			{name: "next-requested", format: "pack", requested: "manual/", want: "REQUESTED", next: true},
			{name: "requested", format: "pack", requested: "manual/", want: "REQUESTED"},
			{name: "declared", format: "pack", declared: `["declared/"]`, requested: "manual/", want: "DECLARED"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r := fixture.TempRepo(t)
				git := func(args ...string) {
					t.Helper()
					c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
					c.Dir = r.Root
					c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
					if out, err := c.CombinedOutput(); err != nil {
						t.Fatalf("git: %v %s", err, out)
					}
				}
				git("init", "-q", "-b", "main")
				git("config", "user.name", "test")
				git("config", "user.email", "test@example.test")
				fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
				policy := fixture.PolicyValue()
				budgets, _ := policy.Obj.Get("budgets")
				budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
				fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
				if x := atm(t, r.Root, nil, "init"); x.code != 0 {
					t.Fatalf("init: %+v", x.res)
				}
				paths := tc.declared
				if paths == "" {
					paths = "[]"
				}
				payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"Repair HydrateWidget"`, `"body":null`, `"body":"widget.go"`, `"touchPaths":[]`, `"touchPaths":`+paths).Replace(createPayloadJSON)
				created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-widget", "--payload", payload)
				if created.code != 0 {
					t.Fatalf("create: %+v", created.res)
				}
				id := field(created.res.Items[0], "ticketId").Str
				fixture.Write(t, filepath.Join(r.Root, "go.mod"), []byte("module example.test/widget\n\ngo 1.27\n"))
				fixture.Write(t, filepath.Join(r.Root, "widget.go"), []byte("package widget\nfunc HydrateWidget(value string) string { return value }\n"))
				// Both the ticket and source repeat the task terms. Give the query a
				// realistic corpus where those terms are selective; tiny corpora abstain.
				if tc.name != "unselective" {
					for i := 0; i < 64; i++ {
						fixture.Write(t, filepath.Join(r.Root, fmt.Sprintf("notes/%02d.txt", i)), []byte("unrelated documentation\n"))
					}
				}
				git("add", ".")
				git("commit", "-qm", "fixture")
				t.Setenv("CORVINT_SNAPSHOT_FORMAT", "pack")
				index, err := contextindex.BuildForSnapshot(context.Background(), r.Root)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = contextindex.WriteSnapshot(index); err != nil {
					t.Fatal(err)
				}
				before := fixture.TreeSnapshot(t, filepath.Join(r.Root, ".corvint"))
				t.Setenv("CORVINT_SNAPSHOT_FORMAT", tc.format)
				args := []string{"claim", id, "--request-id", "scope-claim", "--holder", "scope-test"}
				if tc.next {
					args[1] = "--next"
				}
				if tc.requested != "" {
					args = append(args, "--scope", tc.requested)
				}
				x := atm(t, r.Root, nil, args...)
				if x.code != 0 {
					t.Fatalf("claim: %+v", x.res)
				}
				raw, err := os.ReadFile(filepath.Join(r.StateDir, "attempts", field(x.res.Items[0], "attemptId").Str+".json"))
				if err != nil {
					t.Fatal(err)
				}
				attempt, err := snapshot.DecodeAttempt(raw)
				if err != nil {
					t.Fatal(err)
				}
				if attempt.Scope.Source != tc.want {
					t.Fatalf("scope: %+v", attempt.Scope)
				}
				if tc.want == "DERIVED" {
					found := false
					for _, p := range attempt.Scope.Resources {
						if p.Key == "widget.go" {
							found = true
						}
					}
					if !found || attempt.Scope.DerivationSha256 == nil {
						t.Fatalf("unpinned source scope: %+v", attempt.Scope)
					}
				}
				if !fixture.SameTree(before, fixture.TreeSnapshot(t, filepath.Join(r.Root, ".corvint"))) {
					t.Fatal("claim changed the context index")
				}
			})
		}
	})
}
