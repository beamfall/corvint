package gitnotes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/contextindex"
)

const fixtureMap = `{"spec":"cem/0.2","baseRevision":"4ca153370afd9bd8c6034ad73acc3925150ab681",` +
	`"patchSha256":"dec61287f7b726144fc19d67f0e07f3c40410c28bc19831a4b0f9fb96487717c",` +
	`"excludedPath":".corvint/change.cem.json","evidence":[],"hunks":[]}`

const mapPath = ".corvint/change.cem.json"

// aiNote is a hand-written Git AI authorship/3.0.0 log (specs/git_ai_standard_v3.0.0.md):
// two attested files, one session and one legacy prompt record, a transcript URL, and
// a control character and an over-long value that must reach the row bounded.
var aiNote = "src/main.rs\n" +
	"  s_c9883b05a2487d::t_9f8e7d6c5b4a32 1-10,15-20\n" +
	"  h_31dce776f88375 42-50\n" +
	"docs/readme.md\n" +
	"  p_0011 1\n" +
	"---\n" +
	`{"schema_version":"authorship/3.0.0","base_commit_sha":"7734793b756b3921c88db5375a8c156e9532447b",` +
	`"git_ai_version":"1.2.3","prompts":{"p_0011":{"agent_id":{"tool":"claude","id":"x","model":"m1"},` +
	`"messages_url":"https://example.invalid/transcript/1","total_additions":1,"total_deletions":0,` +
	`"accepted_lines":1,"overriden_lines":0}},"sessions":{"s_c9883b05a2487d":{"agent_id":{"tool":"cursor",` +
	`"id":"y","model":"` + strings.Repeat("M", 300) + `"},"human_author":"dev\u0007@example.com"}}}` + "\n"

func fixtureGit(t *testing.T, dir string, stdin []byte, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRepo holds one committed CEM map at HEAD.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, nil, "init", "-q", "-b", "main")
	writeFile(t, root, "src/app.txt", "alpha\n")
	writeFile(t, root, mapPath, fixtureMap)
	fixtureGit(t, root, nil, "add", ".")
	fixtureGit(t, root, nil, "commit", "-qm", "change with its map")
	return root
}

// refState is every ref plus the porcelain status: what a read must not move.
func refState(t *testing.T, root string) string {
	return fixtureGit(t, root, nil, "for-each-ref") + "\n" +
		fixtureGit(t, root, nil, "status", "--porcelain=v1", "--untracked-files=all")
}

func TestAnchorWritesAVerifiedPointerAndReadsItBack(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	ctx := context.Background()
	receipt, err := Anchor(ctx, root, mapPath, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	blob := fixtureGit(t, root, nil, "rev-parse", "HEAD:"+mapPath)
	if receipt["ok"] != true || receipt["mutates"] != true || receipt["written"] != true ||
		receipt["verification"] != "verified" || receipt["commit"] != head || receipt["ref"] != Ref {
		t.Fatalf("receipt: %v", receipt)
	}
	var stored Pointer
	if err := json.Unmarshal([]byte(fixtureGit(t, root, nil, "notes", "--ref="+Ref, "show", head)), &stored); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(fixtureMap))
	if stored.MapBlob != blob || stored.CEMCommit != head || stored.MapPath != mapPath || stored.Schema != PointerSchema ||
		stored.MapSha256 != hex.EncodeToString(digest[:]) || stored.MapSpec != "cem/0.2" {
		t.Fatalf("stored pointer: %+v", stored)
	}
	again, err := Anchor(ctx, root, mapPath, "HEAD")
	if err != nil || again["written"] != false || again["verification"] != "verified" {
		t.Fatalf("idempotent re-anchor: %v %v", again, err)
	}
	before := refState(t, root)
	read, err := Provenance(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if refState(t, root) != before || read["mutates"] != false {
		t.Fatal("provenance moved a ref or the worktree")
	}
	rows := read["evidence"].([]map[string]any)
	if len(rows) != 1 || rows[0]["kind"] != KindAnchor || rows[0]["state"] != "verified" || rows[0]["trust"] != Trust {
		t.Fatalf("anchor row: %v", rows)
	}
	// A pointer whose digest no longer matches the named blob is reported, never trusted.
	forged := stored
	forged.MapSha256 = strings.Repeat("0", 64)
	body, _ := json.Marshal(forged)
	fixtureGit(t, root, body, "notes", "--ref="+Ref, "add", "-f", "-F", "-", head)
	read, err = Provenance(ctx, root, "HEAD")
	if err != nil || read["evidence"].([]map[string]any)[0]["state"] != "digest-mismatch" {
		t.Fatalf("forged digest: %v %v", read, err)
	}
}

func TestAnchorRefusesAnUncommittedDirtyOrMissingMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		code  string
	}{
		{"untracked", func(t *testing.T, root string) {
			fixtureGit(t, root, nil, "rm", "-q", "--cached", mapPath)
			fixtureGit(t, root, nil, "commit", "-qm", "untrack")
		}, CodeMapUncommitted},
		{"modified", func(t *testing.T, root string) { writeFile(t, root, mapPath, fixtureMap+"\n") }, CodeMapDirty},
		{"staged", func(t *testing.T, root string) {
			writeFile(t, root, mapPath, fixtureMap+"\n")
			fixtureGit(t, root, nil, "add", mapPath)
		}, CodeMapDirty},
		{"absent", func(t *testing.T, root string) {
			fixtureGit(t, root, nil, "rm", "-q", mapPath)
			fixtureGit(t, root, nil, "commit", "-qm", "drop")
		}, CodeMapUncommitted},
		{"blob not in the object database", func(t *testing.T, root string) {
			blob := fixtureGit(t, root, nil, "rev-parse", "HEAD:"+mapPath)
			if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
				t.Fatal(err)
			}
		}, CodeBlobUnavailable},
		{"different note present", func(t *testing.T, root string) {
			fixtureGit(t, root, []byte("someone else's note"), "notes", "--ref="+Ref, "add", "-F", "-", "HEAD")
		}, CodeNoteConflict},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := fixtureRepo(t)
			test.setup(t, root)
			notesBefore := fixtureGit(t, root, nil, "for-each-ref", Ref)
			_, err := Anchor(ctx, root, mapPath, "HEAD")
			if cemcode.CodeOf(err) != test.code {
				t.Fatalf("code %q, want %q (%v)", cemcode.CodeOf(err), test.code, err)
			}
			if fixtureGit(t, root, nil, "for-each-ref", Ref) != notesBefore {
				t.Fatal("a refused anchor moved the notes ref")
			}
		})
	}
}

