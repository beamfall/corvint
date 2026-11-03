// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTCXV0977_CaptureReusesOnlyStableContent(t *testing.T) {
	repo := writerStore(t, 70)
	// Make the first file definitely recent even on a loaded test host.
	initial, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	path0 := filepath.Join(repo.IntentRoot(), intent.Dir, initial.Files[0].Path)
	if err := os.WriteFile(path0, initial.Files[0].Raw, 0600); err != nil {
		t.Fatal(err)
	}
	var cache intent.TreeCapture
	first, err := cache.Capture(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	// A stamp that was recent at the retained capture must never become
	// reusable merely by waiting. The next capture must read it again.
	time.Sleep(2100 * time.Millisecond)
	stable, err := cache.Capture(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	if &first.Files[0].Raw[0] == &stable.Files[0].Raw[0] {
		t.Fatal("racy original capture reused after aging")
	}
	reused, err := cache.Capture(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" && &stable.Files[0].Raw[0] != &reused.Files[0].Raw[0] {
		t.Fatal("stable Linux content was read again")
	}
	file := stable.Files[0]
	path := filepath.Join(repo.IntentRoot(), intent.Dir, file.Path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(nil), file.Raw...)
	raw[0] ^= 1
	if err := os.WriteFile(path, raw, info.Mode()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, err := cache.Capture(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	if changed.Sha256 == stable.Sha256 || changed.Files[0].Sha256 != wire.Sum(raw) {
		t.Fatal("restored mtime hid changed content")
	}
}

func TestTCXV0978_AuditDigestsPreservePlansAndRefusals(t *testing.T) {
	repo := writerStore(t, 70)
	indexAgedHere(t, repo)
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	w, decline, terminal := observeWriter(context.Background(), repo, head, "digest-probe", false, false, nil, nil, &tree)
	if decline != nil || terminal != nil {
		t.Fatalf("observation: %v %v", decline, terminal)
	}
	in := transaction.Input{Inventory: w.inv, Head: w.head, HeadReceipt: w.headReceipt, Queue: w.proof.Records["intent/queue.json"].Raw, Policy: w.proof.Records["intent/policy.json"].Raw, Barrier: w.barrier, Reservations: w.reservations, Premise: transaction.LocalOperator, Branch: w.branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: WallClock()}
	for path, record := range w.proof.Records {
		if strings.HasPrefix(path, "intent/tickets/") {
			in.CanonicalTickets = append(in.CanonicalTickets, record.Raw)
			in.CanonicalTicketPaths = append(in.CanonicalTicketPaths, path)
			in.CanonicalTicketDigests = append(in.CanonicalTicketDigests, *record.Sha256)
		}
	}
	in.TicketIndex = intent.ReadClosedIndex(repo.StateDir).WriterSummaries(head.QueueID.Raw, wire.Sum(in.Policy))
	req := transaction.Request{Operation: transaction.Pause, QueueID: head.QueueID.Raw, RequestID: "digest-pause", Actor: historyActor}
	want := transaction.Model(req, in)
	if want.Plan == nil {
		t.Fatalf("no plan: %+v", want)
	}
	wantCost, err := transaction.CheckCapacity(want.Plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, digests := range [][]wire.Digest{nil, in.CanonicalTicketDigests[:1], in.CanonicalTicketDigests} {
		copy := in
		copy.CanonicalTicketDigests = digests
		got := transaction.Model(req, copy)
		if got.Plan == nil || !reflect.DeepEqual(got.Plan.Artifacts(), want.Plan.Artifacts()) || string(got.Plan.Descriptor()) != string(want.Plan.Descriptor()) {
			t.Fatalf("digest vector changed plan: %+v", got)
		}
		cost, err := transaction.CheckCapacity(got.Plan)
		if err != nil || !reflect.DeepEqual(cost.Prefixes, wantCost.Prefixes) {
			t.Fatalf("capacity differs: %v", err)
		}
		// The same physical-projection mismatch must refuse with or without
		// a supplied vector. Its audited digest follows its changed bytes.
		copy.CanonicalTickets = append([][]byte(nil), in.CanonicalTickets...)
		copy.CanonicalTickets[0] = sameSizeEdit(t, copy.CanonicalTickets[0])
		if len(digests) == len(copy.CanonicalTickets) {
			copy.CanonicalTicketDigests = append([]wire.Digest(nil), digests...)
			copy.CanonicalTicketDigests[0] = wire.Sum(copy.CanonicalTickets[0])
		}
		if refused := transaction.Model(req, copy); refused.Plan != nil || refused.Outcome.Outcome == "COMPLETED" {
			t.Fatalf("projection mismatch admitted: %+v", refused)
		}
	}
}

func TestTCXV0978_RetainedIndexScope(t *testing.T) {
	repo := writerStore(t, 70)
	indexAgedHere(t, repo)
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, intent.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	sum := wire.Sum(policy)
	ctx := WithBatchWrites(context.Background())
	first := writerSummaries(ctx, repo, head.QueueID.Raw, sum)
	if len(first) == 0 {
		t.Fatal("fixture has no admitted summaries")
	}
	// A later derived-file rewrite does not alter the already admitted,
	// immutable entries. Canonical bytes still have to match each digest.
	if err := os.WriteFile(intent.ClosedIndexPath(repo.StateDir), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := writerSummaries(ctx, repo, head.QueueID.Raw, sum); !reflect.DeepEqual(first, got) {
		t.Fatal("admitted immutable index not retained")
	}
	if got := writerSummaries(ctx, repo, head.QueueID.Raw, wire.Sum([]byte("other policy"))); got != nil {
		t.Fatal("index crossed policy scope")
	}
	if got := writerSummaries(ctx, repo, "queue:other", sum); got != nil {
		t.Fatal("index crossed queue scope")
	}
	if got := writerSummaries(ctx, repo, head.QueueID.Raw, sum); len(got) != len(first) {
		t.Fatal("governing scope did not recover")
	}
	other := writerStore(t, 70)
	if got := writerSummaries(ctx, other, head.QueueID.Raw, sum); got != nil {
		t.Fatal("index crossed store scope")
	}
	if got := writerSummaries(ctx, repo, head.QueueID.Raw, sum); got != nil {
		t.Fatal("returning store reused an unadmitted corrupt index")
	}
}

func TestTCXV0978_ExternalPreloadCannotForgeAuditedDigest(t *testing.T) {
	repo := writerStore(t, 70)
	indexAgedHere(t, repo)
	forged := preloadOf(t, repo)
	for i, f := range forged.Files {
		if strings.HasPrefix(f.Path, "tickets/") {
			forged.Files[i].Raw = sameSizeEdit(t, f.Raw)
			break
		}
	}
	before := mutationPublished(t, repo)
	rep, err := Mutate(WithPreloadTree(context.Background(), forged), repo, historyActor, historyCreate("forged-indexed-preload", "forged indexed preload"), WallClock())
	if err == nil && rep != nil && rep.Outcome.Outcome == "COMPLETED" {
		t.Fatalf("forged preload bypassed content hashing: %+v", rep)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("forged preload published: %s vs %s", after, before)
	}
}

// Exercise the same context across consecutive writes, including a peer
// rewriting a non-target file while preserving its length and mtime.
func TestTCXV0977_ConcurrentTamperBetweenWritesRefused(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "in-place-restored-mtime", true: "replacement-restored-mtime"}[replacement], func(t *testing.T) {
			s := newGatedStore(t)
			indexAgedHere(t, s.repo)
			ctx := WithBatchWrites(context.Background())
			if rep, err := s.submit("cache-submit")(ctx); err != nil || rep.Outcome.Outcome != "COMPLETED" {
				t.Fatalf("submit: %+v %v", rep, err)
			}
			_, target := mutationBoundaryFiles(t, s.repo)
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			// The peer writes after the first transaction released its lock.
			done := make(chan error, 1)
			edited := sameSizeEdit(t, raw)
			go func() {
				path := target
				if replacement {
					path += ".peer"
				}
				err := os.WriteFile(path, edited, info.Mode())
				if err == nil {
					err = os.Chtimes(path, info.ModTime(), info.ModTime())
				}
				if err == nil && replacement {
					err = os.Rename(path, target)
				}
				done <- err
			}()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			before := mutationPublished(t, s.repo)
			rep, err := s.gateRun("cache-gate-tampered")(ctx)
			if err == nil && rep != nil && rep.Outcome.Outcome == "COMPLETED" {
				t.Fatalf("tampered write admitted: %+v", rep)
			}
			if after := mutationPublished(t, s.repo); after != before {
				t.Fatalf("refused write published: %s vs %s", after, before)
			}
		})
	}
}

func TestTCXV0977_PreEffectTamperRefused(t *testing.T) {
	s := newGatedStore(t)
	ctx := WithBatchWrites(context.Background())
	if _, err := captureWriteTree(ctx, s.repo); err != nil {
		t.Fatal(err)
	}
	_, target := mutationBoundaryFiles(t, s.repo)
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	before := mutationPublished(t, s.repo)
	edited := false
	ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
		if !edited && strings.HasPrefix(stage, "fast.lease.model=") {
			edited = true
			if err := os.WriteFile(target, sameSizeEdit(t, raw), info.Mode()); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		}
	})
	rep, err := s.submit("cache-during-model")(ctx)
	if !edited {
		t.Fatal("did not reach the model boundary")
	}
	if err == nil && rep != nil && rep.Outcome.Outcome == "COMPLETED" {
		t.Fatalf("pre-effect tamper admitted: %+v", rep)
	}
	if after := mutationPublished(t, s.repo); after != before {
		t.Fatalf("published %s vs %s", after, before)
	}
}

