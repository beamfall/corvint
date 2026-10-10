package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The doctor (TQD-V0-001..012) is an advisory pure read: it names stuck
// queue patterns from bounded journal, pool and attempt data, adds bounded
// project plugin findings, and writes only its own summary cache on an
// explicit --refresh. Its findings carry no authority.
const (
	doctorProfile          = "taskman-doctor/0"
	doctorCacheProfile     = "taskman-doctor-cache/0"
	doctorWindow           = 7 * 24 * time.Hour
	doctorMaxReceipts      = 4096
	doctorThreshold        = 3
	doctorMaxFindings      = 512
	doctorCacheBytes       = 1 << 20
	doctorLineStale        = 15 * time.Minute
	doctorMaxPlugins       = 32
	doctorMaxPluginEntries = 4096
	doctorPluginOutput     = 64 << 10
	doctorPluginItems      = 64
	doctorPluginProse      = 1024
	doctorPsTimeout        = 5 * time.Second
	doctorPsBytes          = 4 << 20
	doctorPsProcs          = 4096
	doctorCacheDirName     = "taskman-doctor"
	doctorCacheFileName    = "summary.json"
	doctorCacheLockName    = ".lock"
	doctorLineUnavailable  = "doctor cache unavailable; run corvint-tasks doctor --refresh\n"
)

var (
	doctorClock         = time.Now
	doctorPluginTimeout = 10 * time.Second
	doctorKindPattern   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	// doctorRemedies is the fixed operator guidance per built-in kind.
	doctorRemedies = map[string]string{
		"NO_PROGRESS_HANDOFF": "Read the hand-off evidence, then refine, split or hold the ticket instead of claiming it again.",
		"REPEAT_REFUSAL":      "Compare the review returns and failed gates for this tree; change the candidate or the criteria before resubmitting.",
		"SLOW_LANE_RECOVERY":  "Inspect the member's cleanup output and the external resource; repair the cleanup, then confirm safety.",
		"SETUP_ONLY_PROOF":    "Witness the core obligations or reopen the ticket; its completion proved only setup.",
		"FALSE_IDLE":          "The session's process tree is busy while its heartbeat is stale; inspect the run before reaping or relaunching.",
		"PLUGIN_FAILED":       "Fix or remove the project plugin named in `who`.",
	}
)

type doctorFinding struct {
	kind, source, who, detail, remedy string
	firstSeen                         time.Time
	seqs                              []uint64
	// carried findings take firstSeen from the previous cache.
	carried bool
}

func (f doctorFinding) key() string { return f.kind + "\x00" + f.source + "\x00" + f.who }

// doctorEvent is one scanned receipt's verified attempt and pool afterimages.
type doctorEvent struct {
	seq      uint64
	at       time.Time
	attempts []*snapshot.Attempt
	// fresh[i] is true when attempts[i] was created by this receipt (its
	// pre entry records no prior content).
	fresh []bool
	pools *snapshot.PoolState
}

// doctorIdle is a live attempt whose holder is stale or lease-expired.
type doctorIdle struct {
	a      *snapshot.Attempt
	status string
}

type doctorScan struct {
	receipts        int
	fromSeq, head   uint64
	truncated       bool
	warnings        []string
	events          []doctorEvent // chronological
	findingsCut     bool
	findingsTotal   int
	observedAt      time.Time
	idleCandidates  []doctorIdle
	lanesFree       int
	lanesTotal      int
	runningSessions int
	completions24h  string
}

func doctorUsage() error {
	return wire.Errorf(wire.CodeMalformed, "argv", "doctor takes [--refresh] [--plugins DIR], or --line alone")
}

