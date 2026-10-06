package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0645 / CAL-V0-070: writer cost as a function of retained receipt history.
// The fixture is a synthetic O(N) journal, not live API history: historyTickets
// tickets are created through Mutate, then every further receipt posts one
// request afterimage and one inline afterimage of a rotating ticket whose body
// is padded so the mean receipt matches the observed ~6 KB. It has no evidence
// blobs, attempts or reservations. A full audit must agree before any timing.
const (
	historyTickets  = 50
	historyBodySize = 4096
)

var historyActor = mutation.Binding{ID: "tester", Role: "OWNER"}

func historyObject(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func historyWrite(tb testing.TB, path string, raw []byte) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		tb.Fatal(err)
	}
}

func historyCreate(requestID, title string) []byte {
	s := wire.String
	payload := historyObject(
		"acceptanceCriteria", wire.Strings([]string{"it exists"}),
		"body", wire.Null(),
		"capabilities", wire.Strings(nil),
		"dependencies", wire.Array(),
		"dueDate", wire.Null(),
		"effects", historyObject("coverage", s("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(nil)),
		"estimateMinutes", wire.Null(),
		"executionClass", s("AUTONOMOUS"),
		"kind", s("FEATURE"),
		"labels", wire.Strings(nil),
		"milestone", wire.Null(),
		"order", s("0"),
		"owner", wire.Null(),
		"priority", s("P2"),
		"requiredGates", wire.Strings(nil),
		"requirementRefs", wire.Strings(nil),
		"source", historyObject("kind", s("NATIVE"), "sourceItemId", wire.Null(), "sourceQueueId", s(fixture.QueueID), "sourceRevisionSha256", wire.Null()),
		"supersededBy", wire.Null(),
		"supersedes", wire.Null(),
		"title", s(title),
	)
	return wire.EncodeFile(historyObject(
		"profile", s(mutation.Profile),
		"requestId", s(requestID),
		"actor", historyObject("id", s(historyActor.ID), "role", s(historyActor.Role)),
		"queueId", s(fixture.QueueID),
		"targetId", wire.Null(),
		"expectedRevision", wire.Null(),
		"operation", s("CREATE"),
		"payload", payload,
		"issuedAt", s("2026-09-07T12:00:00Z"),
	))
}

func historyMutate(tb testing.TB, repo *intent.Repository, requestID string) {
	tb.Helper()
	rep, err := Mutate(context.Background(), repo, historyActor, historyCreate(requestID, "history "+requestID), WallClock())
	if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		tb.Fatalf("mutate %s: %+v %v", requestID, rep, err)
	}
}

// historyAppend links one synthetic receipt, projects its posts and advances
// head. It reads only head and the touched projections, so setup is O(N).
func historyAppend(tb testing.TB, repo *intent.Repository, posts map[string][]byte, request string) {
	tb.Helper()
	headPath := filepath.Join(repo.StateDir, "head.json")
	raw, err := os.ReadFile(headPath)
	if err != nil {
		tb.Fatal(err)
	}
	h, err := snapshot.DecodeHead(raw)
	if err != nil {
		tb.Fatal(err)
	}
	seq := h.LastSeq.Uint64() + 1
	v := fixture.ReceiptValue(seq, h.LastReceiptSha256, "MUTATION", h.Generation.Uint64())
	v.Obj.Set("recordedAt", wire.String(string(WallClock())))
	v.Obj.Set("requestId", wire.String(request))
	paths := make([]string, 0, len(posts))
	for p := range posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var pre, post []wire.Value
	for _, p := range paths {
		dest := filepath.Join(repo.StateDir, p)
		if strings.HasPrefix(p, "intent/") {
			dest = filepath.Join(repo.PrimaryWorktree, intent.Dir, strings.TrimPrefix(p, "intent/"))
		}
		prior := wire.Null()
		if old, err := os.ReadFile(dest); err == nil {
			prior = wire.String(string(wire.Sum(old)))
		} else if !os.IsNotExist(err) {
			tb.Fatal(err)
		}
		record, err := wire.Parse(posts[p])
		if err != nil {
			tb.Fatal(err)
		}
		pre = append(pre, historyObject("path", wire.String(p), "sha256", prior))
		post = append(post, historyObject("path", wire.String(p), "sha256", wire.String(string(wire.Sum(posts[p]))), "record", record, "blobSha256", wire.Null()))
		historyWrite(tb, dest, posts[p])
	}
	v.Obj.Set("pre", wire.Array(pre...))
	v.Obj.Set("post", wire.Array(post...))
	receipt := wire.EncodeFile(v)
	name, _ := snapshot.ReceiptName(seq)
	historyWrite(tb, filepath.Join(repo.StateDir, "receipts", name), receipt)
	historyWrite(tb, headPath, wire.EncodeFile(fixture.HeadValue(repo.PrimaryWorktree, seq, wire.Sum(receipt), h.Generation.Uint64(), h.InitSha256)))
}

func historyRequest(id string, seq uint64) []byte {
	s := wire.SizeOf(seq)
	out := mutation.Outcome{RequestID: id, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &s, Codes: []string{}}
	return wire.EncodeFile(historyObject("requestId", wire.String(id), "seq", wire.String(string(s)), "mutationSha256", wire.String(string(wire.Sum([]byte(id)))), "outcome", out.Value()))
}

// historyStore returns a settled store whose head is exactly receipts, verified
// CONSISTENT/AGREES by one complete audit. It never touches a live journal.
func historyStore(tb testing.TB, receipts int) *intent.Repository {
	tb.Helper()
	base, err := os.MkdirTemp("", "atm-history-")
	if err != nil {
		tb.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.RemoveAll(real) })
	return historyStoreAt(tb, filepath.Join(real, "repo"), historyTickets, receipts)
}

