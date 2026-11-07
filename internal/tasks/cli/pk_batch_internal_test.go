// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// TCL-V0-7404: the precheck uses the same admission as a retry claim.
func TestTCLV07404RestoredScopeAndRequestBinding(t *testing.T) {
	id, err := wire.ParseTicketID("test", "ticket:acme:main:AT-0001")
	if err != nil {
		t.Fatal(err)
	}
	rec := &ticket.Record{TicketID: id, AcceptanceRevision: "1", Effects: ticket.Effects{Coverage: "QUALIFIED", TouchPaths: []string{"docs/"}}}
	prior := &snapshot.Attempt{TicketID: id, TicketRevision: "1", Generation: "1", Phase: "RELEASED", Scope: &snapshot.Scope{Source: "DECLARED", Resources: []ticket.Resource{{Class: "PATH", Key: "src/"}, {Class: "PATH", Key: "docs/"}}}}
	older := *prior
	older.Generation = "0"
	older.Scope = &snapshot.Scope{Source: "DECLARED", Resources: []ticket.Resource{{Class: "PATH", Key: "other/"}, {Class: "PATH", Key: "src/a.go"}}}
	restored := transaction.RestorableScope(rec, map[string]*snapshot.Attempt{"a": prior, "old": &older})
	if restored == nil || strings.Join(restored.Paths, " ") != "src/" {
		t.Fatalf("restored=%+v, want only last generation's widen", restored)
	}
	row := &closeRow{local: "AT-0001", hit: batchHit{ticketID: id.Raw}, g: &gatedRow{candidate: "aaa", base: "bbb", scope: &claimScope{source: scopeDeclared, keys: []string{"src/"}}}}
	first := row.gatedStepID(50, "claim")
	if first != row.gatedStepID(50, "claim") {
		t.Fatal("unstable")
	}
	row.g.candidate = "ccc"
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("candidate omitted")
	}
	row.g.candidate = "aaa"
	row.g.scope.source = scopeDiff
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("scope source omitted")
	}
	row.g.scope.source = scopeDeclared
	row.g.scope.keys = []string{"docs/"}
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("scope keys omitted")
	}
	prior.TicketRevision = "2"
	if restored := transaction.RestorableScope(rec, map[string]*snapshot.Attempt{"a": prior}); restored != nil {
		t.Fatal("stale acceptance restored")
	}
}

// TCL-V0-7404: a narrow retry after a large widen restores only the widened
// keys its own diff needs. 257 historical keys beside a docs/ declaration fit
// the native resource limit, but carrying them all would exceed the request
// limit and refuse a candidate that changes only declared docs/a.md.
func TestTCLV07404NarrowRetryAfterLargeWiden(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	write("docs/keep.md")
	run("add", "-A")
	run("commit", "-qm", "base")
	base := run("rev-parse", "HEAD")
	write("docs/a.md")
	run("add", "-A")
	run("commit", "-qm", "narrow")
	narrow := run("rev-parse", "HEAD")
	write("w003/x.go")
	run("add", "-A")
	run("commit", "-qm", "wide")
	wide := run("rev-parse", "HEAD")
	var history []string
	for i := 0; i < 257; i++ {
		history = append(history, fmt.Sprintf("w%03d/", i))
	}
	rec := &ticket.Record{Effects: ticket.Effects{Coverage: "QUALIFIED", TouchPaths: []string{"docs/"}}}
	b := &gatedBatch{root: root}
	r := &closeRow{local: "AT-0001"}
	g := &gatedRow{base: base, candidate: narrow, restored: append([]string(nil), history...)}
	b.planScope(r, rec, g)
	if r.code != "" || len(g.restored) != 0 || g.scope.source != scopeDiff {
		t.Fatalf("narrow retry: code=%q detail=%q restored=%d source=%v", r.code, r.detail, len(g.restored), g.scope.source)
	}
	// Control: a candidate reaching one widened key restores exactly that key.
	r = &closeRow{local: "AT-0001"}
	g = &gatedRow{base: base, candidate: wide, restored: append([]string(nil), history...)}
	b.planScope(r, rec, g)
	if r.code != "" || !reflect.DeepEqual(g.restored, []string{"w003/"}) || g.scope.source != scopeDeclared || !reflect.DeepEqual(g.scope.keys, []string{"docs/", "w003/"}) {
		t.Fatalf("needed widen: code=%q detail=%q restored=%v scope=%+v", r.code, r.detail, g.restored, g.scope)
	}
}
