package gitnotes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Pointer is the one note body Anchor writes: the committed map's blob, its
// digest, and the commit that holds it.
type Pointer struct {
	Schema    string `json:"schema"`
	CEMCommit string `json:"cem_commit"`
	MapPath   string `json:"map_path"`
	MapBlob   string `json:"map_blob"`
	MapSha256 string `json:"map_sha256"`
	MapSpec   string `json:"map_spec"`
}

func (p Pointer) fields() map[string]any {
	return map[string]any{
		"schema": p.Schema, "cem_commit": p.CEMCommit, "map_path": p.MapPath,
		"map_blob": p.MapBlob, "map_sha256": p.MapSha256, "map_spec": p.MapSpec,
	}
}

// Anchor records a pointer to the CEM committed at HEAD under mapPath as the
// Corvint note of commit (FPK-V0-037). It refuses an untracked, dirty, or
// uncommitted map and a map blob absent from the object database, never
// replaces a different note, and reads the note back before reporting.
func Anchor(ctx context.Context, root, mapPath, commit string) (map[string]any, error) {
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
	pointer, err := repo.committedPointer(ctx, head, mapPath)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(pointer)
	if err != nil {
		return nil, cemcode.New("output-failed", "cannot encode anchor pointer")
	}
	written, err := repo.writeNote(ctx, target, body)
	if err != nil {
		return nil, err
	}
	readBack, err := repo.anchorRow(ctx, target)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": readBack["state"] == "verified", "mutates": true, "tool": "cem-anchor",
		"ref": Ref, "commit": target, "written": written, "pointer": pointer.fields(),
		"verification": readBack["state"],
	}, nil
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
// database and a valid CEM, and digests the exact bytes.
func (r *repository) committedPointer(ctx context.Context, head, mapPath string) (Pointer, error) {
	out, err := r.git(ctx, 256, nil, "rev-parse", "--verify", "--end-of-options", head+":"+mapPath)
	if err != nil {
		return Pointer{}, cemcode.New(CodeMapUncommitted, "map %s is not committed at HEAD", mapPath)
	}
	blob := strings.TrimSpace(string(out))
	data, err := r.blob(ctx, blob, wire.MaxMapBytes)
	if err != nil {
		return Pointer{}, cemcode.New(CodeBlobUnavailable, "map blob %s is not in the object database", blob)
	}
	parsed, err := wire.ParseMap(data)
	if err != nil {
		return Pointer{}, err
	}
	digest := sha256.Sum256(data)
	return Pointer{
		Schema: PointerSchema, CEMCommit: head, MapPath: mapPath, MapBlob: blob,
		MapSha256: hex.EncodeToString(digest[:]), MapSpec: parsed.Spec,
	}, nil
}

// writeNote stores body byte-exact as a blob and attaches it with `notes add
// -C`, so Git's message cleanup never rewrites it. An identical note is a
// no-op; a different one is refused, never overwritten.
func (r *repository) writeNote(ctx context.Context, target string, body []byte) (bool, error) {
	out, err := r.git(ctx, 256, body, "hash-object", "-w", "--stdin")
	if err != nil {
		return false, err
	}
	blob := strings.TrimSpace(string(out))
	existing, err := r.noteBlob(ctx, Ref, target)
	if err != nil {
		return false, err
	}
	if existing == blob {
		return false, nil
	}
	if existing != "" {
		return false, cemcode.New(CodeNoteConflict, "commit %s already carries a different %s note", target, Ref).
			Guided("inspect it with git notes --ref=" + Ref + " show " + target + "; remove it deliberately before re-anchoring")
	}
	args := append(append([]string{}, committer...), "notes", "--ref="+Ref, "add", "-C", blob, target)
	if _, err := r.git(ctx, 64<<10, nil, args...); err != nil {
		return false, err
	}
	return true, nil
}
