package gitnotes

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Pointer is the one note body Anchor writes: the committed map's blob, its
// digest, and the commit that holds it, plus the scope the map describes: its
// baseRevision, its patchSha256 and the anchored commit's tree (CEM-V1-067).
// A legacy `/0` pointer has no scope members.
type Pointer struct {
	Schema      string `json:"schema"`
	CEMCommit   string `json:"cem_commit"`
	MapPath     string `json:"map_path"`
	MapBlob     string `json:"map_blob"`
	MapSha256   string `json:"map_sha256"`
	MapSpec     string `json:"map_spec"`
	Base        string `json:"base,omitempty"`
	PatchSha256 string `json:"patch_sha256,omitempty"`
	TargetTree  string `json:"target_tree,omitempty"`
}

func (p Pointer) fields() map[string]any {
	fields := map[string]any{
		"schema": p.Schema, "cem_commit": p.CEMCommit, "map_path": p.MapPath,
		"map_blob": p.MapBlob, "map_sha256": p.MapSha256, "map_spec": p.MapSpec,
	}
	if p.Schema == PointerSchema {
		fields["base"], fields["patch_sha256"], fields["target_tree"] = p.Base, p.PatchSha256, p.TargetTree
	}
	return fields
}

// Anchor records a pointer to the CEM committed at HEAD under mapPath as the
// Corvint note of commit (FPK-V0-037). It refuses an untracked, dirty, or
// uncommitted map, a map blob absent from the object database, and a map
// that does not describe commit (CEM-V1-067: the canonical base..commit patch
// must hash to the map's patchSha256), never replaces a different note, and
// reads the note back before reporting.
func Anchor(ctx context.Context, root, mapPath, commit string) (map[string]any, error) {
	return AnchorWithVerdict(ctx, root, mapPath, commit, "", false)
}

// AnchorWithVerdict is Anchor that, when record is set, also runs the native
// stable verifier over the committed map against commit with the absolute
// artifact root artifacts, and appends its verdictSha256 to commit's verdict
// ledger (CEM-V1-273..275). Every refusal, of the pointer or of the verdict,
// is decided before any note is written.
func AnchorWithVerdict(ctx context.Context, root, mapPath, commit, artifacts string, record bool) (map[string]any, error) {
	repo, err := open(root)
	if err != nil {
		return nil, err
	}
	if err := repo.requireCleanMap(ctx, mapPath); err != nil {
		return nil, err
	}
	head, err := repo.resolveCommit(ctx, "HEAD")
	if err != nil {
		return nil, err
	}
	target, err := repo.resolveCommit(ctx, commit)
	if err != nil {
		return nil, err
	}
	pointer, raw, err := repo.committedPointer(ctx, head, mapPath)
	if err != nil {
		return nil, err
	}
	if pointer.TargetTree, err = repo.tree(ctx, target); err != nil {
		return nil, err
	}
	matches, err := repo.describes(ctx, pointer, target)
	if err != nil {
		return nil, err
	}
	if !matches {
		return nil, cemcode.New(CodeScopeMismatch, "map %s does not describe commit %s: the canonical %s..%s patch does not hash to its patchSha256",
			mapPath, target, pointer.Base, target).Guided("anchor the map onto the commit it was built for")
	}
	// Both tips are read before the notes they hold: the joint publish below
	// is a compare-and-swap against exactly these values (CEM-V1-275).
	pointerTip, err := repo.refTip(ctx, Ref)
	if err != nil {
		return nil, err
	}
	ledgerTip, err := repo.refTip(ctx, VerdictRef)
	if err != nil {
		return nil, err
	}
	var ledgerBody []byte
	var recorded *VerdictRecord
	if record {
		candidate, err := repo.verdictRecord(ctx, raw, pointer, target, artifacts)
		if err != nil {
			return nil, err
		}
		current, err := repo.readLedger(ctx, target)
		if err != nil {
			return nil, err
		}
		if ledgerBody, recorded, err = planAppend(current, candidate); err != nil {
			return nil, err
		}
	}
	body, err := json.Marshal(pointer)
	if err != nil {
		return nil, cemcode.New("output-failed", "cannot encode anchor pointer")
	}
	pointerBlob, written, err := repo.planNote(ctx, target, body)
	if err != nil {
		return nil, err
	}
	if err := repo.publishNotes(ctx, target, record, []noteUpdate{
		{ref: Ref, old: pointerTip, blob: pointerBlob, write: written},
		{ref: VerdictRef, old: ledgerTip, body: ledgerBody, write: ledgerBody != nil, check: record},
	}); err != nil {
		return nil, err
	}
	readBack, err := repo.anchorRow(ctx, target)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"ok": readBack["state"] == "verified", "mutates": true, "tool": "cem-anchor",
		"ref": Ref, "commit": target, "written": written, "pointer": pointer.fields(),
		"verification": readBack["state"],
	}
	if record {
		verdict := map[string]any{"ref": VerdictRef, "written": ledgerBody != nil, "record": *recorded, "state": "verified"}
		if ledgerBody != nil {
			after, err := repo.readLedger(ctx, target)
			if err != nil {
				return nil, err
			}
			verdict["state"] = after.state
			appended, _ := json.Marshal(*recorded)
			if after.state != "verified" || len(after.lines) == 0 || !bytes.Equal(after.lines[len(after.lines)-1], appended) {
				result["ok"] = false
			}
		}
		result["verdict"] = verdict
	}
	return result, nil
}

