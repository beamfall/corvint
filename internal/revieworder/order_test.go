package revieworder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitnotes"
	"github.com/Beamfall/corvint/internal/witness"
)

var base = strings.Repeat("a", 40)
var target = strings.Repeat("b", 40)
var digest = strings.Repeat("c", 64)

func input() Inputs {
	h := patch.Hunk{ID: "h1", DisplayPath: "app.go", OldRange: wire.Range{Start: 1, Count: 1}, NewRange: wire.Range{Start: 1, Count: 2},
		Body: []patch.BodyLine{{Prefix: ' ', Payload: []byte("context\n")}, {Prefix: '+', Payload: []byte("added\n")}}}
	return Inputs{BaseRevision: base, TargetRevision: target, PatchSha256: digest, Hunks: []patch.Hunk{h},
		Note: note("app.go\n  9f8e7d6c5b4a3210 2\n"),
		Witness: &witness.Report{Profile: witness.Profile, Range: witness.Range{Base: base, Head: target},
			Obligations: []witness.Obligation{{ID: "change:app.go", Kind: witness.KindChange, Path: "app.go", Verdict: witness.Unproven}}},
		CEM: &CEM{TargetRevision: target, MapSha256: digest, Verified: true, Map: &wire.Map{Spec: wire.StableSpec, BaseRevision: base, PatchSha256: digest,
			Hunks: []wire.Hunk{{ID: "h1", Path: "app.go", OldRange: h.OldRange, NewRange: h.NewRange,
				Coverage: &wire.CoverageWitness{State: wire.CoverageUncovered, ProfileSha256: digest, TestRun: "go test ./app", Mode: "set"}}}}}}
}

func note(log string) *Note {
	body := log + "---\n" + `{"schema_version":"authorship/3.0.0","base_commit_sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","humans":{"h_31dce776f88375":{"author":"Fixture <fixture@example.invalid>"}},"prompts":{"9f8e7d6c5b4a3210":{"agent_id":{"tool":"codex"}}},"sessions":{"s_c9883b05a2487d":{"agent_id":{"tool":"cursor"}}}}` + "\n"
	return &Note{Commit: target, BlobOID: strings.Repeat("d", 40), Body: []byte(body)}
}

// FPK-V0-053, FPK-V0-055: explicit rank expectations, not implementation mirroring.
func TestRankHunksWithCitedFactors(t *testing.T) {
	in := input()
	h := in.Hunks[0]
	names := []string{"human.go", "unknown.go", "covered.go", "first.go"}
	in.Hunks = nil
	in.CEM.Map.Hunks = nil
	in.Witness.Obligations = nil
	log := ""
	for _, name := range names {
		h.ID, h.DisplayPath = name, name
		in.Hunks = append(in.Hunks, h)
		value := "9f8e7d6c5b4a3210"
		if name == "human.go" {
			value = "h_31dce776f88375"
		}
		if name != "unknown.go" {
			log += name + "\n  " + value + " 2\n"
		}
		in.Witness.Obligations = append(in.Witness.Obligations, witness.Obligation{ID: "change:" + name, Kind: witness.KindChange, Path: name, Verdict: witness.Unproven})
		coverage := &wire.CoverageWitness{State: wire.CoverageUncovered, ProfileSha256: digest, TestRun: "go test ./app", Mode: "set"}
		if name == "covered.go" {
			coverage.State = wire.CoverageCovered
			coverage.Covered = []wire.Range{{Start: 2, Count: 1}}
		}
		in.CEM.Map.Hunks = append(in.CEM.Map.Hunks, wire.Hunk{ID: h.ID, Path: name, NewRange: h.NewRange, OldRange: h.OldRange, Coverage: coverage})
	}
	in.Note = note(log)
	rows := Rank(in)
	got := []string{}
	for _, r := range rows {
		got = append(got, r.Path)
		for _, f := range []Factor{r.AIAuthored, r.Unwitnessed, r.Uncovered} {
			if f.Citation.Source == "" || f.Citation.Detail == "" || f.Citation.Path != r.Path || f.Citation.HunkID != r.ID || f.Citation.Revision != target || f.Citation.Object == "" {
				t.Fatalf("uncited factor: %+v", f)
			}
		}
		if r.AIAuthored.Citation.Trust != gitnotes.Trust {
			t.Fatal("authorship lost repository-history trust")
		}
	}
	if !reflect.DeepEqual(got, []string{"first.go", "covered.go", "unknown.go", "human.go"}) {
		t.Fatalf("order: %v", got)
	}
	if rows[0].AIAuthored.Value != Yes || rows[0].Unwitnessed.Value != Yes || rows[0].Uncovered.Value != Yes ||
		rows[1].Uncovered.Value != No || rows[2].AIAuthored.Value != Unknown || rows[3].AIAuthored.Value != No {
		t.Fatalf("factors: %+v", rows)
	}
}

