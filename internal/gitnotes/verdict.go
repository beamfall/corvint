package gitnotes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

// The verdict ledger (CEM-V1-271..279) is an append-only record of the
// verdictSha256 (CEM-V1-066) the native stable verifier reached for one
// anchored map on one target commit, kept as the note of that commit under
// VerdictRef. One note holds one JSON record per LF-terminated line; each
// record names the SHA-256 of the line before it, so a reader detects an
// edited, removed or reordered record. Only an explicit `cem anchor
// --artifacts DIR` appends; every read is read-only.
const (
	// VerdictRef is the sibling of Ref in the Corvint notes namespace: a ref
	// below refs/notes/corvint/ cannot coexist with the refs/notes/corvint ref,
	// and the anchor pointer note must stay one object (CEM-V1-067).
	VerdictRef = "refs/notes/corvint-verdicts"
	// VerdictSchema names the one closed record shape (CEM-V1-272).
	VerdictSchema = "corvint-cem-verdict/1"
	// VerdictConsumer is the only consumer a /1 record names: the native
	// stable verifier anchor runs in process.
	VerdictConsumer = "native"
	// MaxVerdictRecords bounds the records one target commit's note holds.
	MaxVerdictRecords = 256
)

// Registered refusal codes of the verdict ledger append (CEM-V1-275).
const (
	CodeVerdictConflict    = "verdict-ledger-conflict"
	CodeVerdictMalformed   = "verdict-ledger-malformed"
	CodeVerdictUnsupported = "verdict-ledger-unsupported-result"
	CodeVerdictFull        = "verdict-ledger-full"
)

// verdictNow is the clock recordedAt reads; tests pin it.
var verdictNow = time.Now

// VerdictRecord is the closed corvint-cem-verdict/1 record. Its canonical
// bytes are exactly json.Marshal of the struct: the members in this order and
// no others (CEM-V1-272).
type VerdictRecord struct {
	Schema               string         `json:"schema"`
	MapSha256            string         `json:"mapSha256"`
	Target               VerdictTarget  `json:"target"`
	Outcome              string         `json:"outcome"`
	VerdictSha256        string         `json:"verdictSha256"`
	Consumer             string         `json:"consumer"`
	Runtime              VerdictRuntime `json:"runtime"`
	RecordedAt           string         `json:"recordedAt"`
	PreviousRecordSha256 *string        `json:"previousRecordSha256"`
}

// VerdictTarget is the commit the verdict was reached for and its tree.
type VerdictTarget struct {
	GitCommit string `json:"gitCommit"`
	GitTree   string `json:"gitTree"`
}

// VerdictRuntime is the Go release of the consumer that reached the verdict.
type VerdictRuntime struct {
	GoVersion string `json:"goVersion"`
}

// wellFormedRecord checks every member's value domain (CEM-V1-272).
func wellFormedRecord(r VerdictRecord) bool {
	recorded, err := time.Parse(time.RFC3339, r.RecordedAt)
	return r.Schema == VerdictSchema && sha256Hex(r.MapSha256) && sha256Hex(r.VerdictSha256) &&
		objectID(r.Target.GitCommit) && objectID(r.Target.GitTree) &&
		(r.Outcome == "ACCEPT" || r.Outcome == "REJECT") && r.Consumer == VerdictConsumer &&
		r.Runtime.GoVersion != "" && len(r.Runtime.GoVersion) <= MaxTextBytes && printableASCII(r.Runtime.GoVersion) &&
		err == nil && recorded.UTC().Format(time.RFC3339) == r.RecordedAt &&
		(r.PreviousRecordSha256 == nil || sha256Hex(*r.PreviousRecordSha256))
}

func sha256Hex(value string) bool { return len(value) == 64 && hexadecimal(value) }

func printableASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

// ledger is one target commit's parsed verdict note: its records in order,
// their exact line bytes, and the closed reader state (CEM-V1-276).
type ledger struct {
	state   string
	records []VerdictRecord
	lines   [][]byte
}

// readLedger reads and checks target's verdict note. A note is `verified`
// only when every line is a canonical record, the previousRecordSha256 chain
// is unbroken from a null first link, every record names target and its tree,
// and no mapSha256 repeats; otherwise its state names the first failure in
// that order. An absent note is `absent` with no records.
func (r *repository) readLedger(ctx context.Context, target string) (ledger, error) {
	return r.readLedgerAt(ctx, target, false, "")
}

// readLedgerAt is readLedger over a fixed notes commit when pinned is true:
// the note is found in that commit's tree by object ID, never through the
// mutable ref, and an empty notesCommit (absent ref) reads `absent`.
func (r *repository) readLedgerAt(ctx context.Context, target string, pinned bool, notesCommit string) (ledger, error) {
	var noteOid string
	var err error
	switch {
	case !pinned:
		noteOid, err = r.noteBlob(ctx, VerdictRef, target)
	case notesCommit != "":
		noteOid, err = r.noteBlobAt(ctx, notesCommit, target)
	}
	if err != nil || noteOid == "" {
		return ledger{state: "absent"}, err
	}
	body, err := r.blob(ctx, noteOid, MaxNoteBytes)
	if err != nil {
		return ledger{}, err
	}
	malformed := ledger{state: "malformed"}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		return malformed, nil
	}
	lines := bytes.Split(body[:len(body)-1], []byte("\n"))
	if len(lines) > MaxVerdictRecords {
		return malformed, nil
	}
	parsed := ledger{state: "verified", records: make([]VerdictRecord, 0, len(lines)), lines: lines}
	for _, line := range lines {
		var record VerdictRecord
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&record) != nil || decoder.More() || !wellFormedRecord(record) {
			return malformed, nil
		}
		canonical, err := json.Marshal(record)
		if err != nil || !bytes.Equal(canonical, line) {
			return malformed, nil
		}
		parsed.records = append(parsed.records, record)
	}
	for index, record := range parsed.records {
		want := (*string)(nil)
		if index > 0 {
			digest := lineDigest(lines[index-1])
			want = &digest
		}
		if (want == nil) != (record.PreviousRecordSha256 == nil) || (want != nil && *want != *record.PreviousRecordSha256) {
			return ledger{state: "chain-broken"}, nil
		}
	}
	tree, err := r.tree(ctx, target)
	if err != nil {
		return ledger{}, err
	}
	seen := map[string]bool{}
	for _, record := range parsed.records {
		if record.Target.GitCommit != target || record.Target.GitTree != tree {
			return ledger{state: "scope-mismatch"}, nil
		}
		if seen[record.MapSha256] {
			return ledger{state: "conflict"}, nil
		}
		seen[record.MapSha256] = true
	}
	return parsed, nil
}

