package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func cachePath(repo *intent.Repository) string {
	return filepath.Join(repo.CommonDir, store.AuditCacheNameForTest)
}

// outcome renders everything a writer reports, so a cached and an uncached
// run can be compared.
func outcome(report *store.Report, err error) string {
	return fmt.Sprintf("%s|%s|%v|%s|%s|%s|%s", wire.CodeOf(err), report.Outcome.Outcome, report.Outcome.Codes, report.Kind, report.Receipt, report.Ticket, report.Detail)
}

func mutateAt(repo *intent.Repository, env []byte, ts wire.Timestamp) string {
	report, err := store.Mutate(context.Background(), repo, operator(), env, ts)
	return outcome(report, err)
}

// mutateTwice runs env against the store with the cache in place, and, for a
// refusal, again with the cache removed, and requires the same result.
func mutateTwice(t *testing.T, repo *intent.Repository, env []byte, ts wire.Timestamp) string {
	t.Helper()
	cached := mutateAt(repo, env, ts)
	if strings.Contains(cached, "|"+string(mutation.OutcomeCompleted)+"|") {
		return cached
	}
	if err := os.Remove(cachePath(repo)); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if full := mutateAt(repo, env, ts); full != cached {
		t.Fatalf("cached run %q, full audit %q", cached, full)
	}
	return cached
}

func treeSha(t *testing.T, repo *intent.Repository) wire.Digest {
	t.Helper()
	tree, err := intent.TreeDigest(repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	return tree.Sha256
}

// TestCALV0026_WritesReuseTheVerifiedAudit: after one full audit, ticket
// mutations, a replay, PAUSE/UNPAUSE and a policy update all plan against
// the verified cache and take no further full audit, and the store still
// audits clean.
func TestCALV0026_WritesReuseTheVerifiedAudit(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("cached")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted || !store.AuditCacheHitForTest(repo) {
		t.Fatalf("create: %+v hit %v", created, store.AuditCacheHitForTest(repo))
	}
	before := store.FullAuditsForTest()
	refine := envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("Refined")))
	if r := mutate(t, repo, refine); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("refine: %+v", r)
	}
	if r := mutate(t, repo, refine); r.Kind != "Replay" || r.Ticket != created.Ticket {
		t.Fatalf("replay: %+v", r)
	}
	if r := changeBarrier(t, repo, transaction.Pause, "pause"); r.Receipt == "" {
		t.Fatalf("pause: %+v", r)
	}
	if r := changeBarrier(t, repo, transaction.Unpause, "unpause"); r.Receipt == "" {
		t.Fatalf("unpause: %+v", r)
	}
	report, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t))
	if err != nil || report.Receipt == "" || report.OldPolicySha256 != wire.Sum(fixture.PolicyBytes()) {
		t.Fatalf("policy: %+v %v", report, err)
	}
	if got := store.FullAuditsForTest() - before; got != 0 {
		t.Fatalf("%d full audits after the first, want 0", got)
	}
	if !store.AuditCacheHitForTest(repo) {
		t.Fatal("cache does not validate after its own writes")
	}
	auditOK(t, repo)
}