// FPK-V0-053, FPK-V0-055: only Unwitnessed distinguishes these factors;
// path and input order oppose its yes > unknown > no priority.
func TestRankUnwitnessedIsOnlyDiscriminatingFactor(t *testing.T) {
	in := input()
	h, mapped := in.Hunks[0], in.CEM.Map.Hunks[0]
	in.Hunks, in.CEM.Map.Hunks, in.Witness.Obligations = nil, nil, nil
	log := ""
	for _, tc := range []struct {
		path    string
		verdict witness.Verdict
	}{
		{"a-closed.go", witness.Closed},
		{"b-not-run.go", witness.NotRun},
		{"c-unproven.go", witness.Unproven},
	} {
		h.ID, h.DisplayPath = tc.path, tc.path
		mapped.ID, mapped.Path = h.ID, h.DisplayPath
		in.Hunks = append(in.Hunks, h)
		in.CEM.Map.Hunks = append(in.CEM.Map.Hunks, mapped)
		log += tc.path + "\n  9f8e7d6c5b4a3210 2\n"
		obligation := witness.Obligation{ID: "change:" + tc.path, Kind: witness.KindChange, Path: tc.path, Verdict: tc.verdict}
		if tc.verdict == witness.Closed {
			obligation.Witnesses = []witness.Witness{{HunkID: h.ID, Closing: true}}
		}
		in.Witness.Obligations = append(in.Witness.Obligations, obligation)
	}
	in.Note = note(log)
	rows := Rank(in)
	got := []string{}
	values := []Value{}
	for _, row := range rows {
		if row.AIAuthored.Value != Yes || row.Uncovered.Value != Yes {
			t.Fatalf("other factors must tie: %+v", row)
		}
		got = append(got, row.Path)
		values = append(values, row.Unwitnessed.Value)
	}
	if !reflect.DeepEqual(got, []string{"c-unproven.go", "b-not-run.go", "a-closed.go"}) ||
		!reflect.DeepEqual(values, []Value{Yes, Unknown, No}) {
		t.Fatalf("Unwitnessed priority: paths %v, values %v", got, values)
	}
}

// FPK-V0-054: ambiguous path syntax cannot establish human authorship even
// when the following attestation has a valid human metadata record.
func TestAuthorshipPathQuoting(t *testing.T) {
	for _, tc := range []struct {
		name, path, line string
		want             Value
	}{
		{"unquoted-space", "my app.go", "my app.go", Unknown},
		{"quoted-space", "my app.go", `"my app.go"`, No},
		{"unquoted-tab", "my\tapp.go", "my\tapp.go", Unknown},
		{"quoted-tab", "my\tapp.go", `"my\tapp.go"`, No},
		{"unquoted-newline", "app.go", "my\napp.go", Unknown},
		{"quoted-newline", "my\napp.go", `"my\napp.go"`, No},
		{"unclosed-quote", `"my app.go`, `"my app.go`, Unknown},
		{"trailing-junk", `"my app.go"junk`, `"my app.go"junk`, Unknown},
		{"empty-quoted-path", "", `""`, Unknown},
		{"leading-tab", "\tapp.go", "\tapp.go", Unknown},
		{"leading-vertical-tab", "\vapp.go", "\vapp.go", Unknown},
		{"leading-form-feed", "\fapp.go", "\fapp.go", Unknown},
		{"leading-nonbreaking-space", "\u00a0app.go", "\u00a0app.go", Unknown},
		{"unmatched-trailing-quote", `app.go"`, `app.go"`, Unknown},
		{"unquoted-carriage-return", "app.go\r", "app.go\r", Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input()
			in.Hunks[0].DisplayPath = tc.path
			in.Note = note(tc.line + "\n  h_31dce776f88375 2\n")
			got := Rank(in)[0].AIAuthored
			if got.Value != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
			if tc.want == Unknown && got.Citation.Detail == "" {
				t.Fatal("unknown path lacks a reason")
			}
		})
	}
}