// requireCleanMap refuses an untracked map and any staged, unstaged, or
// deleted change to it: the pointer must name what HEAD holds.
func (r *repository) requireCleanMap(ctx context.Context, mapPath string) error {
	out, err := r.git(ctx, 64<<10, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all",
		"--ignore-submodules=none", "--", mapPath)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(out, []byte("?? ")) {
		return cemcode.New(CodeMapUncommitted, "map %s is untracked; commit it before anchoring", mapPath)
	}
	if len(out) != 0 {
		return cemcode.New(CodeMapDirty, "map %s differs from HEAD; commit it before anchoring", mapPath)
	}
	return nil
}

// committedPointer reads the map blob HEAD holds, requires it in the object
// database and a valid CEM, digests the exact bytes, and copies the map's
// baseRevision and patchSha256 into the pointer. Only a cem/1.0 map is read,
// by the stable parser (CEM-V1-065); a retired cem/0.x map refuses with
// unsupported-spec and the migration fix line (CEM-V1-210), and no note is
// written.
func (r *repository) committedPointer(ctx context.Context, head, mapPath string) (Pointer, []byte, error) {
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", head+":"+mapPath)
	if err != nil {
		return Pointer{}, nil, cemcode.New(CodeMapUncommitted, "map %s is not committed at HEAD", mapPath)
	}
	blob := strings.TrimSpace(string(out))
	data, err := r.blob(ctx, blob, wire.MaxMapBytes)
	if err != nil {
		return Pointer{}, nil, cemcode.New(CodeBlobUnavailable, "map blob %s is not in the object database", blob)
	}
	parsed, err := wire.ParseStable(data)
	if err != nil {
		return Pointer{}, nil, err
	}
	digest := sha256.Sum256(data)
	return Pointer{
		Schema: PointerSchema, CEMCommit: head, MapPath: mapPath, MapBlob: blob,
		MapSha256: hex.EncodeToString(digest[:]), MapSpec: parsed.Change.Spec,
		Base: parsed.Change.BaseRevision, PatchSha256: parsed.Change.PatchSha256,
	}, data, nil
}

// tree returns the tree object ID of commit.
func (r *repository) tree(ctx context.Context, commit string) (string, error) {
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", commit+"^{tree}")
	if err != nil {
		return "", cemcode.New(cemcode.RepositoryObjectUnavailable, "commit %s has no readable tree", commit)
	}
	return strings.TrimSpace(string(out)), nil
}

// describes reports whether the pointer's scope is the change commit carries
// relative to the pointer's base: the canonical CEM patch of base..commit,
// derived by the one recipe every CEM consumer uses, must hash to the
// pointer's patch_sha256, and commit's tree must be target_tree. A patch that
// cannot be derived (an absent base, a budget or Git failure) is an error,
// never a match.
func (r *repository) describes(ctx context.Context, p Pointer, commit string) (bool, error) {
	tree, err := r.tree(ctx, commit)
	if err != nil {
		return false, err
	}
	if tree != p.TargetTree {
		return false, nil
	}
	patchBytes, err := r.cem.CanonicalDiff(ctx, p.Base, commit)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(patchBytes)
	return hex.EncodeToString(digest[:]) == p.PatchSha256, nil
}

// planNote stores body byte-exact as a blob, so Git's message cleanup never
// rewrites it, and decides whether target's pointer note needs writing. An
// identical note is a no-op; a different one is refused, never overwritten.
func (r *repository) planNote(ctx context.Context, target string, body []byte) (string, bool, error) {
	out, err := r.git(ctx, 256, body, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", false, err
	}
	blob := strings.TrimSpace(string(out))
	existing, err := r.noteBlob(ctx, Ref, target)
	if err != nil {
		return "", false, err
	}
	if existing == blob {
		return blob, false, nil
	}
	if existing != "" {
		return "", false, cemcode.New(CodeNoteConflict, "commit %s already carries a different %s note", target, Ref).
			Guided("inspect it with git notes --ref=" + Ref + " show " + target + "; remove it deliberately before re-anchoring")
	}
	return blob, true, nil
}