func doctorCommand(env Env, args []string) *wire.Result {
	cmd := []string{"doctor"}
	refresh, plugins, pluginsSet := false, "", false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--refresh" && !refresh:
			refresh = true
		case a == "--plugins" && !pluginsSet && i+1 < len(args) && args[i+1] != "" && !strings.HasPrefix(args[i+1], "--"):
			plugins, pluginsSet = args[i+1], true
			i++
		default:
			return failure(cmd, nil, doctorUsage())
		}
	}
	if pluginsSet {
		given := plugins
		if !filepath.IsAbs(plugins) {
			plugins = filepath.Join(env.Cwd, plugins)
		}
		if st, err := os.Stat(plugins); err != nil || !st.IsDir() {
			return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "--plugins", "plugin directory %q is not a directory", given))
		}
	}
	scan := &doctorScan{}
	var findings []doctorFinding
	// The inventory read admits an absent journal so that a store with valid
	// tracked intent and no local journal refuses MISSING_EVIDENCE rather
	// than UNINITIALIZED (TQD-V0-001); a present journal is a TM-V0-008 read.
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		*scan = doctorScan{}
		var e error
		findings, e = doctorStoreFindings(rc, scan)
		return e
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	findings = append(findings, doctorFalseIdle(rc.repo, scan)...)
	if pluginsSet {
		findings = append(findings, doctorPlugins(plugins, rc.repo.PrimaryWorktree)...)
	}
	cacheDir := filepath.Join(rc.repo.CommonDir, doctorCacheDirName)
	carry, cerr := doctorCarry(filepath.Join(cacheDir, doctorCacheFileName))
	item := doctorItem(rc, scan, findings, carry, doctorProfile, 0)
	if refresh {
		// A cache written by a later build is never replaced (TQD-V0-011).
		if wire.CodeOf(cerr) == wire.CodeUnsupportedVersion {
			return failure(cmd, rc, cerr)
		}
		cache := doctorItem(rc, scan, findings, carry, doctorCacheProfile, doctorCacheBytes)
		if e := writeDoctorCache(rc.repo.CommonDir, wire.EncodeFile(cache)); e != nil {
			return failure(cmd, rc, e)
		}
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	res.Untrusted = pluginsSet
	res.Warnings = append(res.Warnings, scan.warnings...)
	return res
}

// doctorStoreFindings reads everything the doctor needs from the store
// inside one TM-V0-008 read; process and plugin probes run after it.
func doctorStoreFindings(rc *readCtx, scan *doctorScan) ([]doctorFinding, error) {
	if rc.journalAbsent {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "journal", "no journal: the doctor reads journal history")
	}
	scan.observedAt = doctorClock().UTC().Truncate(time.Second)
	in, _, err := planInput(rc)
	if err != nil {
		return nil, err
	}
	occupied := map[string]bool{}
	if in.Pools != nil {
		for _, en := range in.Pools.Entries {
			occupied[en.PoolID+"/"+en.MemberID] = true
		}
	}
	for _, p := range rc.store.Policy.Pools {
		for _, m := range p.Members {
			scan.lanesTotal++
			if !occupied[p.ID+"/"+m] {
				scan.lanesFree++
			}
		}
	}
	ttl := rc.store.Policy.HeartbeatTTLSeconds()
	ids := make([]string, 0, len(in.Attempts))
	for id := range in.Attempts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		a := in.Attempts[id]
		if !a.Live() {
			continue
		}
		scan.runningSessions++
		if s := holderStatus(a, scan.observedAt, ttl); s == "STALE_HOLDER" || s == "LEASE_EXPIRED" {
			scan.idleCandidates = append(scan.idleCandidates, doctorIdle{a: a, status: s})
		}
	}
	_, counts := completionSummary(rc, scan.observedAt)
	scan.completions24h = "0"
	if v, ok := counts.Obj.Get("last24Hours"); ok {
		scan.completions24h = v.Str
	}
	src := journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}
	doctorScanReceipts(src, rc.snap.Head.LastSeq.Uint64(), scan)
	var out []doctorFinding
	out = append(out, doctorNoProgress(scan)...)
	out = append(out, doctorRepeatRefusal(doctorGateReader(src), scan)...)
	out = append(out, doctorSlowLane(scan)...)
	out = append(out, doctorSetupOnly(rc, scan)...)
	return out, nil
}

// doctorScanReceipts walks receipts backward from the head within the
// receipt and time bounds (TQD-V0-004). Any unreadable or unverifiable
// receipt stops the walk and marks it truncated.
func doctorScanReceipts(src journal.Native, head uint64, scan *doctorScan) {
	scan.head = head
	cutoff := scan.observedAt.Add(-doctorWindow)
	var events []doctorEvent
	seq := head
	for ; seq >= 1; seq-- {
		if scan.receipts >= doctorMaxReceipts {
			scan.truncated = true
			break
		}
		ev, recorded, err := doctorReadReceipt(src, seq)
		if err != nil {
			scan.truncated = true
			scan.warnings = append(scan.warnings, prose(fmt.Sprintf("doctor scan stopped at receipt %d: %v", seq, err)))
			break
		}
		if recorded.Before(cutoff) {
			break
		}
		scan.receipts++
		scan.fromSeq = seq
		events = append(events, ev)
	}
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	scan.events = events
}