// FPK-V0-054: absent provenance is not human provenance; notes name actual
// added lines, not context lines, and huge foreign ranges remain intervals.
func TestAuthorshipUnknownAndExplicitLineAttestations(t *testing.T) {
	cases := []struct {
		name, log string
		want      Value
	}{
		{"bad-indent", "app.go\n   9f8e7d6c5b4a3210 2\n", Unknown},
		{"signed-range", "app.go\n  9f8e7d6c5b4a3210 +2\n", Unknown},
		{"prompt", "app.go\n  9f8e7d6c5b4a3210 2\n", Yes},
		{"session", "app.go\n  s_c9883b05a2487d::t_9f8e7d6c5b4a32 2\n", Yes},
		{"human", "app.go\n  h_31dce776f88375 2\n", No},
		{"context-only", "app.go\n  9f8e7d6c5b4a3210 1\n", Unknown},
		{"other-path", "other.go\n  9f8e7d6c5b4a3210 2\n", Unknown},
		{"unknown-record", "app.go\n  p_missing 2\n", Unknown},
		{"conflict", "app.go\n  9f8e7d6c5b4a3210 2\n  h_31dce776f88375 2\n", Unknown},
		{"malformed-range", "app.go\n  9f8e7d6c5b4a3210 2-1\n", Unknown},
		{"huge-range", "app.go\n  9f8e7d6c5b4a3210 1-9223372036854775807\n", Yes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := input()
			in.Note = note(tc.log)
			r := Rank(in)[0]
			if r.AIAuthored.Value != tc.want {
				t.Fatalf("got %+v", r.AIAuthored)
			}
		})
	}
	for _, kind := range []string{"absent", "malformed", "unsupported", "oversized", "wrong-commit", "unpinned", "deletion", "missing-human-record", "null-agent"} {
		t.Run(kind, func(t *testing.T) {
			in := input()
			switch kind {
			case "missing-human-record":
				in.Note = note("app.go\n  h_31dce776f88375 2\n")
				in.Note.Body = bytes.ReplaceAll(in.Note.Body, []byte(`"author":"Fixture <fixture@example.invalid>"`), []byte(`"author":""`))
			case "null-agent":
				in.Note.Body = bytes.ReplaceAll(in.Note.Body, []byte(`{"agent_id":{"tool":"codex"}}`), []byte(`null`))
			case "absent":
				in.Note = nil
			case "malformed":
				in.Note.Body = []byte("not a note")
			case "unsupported":
				in.Note.Body = bytes.ReplaceAll(in.Note.Body, []byte("authorship/3.0.0"), []byte("authorship/9.0.0"))
			case "oversized":
				in.Note.Body = make([]byte, gitnotes.MaxNoteBytes+1)
			case "wrong-commit":
				in.Note.Commit = base
			case "unpinned":
				in.Note.BlobOID = ""
			case "deletion":
				in.Hunks[0].Body = []patch.BodyLine{{Prefix: '-', Payload: []byte("deleted\n")}}
				in.Hunks[0].NewRange.Count = 0
			}
			f := Rank(in)[0].AIAuthored
			if f.Value != Unknown || f.Citation.Detail == "" {
				t.Fatalf("got %+v", f)
			}
		})
	}
}