// beforePublish runs between the last validation and the joint publish of
// the notes refs; tests set it to interleave a second writer
// deterministically. Like verdictNow it is package state, so no test that
// sets it runs in parallel.
var beforePublish = func() {}

// noteUpdate is one notes ref of the joint publish: the tip its note was
// validated against ("" when the ref was absent) and, when write is set, the
// note blob (or body to store) to attach to the target. check verifies an
// unwritten ref still holds its validated tip.
type noteUpdate struct {
	ref, old, blob string
	body           []byte
	write, check   bool
}

// publishNotes builds each written ref's new notes commit on a private
// staging ref seeded with the validated tip, then publishes every update in
// one `git update-ref --stdin` transaction whose old values are the validated
// tips (the zero object ID for an absent ref). Either every ref moves or none
// does: a ref that moved since validation, a lock, or any other failure
// leaves both refs as they were. A lost race is refused after re-reading the
// tips: verdict-ledger-conflict when the ledger moved, anchor-note-conflict
// when only the pointer did (CEM-V1-275).
func (r *repository) publishNotes(ctx context.Context, target string, record bool, updates []noteUpdate) error {
	var script strings.Builder
	script.WriteString("start\n")
	staged := false
	zero := ""
	for index := range updates {
		update := &updates[index]
		if !update.write {
			continue
		}
		if update.blob == "" {
			out, err := r.git(ctx, 256, update.body, "hash-object", "-w", "--stdin")
			if err != nil {
				return err
			}
			update.blob = strings.TrimSpace(string(out))
		}
		commit, err := r.stageNote(ctx, update.old, update.blob, target)
		if err != nil {
			return err
		}
		zero = strings.Repeat("0", len(commit))
		update.blob = commit
		staged = true
	}
	if !staged {
		return nil
	}
	for _, update := range updates {
		old := update.old
		if old == "" {
			old = zero
		}
		switch {
		case update.write:
			fmt.Fprintf(&script, "update %s %s %s\n", update.ref, update.blob, old)
		case update.check:
			fmt.Fprintf(&script, "verify %s %s\n", update.ref, old)
		}
	}
	script.WriteString("prepare\ncommit\n")
	beforePublish()
	_, publishErr := r.git(ctx, 64<<10, []byte(script.String()), "update-ref", "--stdin")
	if publishErr == nil {
		return nil
	}
	moved := ""
	for _, update := range updates {
		if !update.write && !update.check {
			continue
		}
		now, err := r.refTip(ctx, update.ref)
		if err != nil {
			return err
		}
		if now == update.old {
			continue
		}
		if record && update.ref == VerdictRef {
			return cemcode.New(CodeVerdictConflict, "the %s ref moved after the ledger of %s was validated; nothing was written", VerdictRef, target).
				Guided("another writer recorded first; rerun cem anchor --artifacts to read the ledger it left")
		}
		moved = update.ref
	}
	if moved != "" {
		return cemcode.New(CodeNoteConflict, "the %s ref moved after the note of %s was validated; nothing was written", moved, target).
			Guided("another writer anchored first; rerun cem anchor to read the note it left")
	}
	return publishErr
}

// stageNote returns the notes commit that attaches blob to target on top of
// the validated tip old ("" for an absent ref), built by `git notes` on a
// private staging ref under refs/notes/ that it removes before returning. No
// public ref moves.
func (r *repository) stageNote(ctx context.Context, old, blob, target string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", cemcode.New("output-failed", "cannot name a staging ref")
	}
	staging := stagingRefPrefix + hex.EncodeToString(nonce)
	defer func() { _, _ = r.git(context.WithoutCancel(ctx), 64<<10, nil, "update-ref", "-d", staging) }()
	if old != "" {
		if _, err := r.git(ctx, 64<<10, nil, "update-ref", staging, old); err != nil {
			return "", err
		}
	}
	args := append(append([]string{}, committer...), "notes", "--ref="+staging, "add", "-f", "-C", blob, target)
	if _, err := r.git(ctx, 64<<10, nil, args...); err != nil {
		return "", err
	}
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", staging)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// stagingRefPrefix holds the transient refs publishNotes builds notes commits
// on; each lives only for the duration of one stageNote call.
const stagingRefPrefix = "refs/notes/corvint-staging/"

// refTip is ref's current object ID, "" when the ref is absent.
func (r *repository) refTip(ctx context.Context, ref string) (string, error) {
	out, err := r.git(ctx, 4<<10, nil, "for-each-ref", "--format=%(objectname)", ref)
	if err != nil {
		return "", err
	}
	tip := strings.TrimSpace(string(out))
	if strings.Contains(tip, "\n") {
		return "", cemcode.New(cemcode.RepositoryObjectUnavailable, "ref %s does not name one object", ref)
	}
	return tip, nil
}