func doctorReadReceipt(src journal.Native, seq uint64) (doctorEvent, time.Time, error) {
	ev := doctorEvent{seq: seq}
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return ev, time.Time{}, err
	}
	raw, err := src.Read("receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return ev, time.Time{}, err
	}
	r, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		return ev, time.Time{}, err
	}
	at, err := time.Parse(time.RFC3339, string(r.RecordedAt))
	if err != nil {
		return ev, time.Time{}, err
	}
	ev.at = at
	created := map[string]bool{}
	for _, p := range r.Pre {
		created[p.Path] = p.Sha256 == nil
	}
	for _, p := range r.Post {
		if p.Sha256 == nil || (p.Path != "pools.json" && !strings.HasPrefix(p.Path, "attempts/")) {
			continue
		}
		b, err := doctorPostBytes(src, p)
		if err != nil {
			return ev, at, err
		}
		if p.Path == "pools.json" {
			if ev.pools, err = snapshot.DecodePools(b); err != nil {
				return ev, at, err
			}
			continue
		}
		a, err := snapshot.DecodeAttempt(b)
		if err != nil {
			return ev, at, err
		}
		ev.attempts = append(ev.attempts, a)
		ev.fresh = append(ev.fresh, created[p.Path])
	}
	return ev, at, nil
}

func doctorPostBytes(src journal.Native, p snapshot.PostEntry) ([]byte, error) {
	var b []byte
	switch {
	case p.Record != nil:
		b = wire.EncodeFile(*p.Record)
	case p.BlobSha256 != nil:
		bound, err := snapshot.PostBound(p.Path)
		if err != nil {
			return nil, err
		}
		if b, err = src.Read("evidence/"+string(*p.BlobSha256), bound); err != nil {
			return nil, err
		}
	}
	if wire.Sum(b) != *p.Sha256 {
		return nil, fmt.Errorf("%s afterimage digest mismatch", p.Path)
	}
	return b, nil
}

// doctorEnded is one generation's first terminal afterimage in the scan.
type doctorEnded struct {
	a   *snapshot.Attempt
	seq uint64
	at  time.Time
}

// doctorEndings returns each ticket's ended generations in journal order.
func doctorEndings(scan *doctorScan) (map[string][]doctorEnded, []string) {
	seen := map[string]bool{}
	out := map[string][]doctorEnded{}
	var order []string
	for _, ev := range scan.events {
		for _, a := range ev.attempts {
			key := a.AttemptID + "/" + string(a.Generation)
			if a.Live() || a.RetryAccounting == nil || seen[key] {
				continue
			}
			seen[key] = true
			t := a.TicketID.Raw
			if _, ok := out[t]; !ok {
				order = append(order, t)
			}
			out[t] = append(out[t], doctorEnded{a: a, seq: ev.seq, at: ev.at})
		}
	}
	return out, order
}

// doctorNoProgress applies the CAL-V0-102 no-progress test to each ticket's
// newest ended generations (TQD-V0-005).
func doctorNoProgress(scan *doctorScan) []doctorFinding {
	endings, order := doctorEndings(scan)
	var out []doctorFinding
	for _, t := range order {
		gens := endings[t]
		verdicts := make([]bool, len(gens))
		var tree *string
		known := true
		for i, g := range gens {
			a := g.a
			comparable := a.CandidateTreeOid == nil || (known && tree != nil && *tree == *a.CandidateTreeOid)
			if a.CandidateTreeOid != nil {
				tree, known = a.CandidateTreeOid, true
			}
			verdicts[i] = a.RetryAccounting.Disposition == wire.CodeHandoff && len(a.GateResults) == 0 && len(a.Reviews) == 0 && comparable
		}
		start := len(gens)
		for start > 0 && verdicts[start-1] {
			start--
		}
		run := gens[start:]
		if len(run) < doctorThreshold {
			continue
		}
		f := doctorFinding{kind: "NO_PROGRESS_HANDOFF", who: t, firstSeen: run[0].at,
			detail: fmt.Sprintf("%d consecutive hand-offs with no gate result, review or new candidate tree", len(run))}
		for _, g := range run {
			f.seqs = append(f.seqs, g.seq)
		}
		out = append(out, f)
	}
	return out
}

type doctorRefusal struct {
	seqs []uint64
	at   time.Time
}

// doctorGateReader reads one gate result from its verified evidence blob.
func doctorGateReader(src journal.Native) func(string) (*snapshot.GateResult, error) {
	return func(d string) (*snapshot.GateResult, error) {
		raw, err := src.Read("evidence/"+d, snapshot.MaxGateRecordBytes)
		if err != nil {
			return nil, err
		}
		if wire.Sum(raw) != wire.Digest(d) {
			return nil, errors.New("digest mismatch")
		}
		return snapshot.DecodeGateResult(raw)
	}
}

