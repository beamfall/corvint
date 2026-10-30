package gitnotes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// anchorState is the one anchor row's state for commit, "" when there is none.
func anchorState(t *testing.T, root, commit string) string {
	t.Helper()
	read, err := Provenance(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range read["evidence"].([]map[string]any) {
		if row["kind"] == KindAnchor {
			return row["state"].(string)
		}
	}
	return ""
}

// attach writes body as commit's Corvint note by hand, replacing any note.
func attach(t *testing.T, root, commit string, body []byte) {
	t.Helper()
	fixtureGit(t, root, body, "notes", "--ref="+Ref, "add", "-f", "-F", "-", commit)
}

// storeState extends refState with what refState cannot see: the object
// count and the index and config bytes.
func storeState(t *testing.T, root string) string {
	t.Helper()
	state := refState(t, root) + fixtureGit(t, root, nil, "count-objects", "-v")
	for _, name := range []string{"index", "config"} {
		data, err := os.ReadFile(filepath.Join(root, ".git", name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		state += name + hex.EncodeToString(digest[:])
	}
	return state
}

func canonicalPatchSha256(t *testing.T, root, base, target string) string {
	t.Helper()
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	patch, err := repo.CanonicalDiff(context.Background(), base, target)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(patch)
	return hex.EncodeToString(digest[:])
}

// CEM-V1-067 (a): anchoring map(A) onto a sibling commit with the same parent
// is refused before any note is written, and a scope-bound pointer attached
// to the sibling by hand reads `scope-mismatch`, whichever scope member the
// forger aligns with the sibling.
func TestCEMV1067SiblingCommitIsScopeMismatch(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	if _, err := Anchor(ctx, root, mapPath, "HEAD"); err != nil {
		t.Fatal(err)
	}
	var pointer Pointer
	if err := json.Unmarshal([]byte(fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head)), &pointer); err != nil {
		t.Fatal(err)
	}
	// The sibling shares HEAD's parent and changes the same file differently.
	fixtureGit(t, root, nil, "checkout", "-q", "--detach", "HEAD~1")
	writeFile(t, root, "src/app.txt", "gamma\n")
	fixtureGit(t, root, nil, "commit", "-qam", "sibling")
	sibling := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	fixtureGit(t, root, nil, "checkout", "-q", "main")
	if fixtureGit(t, root, nil, "rev-parse", sibling+"~1") != fixtureGit(t, root, nil, "rev-parse", head+"~1") {
		t.Fatal("fixture: the sibling does not share HEAD's parent")
	}

	notesBefore := fixtureGit(t, root, nil, "for-each-ref", Ref)
	_, err := Anchor(ctx, root, mapPath, sibling)
	if cemcode.CodeOf(err) != CodeScopeMismatch {
		t.Fatalf("anchor onto the sibling: code %q (%v)", cemcode.CodeOf(err), err)
	}
	if fixtureGit(t, root, nil, "for-each-ref", Ref) != notesBefore {
		t.Fatal("a scope-mismatch refusal moved the notes ref")
	}

	siblingTree := fixtureGit(t, root, nil, "rev-parse", sibling+"^{tree}")
	siblingPatch := canonicalPatchSha256(t, root, pointer.Base, sibling)
	cases := map[string]func(p *Pointer){
		"copied unchanged":           func(*Pointer) {},
		"target tree aligned":        func(p *Pointer) { p.TargetTree = siblingTree },
		"patch and tree aligned":     func(p *Pointer) { p.TargetTree, p.PatchSha256 = siblingTree, siblingPatch },
		"base aligned to the parent": func(p *Pointer) { p.Base = fixtureGit(t, root, nil, "rev-parse", sibling+"~1") },
	}
	for name, forge := range cases {
		forged := pointer
		forge(&forged)
		body, err := json.Marshal(forged)
		if err != nil {
			t.Fatal(err)
		}
		attach(t, root, sibling, body)
		before := refState(t, root)
		if state := anchorState(t, root, sibling); state != "scope-mismatch" {
			t.Fatalf("%s: state %q, want scope-mismatch", name, state)
		}
		if refState(t, root) != before {
			t.Fatalf("%s: provenance moved a ref", name)
		}
	}
	// The pointer is still verified on the commit its map describes.
	if state := anchorState(t, root, head); state != "verified" {
		t.Fatalf("HEAD: %q", state)
	}
}

// CEM-V1-067 (b): `verified` implies the anchored map's patchSha256 is the
// canonical base..REV patch digest and REV's tree is target_tree. The map at
// HEAD also describes the change commit below it, because the canonical patch
// excludes the sidecar path; it describes neither the base nor the root.
func TestCEMV1067VerifiedImpliesTheAnchoredPatchIsBaseToREV(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	parsed, err := wire.ParseStable(committedMap(t, root))
	if err != nil {
		t.Fatal(err)
	}
	base := parsed.Change.BaseRevision
	for _, rev := range []string{"HEAD", "HEAD~1", "HEAD~2"} {
		commit := fixtureGit(t, root, nil, "rev-parse", rev)
		describes := canonicalPatchSha256(t, root, base, commit) == parsed.Change.PatchSha256
		receipt, err := Anchor(ctx, root, mapPath, commit)
		if !describes {
			if cemcode.CodeOf(err) != CodeScopeMismatch {
				t.Fatalf("%s: code %q (%v)", rev, cemcode.CodeOf(err), err)
			}
			continue
		}
		if err != nil || receipt["verification"] != "verified" {
			t.Fatalf("%s: %v %v", rev, receipt, err)
		}
		row := provenanceRow(t, root, commit)
		pointer := row["pointer"].(map[string]any)
		if row["state"] != "verified" || pointer["schema"] != PointerSchema || pointer["base"] != base ||
			pointer["patch_sha256"] != canonicalPatchSha256(t, root, base, commit) ||
			pointer["target_tree"] != fixtureGit(t, root, nil, "rev-parse", commit+"^{tree}") {
			t.Fatalf("%s: %v", rev, row)
		}
	}
	if anchorState(t, root, "HEAD~2") != "" || anchorState(t, root, "HEAD~1") != "verified" {
		t.Fatal("the base must carry no anchor and the change commit a verified one")
	}
}

func provenanceRow(t *testing.T, root, commit string) map[string]any {
	t.Helper()
	read, err := Provenance(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	return read["evidence"].([]map[string]any)[0]
}

// CEM-V1-067 (c): a pinned notes tip that is not an ancestor of the live
// refs/notes/corvint reads `rollback`, never an absent anchor; without the
// pin the read is unchanged; nothing is written.
func TestCEMV1067ResetNotesRefIsRollback(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	change := fixtureGit(t, root, nil, "rev-parse", "HEAD~1")
	if _, err := Anchor(ctx, root, mapPath, "HEAD"); err != nil {
		t.Fatal(err)
	}
	first := fixtureGit(t, root, nil, "rev-parse", Ref)
	if _, err := Anchor(ctx, root, mapPath, change); err != nil {
		t.Fatal(err)
	}
	second := fixtureGit(t, root, nil, "rev-parse", Ref)
	read := func(tip string) (map[string]any, string) {
		t.Helper()
		before := storeState(t, root)
		result, err := ProvenanceWithNotesTip(ctx, root, change, tip)
		if err != nil {
			t.Fatal(err)
		}
		if storeState(t, root) != before || result["mutates"] != false {
			t.Fatal("provenance moved a ref, an object, the index, the config or the worktree")
		}
		state := ""
		for _, row := range result["evidence"].([]map[string]any) {
			if row["kind"] == KindAnchor {
				state = row["state"].(string)
			}
		}
		return result, state
	}
	for _, tip := range []string{second, first} {
		result, state := read(tip)
		notes := result["notes_tip"].(map[string]any)
		if state != "verified" || notes["state"] != "current" || notes["live"] != second || notes["pinned"] != tip {
			t.Fatalf("live tip descends from %s: %v", tip, result)
		}
	}
	if result, _ := read(""); result["notes_tip"] != nil {
		t.Fatalf("no pin, no notes_tip member: %v", result)
	}

	// Reset the notes ref to its parent: the change commit's note is gone.
	fixtureGit(t, root, nil, "update-ref", Ref, first)
	if result, state := read(""); state != "" || result["notes_tip"] != nil {
		t.Fatalf("unpinned read after reset: %v", result)
	}
	result, state := read(second)
	notes := result["notes_tip"].(map[string]any)
	if state != "rollback" || notes["state"] != "rollback" || notes["live"] != first || notes["pinned"] != second {
		t.Fatalf("pinned read after reset: %v", result)
	}
	// A deleted ref is a rollback too.
	fixtureGit(t, root, nil, "update-ref", "-d", Ref)
	result, state = read(second)
	if notes := result["notes_tip"].(map[string]any); state != "rollback" || notes["state"] != "rollback" || notes["live"] != nil {
		t.Fatalf("pinned read after delete: %v", result)
	}
	if _, err := ProvenanceWithNotesTip(ctx, root, change, strings.Repeat("0", 40)); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("unresolvable tip: %v", err)
	}
	// A revision name would resolve to the live tip it is meant to check.
	for _, tip := range []string{Ref, "HEAD", second[:12], strings.ToUpper(second)} {
		if _, err := ProvenanceWithNotesTip(ctx, root, change, tip); cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Fatalf("tip %q: %v", tip, err)
		}
	}
}

// CEM-V1-067 (d): a legacy `/0` pointer stays readable and is `unscoped`,
// never `verified`; mixed shapes are malformed.
func TestCEMV1067LegacyPointerIsReadableAndUnscoped(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	if _, err := Anchor(context.Background(), root, mapPath, "HEAD"); err != nil {
		t.Fatal(err)
	}
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	var scoped Pointer
	if err := json.Unmarshal([]byte(fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head)), &scoped); err != nil {
		t.Fatal(err)
	}
	legacy := `{"schema":"corvint-cem-anchor/0","cem_commit":"` + scoped.CEMCommit + `","map_path":"` + scoped.MapPath +
		`","map_blob":"` + scoped.MapBlob + `","map_sha256":"` + scoped.MapSha256 + `","map_spec":"cem/1.0"}`
	attach(t, root, head, []byte(legacy))
	row := provenanceRow(t, root, head)
	pointer := row["pointer"].(map[string]any)
	if row["state"] != "unscoped" || pointer["schema"] != LegacyPointerSchema || pointer["map_blob"] != scoped.MapBlob {
		t.Fatalf("legacy row: %v", row)
	}
	if _, scope := pointer["patch_sha256"]; scope {
		t.Fatalf("legacy row grew scope members: %v", pointer)
	}
	// A legacy pointer to changed bytes is still reported, never trusted.
	attach(t, root, head, []byte(strings.Replace(legacy, scoped.MapSha256, strings.Repeat("0", 64), 1)))
	if state := anchorState(t, root, head); state != "digest-mismatch" {
		t.Fatalf("legacy forged digest: %q", state)
	}
	unscopedV1 := strings.Replace(legacy, LegacyPointerSchema, PointerSchema, 1)
	scoped0 := func(member string) string { return strings.Replace(legacy, `}`, `,`+member+`}`, 1) }
	scopedV1, err := json.Marshal(scoped)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"/1 without scope": unscopedV1, "/0 with scope": scoped0(`"base":"` + scoped.Base + `"`),
		"/0 with an empty scope member": scoped0(`"patch_sha256":""`), "/0 with a null case-variant member": scoped0(`"TARGET_TREE":null`),
		"/0 with a trailing brace": legacy + "}", "/1 with a trailing brace": string(scopedV1) + "}",
		"/1 followed by a second object": string(scopedV1) + string(scopedV1),
	} {
		attach(t, root, head, []byte(body))
		if state := anchorState(t, root, head); state != "malformed" {
			t.Fatalf("%s: %q", name, state)
		}
	}
}