// FPK-V0-055: no stale inputs or caller test claims close a witness obligation.
func TestWitnessAndCoverageUnknownsRemainIndependent(t *testing.T) {
	cases := []struct {
		name              string
		change            func(*Inputs)
		witness, coverage Value
	}{
		{"baseline", func(in *Inputs) {}, Yes, Yes},
		{"missing-witness", func(in *Inputs) { in.Witness = nil }, Unknown, Yes},
		{"stale-witness", func(in *Inputs) { in.Witness.Range.Head = base }, Unknown, Yes},
		{"not-run", func(in *Inputs) { in.Witness.Obligations[0].Verdict = witness.NotRun }, Unknown, Yes},
		{"missing-obligation", func(in *Inputs) { in.Witness.Obligations = nil }, Unknown, Yes},
		{"duplicate-obligation", func(in *Inputs) { in.Witness.Obligations = append(in.Witness.Obligations, in.Witness.Obligations[0]) }, Unknown, Yes},
		{"path-closed-only", func(in *Inputs) { in.Witness.Obligations[0].Verdict = witness.Closed }, Unknown, Yes},
		{"hunk-closed", func(in *Inputs) {
			in.Witness.Obligations[0].Verdict = witness.Closed
			in.Witness.Obligations[0].Witnesses = []witness.Witness{{HunkID: "h1", Closing: true}}
		}, No, Yes},
		{"nonclosing-test-claim", func(in *Inputs) {
			in.Witness.Obligations[0].Witnesses = []witness.Witness{{HunkID: "h1", Relation: "test-claim", Closing: false}}
		}, Yes, Yes},
		{"missing-cem", func(in *Inputs) { in.CEM = nil }, Yes, Unknown},
		{"unverified-cem", func(in *Inputs) { in.CEM.Verified = false }, Yes, Unknown},
		{"stale-cem", func(in *Inputs) { in.CEM.TargetRevision = base }, Yes, Unknown},
		{"wrong-base", func(in *Inputs) { in.CEM.Map.BaseRevision = target }, Yes, Unknown},
		{"wrong-patch", func(in *Inputs) { in.CEM.Map.PatchSha256 = strings.Repeat("e", 64) }, Yes, Unknown},
		{"missing-map-pin", func(in *Inputs) { in.CEM.MapSha256 = "" }, Yes, Unknown},
		{"missing-hunk", func(in *Inputs) { in.CEM.Map.Hunks = nil }, Yes, Unknown},
		{"duplicate-hunk", func(in *Inputs) { in.CEM.Map.Hunks = append(in.CEM.Map.Hunks, in.CEM.Map.Hunks[0]) }, Yes, Unknown},
		{"mismatched-range", func(in *Inputs) { in.CEM.Map.Hunks[0].NewRange.Count++ }, Yes, Unknown},
		{"missing-coverage", func(in *Inputs) { in.CEM.Map.Hunks[0].Coverage = nil }, Yes, Unknown},
		{"covered", func(in *Inputs) {
			w := in.CEM.Map.Hunks[0].Coverage
			w.State = wire.CoverageCovered
			w.Covered = []wire.Range{{Start: 2, Count: 1}}
		}, Yes, No},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := input()
			tc.change(&in)
			r := Rank(in)[0]
			if r.Unwitnessed.Value != tc.witness || r.Uncovered.Value != tc.coverage {
				t.Fatalf("got %+v", r)
			}
			if r.Unwitnessed.Citation.Detail == "" || r.Uncovered.Citation.Detail == "" {
				t.Fatal("missing reason")
			}
		})
	}
}

// FPK-V0-055: observed human-line coverage cannot hide an uncovered AI line.
func TestCoverageTargetsAIAddedLines(t *testing.T) {
	in := input()
	in.Hunks[0].Body = append(in.Hunks[0].Body, patch.BodyLine{Prefix: '+', Payload: []byte("unknown\n")})
	in.Hunks[0].NewRange.Count++
	row := Rank(in)[0]
	if row.AIAuthored.Value != Yes || !strings.Contains(row.AIAuthored.Citation.Detail, "some-line-authorship-unknown") {
		t.Fatalf("known AI line lost to partial note: %+v", row.AIAuthored)
	}
	in = input()
	in.Hunks[0].Body = append(in.Hunks[0].Body, patch.BodyLine{Prefix: '+', Payload: []byte("human\n")})
	in.Hunks[0].NewRange.Count++
	in.Note = note("app.go\n  9f8e7d6c5b4a3210 2\n  h_31dce776f88375 3\n")
	m := &in.CEM.Map.Hunks[0]
	m.NewRange = in.Hunks[0].NewRange
	m.Coverage.State = wire.CoverageCovered
	m.Coverage.Covered = []wire.Range{{Start: 3, Count: 1}}
	if r := Rank(in)[0]; r.AIAuthored.Value != Yes || r.Uncovered.Value != Yes {
		t.Fatalf("human coverage hid AI gap: %+v", r)
	}
	m.Coverage.Covered = []wire.Range{{Start: 2, Count: 1}}
	if r := Rank(in)[0]; r.Uncovered.Value != No {
		t.Fatalf("AI coverage not recognized: %+v", r)
	}
}