// doctorRepeatRefusal counts review returns and distinct failed gate results
// per ticket and candidate tree (TQD-V0-006).
func doctorRepeatRefusal(gateOf func(string) (*snapshot.GateResult, error), scan *doctorScan) []doctorFinding {
	refusals := map[[2]string]*doctorRefusal{}
	var order [][2]string
	count := func(ticketID, tree string, seq uint64, at time.Time) {
		k := [2]string{ticketID, tree}
		r := refusals[k]
		if r == nil {
			r = &doctorRefusal{at: at}
			refusals[k] = r
			order = append(order, k)
		}
		r.seqs = append(r.seqs, seq)
	}
	lastTree := map[string]string{}
	ended := map[string]bool{}
	gates := map[string]bool{}
	// prior holds each attempt's gate results at its previous scanned
	// afterimage. A result counts only at the receipt that added it: an
	// attempt the scan first meets already existing carries a baseline
	// recorded before the window, which never counts (TQD-V0-006).
	prior := map[string]map[string]bool{}
	for _, ev := range scan.events {
		for i, a := range ev.attempts {
			t := a.TicketID.Raw
			before, seen := prior[a.AttemptID]
			now := make(map[string]bool, len(a.GateResults))
			for _, d := range a.GateResults {
				now[d] = true
			}
			prior[a.AttemptID] = now
			if !seen && !ev.fresh[i] {
				before = now
			}
			for _, d := range a.GateResults {
				if before[d] || gates[d] {
					gates[d] = true
					continue
				}
				gates[d] = true
				g, err := gateOf(d)
				if err != nil {
					scan.warnings = append(scan.warnings, prose("doctor could not verify gate result "+d))
					continue
				}
				if g.State == "FAILED" {
					count(t, g.CandidateTreeOid, ev.seq, ev.at)
				}
			}
			key := a.AttemptID + "/" + string(a.Generation)
			if !a.Live() && a.RetryAccounting != nil && !ended[key] {
				ended[key] = true
				if a.RetryAccounting.Disposition == wire.CodeReviewReturned {
					tree := lastTree[t]
					if a.CandidateTreeOid != nil {
						tree = *a.CandidateTreeOid
					}
					if tree != "" {
						count(t, tree, ev.seq, ev.at)
					}
				}
			}
			if a.CandidateTreeOid != nil {
				lastTree[t] = *a.CandidateTreeOid
			}
		}
	}
	var out []doctorFinding
	for _, k := range order {
		r := refusals[k]
		if len(r.seqs) < doctorThreshold {
			continue
		}
		out = append(out, doctorFinding{kind: "REPEAT_REFUSAL", who: k[0], firstSeen: r.at, seqs: r.seqs,
			detail: fmt.Sprintf("candidate tree %s refused %d times", k[1], len(r.seqs))})
	}
	return out
}

// doctorSlowLane counts each allocation's entries into CLEANING
// (TQD-V0-007).
func doctorSlowLane(scan *doctorScan) []doctorFinding {
	type tries struct {
		pool, member, allocation string
		seqs                     []uint64
		at                       time.Time
		state                    string
	}
	all := map[string]*tries{}
	var order []string
	prev := map[string]string{}
	for _, ev := range scan.events {
		if ev.pools == nil {
			continue
		}
		cur := map[string]string{}
		for _, en := range ev.pools.Entries {
			k := en.PoolID + "/" + en.MemberID + "/" + string(en.AllocationID)
			cur[k] = en.State
			t := all[k]
			if t == nil {
				t = &tries{pool: en.PoolID, member: en.MemberID, allocation: string(en.AllocationID)}
				all[k] = t
				order = append(order, k)
			}
			t.state = en.State
			if en.State == "CLEANING" && prev[k] != "CLEANING" {
				if len(t.seqs) == 0 {
					t.at = ev.at
				}
				t.seqs = append(t.seqs, ev.seq)
			}
		}
		for k := range prev {
			if _, ok := cur[k]; !ok {
				all[k].state = "FREE"
			}
		}
		prev = cur
	}
	var out []doctorFinding
	for _, k := range order {
		t := all[k]
		if len(t.seqs) < doctorThreshold {
			continue
		}
		out = append(out, doctorFinding{kind: "SLOW_LANE_RECOVERY", who: t.pool + "/" + t.member, firstSeen: t.at, seqs: t.seqs,
			detail: fmt.Sprintf("allocation %s entered cleanup %d times; now %s", t.allocation, len(t.seqs), t.state)})
	}
	return out
}

