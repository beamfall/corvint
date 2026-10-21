package gitnotes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Provenance reads, without writing anything, every Git-native provenance row
// of one commit (FPK-V0-038, FPK-V0-039): the Corvint anchor note with its
// digest and scope re-verified, the Git AI authorship note, and the
// Assisted-by and Agent-Logs-Url trailers. An absent source yields no row.
func Provenance(ctx context.Context, root, commit string) (map[string]any, error) {
	return ProvenanceWithNotesTip(ctx, root, commit, "")
}

// ProvenanceWithNotesTip is Provenance with an optional pinned notes tip, an
// earlier refs/notes/corvint commit the caller recorded (CEM-V1-067). When
// notesTip is not an ancestor of, or equal to, the live tip, or the live ref
// is gone, the notes ref was rolled back: the result carries `notes_tip` with
// state `rollback` and the anchor row is a `rollback` row whatever the live
// note holds, so a reset ref never reads as an absent anchor. An empty
// notesTip reads exactly as Provenance.
func ProvenanceWithNotesTip(ctx context.Context, root, commit, notesTip string) (map[string]any, error) {
	return ProvenanceWithVerdicts(ctx, root, commit, notesTip, false)
}

// ProvenanceWithVerdicts is ProvenanceWithNotesTip that, when verdicts is
// set, adds the read-only `verdicts` listing of commit's verdict ledger
// (CEM-V1-277). It writes nothing either way.
func ProvenanceWithVerdicts(ctx context.Context, root, commit, notesTip string, verdicts bool) (map[string]any, error) {
	return ProvenanceWithVerdictsTip(ctx, root, commit, notesTip, verdicts, "")
}

// ProvenanceWithVerdictsTip is ProvenanceWithVerdicts with an optional pinned
// verdict ledger tip, an earlier refs/notes/corvint-verdicts commit the
// caller recorded (CEM-V1-345). It needs verdicts. When verdictsTip is not an
// ancestor of, or equal to, the live ledger tip, or the live ref is gone, the
// ledger was rewound: the `verdicts` member carries `tip` with state
// `rollback`, its own state is `rollback` and it lists no records, whatever
// the live note holds. A ref that moves during the read is refused.
func ProvenanceWithVerdictsTip(ctx context.Context, root, commit, notesTip string, verdicts bool, verdictsTip string) (map[string]any, error) {
	if verdictsTip != "" && !verdicts {
		return nil, cemcode.New(cemcode.InvalidArguments, "argument --verdicts-tip: requires --verdicts")
	}
	repo, err := open(root)
	if err != nil {
		return nil, err
	}
	target, err := repo.resolveCommit(ctx, commit)
	if err != nil {
		return nil, err
	}
	var tip, ledgerTip map[string]any
	if notesTip != "" {
		if tip, err = repo.refTipState(ctx, Ref, "--notes-tip", notesTip); err != nil {
			return nil, err
		}
	}
	if verdictsTip != "" {
		if ledgerTip, err = repo.refTipState(ctx, VerdictRef, "--verdicts-tip", verdictsTip); err != nil {
			return nil, err
		}
	}
	rows := []map[string]any{}
	// With --notes-tip the anchor note is read from the live commit the tip
	// state captured, by object ID, so a ref that moves away and back during
	// the read cannot supply other content (CEM-V1-067, V1-2157).
	anchorRead("before")
	var anchor map[string]any
	if tip != nil {
		notesCommit, _ := tip["live"].(string)
		anchor, err = repo.anchorRowAt(ctx, target, true, notesCommit)
	} else {
		anchor, err = repo.anchorRow(ctx, target)
	}
	anchorRead("after")
	if err != nil {
		return nil, err
	}
	if tip != nil && tip["state"] == "rollback" {
		anchor = historyRow(KindAnchor, Ref, "rollback")
	}
	if anchor != nil {
		rows = append(rows, anchor)
	}
	foreign, err := repo.aiNoteRow(ctx, target)
	if err != nil {
		return nil, err
	}
	if foreign != nil {
		rows = append(rows, foreign)
	}
	trailers, err := repo.trailerRows(ctx, target)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"ok": true, "mutates": false, "tool": "cem-provenance", "commit": target,
		"evidence": append(rows, trailers...),
	}
	if verdicts {
		// With --verdicts-tip the ledger is read from the live commit the
		// tip state captured, by object ID, so a ref that moves away and
		// back during the read cannot supply other content (V1-2024).
		notesCommit := ""
		if ledgerTip != nil {
			notesCommit, _ = ledgerTip["live"].(string)
		}
		verdictsRead("before")
		member, err := repo.verdictsMember(ctx, target, ledgerTip != nil, notesCommit)
		verdictsRead("after")
		if err != nil {
			return nil, err
		}
		if ledgerTip != nil {
			if ledgerTip["state"] == "rollback" {
				member["state"], member["records"] = "rollback", []VerdictRecord{}
			}
			member["tip"] = ledgerTip
		}
		result["verdicts"] = member
	}
	// The rows must come from the tips the rollback states were computed
	// against; a ref that moved during the read is refused, not reported.
	if err := repo.requireUnmoved(ctx, Ref, tip); err != nil {
		return nil, err
	}
	if err := repo.requireUnmoved(ctx, VerdictRef, ledgerTip); err != nil {
		return nil, err
	}
	if tip != nil {
		result["notes_tip"] = tip
	}
	return result, nil
}

