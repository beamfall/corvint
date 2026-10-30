package cli_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const batchIssuedAt = "2026-09-07T12:00:00Z"

// batchRepo is an initialized store with n tickets, each at revision 1.
func batchRepo(t *testing.T, n int) (*fixture.Repo, []string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ids := make([]string, n)
	for i := range ids {
		payload := strings.Replace(createPayloadJSON, `"Console ticket"`, fmt.Sprintf(`"Batch ticket %d"`, i), 1)
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", fmt.Sprintf("create-%d", i),
			"--issued-at", batchIssuedAt, "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %d: %+v", i, x.res)
		}
		ids[i] = field(x.res.Items[0], "ticketId").Str
	}
	return r, ids
}

// batchInput is the canonical batch array: entry i retitles ids[i] at revs[i].
func batchInput(ids []string, revs []string, prefix string) []byte {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf(`{"expectedRevision":%q,"payload":{"title":"%s %d"},"target":%q}`, revs[i], prefix, i, id)
	}
	return []byte("[" + strings.Join(parts, ",") + "]")
}

func ones(n int) []string {
	revs := make([]string, n)
	for i := range revs {
		revs[i] = "1"
	}
	return revs
}

func refineBatch(t *testing.T, r *fixture.Repo, requestID string, input []byte, extra ...string) run {
	t.Helper()
	args := append([]string{"ticket", "refine", "--batch", "--request-id", requestID, "--issued-at", batchIssuedAt, "--payload-stdin"}, extra...)
	return atm(t, r.Root, input, args...)
}

func batchEntries(t *testing.T, x run) []wire.Value {
	t.Helper()
	if len(x.res.Items) != 1 {
		t.Fatalf("batch result has %d items, want 1: %+v", len(x.res.Items), x.res)
	}
	return field(x.res.Items[0], "entries").Arr
}

// stateDigest hashes every file of the intent and journal directories.
func stateDigest(t *testing.T, r *fixture.Repo) [32]byte {
	t.Helper()
	var paths []string
	for _, root := range []string{r.IntentDir, r.StateDir} {
		if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				paths = append(paths, p)
			}
			return err
		}); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(raw))
		h.Write(raw)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// TestCALV0106_BatchRefineAllSuccess applies a batch longer than one chunk;
// every entry commits with its own receipt and request ID.
func TestCALV0106_BatchRefineAllSuccess(t *testing.T) {
	r, ids := batchRepo(t, 10)
	x := refineBatch(t, r, "batch-1", batchInput(ids, ones(len(ids)), "Refined"))
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("batch: %+v", x.res)
	}
	item := x.res.Items[0]
	if field(item, "completed").Str != "10" || field(item, "failed").Str != "0" {
		t.Errorf("counts: completed %s failed %s", field(item, "completed").Str, field(item, "failed").Str)
	}
	// The 2 s hold bound may cut chunks early on a loaded host, so only the
	// bounds are fixed: chunks numbered 1..chunks in order, each holding 1 to
	// 8 entries, and receipts strictly increasing with the entry index.
	chunks, err := strconv.Atoi(field(item, "chunks").Str)
	if err != nil || chunks < 2 {
		t.Fatalf("chunks %q for 10 entries", field(item, "chunks").Str)
	}
	sizes := make([]int, chunks+1)
	previous, last := "", 1
	for i, e := range batchEntries(t, x) {
		if field(e, "outcome").Str != "COMPLETED" || field(e, "resultingRevision").Str != "2" {
			t.Errorf("entry %d: %v", i, e)
		}
		if got, want := field(e, "requestId").Str, fmt.Sprintf("batch-1/%d", i); got != want {
			t.Errorf("entry %d request ID %q, want %q", i, got, want)
		}
		chunk, err := strconv.Atoi(field(e, "chunk").Str)
		if err != nil || chunk < last || chunk > last+1 || chunk > chunks {
			t.Fatalf("entry %d in chunk %q after chunk %d", i, field(e, "chunk").Str, last)
		}
		last = chunk
		sizes[chunk]++
		receipt := field(e, "receipt").Str
		if receipt == "" || receipt <= previous {
			t.Errorf("entry %d receipt %q does not follow %q", i, receipt, previous)
		}
		previous = receipt
	}
	for chunk, size := range sizes[1:] {
		if size < 1 || size > 8 {
			t.Errorf("chunk %d holds %d entries, want 1..8", chunk+1, size)
		}
	}
	shown := atm(t, r.Root, nil, "ticket", "show", ids[9])
	if got := field(shown.res.Items[0], "title").Str; got != "Refined 9" {
		t.Errorf("ticket 9 title %q after the batch", got)
	}
}