// doctorSetupOnly names tickets completed within the window whose ledger
// witnessed none of its core obligations (TQD-V0-008).
func doctorSetupOnly(rc *readCtx, scan *doctorScan) []doctorFinding {
	cutoff := scan.observedAt.Add(-doctorWindow)
	var out []doctorFinding
	for _, id := range rc.store.Inventory.IDs() {
		rec, ok := rc.store.Inventory.Get(id)
		if !ok || rec.Status != ticket.StatusCompleted || rec.Completion == nil || rec.ObligationsRef == nil {
			continue
		}
		c := rec.ObligationsRef.Counts
		if c.CoreTotal == 0 || c.CoreWitnessed != 0 {
			continue
		}
		at, err := time.Parse(time.RFC3339, string(rec.Completion.RecordedAt))
		if err != nil || at.Before(cutoff) {
			continue
		}
		out = append(out, doctorFinding{kind: "SETUP_ONLY_PROOF", who: rec.TicketID.Raw, firstSeen: at,
			detail: fmt.Sprintf("completed with 0 of %d core obligations witnessed (%d of %d total)", c.CoreTotal, c.Witnessed, c.Total)})
	}
	return out
}

// doctorFalseIdle names stale or lease-expired live attempts whose current
// detached run supervisor is alive with a busy process tree (TQD-V0-009).
func doctorFalseIdle(repo *intent.Repository, scan *doctorScan) []doctorFinding {
	var out []doctorFinding
	var table map[int][]doctorProc
	var tableErr error
	loaded := false
	for _, c := range scan.idleCandidates {
		a := c.a
		base := runsDir(repo, a.AttemptID)
		entries, err := listRuns(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !validRunID(e.Name()) {
				continue
			}
			rec, err := readRunRecord(filepath.Join(base, e.Name()))
			if err != nil || (rec.State != runStarting && rec.State != runRunning) || rec.Generation != string(a.Generation) {
				continue
			}
			if live, err := processLive(rec.SupervisorPid, rec.SupervisorIdentity); err != nil || !live {
				continue
			}
			if !loaded {
				table, tableErr = doctorProcessTable()
				loaded = true
				if tableErr != nil {
					scan.warnings = append(scan.warnings, prose("doctor process walk incomplete: "+tableErr.Error()))
				}
			}
			if tableErr != nil {
				return out
			}
			if pid, busy := doctorBusyDescendant(table, rec.SupervisorPid); busy {
				out = append(out, doctorFinding{kind: "FALSE_IDLE", who: a.AttemptID, carried: true, firstSeen: scan.observedAt,
					detail: fmt.Sprintf("run %s supervisor %d has live descendant %d while the holder is %s", rec.RunID, rec.SupervisorPid, pid, c.status)})
				break
			}
		}
	}
	return out
}

type doctorProc struct {
	pid    int
	zombie bool
}

// doctorProcessTable is the bounded local process walk, children by parent.
var doctorProcessTable = func() (map[int][]doctorProc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), doctorPsTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,stat=")
	c.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin"}
	out := &doctorLimited{max: doctorPsBytes}
	c.Stdout = out
	if err := c.Run(); err != nil {
		return nil, err
	}
	if out.over {
		return nil, errors.New("process table larger than the walk bound")
	}
	table := map[int][]doctorProc{}
	n := 0
	for _, line := range strings.Split(out.buf.String(), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if n++; n > doctorPsProcs || len(f) < 3 {
			return nil, errors.New("process table exceeds the walk bound or is malformed")
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		if e1 != nil || e2 != nil {
			return nil, errors.New("malformed process table line")
		}
		table[ppid] = append(table[ppid], doctorProc{pid: pid, zombie: strings.HasPrefix(f[2], "Z")})
	}
	return table, nil
}

func doctorBusyDescendant(table map[int][]doctorProc, root int) (int, bool) {
	queue, seen := []int{root}, map[int]bool{root: true}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, c := range table[p] {
			if seen[c.pid] {
				continue
			}
			seen[c.pid] = true
			if !c.zombie {
				return c.pid, true
			}
			queue = append(queue, c.pid)
		}
	}
	return 0, false
}

