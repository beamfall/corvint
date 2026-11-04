// SPDX-License-Identifier: Apache-2.0
package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cliconformance"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTCLV07704ReadPapercuts(t *testing.T) {
	t.Run("attempt fields", func(t *testing.T) {
		root, claims := leaseCLIStore(t, 1, time.Now().UTC().Add(-time.Minute))
		x := atm(t, root, nil, "attempt", "show", claims[0].AttemptID, "--fields", "leaseExpiresAt,phase")
		if x.code != 0 && (!hasCode(x.res, wire.CodeMalformed) || !strings.Contains(strings.Join(x.res.Warnings, " "), "lease")) {
			t.Fatalf("fields need valid-key guidance: %s", x.stdout)
		}
		phase := atm(t, root, nil, "attempt", "show", claims[0].AttemptID, "--fields", "phase")
		if phase.code != 0 || field(phase.res.Items[0], "phase").Str == "" {
			t.Fatalf("phase: %s", phase.stdout)
		}
	})
	t.Run("search positional", func(t *testing.T) {
		r, _, _ := papercutRepo(t)
		x := atm(t, r.Root, nil, "ticket", "search", "a query")
		if x.code != 0 && (!hasCode(x.res, wire.CodeMalformed) || !strings.Contains(strings.Join(x.res.Warnings, " "), "--text")) {
			t.Fatalf("positional query needs --text remedy: %s", x.stdout)
		}
	})
	t.Run("gate local", func(t *testing.T) {
		r, local, _ := papercutRepo(t)
		x := atm(t, r.Root, nil, "gate", "history", local, "--gate", "gate")
		if x.code != 0 {
			t.Fatalf("local gate history: %s", x.stdout)
		}
	})
	t.Run("progress help", func(t *testing.T) {
		r, _, _ := papercutRepo(t)
		x := atm(t, r.Root, nil, "ticket", "progress", "--help")
		if x.code != 0 {
			t.Fatalf("help: %s", x.stdout)
		}
		types := field(x.res.Items[0], "payloadKeyTypes")
		for _, key := range []string{"host", "lane", "pr"} {
			if !strings.Contains(field(types, key).Str, "null") {
				t.Errorf("%s must document null: %s", key, x.stdout)
			}
		}
		if !strings.Contains(field(types, "phase").Str, "BLOCKED") {
			t.Fatalf("missing phase enum: %s", x.stdout)
		}
		for _, key := range []string{"host", "lane", "pr", "phase"} {
			found := false
			for _, v := range field(x.res.Items[0], "payloadKeys").Arr {
				found = found || v.Str == key
			}
			if !found {
				t.Errorf("required key %s absent", key)
			}
		}
	})
}

func TestCAPV07707TasksMissingTicketProcess(t *testing.T) {
	r, _, _ := papercutRepo(t)
	e := cliconformance.Failure(t, cli.CompiledTasksBinary(t), []string{"--root", r.Root, "ticket", "show", "NOPE-9999"}, 2)
	if e.Code == "" || !strings.Contains(e.Error, "does not exist") {
		t.Fatalf("missing ticket: %+v", e)
	}
}