func TestProvenanceReadsForeignNotesAndTrailersAsUntrustedHistory(t *testing.T) {
	t.Parallel()
	root := fixtureRepo(t)
	fixtureGit(t, root, nil, "commit", "-q", "--allow-empty", "-m",
		"assisted change\n\nAssisted-by: Claude <noreply@anthropic.com>\n"+
			"Agent-Logs-Url: https://example.invalid/logs/42\nSigned-off-by: t <t@example.invalid>")
	fixtureGit(t, root, []byte(aiNote), "notes", "--ref="+ForeignRef, "add", "-F", "-", "HEAD")
	before := refState(t, root)
	read, err := Provenance(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if refState(t, root) != before || read["mutates"] != false || read["ok"] != true {
		t.Fatalf("provenance is not a pure read: %v", read)
	}
	rows := read["evidence"].([]map[string]any)
	kinds := []string{}
	for _, row := range rows {
		kinds = append(kinds, row["kind"].(string))
		class := contextindex.TrustClass(row["authority"].(string))
		if row["trust"] != contextindex.TrustRepositoryHistory || class != contextindex.TrustRepositoryHistory ||
			class == contextindex.TrustProjectAuthority {
			t.Fatalf("row trust %v / derived %s", row, class)
		}
	}
	if strings.Join(kinds, ",") != KindAINote+","+KindAssistedBy+","+KindAgentLogsURL {
		t.Fatalf("kinds %v", kinds)
	}
	note := rows[0]
	untrusted := note["untrusted"].(map[string]any)
	agents := untrusted["agents"].([]map[string]any)
	if note["state"] != "parsed" || note["files_total"] != 2 || note["agents_total"] != 2 || note["truncated"] != true ||
		untrusted["schema_version"] != "authorship/3.0.0" || strings.Join(untrusted["files"].([]string), ",") != "src/main.rs,docs/readme.md" ||
		agents[0]["messages_url"] != "https://example.invalid/transcript/1" || agents[1]["human_author"] != "dev@example.com" ||
		len(agents[1]["model"].(string)) != MaxTextBytes {
		t.Fatalf("ai note row: %v", note)
	}
	if rows[1]["untrusted"].(map[string]any)["value"] != "Claude <noreply@anthropic.com>" ||
		rows[2]["untrusted"].(map[string]any)["value"] != "https://example.invalid/logs/42" {
		t.Fatalf("trailer rows: %v", rows[1:])
	}
	// A foreign note that is not an authorship log is reported malformed, carrying no text.
	fixtureGit(t, root, []byte("free text, no divider"), "notes", "--ref="+ForeignRef, "add", "-f", "-F", "-", "HEAD")
	read, err = Provenance(context.Background(), root, "HEAD")
	row := read["evidence"].([]map[string]any)[0]
	if err != nil || row["state"] != "malformed" || row["untrusted"] != nil {
		t.Fatalf("malformed note: %v %v", row, err)
	}
}

// TestGitNotesFetchesNothing: the package that surfaces transcript URLs links
// no network client, so no URL it reports can be fetched (AGENTS.md invariant 7).
func TestGitNotesFetchesNothing(t *testing.T) {
	t.Parallel()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable")
	}
	out, err := exec.Command(goTool, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dependency := range strings.Fields(string(out)) {
		if dependency == "net" || strings.HasPrefix(dependency, "net/") {
			t.Fatalf("internal/gitnotes depends on %s", dependency)
		}
	}
}