// TestCALV0026_CacheHitMatchesFullAudit runs one sequence on two stores, one
// planning against the cache and one with the cache removed before every
// write, and requires identical reports and intent trees.
func TestCALV0026_CacheHitMatchesFullAudit(t *testing.T) {
	cached, _ := initialized(t)
	full, _ := initialized(t)
	ts := now(t)
	created := mutate(t, cached, envelope("c1", mutation.OpCreate, "", "", createPayload("one")))
	mutate(t, full, envelope("c1", mutation.OpCreate, "", "", createPayload("one")))
	id := created.Ticket
	steps := [][]byte{
		envelope("c2", mutation.OpCreate, "", "", createPayload("two")),
		envelope("r1", mutation.OpRefine, id, "1", obj("title", str("one'"))),
		envelope("r1", mutation.OpRefine, id, "1", obj("title", str("one'"))),
		envelope("r1", mutation.OpRefine, id, "1", obj("title", str("other bytes"))),
		envelope("r2", mutation.OpRefine, id, "1", obj("title", str("stale"))),
		envelope("r3", mutation.OpRefine, id, "2", obj("title", str("fresh"))),
	}
	kinds := map[string]int{}
	for i, env := range steps {
		a := mutateAt(cached, env, ts)
		if err := os.Remove(cachePath(full)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		b := mutateAt(full, env, ts)
		if a != b {
			t.Fatalf("step %d: cached %q, full %q", i, a, b)
		}
		kinds[strings.Split(a, "|")[1]]++
	}
	if kinds[string(mutation.OutcomeCompleted)] != 4 || len(kinds) < 2 {
		t.Fatalf("outcomes %v, want commits, a replay and refusals", kinds)
	}
	if treeSha(t, cached) != treeSha(t, full) {
		t.Fatal("intent trees differ")
	}
	auditOK(t, cached)
}

// TestCALV0026_TamperedTicketTakesTheFullAudit: an edited ticket file moves
// the intent tree digest, so the cache misses and the write is refused as the
// full audit refuses it.
func TestCALV0026_TamperedTicketTakesTheFullAudit(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("x")))
	path := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", created.Ticket[strings.LastIndex(created.Ticket, ":")+1:]+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, path, []byte(strings.Replace(string(raw), `"x"`, `"y"`, 1)))
	if store.AuditCacheHitForTest(repo) {
		t.Fatal("cache validates over an edited ticket")
	}
	before := storeDigest(t, repo)
	got := mutateTwice(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("z"))), now(t))
	if strings.Contains(got, "|"+string(mutation.OutcomeCompleted)+"|") || storeDigest(t, repo) != before {
		t.Fatalf("write over an edited ticket: %s", got)
	}
}

// TestCALV0026_ChangedStateStampTakesTheFullAudit rewrites a request file in
// place, same size, and restores its mtime. Its change time still moves, so
// the cache misses without relying on the racy re-hash.
func TestCALV0026_ChangedStateStampTakesTheFullAudit(t *testing.T) {
	defer store.SetRacyWindowForTest(0)()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("x")))
	mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("y"))))
	if !store.AuditCacheHitForTest(repo) {
		t.Fatal("no cache hit before tampering")
	}
	rel, err := snapshot.RequestPath("req-create")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(raw), "COMPLETED")
	copy(raw[i:], "COMPLETEX")
	fixture.Write(t, path, raw)
	if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if store.AuditCacheHitForTest(repo) {
		t.Fatal("cache validates over a rewritten request file")
	}
	got := mutateTwice(t, repo, envelope("req-2", mutation.OpRefine, created.Ticket, "2", obj("title", str("z"))), now(t))
	if strings.Contains(got, "|"+string(mutation.OutcomeCompleted)+"|") {
		t.Fatalf("write over a rewritten request: %s", got)
	}
}

// TestCALV0026_ForeignWriteTakesTheFullAudit: a receipt appended by a writer
// that did not refresh the cache moves the head, so the next write takes the
// full audit and commits; a stray receipt file is refused as the full audit
// refuses it.
func TestCALV0026_ForeignWriteTakesTheFullAudit(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("x")))
	stale, err := os.ReadFile(cachePath(repo))
	if err != nil {
		t.Fatal(err)
	}
	mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("y"))))
	fixture.Write(t, cachePath(repo), stale)
	if store.AuditCacheHitForTest(repo) {
		t.Fatal("stale cache validates after a foreign receipt")
	}
	before := store.FullAuditsForTest()
	if r := mutate(t, repo, envelope("req-2", mutation.OpRefine, created.Ticket, "2", obj("title", str("z")))); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("write after foreign receipt: %+v", r)
	}
	if store.FullAuditsForTest() != before+1 || !store.AuditCacheHitForTest(repo) {
		t.Fatal("foreign receipt did not force exactly one full audit and a fresh cache")
	}
	auditOK(t, repo)

	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", "000000000009.json"), []byte("{}\n"))
	if store.AuditCacheHitForTest(repo) {
		t.Fatal("cache validates over a stray receipt")
	}
	got := mutateTwice(t, repo, envelope("req-3", mutation.OpRefine, created.Ticket, "3", obj("title", str("w"))), now(t))
	if strings.Contains(got, "|"+string(mutation.OutcomeCompleted)+"|") {
		t.Fatalf("write over a stray receipt: %s", got)
	}
}