// doctorLimited keeps at most max bytes and records an overflow.
type doctorLimited struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (l *doctorLimited) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); len(p) > room {
		l.over = true
		if room > 0 {
			l.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return l.buf.Write(p)
}

// doctorPlugins runs each regular executable in dir under the TQD-V0-010
// bounds; every failure is a PLUGIN_FAILED finding, never a command failure.
func doctorPlugins(dir, cwd string) []doctorFinding {
	names, err := doctorPluginNames(dir)
	if err != nil {
		return []doctorFinding{doctorPluginFailed(filepath.Base(dir), err.Error())}
	}
	sort.Strings(names)
	var out []doctorFinding
	if len(names) > doctorMaxPlugins {
		out = append(out, doctorPluginFailed(names[doctorMaxPlugins], fmt.Sprintf("more than %d plugins; the rest were not run", doctorMaxPlugins)))
		names = names[:doctorMaxPlugins]
	}
	for _, name := range names {
		out = append(out, doctorRunPlugin(filepath.Join(dir, name), name, cwd)...)
	}
	return out
}

// doctorPluginNames lists the regular executables in dir, reading at most
// doctorMaxPluginEntries entries in fixed-size batches; a larger directory
// is refused whole, so no plugin runs (TQD-V0-010).
func doctorPluginNames(dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("plugin directory unreadable: %v", err)
	}
	defer d.Close()
	var names []string
	for seen := 0; ; {
		batch, err := d.ReadDir(256)
		seen += len(batch)
		if seen > doctorMaxPluginEntries {
			return nil, fmt.Errorf("plugin directory holds more than %d entries; no plugin was run", doctorMaxPluginEntries)
		}
		for _, e := range batch {
			if !e.Type().IsRegular() {
				continue
			}
			if st, err := e.Info(); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
				names = append(names, e.Name())
			}
		}
		if errors.Is(err, io.EOF) {
			return names, nil
		}
		if err != nil {
			return nil, fmt.Errorf("plugin directory unreadable: %v", err)
		}
	}
}

func doctorPluginFailed(name, reason string) doctorFinding {
	return doctorFinding{kind: "PLUGIN_FAILED", source: "plugin:" + name, who: name, detail: reason, carried: true}
}

type doctorPluginItem struct {
	Kind   *string `json:"kind"`
	Who    *string `json:"who"`
	Detail *string `json:"detail"`
	Remedy *string `json:"remedy"`
}

func doctorRunPlugin(path, name, cwd string) []doctorFinding {
	ctx, cancel := context.WithTimeout(context.Background(), doctorPluginTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, path)
	c.Dir = cwd
	c.Stdin = nil
	out := &doctorLimited{max: doctorPluginOutput}
	c.Stdout = out
	doctorPluginGroup(c)
	c.WaitDelay = time.Second
	err := c.Run()
	doctorPluginReap(c)
	switch {
	case ctx.Err() != nil:
		return []doctorFinding{doctorPluginFailed(name, fmt.Sprintf("timed out after %s", doctorPluginTimeout))}
	case err != nil:
		return []doctorFinding{doctorPluginFailed(name, "failed: "+err.Error())}
	case out.over:
		return []doctorFinding{doctorPluginFailed(name, fmt.Sprintf("output larger than %d bytes", doctorPluginOutput))}
	}
	items, err := doctorDecodePlugin(out.buf.Bytes())
	if err != nil {
		return []doctorFinding{doctorPluginFailed(name, "malformed output: "+err.Error())}
	}
	var findings []doctorFinding
	for _, it := range items {
		f := doctorFinding{kind: *it.Kind, source: "plugin:" + name, who: *it.Who, detail: *it.Detail, carried: true}
		if it.Remedy != nil {
			f.remedy = *it.Remedy
		}
		findings = append(findings, f)
	}
	return findings
}

func doctorDecodePlugin(raw []byte) ([]doctorPluginItem, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var items []doctorPluginItem
	if err := dec.Decode(&items); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the array")
	}
	if items == nil {
		return nil, errors.New("not a JSON array")
	}
	if len(items) > doctorPluginItems {
		return nil, fmt.Errorf("more than %d findings", doctorPluginItems)
	}
	for _, it := range items {
		if it.Kind == nil || it.Who == nil || it.Detail == nil {
			return nil, errors.New("finding lacks kind, who or detail")
		}
		if !doctorKindPattern.MatchString(*it.Kind) {
			return nil, fmt.Errorf("kind %q is not an upper-case code", *it.Kind)
		}
		for _, s := range []*string{it.Who, it.Detail, it.Remedy} {
			if s == nil {
				continue
			}
			if _, err := wire.ParseProse("", *s, 1, doctorPluginProse); err != nil {
				return nil, errors.New("finding text is not valid prose")
			}
		}
	}
	return items, nil
}