// anchorRead is a test seam around the anchor note read of a provenance
// listing; it does nothing in production.
var anchorRead = func(stage string) {}

// verdictsRead is a test seam around the verdict ledger read of a
// provenance listing; it does nothing in production.
var verdictsRead = func(stage string) {}

// requireUnmoved refuses a read during which ref left the live tip that
// state, a refTipState result, recorded. A nil state checks nothing.
func (r *repository) requireUnmoved(ctx context.Context, ref string, state map[string]any) error {
	if state == nil {
		return nil
	}
	live, err := r.liveRefTip(ctx, ref)
	if err != nil {
		return err
	}
	if (state["live"] == nil && live != "") || (state["live"] != nil && state["live"] != live) {
		return cemcode.New(CodeNotesMoved, "%s moved during the read; read provenance again", ref)
	}
	return nil
}

// refTipState compares a pinned commit of a notes ref (refs/notes/corvint
// for --notes-tip, refs/notes/corvint-verdicts for --verdicts-tip) with the
// live tip: `current` when the pinned commit is the live tip or one of its
// ancestors, `rollback` otherwise, including when the live ref is absent. A
// pinned tip that does not resolve to a commit is an error, never a state.
func (r *repository) refTipState(ctx context.Context, ref, flag, pinnedTip string) (map[string]any, error) {
	// A revision name would resolve to the live tip it is meant to check.
	if !objectID(pinnedTip) {
		return nil, cemcode.New(cemcode.InvalidArguments, "argument %s: must be a full lowercase hexadecimal commit object ID", flag)
	}
	pinned, err := r.resolveCommit(ctx, pinnedTip)
	if err != nil {
		return nil, err
	}
	live, err := r.liveRefTip(ctx, ref)
	if err != nil {
		return nil, err
	}
	state := map[string]any{"ref": ref, "pinned": pinned, "live": nil, "state": "rollback"}
	if live == "" {
		return state, nil
	}
	state["live"] = live
	// Empty output: every commit reachable from pinned is reachable from live.
	out, err := r.git(ctx, 256, nil, "rev-list", "--max-count=1", "--end-of-options", pinned, "^"+live, "--")
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		state["state"] = "current"
	}
	return state, nil
}

// liveRefTip is the object ref names, "" when it is absent. for-each-ref
// also matches refs below ref; only the exact name is the tip.
func (r *repository) liveRefTip(ctx context.Context, ref string) (string, error) {
	out, err := r.git(ctx, MaxListBytes, nil, "for-each-ref", "--format=%(refname) %(objectname)", ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if name, oid, found := strings.Cut(line, " "); found && name == ref {
			return oid, nil
		}
	}
	return "", nil
}

func historyRow(kind, source, state string) map[string]any {
	return map[string]any{"kind": kind, "trust": Trust, "authority": Authority, "source": source, "state": state}
}

// anchorRow re-verifies the Corvint pointer attached to target: the named
// commit must hold the named blob at the named path, the blob's bytes must
// hash to the digest, and, for a `/1` pointer, the map must describe target
// (pointerState).
func (r *repository) anchorRow(ctx context.Context, target string) (map[string]any, error) {
	return r.anchorRowAt(ctx, target, false, "")
}