// TestCALV0106_BatchRefineStaleEntryFailsAlone: a stale expectedRevision
// refuses that entry only; the rest commit.
func TestCALV0106_BatchRefineStaleEntryFailsAlone(t *testing.T) {
	r, ids := batchRepo(t, 3)
	revs := ones(3)
	revs[1] = "7"
	x := refineBatch(t, r, "batch-stale", batchInput(ids, revs, "Refined"))
	if x.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("batch with a stale entry: %+v", x.res)
	}
	entries := batchEntries(t, x)
	for i, e := range entries {
		if got := field(e, "outcome").Str; (got == "COMPLETED") != (i != 1) {
			t.Errorf("entry %d outcome %s: %v", i, got, e)
		}
	}
	if got := field(entries[1], "outcome").Str; got != "REVISION_CONFLICT" {
		t.Errorf("stale entry outcome %s, want REVISION_CONFLICT", got)
	}
	if field(x.res.Items[0], "failed").Str != "1" || field(x.res.Items[0], "completed").Str != "2" {
		t.Errorf("counts: %v", x.res.Items[0])
	}
	if field(entries[1], "receipt").Str != "" {
		t.Error("the stale entry wrote a receipt")
	}
	shown := atm(t, r.Root, nil, "ticket", "show", ids[1])
	if got := field(shown.res.Items[0], "title").Str; got != "Batch ticket 1" {
		t.Errorf("the stale entry was partly applied: title %q", got)
	}
}

// TestCALV0106_BatchRefineMalformedEntryWritesNothing: every entry is
// validated before the first lock, so one bad entry refuses the whole batch
// and the store is byte-for-byte unchanged.
func TestCALV0106_BatchRefineMalformedEntryWritesNothing(t *testing.T) {
	r, ids := batchRepo(t, 2)
	good := fmt.Sprintf(`{"expectedRevision":"1","payload":{"title":"Refined"},"target":%q}`, ids[0])
	cases := map[string]string{
		"not an array":     `{"target":"x"}`,
		"empty":            `[]`,
		"unknown key":      fmt.Sprintf(`[%s,{"expectedRevision":"1","extra":1,"payload":{"title":"x"},"target":%q}]`, good, ids[1]),
		"missing target":   fmt.Sprintf(`[%s,{"expectedRevision":"1","payload":{"title":"x"}}]`, good),
		"bad payload":      fmt.Sprintf(`[%s,{"expectedRevision":"1","payload":{"nope":true},"target":%q}]`, good, ids[1]),
		"bad revision":     fmt.Sprintf(`[%s,{"expectedRevision":"one","payload":{"title":"x"},"target":%q}]`, good, ids[1]),
		"unknown target":   fmt.Sprintf(`[%s,{"expectedRevision":"1","payload":{"title":"x"},"target":"ticket:acme:main:zzzz"}]`, good),
		"duplicate target": fmt.Sprintf(`[%s,%s]`, good, good),
	}
	before := stateDigest(t, r)
	for name, input := range cases {
		x := refineBatch(t, r, "batch-bad", []byte(input))
		if x.res.Outcome == wire.OutcomeOK || len(x.res.Codes) == 0 {
			t.Errorf("%s: accepted: %+v", name, x.res)
		}
		if after := stateDigest(t, r); after != before {
			t.Fatalf("%s: a refused batch changed the store", name)
		}
	}
}