// doctorCarry maps each cached finding's kind, source and who to its
// firstSeen; an absent or invalid cache carries nothing, and the read error
// is returned so a refresh can refuse a cache written by a later build.
func doctorCarry(path string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	v, err := readDoctorCache(path)
	if err != nil {
		return out, err
	}
	fs, _ := v.Obj.Get("findings")
	for _, f := range fs.Arr {
		if f.Kind != wire.KindObject || f.Obj == nil {
			continue
		}
		get := func(k string) string { s, _ := f.Obj.Get(k); return s.Str }
		at, err := time.Parse(time.RFC3339, get("firstSeen"))
		if err != nil {
			continue
		}
		out[get("kind")+"\x00"+get("source")+"\x00"+get("who")] = at
	}
	return out, nil
}

// doctorItem renders the finding report. A positive maxBytes bounds the
// encoded cache file: trailing findings, in report order, are dropped until
// it fits and findingsTruncated is set (TQD-V0-003, TQD-V0-011).
func doctorItem(rc *readCtx, scan *doctorScan, findings []doctorFinding, carry map[string]time.Time, profile string, maxBytes int) wire.Value {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.source != b.source {
			return a.source < b.source
		}
		return a.who < b.who
	})
	total := len(findings)
	cut := false
	if total > doctorMaxFindings {
		findings, cut = findings[:doctorMaxFindings], true
	}
	arr := make([]wire.Value, 0, len(findings))
	for _, f := range findings {
		if f.source == "" {
			f.source = "builtin"
		}
		if f.remedy == "" {
			f.remedy = doctorRemedies[f.kind]
		}
		if f.remedy == "" {
			f.remedy = "See the plugin's documentation."
		}
		if f.carried {
			if at, ok := carry[f.key()]; ok && !at.After(scan.observedAt) {
				f.firstSeen = at
			}
		}
		if f.firstSeen.IsZero() || f.firstSeen.After(scan.observedAt) {
			f.firstSeen = scan.observedAt
		}
		seqs := make([]string, 0, len(f.seqs))
		for _, s := range f.seqs {
			seqs = append(seqs, strconv.FormatUint(s, 10))
		}
		o := wire.NewObject().Set("kind", wire.String(f.kind)).Set("source", wire.String(f.source)).Set("who", wire.String(f.who))
		o.Set("detail", wire.String(prose(f.detail))).Set("remedy", wire.String(f.remedy))
		o.Set("firstSeen", wire.String(f.firstSeen.UTC().Format(time.RFC3339)))
		o.Set("ageSeconds", wire.String(strconv.FormatInt(int64(scan.observedAt.Sub(f.firstSeen)/time.Second), 10)))
		o.Set("evidenceSeqs", wire.Strings(seqs))
		arr = append(arr, wire.ObjectValue(o))
	}
	build := func(n int) wire.Value {
		return doctorItemOf(rc, scan, arr[:n], cut || n < len(arr), total, profile, maxBytes > 0)
	}
	v := build(len(arr))
	if maxBytes <= 0 || len(wire.EncodeFile(v)) <= maxBytes {
		return v
	}
	// The largest prefix that fits; the empty report always fits.
	lo, hi := 0, len(arr)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if len(wire.EncodeFile(build(mid))) <= maxBytes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return build(lo)
}

func doctorItemOf(rc *readCtx, scan *doctorScan, arr []wire.Value, cut bool, total int, profile string, cache bool) wire.Value {
	count := func(n int) wire.Value { return wire.String(string(wire.CountOf(int64(n)))) }
	lanes := wire.NewObject().Set("free", count(scan.lanesFree)).Set("total", count(scan.lanesTotal))
	summary := wire.NewObject().Set("lanes", wire.ObjectValue(lanes)).Set("runningSessions", count(scan.runningSessions))
	summary.Set("completions24h", wire.String(scan.completions24h)).Set("alerts", count(total))
	sc := wire.NewObject().Set("receipts", count(scan.receipts)).Set("headSeq", wire.String(strconv.FormatUint(scan.head, 10)))
	from := wire.Null()
	if scan.receipts > 0 {
		from = wire.String(strconv.FormatUint(scan.fromSeq, 10))
	}
	sc.Set("fromSeq", from).Set("windowSeconds", wire.String(strconv.FormatInt(int64(doctorWindow/time.Second), 10)))
	sc.Set("truncated", wire.Bool(scan.truncated)).Set("findingsTruncated", wire.Bool(cut))
	o := wire.NewObject().Set("profile", wire.String(profile)).Set("queueId", wire.String(rc.store.Queue.QueueID.Raw))
	o.Set("observedAt", wire.String(scan.observedAt.Format(time.RFC3339)))
	o.Set("summary", wire.ObjectValue(summary)).Set("findings", wire.Array(arr...)).Set("scan", wire.ObjectValue(sc))
	o.Set("mutationAuthority", wire.Bool(false))
	if cache {
		o.Set("refreshedAt", wire.String(scan.observedAt.Format(time.RFC3339)))
	}
	return wire.ObjectValue(o)
}