// TestCALV0026_DeletedOrCorruptCacheTakesTheFullAudit: the cache is derived
// state; without it, or with any byte changed, a write takes the full audit,
// commits, and leaves a fresh cache.
func TestCALV0026_DeletedOrCorruptCacheTakesTheFullAudit(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("x")))
	damage := []func(){
		func() {
			if err := os.Remove(cachePath(repo)); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			raw, err := os.ReadFile(cachePath(repo))
			if err != nil {
				t.Fatal(err)
			}
			raw[len(raw)/2] ^= 1
			fixture.Write(t, cachePath(repo), raw)
		},
	}
	for i, d := range damage {
		d()
		if store.AuditCacheHitForTest(repo) {
			t.Fatalf("case %d: damaged cache validates", i)
		}
		before := store.FullAuditsForTest()
		rev := string(wire.CountOf(int64(i + 1)))
		r := mutate(t, repo, envelope("req-"+rev, mutation.OpRefine, created.Ticket, rev, obj("title", str("t"+rev))))
		if r.Outcome.Outcome != mutation.OutcomeCompleted || store.FullAuditsForTest() != before+1 || !store.AuditCacheHitForTest(repo) {
			t.Fatalf("case %d: %+v", i, r)
		}
	}
	auditOK(t, repo)
}

func importItems(n int) []byte {
	items := []string{importItem("BF-0", strings.Repeat("x", wire.MaxBodyBytes), nil)}
	for i := 1; i < n; i++ {
		id := "BF-" + string(wire.CountOf(int64(i)))
		items = append(items, importItem(id, id+"\n", nil))
	}
	return importExport(items...)
}

// TestCALV0018_ImportAuditsOnce: a multi-batch first import takes one full
// audit, and a re-import plans against the cache it left.
func TestCALV0018_ImportAuditsOnce(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	export := importItems(25)
	before := store.FullAuditsForTest()
	report := runImport(t, repo, export)
	if report.Planned != 25 || len(report.Batches) < 3 || store.FullAuditsForTest() != before+1 {
		t.Fatalf("planned %d batches %d full audits %d", report.Planned, len(report.Batches), store.FullAuditsForTest()-before)
	}
	auditOK(t, repo)
	if again := runImport(t, repo, export); again.Planned != 0 || store.FullAuditsForTest() != before+1 {
		t.Fatalf("re-import: %+v", again)
	}
}

// TestCALV0018_ImportResumesAfterCrash stops an import before its second
// batch's receipt; a rerun completes the rest and the store audits clean.
func TestCALV0018_ImportResumesAfterCrash(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	export := importItems(25)
	calls := 0
	crash := func() error {
		calls++
		if calls == 2 {
			return errors.New("crash")
		}
		return nil
	}
	report, err := store.ImportWithFaultsForTest(context.Background(), repo, operator(), fixture.QueueID, export, now(t), crash)
	if err == nil || len(report.Batches) != 1 {
		t.Fatalf("crashed import: %+v %v", report, err)
	}
	rest := runImport(t, repo, export)
	if rest.Planned == 0 || rest.Planned >= 25 {
		t.Fatalf("resume planned %d", rest.Planned)
	}
	for i := 0; i < 25; i++ {
		readImported(t, repo, "BF-"+string(wire.CountOf(int64(i))))
	}
	auditOK(t, repo)
	if again := runImport(t, repo, export); again.Planned != 0 {
		t.Fatalf("re-import planned %d", again.Planned)
	}
}

// TestCALV0018_ImportRechecksPostedDestinations: a file appearing at a later
// batch's destination after the import's audit (here while the second batch
// commits) stops that batch with SNAPSHOT_MOVED before anything of it is
// written.
func TestCALV0018_ImportRechecksPostedDestinations(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	stray := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "BF-24.json")
	calls := 0
	intrude := func() error {
		calls++
		if calls == 2 {
			fixture.Write(t, stray, []byte("{}\n"))
		}
		return nil
	}
	report, err := store.ImportWithFaultsForTest(context.Background(), repo, operator(), fixture.QueueID, importItems(25), now(t), intrude)
	if wire.CodeOf(err) != wire.CodeSnapshotMoved || len(report.Batches) != 2 {
		t.Fatalf("import: %+v %v", report, err)
	}
	if raw, err := os.ReadFile(stray); err != nil || string(raw) != "{}\n" {
		t.Fatalf("stray destination overwritten: %q %v", raw, err)
	}
}