func lineDigest(line []byte) string {
	digest := sha256.Sum256(line)
	return hex.EncodeToString(digest[:])
}

// verdictRecord runs the native stable verifier over the exact committed map
// bytes against target, with the map's base as the expected base and the
// caller's absolute artifact root, and builds the record it would append. A
// verifier that reaches no verdict (UNSUPPORTED) is refused: a ledger records
// only ACCEPT or REJECT (CEM-V1-274).
func (r *repository) verdictRecord(ctx context.Context, raw []byte, p Pointer, target, artifacts string) (VerdictRecord, error) {
	result, _ := verify.Stable(ctx, raw, verify.StableOptions{
		Repository: r.root, ExpectedBase: p.Base, Target: target, ArtifactRoot: artifacts,
	})
	if result.Outcome != "ACCEPT" && result.Outcome != "REJECT" {
		code := ""
		if result.Code != nil {
			code = *result.Code
		}
		return VerdictRecord{}, cemcode.New(CodeVerdictUnsupported,
			"the stable verifier reached no verdict (%s at %s: %s); no note was written", result.Outcome, result.Stage, code).
			Guided("pass --artifacts with the absolute directory holding the map's declared artifacts, as cem verify does")
	}
	digest, err := result.VerdictDigest()
	if err != nil {
		return VerdictRecord{}, cemcode.New("output-failed", "cannot compute verdictSha256")
	}
	if result.MapSha256 == nil || *result.MapSha256 != p.MapSha256 {
		return VerdictRecord{}, cemcode.New("output-failed", "the verifier result names a different map digest")
	}
	return VerdictRecord{
		Schema: VerdictSchema, MapSha256: p.MapSha256,
		Target:  VerdictTarget{GitCommit: target, GitTree: p.TargetTree},
		Outcome: result.Outcome, VerdictSha256: digest, Consumer: VerdictConsumer,
		Runtime:    VerdictRuntime{GoVersion: result.Runtime.GoVersion},
		RecordedAt: verdictNow().UTC().Format(time.RFC3339),
	}, nil
}

// planAppend decides, before anything is written, what appending record to
// the current ledger does: nil body when an identical verdict is already
// recorded for the key (idempotent), the extended note body otherwise. A
// ledger that does not verify is never extended, and a different verdict for
// the same key is refused (CEM-V1-275).
func planAppend(current ledger, record VerdictRecord) ([]byte, *VerdictRecord, error) {
	switch current.state {
	case "absent", "verified":
	default:
		return nil, nil, cemcode.New(CodeVerdictMalformed, "the %s note of %s reads %s; it is never extended",
			VerdictRef, record.Target.GitCommit, current.state).
			Guided("inspect it with git notes --ref=" + VerdictRef + " show " + record.Target.GitCommit + "; remove it deliberately before recording again")
	}
	for index, existing := range current.records {
		if existing.MapSha256 != record.MapSha256 {
			continue
		}
		if existing.VerdictSha256 == record.VerdictSha256 {
			return nil, &current.records[index], nil
		}
		return nil, nil, cemcode.New(CodeVerdictConflict, "map %s already has verdictSha256 %s on %s; this verification reached %s",
			record.MapSha256, existing.VerdictSha256, record.Target.GitCommit, record.VerdictSha256).
			Guided("the recorded verdict is never replaced; find why the verification differs (artifacts, verifier release) before removing it deliberately")
	}
	if len(current.records) >= MaxVerdictRecords {
		return nil, nil, cemcode.New(CodeVerdictFull, "the %s note of %s already holds %d records", VerdictRef, record.Target.GitCommit, MaxVerdictRecords)
	}
	if count := len(current.lines); count > 0 {
		previous := lineDigest(current.lines[count-1])
		record.PreviousRecordSha256 = &previous
	}
	line, err := json.Marshal(record)
	if err != nil {
		return nil, nil, cemcode.New("output-failed", "cannot encode verdict record")
	}
	var body []byte
	for _, existing := range current.lines {
		body = append(append(body, existing...), '\n')
	}
	body = append(append(body, line...), '\n')
	return body, &record, nil
}

// verdictsMember is the read-only listing of target's verdict ledger that
// `cem provenance --verdicts` adds (CEM-V1-277): records only when the note
// verifies, never records from a note that does not.
func (r *repository) verdictsMember(ctx context.Context, target string, pinned bool, notesCommit string) (map[string]any, error) {
	current, err := r.readLedgerAt(ctx, target, pinned, notesCommit)
	if err != nil {
		return nil, err
	}
	records := []VerdictRecord{}
	if current.state == "verified" {
		records = current.records
	}
	return map[string]any{
		"ref": VerdictRef, "trust": Trust, "authority": Authority,
		"state": current.state, "records": records,
	}, nil
}