// anchorRowAt is anchorRow over a fixed notes commit when pinned is true: the
// note is found in that commit's tree by object ID, never through the mutable
// ref, and an empty notesCommit (absent ref) reads no row.
func (r *repository) anchorRowAt(ctx context.Context, target string, pinned bool, notesCommit string) (map[string]any, error) {
	var noteOid string
	var err error
	switch {
	case !pinned:
		noteOid, err = r.noteBlob(ctx, Ref, target)
	case notesCommit != "":
		noteOid, err = r.noteBlobAt(ctx, notesCommit, target)
	}
	if err != nil || noteOid == "" {
		return nil, err
	}
	row := historyRow(KindAnchor, Ref, "malformed")
	body, err := r.blob(ctx, noteOid, MaxNoteBytes)
	if err != nil {
		return nil, err
	}
	var pointer Pointer
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&pointer) != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || !wellFormed(pointer) ||
		(pointer.Schema == LegacyPointerSchema && carriesScope(body)) {
		return row, nil
	}
	row["pointer"] = pointer.fields()
	row["state"] = r.pointerState(ctx, pointer, target)
	return row, nil
}

func wellFormed(p Pointer) bool {
	switch p.Schema {
	case LegacyPointerSchema:
		if p.Base != "" || p.PatchSha256 != "" || p.TargetTree != "" {
			return false
		}
	case PointerSchema:
		if !objectID(p.Base) || !objectID(p.TargetTree) || len(p.PatchSha256) != 64 || !hexadecimal(p.PatchSha256) {
			return false
		}
	default:
		return false
	}
	return objectID(p.CEMCommit) && objectID(p.MapBlob) &&
		len(p.MapSha256) == 64 && hexadecimal(p.MapSha256) && p.MapPath != "" &&
		utf8.ValidString(p.MapPath) && strings.IndexFunc(p.MapPath, unicode.IsControl) < 0 &&
		len(p.MapPath) <= 4096 && len(p.MapSpec) <= MaxTextBytes
}

// carriesScope reports whether a pointer object names any scope member, even
// with an empty or null value; member names match case-insensitively, as the
// decoder matches them.
func carriesScope(body []byte) bool {
	var members map[string]json.RawMessage
	if json.Unmarshal(body, &members) != nil {
		return true
	}
	for name := range members {
		for _, scope := range []string{"base", "patch_sha256", "target_tree"} {
			if strings.EqualFold(name, scope) {
				return true
			}
		}
	}
	return false
}

func objectID(value string) bool { return (len(value) == 40 || len(value) == 64) && hexadecimal(value) }

func hexadecimal(value string) bool {
	return strings.Trim(value, "0123456789abcdef") == ""
}