func TestTCXV0977_GatePreReadRebindsCapturedBytes(t *testing.T) {
	s := newGatedStore(t)
	ctx := WithBatchWrites(context.Background())
	if rep, err := s.submit("rebind-pre-read-submit")(ctx); err != nil || rep.Outcome.Outcome != "COMPLETED" {
		t.Fatalf("submit: %+v %v", rep, err)
	}
	got, err := checkpointedLeaseProof(ctx, s.repo)
	if err != nil {
		t.Fatal(err)
	}
	want, err := checkpointedLeaseProof(context.Background(), s.repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity != want.Identity || got.ProjectionAgreement != want.ProjectionAgreement || !reflect.DeepEqual(got.Records, want.Records) {
		t.Fatal("retained pre-read differs from uncached read")
	}
	_, target := mutationBoundaryFiles(t, s.repo)
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	before := mutationPublished(t, s.repo)
	edited := false
	ctx = context.WithValue(ctx, mutationStageKey{}, func(stage string) {
		if stage == "batch.pre-read.audit" && !edited {
			edited = true
			if err := os.WriteFile(target, sameSizeEdit(t, raw), info.Mode()); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(target, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		}
	})
	if _, err := checkpointedLeaseProof(ctx, s.repo); err == nil {
		t.Fatal("pre-read accepted tamper between audit and content rebind")
	}
	if !edited {
		t.Fatal("pre-read rebind not exercised")
	}
	if after := mutationPublished(t, s.repo); after != before {
		t.Fatalf("read published: %s vs %s", after, before)
	}
}

func TestTCXV0977_CaptureLayoutChanges(t *testing.T) {
	repo := writerStore(t, 70)
	ctx := WithBatchWrites(context.Background())
	first, err := captureWriteTree(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Join(repo.IntentRoot(), intent.Dir), "tickets", "unexpected.txt")
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureWriteTree(ctx, repo); err == nil {
		t.Fatal("unexpected entry admitted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got, err := captureWriteTree(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sha256 != first.Sha256 {
		t.Fatal("capture did not recover after refusal")
	}
	for _, f := range first.Files {
		if strings.HasPrefix(f.Path, "tickets/") {
			if err := os.Remove(filepath.Join(filepath.Join(repo.IntentRoot(), intent.Dir), f.Path)); err != nil {
				t.Fatal(err)
			}
			got, err := captureWriteTree(ctx, repo)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := intent.TreeDigest(repo.IntentRoot())
			if err != nil {
				t.Fatal(err)
			}
			if got.Sha256 != plain.Sha256 || got.Sha256 == first.Sha256 {
				t.Fatal("deletion not observed")
			}
			break
		}
	}
}

// The complete and cached routes must retain identical bytes through all
// gated steps, including capacity and indexed projection checks.
func TestTCXV0978_AuditedDigestAndBatchInventoryParity(t *testing.T) {
	s := newGatedStore(t)
	parent := filepath.Dir(s.repo.PrimaryWorktree)
	backup := filepath.Join(t.TempDir(), "backup")
	copyTree(t, parent, backup)
	run := func(ctx context.Context) map[string]string {
		fixed, err := time.Parse(time.RFC3339, string(s.now))
		if err != nil {
			t.Fatal(err)
		}
		gate := func(ctx context.Context) (*Report, error) {
			choice := LeaseChoice{QueueID: fixture.QueueID, RequestID: "cache-parity-gate", Root: s.root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: s.claim.AttemptID, Generation: s.claim.Generation, Gate: "verify"}}
			return GateRun(ctx, s.repo, historyActor, choice, s.root, func() time.Time { return fixed })
		}
		for _, write := range []func(context.Context) (*Report, error){s.submit("cache-parity-submit"), gate, s.complete("cache-parity-complete")} {
			rep, err := write(ctx)
			if err != nil || rep.Outcome.Outcome != "COMPLETED" {
				t.Fatalf("write: %+v %v", rep, err)
			}
		}
		return storeFiles(t, parent, journal.WriterCheckpointPath(s.repo.StateDir))
	}
	if err := intent.WriteClosedIndex(s.repo.IntentRoot(), s.repo.StateDir); err != nil {
		t.Fatal(err)
	}
	want := run(context.Background())
	restoreTree(t, parent, backup)
	if err := intent.WriteClosedIndex(s.repo.IntentRoot(), s.repo.StateDir); err != nil {
		t.Fatal(err)
	}
	got := run(WithBatchWrites(context.Background()))
	// Gate execution samples wall time: compare stable intent and lifecycle
	// outputs separately from the gate's sampled timestamps and evidence digest.
	for p, d := range want {
		if strings.Contains(p, "intent/tickets/") && got[p] != d {
			t.Fatalf("ticket differs: %s", p)
		}
	}
}

func TestTCXV0979_IncrementalInventoryMatchesRebuild(t *testing.T) {
	repo := writerStore(t, 70)
	ctx := WithBatchWrites(context.Background())
	for step := 0; step < 4; step++ {
		if rep, err := writerCreate(repo, string(rune('a'+step))+"-inventory", WallClock())(ctx); err != nil || rep.Outcome.Outcome != "COMPLETED" {
			t.Fatalf("create: %+v %v", rep, err)
		}
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := captureWriteTree(ctx, repo)
		if err != nil {
			t.Fatal(err)
		}
		w, decline, terminal := observeWriter(ctx, repo, head, "inventory-probe", false, false, nil, nil, &tree)
		if decline != nil || terminal != nil {
			t.Fatalf("observation: %v %v", decline, terminal)
		}
		full, err := writerInventory(context.Background(), repo, w.proof, w.phys, w.wc, &tree)
		if err != nil {
			t.Fatal(err)
		}
		in := transaction.Input{Inventory: w.inv, Head: w.head, HeadReceipt: w.headReceipt, Queue: w.proof.Records["intent/queue.json"].Raw, Policy: w.proof.Records["intent/policy.json"].Raw, Barrier: w.barrier, Reservations: w.reservations, Premise: transaction.LocalOperator, Branch: w.branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: WallClock()}
		for path, record := range w.proof.Records {
			if strings.HasPrefix(path, "intent/tickets/") {
				in.CanonicalTickets = append(in.CanonicalTickets, record.Raw)
			}
		}
		req := transaction.Request{Operation: transaction.Pause, QueueID: head.QueueID.Raw, RequestID: "inventory-pause", Actor: historyActor}
		cachedPlan := transaction.Model(req, in)
		in.Inventory = full
		freshPlan := transaction.Model(req, in)
		if cachedPlan.Plan == nil || freshPlan.Plan == nil || string(cachedPlan.Plan.Descriptor()) != string(freshPlan.Plan.Descriptor()) {
			t.Fatalf("inventory plan differs: %+v %+v", cachedPlan, freshPlan)
		}
		cachedCost, err := transaction.CheckCapacity(cachedPlan.Plan)
		if err != nil {
			t.Fatal(err)
		}
		freshCost, err := transaction.CheckCapacity(freshPlan.Plan)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cachedCost.Prefixes, freshCost.Prefixes) {
			t.Fatal("incremental capacity prefixes differ")
		}
		if !reflect.DeepEqual(w.inv.FilesUnder(""), full.FilesUnder("")) || !reflect.DeepEqual(w.inv.Directories(), full.Directories()) {
			t.Fatal("incremental metadata differs")
		}
	}
	// Compare cache updates, removals and malformed inputs to a fresh build.
	var memo transaction.SummarizedInventoryCache
	digest := wire.Sum([]byte("receipt"))
	e := transaction.Elided{Receipts: 1, FirstReceiptSha256: digest, LastReceiptSha256: digest, ReceiptBytes: 1, HasRequest: func(string) bool { return false }, Cost: archive.FileSetCost{Files: 1, PayloadBytes: 1}}
	dirs := []string{"receipts"}
	file := archive.FileEntry{Path: "intent/tickets/T.json", Sha256: digest, Bytes: wire.SizeOf(1)}
	for _, files := range [][]archive.FileEntry{
		{file}, nil, {file}, {file, file},
		{{Path: file.Path, Sha256: wire.Digest("bad"), Bytes: file.Bytes}},
		{{Path: "unknown", Sha256: digest, Bytes: file.Bytes}},
		{{Path: "requests/00/" + strings.Repeat("0", 64) + ".json", Sha256: digest, Bytes: file.Bytes}},
		{{Path: "receipts/000000000001.json", Sha256: digest, Bytes: file.Bytes}},
		{{Path: file.Path, Sha256: digest, Bytes: wire.SizeOf(wire.MaxIntentTreeBytes + 1)}}, {file},
	} {
		got, err := memo.Build("prefix", files, dirs, e)
		want, werr := transaction.NewSummarizedInventory(files, dirs, e)
		if (err == nil) != (werr == nil) {
			t.Fatalf("cache=%v rebuild=%v", err, werr)
		}
		if err == nil && !reflect.DeepEqual(got.FilesUnder("intent/"), want.FilesUnder("intent/")) {
			t.Fatal("update differs")
		}
	}
}

// Compare the audit identity to the original fingerprint formula, independent
// of the optimized formatter and with both regular-file and directory modes.
func TestTCXV0977_InventoryFingerprintEncodingParity(t *testing.T) {
	repo := writerStore(t, 70)
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := journalReader(repo, head).Audit()
	if err != nil || proof.IntentError != nil {
		t.Fatalf("audit: %v %+v", err, proof)
	}
	native := journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()}
	files := map[string]os.FileInfo{}
	var list func(string)
	list = func(dir string) {
		listing, err := native.List(dir, wire.MaxArchiveScanEntries)
		if err != nil {
			t.Fatal(err)
		}
		files[dir] = listing.DirectoryInfo
		for _, entry := range listing.Entries {
			path := entry.Name
			if dir != "." {
				path = dir + "/" + path
			}
			files[path] = entry.Info
			if entry.Info.IsDir() && path != "worktrees" {
				list(path)
			}
		}
	}
	list(".")
	list("intent")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		info := files[path]
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00%d\n", path, info.Mode(), info.Size(), info.ModTime().UnixNano())
	}
	if got := wire.Digest(hex.EncodeToString(h.Sum(nil))); got != proof.Identity.InventorySha256 {
		t.Fatalf("fingerprint changed: %s vs %s", got, proof.Identity.InventorySha256)
	}
}