// FPK-V0-053, FPK-V0-056: tie breaks and repeated results are stable; inputs
// including nested body bytes, maps, coverage slices and reports remain unchanged.
func TestRankDeterministicAndDoesNotMutateInputs(t *testing.T) {
	in := input()
	in.Note = nil
	in.CEM = nil
	in.Witness = nil
	h := in.Hunks[0]
	in.Hunks = append(in.Hunks, h)
	h.ID = "h0"
	in.Hunks = append(in.Hunks, h)
	h.DisplayPath = "a.go"
	in.Hunks = append(in.Hunks, h)
	before, _ := json.Marshal(in)
	a := Rank(in)
	b := Rank(in)
	after, _ := json.Marshal(in)
	if !bytes.Equal(before, after) || !reflect.DeepEqual(a, b) {
		t.Fatal("mutation or nondeterminism")
	}
	got := []int{}
	for _, h := range a {
		got = append(got, h.Ordinal)
	}
	if !reflect.DeepEqual(got, []int{4, 3, 1, 2}) {
		t.Fatalf("tie order %v", got)
	}
	in = input()
	in.Hunks[0].DisplayPath = "my app.go"
	in.Note = note("\"my app.go\"\n  9f8e7d6c5b4a3210 2\n")
	if Rank(in)[0].AIAuthored.Value != Yes {
		t.Fatal("quoted path lost authorship")
	}
	before, _ = json.Marshal(in)
	Rank(in)
	after, _ = json.Marshal(in)
	if !bytes.Equal(before, after) {
		t.Fatal("nested evidence changed")
	}
	if len(Rank(Inputs{})) != 0 {
		t.Fatal("empty universe fabricated hunks")
	}
}

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"--no-optional-locks", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		files[strings.TrimPrefix(path, root)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// FPK-V0-054, FPK-V0-056: a real commit with no notes ref yields no provenance
// row, but its hunk still appears with unknown authorship. The full fixture,
// including Git objects/refs/logs/index and trace/observation sentinels, is stable.
func TestCommitWithoutGitAINoteReportsUnknownAuthorship(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", "app.go")
	fixtureGit(t, root, "commit", "-qm", "base")
	baseCommit := fixtureGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("before\nafter\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "add", "app.go")
	fixtureGit(t, root, "commit", "-qm", "change without notes")
	commit := fixtureGit(t, root, "rev-parse", "HEAD")
	raw := []byte(fixtureGit(t, root, "diff", "--no-ext-diff", "--no-textconv", "--unified=0", baseCommit, commit, "--") + "\n")
	parsed, err := patch.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Hunks) != 1 {
		t.Fatalf("fixture has %d hunks", len(parsed.Hunks))
	}
	if err := os.Mkdir(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"self-observations.jsonl", "unplanned-reads.jsonl", "traces.jsonl"} {
		if err := os.WriteFile(filepath.Join(root, ".corvint", name), []byte("sentinel\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, root)
	provenance, err := gitnotes.Provenance(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	if provenance["mutates"] != false {
		t.Fatal("provenance mutation flag")
	}
	for _, row := range provenance["evidence"].([]map[string]any) {
		if row["kind"] == gitnotes.KindAINote {
			t.Fatal("missing-note fixture unexpectedly has AI provenance")
		}
	}
	rows := Rank(Inputs{BaseRevision: baseCommit, TargetRevision: commit, Hunks: []patch.Hunk{*parsed.Hunks[0]}})
	if len(rows) != 1 || rows[0].AIAuthored.Value != Unknown || rows[0].AIAuthored.Citation.Source != gitnotes.ForeignRef || rows[0].AIAuthored.Citation.Detail != "git-ai-note-absent-or-unavailable" {
		t.Fatalf("missing note: %+v", rows)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("read changed fixture repository or traces")
	}
}