// pointerState is the closed anchor-row state. `verified` is reported only
// for a `/1` pointer whose map blob is held at cem_commit:map_path, hashes to
// map_sha256, parses as a cem/1.0 map whose baseRevision and patchSha256 are
// the pointer's base and patch_sha256, and describes target: target's tree is
// target_tree and the canonical base..target patch hashes to patch_sha256.
// Any scope failure is `scope-mismatch`; a patch that cannot be derived is
// `scope-unavailable`. A `/0` pointer that passes the blob checks is
// `unscoped`: nothing in it binds the map to target.
func (r *repository) pointerState(ctx context.Context, p Pointer, target string) string {
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", p.CEMCommit+":"+p.MapPath)
	if err != nil || strings.TrimSpace(string(out)) != p.MapBlob {
		return "path-mismatch"
	}
	data, err := r.blob(ctx, p.MapBlob, wire.MaxMapBytes)
	if err != nil {
		return "blob-unavailable"
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != p.MapSha256 {
		return "digest-mismatch"
	}
	if p.Schema == LegacyPointerSchema {
		return "unscoped"
	}
	parsed, err := wire.ParseStable(data)
	if err != nil || parsed.Change.BaseRevision != p.Base || parsed.Change.PatchSha256 != p.PatchSha256 {
		return "scope-mismatch"
	}
	matches, err := r.describes(ctx, p, target)
	if err != nil {
		return "scope-unavailable"
	}
	if !matches {
		return "scope-mismatch"
	}
	return "verified"
}

// aiMetadata is the part of the Git AI authorship/3.x metadata this reader
// surfaces; every other member is ignored.
type aiMetadata struct {
	SchemaVersion string              `json:"schema_version"`
	BaseCommitSha string              `json:"base_commit_sha"`
	Prompts       map[string]aiRecord `json:"prompts"`
	Sessions      map[string]aiRecord `json:"sessions"`
}

type aiRecord struct {
	AgentID struct {
		Tool  string `json:"tool"`
		Model string `json:"model"`
	} `json:"agent_id"`
	HumanAuthor string `json:"human_author"`
	MessagesURL string `json:"messages_url"`
}

// aiNoteRow parses a Git AI authorship log: an attestation section of
// unindented file paths and indented attestations, a `---` line, then JSON
// metadata. Everything it surfaces is bounded untrusted text.
func (r *repository) aiNoteRow(ctx context.Context, target string) (map[string]any, error) {
	noteOid, err := r.noteBlob(ctx, ForeignRef, target)
	if err != nil || noteOid == "" {
		return nil, err
	}
	row := historyRow(KindAINote, ForeignRef, "malformed")
	body, err := r.blob(ctx, noteOid, MaxNoteBytes)
	if err != nil {
		return nil, err
	}
	attestation, metadata, found := strings.Cut("\n"+string(body), "\n---\n")
	var parsed aiMetadata
	if !found || json.Unmarshal([]byte(metadata), &parsed) != nil {
		return row, nil
	}
	text := &boundedText{}
	files := aiFiles(attestation)
	agents := aiAgents(parsed, text)
	row["state"] = "parsed"
	row["files_total"] = len(files)
	row["agents_total"] = len(agents)
	row["untrusted"] = map[string]any{
		"schema_version": text.bound(parsed.SchemaVersion), "base_commit_sha": text.bound(parsed.BaseCommitSha),
		"files": text.boundAll(files), "agents": capped(agents, text),
	}
	row["truncated"] = text.truncated
	return row, nil
}

func aiFiles(attestation string) []string {
	var files []string
	for _, line := range strings.Split(attestation, "\n") {
		if line != "" && !strings.HasPrefix(line, " ") {
			files = append(files, line)
		}
	}
	return files
}

func aiAgents(metadata aiMetadata, text *boundedText) []map[string]any {
	records := map[string]aiRecord{}
	for id, record := range metadata.Prompts {
		records[id] = record
	}
	for id, record := range metadata.Sessions {
		records[id] = record
	}
	ids := make([]string, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	agents := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		record := records[id]
		agents = append(agents, map[string]any{
			"id": text.bound(id), "tool": text.bound(record.AgentID.Tool), "model": text.bound(record.AgentID.Model),
			"human_author": text.bound(record.HumanAuthor), "messages_url": text.bound(record.MessagesURL),
		})
	}
	return agents
}

func capped(agents []map[string]any, text *boundedText) []map[string]any {
	if len(agents) <= MaxListedEntries {
		return agents
	}
	text.truncated = true
	return agents[:MaxListedEntries]
}

// trailerKinds maps a lower-cased trailer key to its row kind; Git compares
// trailer keys case-insensitively.
var trailerKinds = map[string]string{"assisted-by": KindAssistedBy, "agent-logs-url": KindAgentLogsURL}

// trailerRows reads the commit's trailer block with repository trailer
// configuration neutralized and keeps only the two recognized keys.
func (r *repository) trailerRows(ctx context.Context, target string) ([]map[string]any, error) {
	out, err := r.git(ctx, MaxTrailerBytes, nil, "log", "-1", "--no-decorate",
		"--format=%(trailers:only=true,unfold=true)", "--end-of-options", target, "--")
	if err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, found := strings.Cut(line, ":")
		kind, known := trailerKinds[strings.ToLower(strings.TrimSpace(key))]
		if !found || !known || len(rows) == MaxListedEntries {
			continue
		}
		text := &boundedText{}
		row := historyRow(kind, "commit-trailer", "parsed")
		row["untrusted"] = map[string]any{"value": text.bound(strings.TrimSpace(value))}
		row["truncated"] = text.truncated
		rows = append(rows, row)
	}
	return rows, nil
}

// boundedText is the only way foreign text enters a row: invalid UTF-8 is
// replaced, control characters are dropped, each string is cut at
// MaxTextBytes on a rune boundary, and lists at MaxListedEntries.
type boundedText struct{ truncated bool }

func (b *boundedText) bound(value string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(value, "�"))
	if len(clean) <= MaxTextBytes {
		return clean
	}
	b.truncated = true
	cut := MaxTextBytes
	for cut > 0 && !utf8.RuneStart(clean[cut]) {
		cut--
	}
	return clean[:cut]
}

func (b *boundedText) boundAll(values []string) []string {
	if len(values) > MaxListedEntries {
		b.truncated = true
		values = values[:MaxListedEntries]
	}
	bounded := make([]string, 0, len(values))
	for _, value := range values {
		bounded = append(bounded, b.bound(value))
	}
	return bounded
}