// doctorCacheHook observes the refresh between its steps; tests only.
var doctorCacheHook func(stage string)

func doctorCacheRefusal(what string) error {
	return wire.Errorf(wire.CodeUnsupportedFilesystem, doctorCacheDirName, "doctor cache %s; refusing to refresh through it", what)
}

// doctorNewerCache refuses the bytes of a cache written by a later build;
// any other undecodable cache may be replaced (TQD-V0-011).
func doctorNewerCache(raw []byte) error {
	if _, err := decodeDoctorCache(raw); wire.CodeOf(err) == wire.CodeUnsupportedVersion {
		return err
	}
	return nil
}

func readDoctorCache(path string) (wire.Value, error) {
	raw, err := readBounded(path, doctorCacheBytes)
	if err != nil {
		return wire.Value{}, err
	}
	return decodeDoctorCache(raw)
}

// decodeDoctorCache refuses a cache written by a later build as
// UNSUPPORTED_VERSION (CAL-V0-131) and any other profile as malformed.
func decodeDoctorCache(raw []byte) (wire.Value, error) {
	v, err := wire.Parse(raw)
	if err != nil {
		return wire.Value{}, err
	}
	if v.Kind != wire.KindObject || v.Obj == nil {
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "/", "doctor cache is not an object")
	}
	if p, _ := v.Obj.Get("profile"); p.Str != doctorCacheProfile {
		if strings.HasPrefix(p.Str, "taskman-doctor-cache/") {
			return wire.Value{}, wire.Errorf(wire.CodeUnsupportedVersion, "profile", "doctor cache %q is not %s", p.Str, doctorCacheProfile)
		}
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "profile", "not a doctor cache")
	}
	return v, nil
}

// doctorLine prints the status-bar line from the cache alone: repository
// resolution and one bounded read, no lock and no store read (TQD-V0-012).
func doctorLine(env Env) int {
	line, err := doctorLineText(env.Cwd)
	if err != nil {
		_, _ = io.WriteString(env.Stdout, doctorLineUnavailable)
		return 1
	}
	_, _ = io.WriteString(env.Stdout, line)
	return 0
}

func doctorLineText(cwd string) (string, error) {
	common, err := intent.ResolveCommonDir(cwd)
	if err != nil {
		return "", err
	}
	v, err := readDoctorCache(filepath.Join(common, doctorCacheDirName, doctorCacheFileName))
	if err != nil {
		return "", err
	}
	bad := errors.New("cache summary malformed")
	obj := func(v wire.Value, k string) (wire.Value, error) {
		if v.Kind != wire.KindObject || v.Obj == nil {
			return wire.Value{}, bad
		}
		x, ok := v.Obj.Get(k)
		if !ok {
			return wire.Value{}, bad
		}
		return x, nil
	}
	num := func(v wire.Value, k string) (string, error) {
		x, err := obj(v, k)
		if err != nil || x.Kind != wire.KindString {
			return "", bad
		}
		if _, err := wire.ParseCount(k, x.Str); err != nil {
			return "", bad
		}
		return x.Str, nil
	}
	summary, err := obj(v, "summary")
	if err != nil {
		return "", err
	}
	lanes, err := obj(summary, "lanes")
	if err != nil {
		return "", err
	}
	var vals [5]string
	for i, p := range []struct {
		v wire.Value
		k string
	}{{lanes, "free"}, {lanes, "total"}, {summary, "runningSessions"}, {summary, "completions24h"}, {summary, "alerts"}} {
		if vals[i], err = num(p.v, p.k); err != nil {
			return "", err
		}
	}
	ref, err := obj(v, "refreshedAt")
	if err != nil {
		return "", err
	}
	at, err := time.Parse(time.RFC3339, ref.Str)
	if err != nil {
		return "", bad
	}
	line := fmt.Sprintf("lanes %s/%s free | sessions %s | 24h %s done | alerts %s", vals[0], vals[1], vals[2], vals[3], vals[4])
	if age := doctorClock().Sub(at); age >= doctorLineStale {
		line += fmt.Sprintf(" | stale %dm", int64(age/time.Minute))
	}
	return line + "\n", nil
}