// TestCALV0106_BatchRefineReplayIsIdempotent: the same batch with the same
// request ID and issuedAt replays every entry and writes nothing; a changed
// issuedAt is a request ID conflict, not a second application.
func TestCALV0106_BatchRefineReplayIsIdempotent(t *testing.T) {
	r, ids := batchRepo(t, 10)
	input := batchInput(ids, ones(len(ids)), "Refined")
	if x := refineBatch(t, r, "batch-replay", input); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first batch: %+v", x.res)
	}
	before := stateDigest(t, r)
	again := refineBatch(t, r, "batch-replay", input)
	if again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "replayed").Str != "10" {
		t.Fatalf("replay: %+v", again.res)
	}
	for i, e := range batchEntries(t, again) {
		if !field(e, "replayed").Bool || field(e, "receipt").Str != "" {
			t.Errorf("entry %d was not replayed: %v", i, e)
		}
	}
	if stateDigest(t, r) != before {
		t.Error("a replayed batch changed the store")
	}
	conflict := atm(t, r.Root, input, "ticket", "refine", "--batch", "--request-id", "batch-replay",
		"--issued-at", "2026-09-07T12:00:01Z", "--payload-stdin")
	if conflict.res.Outcome == wire.OutcomeOK || !hasCode(conflict.res, wire.CodeRequestIDConflict) {
		t.Errorf("a changed issuedAt under the same request ID: %+v", conflict.res)
	}
	if stateDigest(t, r) != before {
		t.Error("a conflicting batch changed the store")
	}
}

// TestCALV0106_BatchRefineHelpAndFlags: the help documents the batch form,
// and flags of the single form are refused with --batch.
func TestCALV0106_BatchRefineHelpAndFlags(t *testing.T) {
	r, ids := batchRepo(t, 1)
	help := atm(t, r.Root, nil, "ticket", "refine", "--help", "--verbose")
	if !strings.Contains(field(help.res.Items[0], "usage").Str, "--batch") || field(help.res.Items[0], "batch").Str == "" {
		t.Errorf("refine --help does not document --batch: %+v", help.res.Items[0])
	}
	input := batchInput(ids, ones(1), "Refined")
	if x := refineBatch(t, r, "batch-flags", input, "--target", ids[0]); x.res.Outcome == wire.OutcomeOK {
		t.Error("--batch accepted --target")
	}
	if x := atm(t, r.Root, input, "ticket", "refine", "--batch", "--payload-stdin"); x.res.Outcome == wire.OutcomeOK {
		t.Error("--batch ran without --request-id")
	}
	if x := atm(t, r.Root, input, "ticket", "prioritize", "--batch", "--request-id", "b", "--payload-stdin"); x.res.Outcome == wire.OutcomeOK {
		t.Error("prioritize accepted --batch")
	}
}

// TestCALV0106_BatchFlagOnlyInFlagPosition: a "--batch" that is the value of
// another flag is that value, not the batch form.
func TestCALV0106_BatchFlagOnlyInFlagPosition(t *testing.T) {
	r, ids := batchRepo(t, 1)
	x := atm(t, r.Root, nil, "ticket", "refine", "--request-id", "--batch", "--issued-at", batchIssuedAt,
		"--target", ids[0], "--expected-revision", "1", "--payload", `{"title":"Single"}`)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "ticketId").Str != ids[0] {
		t.Fatalf("a single refine whose request ID is --batch: %+v", x.res)
	}
	if x := atm(t, r.Root, nil, "release", "candidate", "--batch", "--request-id", "r", "--target", "x"); x.res.Outcome == wire.OutcomeOK {
		t.Error("release candidate accepted --batch")
	}
}

// TestCALV0106_BatchRefineBoundsInlinePayload: the 8 MiB input bound applies
// to --payload as well as --payload-stdin.
func TestCALV0106_BatchRefineBoundsInlinePayload(t *testing.T) {
	r, ids := batchRepo(t, 1)
	before := stateDigest(t, r)
	big := `[{"expectedRevision":"1","payload":{"title":"` + strings.Repeat("x", 8<<20) + `"},"target":"` + ids[0] + `"}]`
	for name, args := range map[string][]string{
		"inline": {"--payload", big},
		"stdin":  {"--payload-stdin"},
	} {
		argv := append([]string{"ticket", "refine", "--batch", "--request-id", "big", "--issued-at", batchIssuedAt}, args...)
		x := atm(t, r.Root, []byte(big), argv...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeLimitExceeded) {
			t.Errorf("%s: an oversized batch was not refused as %s: %v", name, wire.CodeLimitExceeded, x.res.Codes)
		}
	}
	if stateDigest(t, r) != before {
		t.Error("an oversized batch changed the store")
	}
}