// historyStoreAt builds the historyStore fixture at root with the given number
// of tickets created through Mutate, then pads it to receipts.
func historyStoreAt(tb testing.TB, root string, tickets, receipts int) *intent.Repository {
	tb.Helper()
	historyWrite(tb, filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	historyWrite(tb, filepath.Join(root, intent.Dir, "queue.json"), fixture.QueueBytes())
	historyWrite(tb, filepath.Join(root, intent.Dir, "policy.json"), fixture.PolicyBytes())
	repo, err := intent.Resolve(root)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err = Init(context.Background(), repo, historyActor, "history-init", WallClock()); err != nil {
		tb.Fatal(err)
	}
	created := min(tickets, historyTickets)
	for i := 0; i < created; i++ {
		historyMutate(tb, repo, fmt.Sprintf("history-create-%d", i))
	}
	if created < tickets {
		historyAppendCreates(tb, repo, created, tickets)
	}
	entries, err := os.ReadDir(filepath.Join(root, intent.Dir, "tickets"))
	if err != nil || len(entries) != tickets {
		tb.Fatalf("tickets %d %v", len(entries), err)
	}
	templates := make([]wire.Value, len(entries))
	for i, e := range entries {
		raw, err := os.ReadFile(filepath.Join(root, intent.Dir, "tickets", e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		if templates[i], err = wire.Parse(raw); err != nil {
			tb.Fatal(err)
		}
	}
	pad := strings.Repeat("history padding ", historyBodySize/16)
	for seq := uint64(tickets + 2); seq <= uint64(receipts); seq++ {
		id := fmt.Sprintf("history-%d", seq)
		reqPath, _ := snapshot.RequestPath(id)
		i := int(seq) % len(entries)
		templates[i].Obj.Set("body", wire.String(fmt.Sprintf("receipt %d %s", seq, pad)))
		historyAppend(tb, repo, map[string][]byte{
			reqPath:                               historyRequest(id, seq),
			"intent/tickets/" + entries[i].Name(): wire.EncodeFile(templates[i]),
		}, id)
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		tb.Fatal(err)
	}
	if head.LastSeq.Uint64() != uint64(receipts) {
		tb.Fatalf("head %s, want %d", head.LastSeq, receipts)
	}
	proof, err := journalReader(repo, head).Audit()
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
		tb.Fatalf("fixture audit: %+v %v", proof, err)
	}
	return repo
}

// historyAppendCreates appends one synthetic CREATE-shaped receipt per ticket
// after the first created, cloning a Mutate-built record under the queue's
// next serial and advancing queue.json with it, so setup stays O(N).
func historyAppendCreates(tb testing.TB, repo *intent.Repository, created, tickets int) {
	tb.Helper()
	dir := filepath.Join(repo.PrimaryWorktree, intent.Dir)
	entries, err := os.ReadDir(filepath.Join(dir, "tickets"))
	if err != nil || len(entries) == 0 {
		tb.Fatalf("templates %d %v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "tickets", entries[0].Name()))
	if err != nil {
		tb.Fatal(err)
	}
	template, err := wire.Parse(raw)
	if err != nil {
		tb.Fatal(err)
	}
	for i := created; i < tickets; i++ {
		qraw, err := os.ReadFile(filepath.Join(dir, "queue.json"))
		if err != nil {
			tb.Fatal(err)
		}
		queue, err := wire.Parse(qraw)
		if err != nil {
			tb.Fatal(err)
		}
		next, _ := queue.Obj.Get("nextSerial")
		prefix, _ := queue.Obj.Get("prefix")
		serial, err := strconv.Atoi(next.Str)
		if err != nil {
			tb.Fatal(err)
		}
		local := fmt.Sprintf("%s-%04d", prefix.Str, serial)
		queue.Obj.Set("nextSerial", wire.String(strconv.Itoa(serial+1)))
		template.Obj.Set("ticketId", wire.String(strings.TrimSuffix(fixture.TicketID("X"), "X")+local))
		template.Obj.Set("title", wire.String(fmt.Sprintf("history history-create-%d", i)))
		head, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
		if err != nil {
			tb.Fatal(err)
		}
		h, err := snapshot.DecodeHead(head)
		if err != nil {
			tb.Fatal(err)
		}
		id := fmt.Sprintf("history-create-%d", i)
		reqPath, _ := snapshot.RequestPath(id)
		historyAppend(tb, repo, map[string][]byte{
			reqPath:                             historyRequest(id, h.LastSeq.Uint64()+1),
			"intent/queue.json":                 wire.EncodeFile(queue),
			"intent/tickets/" + local + ".json": wire.EncodeFile(template),
		}, id)
	}
}

// BenchmarkCALV0070_MutateAt2000Receipts is the maintained V1-0645 baseline:
// one complete store.Mutate CREATE on a settled 2,000-receipt history. Each
// iteration adds one receipt, so run it with a small fixed -benchtime (5x).
// cpu-ms/op is the process's CPU time per mutation, for comparing runs made
// under different host load.
func BenchmarkCALV0070_MutateAt2000Receipts(b *testing.B) {
	repo := historyStore(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	cpu, cpuOK := processCPU()
	for i := 0; i < b.N; i++ {
		historyMutate(b, repo, fmt.Sprintf("history-bench-%d", i))
	}
	if end, ok := processCPU(); cpuOK && ok {
		b.ReportMetric(float64(end-cpu)/float64(time.Millisecond)/float64(b.N), "cpu-ms/op")
	}
}

// TestCALV0070_WriterHistoryProfile is an opt-in phase profile, not a timing
// assertion. Each primitive is timed alone on a settled store; Mutate is timed
// end to end. Medians only; host load is captured separately.
func TestCALV0070_WriterHistoryProfile(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_HISTORY_PROFILE")
	if output == "" {
		t.Skip("set CORVINT_TASKS_HISTORY_PROFILE to a report path")
	}
	sizes := []int{500, 2000, 7000}
	if raw := os.Getenv("CORVINT_TASKS_HISTORY_SIZES"); raw != "" {
		sizes = nil
		for _, f := range strings.Split(raw, ",") {
			n, err := strconv.Atoi(f)
			if err != nil || n <= historyTickets+1 {
				t.Fatalf("size %q", f)
			}
			sizes = append(sizes, n)
		}
	}
	const reps = 3
	type row struct {
		Receipts int
		SetupMS  float64
		Phases   map[string]float64
		// CPUPhases is the process's user plus system CPU time per phase.
		CPUPhases map[string]float64
	}
	var rows []row
	for _, n := range sizes {
		setup := time.Now()
		repo := historyStore(t, n)
		r := row{Receipts: n, SetupMS: ms(time.Since(setup)), Phases: map[string]float64{}, CPUPhases: map[string]float64{}}
		samples, cpuSamples := map[string][]float64{}, map[string][]float64{}
		timed := func(name string, f func() error) {
			cpu, cpuOK := processCPU()
			start := time.Now()
			if err := f(); err != nil {
				t.Fatalf("%d %s: %v", n, name, err)
			}
			samples[name] = append(samples[name], ms(time.Since(start)))
			if end, ok := processCPU(); cpuOK && ok {
				cpuSamples[name] = append(cpuSamples[name], ms(end-cpu))
			}
		}
		for rep := 0; rep < reps; rep++ {
			head, err := writerGuards(repo, "MUTATE")
			if err != nil {
				t.Fatal(err)
			}
			reader := journalReader(repo, head)
			timed("mutate.lookupAudit", func() error {
				index := journal.RequestIndex{Reader: reader}
				_, found, err := index.Lookup(fmt.Sprintf("history-absent-%d", rep))
				if found {
					return fmt.Errorf("absent request found")
				}
				return err
			})
			paths := []string{"intent/queue.json", "intent/policy.json"}
			timed("mutate.inventory", func() error {
				inv, err := inventory(repo)
				if err == nil {
					for _, f := range inv.Files() {
						if strings.HasPrefix(f.Path, "intent/tickets/") {
							paths = append(paths, f.Path)
						}
					}
				}
				return err
			})
			var proof *journal.Result
			timed("mutate.fullAudit", func() error {
				proof, err = reader.Audit(paths...)
				return err
			})
			// After CAL-V0-070 Mutate runs the merged audit and a fresh
			// inventory checked against its reads instead of the three above.
			var merged *journal.MutationAudit
			timed("mutate.mergedAudit", func() error {
				merged, err = reader.AuditForMutation(fmt.Sprintf("history-absent-%d", rep))
				if err == nil && (merged.Found || merged.Physical.Files == nil) {
					err = fmt.Errorf("merged audit found %v, published %d", merged.Found, len(merged.Physical.Files))
				}
				return err
			})
			timed("mutate.checkedInventory", func() error {
				inv, err := inventory(repo)
				if err == nil && !readUnchanged(inv, merged.Physical.Files) {
					err = fmt.Errorf("inventory differs from the merged audit's reads")
				}
				return err
			})
			timed("mutate.treeDigest", func() error {
				_, err := intent.TreeDigest(repo.PrimaryWorktree)
				return err
			})
			timed("lease.watchChanges", func() error {
				g, err := authority.WatchChanges(repo)
				if err != nil {
					return err
				}
				return g.Close()
			})
			var observed journal.PhysicalObservation
			timed("lease.auditForWriteObserved", func() error {
				_, observed, err = reader.AuditForWriteObserved()
				return err
			})
			timed("lease.guardedInventory", func() error {
				_, _, err := scanWithReader(repo, intent.ReadFile, observed.Files)
				return err
			})
			cp := proof.Checkpoint()
			if cp == nil {
				t.Fatal("complete settled audit derived no checkpoint")
			}
			timed("read.checkpointPlusTail", func() error {
				resumed := reader
				resumed.Checkpoint = cp
				res, err := resumed.Audit()
				if err == nil && res.Mode != journal.ModeCheckpoint {
					err = fmt.Errorf("mode %s", res.Mode)
				}
				return err
			})
			timed("mutate.total", func() error {
				historyMutate(t, repo, fmt.Sprintf("history-profile-%d-%d", n, rep))
				return nil
			})
		}
		for name, s := range samples {
			sort.Float64s(s)
			r.Phases[name] = s[len(s)/2]
		}
		for name, s := range cpuSamples {
			sort.Float64s(s)
			r.CPUPhases[name] = s[len(s)/2]
		}
		rows = append(rows, r)
		t.Logf("receipts=%d setup=%.0fms %v cpu %v", n, r.SetupMS, r.Phases, r.CPUPhases)
	}
	report := map[string]any{"ticket": "V1-0645", "requirement": "CAL-V0-070", "fixture": fmt.Sprintf("synthetic: %d created tickets, one request plus one ~%d-byte inline ticket afterimage per receipt; no evidence blobs, attempts or reservations", historyTickets, historyBodySize), "repetitions": reps, "statistic": "median milliseconds: Phases wall time, CPUPhases process user plus system CPU time", "cpuCount": runtime.NumCPU(), "goVersion": runtime.Version(), "rows": rows, "hostLoad": "NOT_OBSERVED: capture host load externally alongside this run"}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
