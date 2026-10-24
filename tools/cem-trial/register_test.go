package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func validRegistration(partition, model, effort string, n int) registration {
	return registration{
		Profile: registrationProfile, Hypotheses: []string{"the shipped-profile arm misses less withheld evidence than control"},
		PrimaryMetric: "miss", Alpha: 0.05, N: n, ScorerSHA256: scorerDigest(),
		Model: model, Effort: effort, Partition: partition,
	}
}

// commitRegistration writes the registration at the repository-relative name
// in a fresh Git repository and commits it with the given committer date
// (empty for now).
func commitRegistration(t *testing.T, name string, document any, committerDate string) string {
	t.Helper()
	root := t.TempDir()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q", "-b", "main"}, {"add", name}, {"commit", "-q", "-m", "register"}} {
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@localhost", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@localhost")
		if committerDate != "" {
			command.Env = append(command.Env, "GIT_COMMITTER_DATE="+committerDate, "GIT_AUTHOR_DATE="+committerDate)
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	return path
}

// CRT-V0-014: the scorer digest is the sha256 of the package's non-test
// sources concatenated in the documented order, and that list is complete.
func TestScorerDigestCoversEveryScorerSource(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	present := []string{}
	for _, name := range sources {
		if !strings.HasSuffix(name, "_test.go") {
			present = append(present, name)
		}
	}
	sort.Strings(present)
	if strings.Join(present, ",") != strings.Join(scorerFiles, ",") {
		t.Fatalf("scorer files %v, package sources %v", scorerFiles, present)
	}
	hash := sha256.New()
	for _, name := range scorerFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		hash.Write(data)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != scorerDigest() {
		t.Fatalf("scorerDigest %s, concatenated sources %s", scorerDigest(), got)
	}
}

// CRT-V0-014: the registration is a closed object whose fixed fields are
// checked before anything else reads them.
func TestRegistrationRefusesAnIncompletePlan(t *testing.T) {
	good, err := json.Marshal(validRegistration("heldout", "m", "high", 2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRegistration(good); err != nil {
		t.Fatalf("a complete registration was refused: %v", err)
	}
	cases := map[string]string{
		"unknown field":  strings.Replace(string(good), `"profile"`, `"extra":1,"profile"`, 1),
		"trailing data":  string(good) + "{}",
		"other metric":   strings.Replace(string(good), `"primary_metric":"miss"`, `"primary_metric":"citable"`, 1),
		"other alpha":    strings.Replace(string(good), `"alpha":0.05`, `"alpha":0.1`, 1),
		"no hypothesis":  strings.Replace(string(good), `"hypotheses":["the shipped-profile arm misses less withheld evidence than control"]`, `"hypotheses":[]`, 1),
		"blank effort":   strings.Replace(string(good), `"effort":"high"`, `"effort":" "`, 1),
		"zero n":         strings.Replace(string(good), `"n":2`, `"n":0`, 1),
		"short scorer":   strings.Replace(string(good), scorerDigest(), "abc", 1),
		"wrong profile":  strings.Replace(string(good), registrationProfile, "other/0", 1),
		"no partition":   strings.Replace(string(good), `"partition":"heldout"`, `"partition":""`, 1),
		"not an object":  `[]`,
		"model a number": strings.Replace(string(good), `"model":"m"`, `"model":1`, 1),
	}
	for name, body := range cases {
		t.Run("CRT-V0-014/"+name, func(t *testing.T) {
			if body == string(good) {
				t.Fatal("the case did not change the registration")
			}
			if _, err := parseRegistration([]byte(body)); err == nil {
				t.Fatal("the registration was accepted")
			}
		})
	}
}

// CRT-V0-014: a registration counts only as committed at HEAD byte for byte,
// and the report keeps the commit and its committer time.
func TestRegistrationMustBeCommitted(t *testing.T) {
	path := commitRegistration(t, "registration.json", validRegistration("heldout", "m", "high", 2), "2026-10-01T12:00:00Z")
	record, err := loadRegistration(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if record.Path != "registration.json" || record.CommittedAt != "2026-10-01T12:00:00Z" || len(record.Commit) != 40 {
		t.Fatalf("record = %+v", record)
	}
	if err := os.WriteFile(path, []byte(`{"edited":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRegistration(context.Background(), path); err == nil {
		t.Fatal("an uncommitted registration was accepted")
	}
	loose := filepath.Join(t.TempDir(), "registration.json")
	data, _ := json.Marshal(validRegistration("heldout", "m", "high", 2))
	if err := os.WriteFile(loose, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRegistration(context.Background(), loose); err == nil {
		t.Fatal("a registration outside any repository was accepted")
	}
	nested := commitRegistration(t, "plans/trial/registration.json", validRegistration("heldout", "m", "high", 2), "2026-10-01T12:00:00Z")
	record, err = loadRegistration(context.Background(), nested)
	if err != nil {
		t.Fatalf("a registration committed below the repository root was refused: %v", err)
	}
	if record.Path != "plans/trial/registration.json" || record.CommittedAt != "2026-10-01T12:00:00Z" {
		t.Fatalf("nested record = %+v", record)
	}
}

// registeredReport is syntheticReport as a control-only heldout run whose
// lanes started at the given time, registered at the given commit time.
func registeredReport(started, committed string) *report {
	document := syntheticReport()
	kept := []*lane{}
	for _, item := range document.Lanes {
		if item.Arm == "control" {
			item.StartedAt = started
			kept = append(kept, item)
		}
	}
	document.Lanes, document.Arms, document.Partition, document.Pilot = kept, []string{"control"}, "heldout", false
	document.Prologues = prologueIdentity(document.Arms)
	document.Registration = &registrationRecord{
		Path: "registration.json", SHA256: digestOf("x"), Commit: strings.Repeat("a", 40), CommittedAt: committed,
		Document: validRegistration("heldout", notObserved, notObserved, 2),
	}
	return document
}

// CRT-V0-010, CRT-V0-014, CRT-V0-015: the report is invalid when the
// registration was committed after the first lane or does not describe the
// run; a registered cem/1.0 map arm is valid; a heldout report with no
// registration is invalid.
func TestReportRefusesARegistrationNewerThanItsFirstLane(t *testing.T) {
	t.Run("CRT-V0-014/registered before the first lane", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00.5Z", "2026-10-01T12:00:00Z")
		finalize(document)
		if document.Invalid != "" {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-014/registered after the first lane", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-03T00:00:00Z")
		document.Lanes[1].StartedAt = "2026-10-04T00:00:00Z"
		finalize(document)
		if !strings.Contains(document.Invalid, "is newer than the first lane (2026-10-02T00:00:00Z)") {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-014/one completed lane without a start", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
		document.Lanes[1].StartedAt = ""
		finalize(document)
		if !strings.Contains(document.Invalid, "1 completed lane(s) recorded no start") {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-014/rescore checks the registered plan", func(t *testing.T) {
		edits := map[string]func(*registration){
			"alpha":          func(plan *registration) { plan.Alpha = 0.1 },
			"primary_metric": func(plan *registration) { plan.PrimaryMetric = "citable" },
			"hypotheses":     func(plan *registration) { plan.Hypotheses = nil },
			"profile":        func(plan *registration) { plan.Profile = "other/0" },
		}
		for name, edit := range edits {
			document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
			edit(&document.Registration.Document)
			path := filepath.Join(t.TempDir(), "report.json")
			var stdout, stderr bytes.Buffer
			if code := write(document, path, &stdout, &stderr); code != 0 {
				t.Fatal(stderr.String())
			}
			rebuilt, _, err := rescore([]string{"--report", path})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rebuilt.Invalid, name) {
				t.Fatalf("%s: rescored invalid = %q", name, rebuilt.Invalid)
			}
		}
	})
	t.Run("CRT-V0-014/rescore refuses an unknown registration key", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
		path := filepath.Join(t.TempDir(), "report.json")
		var stdout, stderr bytes.Buffer
		if code := write(document, path, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(data), `"primary_metric"`, `"secondary_metric": "spans", "primary_metric"`, 1)
		if edited == string(data) {
			t.Fatal("the report carries no registered primary_metric")
		}
		if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := rescore([]string{"--report", path}); err == nil || !strings.Contains(err.Error(), "secondary_metric") {
			t.Fatalf("rescore of an unknown registration key: %v", err)
		}
	})
	t.Run("CRT-V0-014/no lane start observed", func(t *testing.T) {
		document := registeredReport("", "2026-10-01T00:00:00Z")
		finalize(document)
		if !strings.Contains(document.Invalid, "registration order is unobserved") {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-014/fixed fields disagree", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
		document.Registration.Document.Model, document.Registration.Document.N = "other-model", 3
		document.Registration.Document.ScorerSHA256 = strings.Repeat("0", 64)
		finalize(document)
		for _, needle := range []string{`registered model "other-model"`, "registered n 3, run 2 pairs", "registered scorer_sha256"} {
			if !strings.Contains(document.Invalid, needle) {
				t.Fatalf("invalid = %q, want %q", document.Invalid, needle)
			}
		}
	})
	t.Run("CRT-V0-015/registered cem/1.0 map arm", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
		document.Arms = []string{"control", "treatment"}
		finalize(document)
		if strings.Contains(document.Invalid, "CRT-V0-015") || strings.Contains(document.Invalid, "shipped profile") {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-014/heldout without registration", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z")
		document.Registration = nil
		finalize(document)
		if !strings.Contains(document.Invalid, "carries no registration") {
			t.Fatalf("invalid = %q", document.Invalid)
		}
	})
	t.Run("CRT-V0-010/rescore keeps the reading", func(t *testing.T) {
		document := registeredReport("2026-10-02T00:00:00Z", "2026-10-03T00:00:00Z")
		finalize(document)
		path := filepath.Join(t.TempDir(), "report.json")
		var stdout, stderr bytes.Buffer
		if code := write(document, path, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt, _, err := rescore([]string{"--report", path})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if code := write(rebuilt, "", &out, &stderr); code != 0 || !bytes.Equal(original, out.Bytes()) {
			t.Fatal("score did not reproduce the registered report's bytes")
		}
		if rebuilt.Invalid == "" {
			t.Fatal("the rescored report lost its invalid reading")
		}
	})
}

// heldoutManifest writes a one-change heldout manifest over the fixture
// repository's last commit.
func heldoutManifest(t *testing.T, repo string) string {
	t.Helper()
	head, err := git(context.Background(), repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base, err := git(context.Background(), repo, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	patch := "diff --git a/internal/play/play.go b/internal/play/play.go\n--- a/internal/play/play.go\n+++ b/internal/play/play.go\n@@ -9,1 +9,1 @@\n-\treturn 0\n+\treturn 10\n"
	if err := os.WriteFile(filepath.Join(directory, "c01.patch"), []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	set := manifest{Partition: "heldout", Seed: "fixture", SelectionRule: selectionRule, Population: 1, Pairs: 1, Tasks: []task{{
		ID: "c01", Commit: head, BaseCommit: base, PatchSHA256: digestOf(patch), PatchPath: "c01.patch",
		SourceFiles: []string{"internal/play/play.go"}, HunkCount: 1, StemBaseline: []string{"internal/play/play_test.go"},
		Gold: gold{Paths: []string{"internal/play/play_test.go"}, Spans: []span{{Path: "internal/play/play_test.go", Lines: "6:7"}}},
	}}}
	data, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "tasks.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// CRT-V0-014, CRT-V0-015: `run` refuses a heldout manifest with no
// registration and a registration the run does not match, before any lane
// runs; a
// matching registration runs, records each lane's start, and the report reads
// invalid only when the registration commit is newer than the first lane.
func TestRunChecksItsRegistration(t *testing.T) {
	repo := fixtureRepo(t)
	tasks := heldoutManifest(t, repo)
	agentScript, err := filepath.Abs(filepath.Join("testdata", "fake-agent.sh"))
	if err != nil {
		t.Fatal(err)
	}
	arguments := func(arms, registrationPath string) []string {
		list := []string{"--tasks", tasks, "--history", repo, "--arms", arms, "--agent", "script", "--agent-command", agentScript, "--seed", "fixture"}
		if registrationPath != "" {
			list = append(list, "--registration", registrationPath)
		}
		return list
	}
	ctx := context.Background()
	if _, _, err := runTrial(ctx, arguments("control", "")); err == nil || !strings.Contains(err.Error(), "needs --registration") {
		t.Fatalf("heldout run without registration: %v", err)
	}
	mismatched := commitRegistration(t, "registration.json", validRegistration("heldout", notObserved, notObserved, 5), "")
	if _, _, err := runTrial(ctx, arguments("control", mismatched)); err == nil || !strings.Contains(err.Error(), "registered n 5, run 1 pairs") {
		t.Fatalf("mismatched registration: %v", err)
	}
	past := commitRegistration(t, "registration.json", validRegistration("heldout", notObserved, notObserved, 1), "2026-01-01T00:00:00Z")
	document, _, err := runTrial(ctx, arguments("control", past))
	if err != nil {
		t.Fatal(err)
	}
	if document.Invalid != "" || document.Registration == nil || document.Lanes[0].StartedAt == "" {
		t.Fatalf("registered run: invalid %q, registration %v, started %q", document.Invalid, document.Registration, document.Lanes[0].StartedAt)
	}
	if document.Changes[0].GoldSpans == nil || document.Lanes[0].CitedNotGold.Paths == notObserved {
		t.Fatalf("the run did not carry span gold or score precision: %+v %+v", document.Changes[0], document.Lanes[0].CitedNotGold)
	}
	// Lanes run under another effort may not enter a registered run, whether
	// they come from --reuse or from a --resume checkpoint.
	document.Effort = "low"
	directory := t.TempDir()
	stale := filepath.Join(directory, "stale.json")
	output := filepath.Join(directory, "report.json")
	for _, path := range []string{stale, checkpointPath(output)} {
		var stdout, stderr bytes.Buffer
		if code := write(document, path, &stdout, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
	}
	if _, _, err := runTrial(ctx, append(arguments("control", past), "--reuse", stale)); err == nil || !strings.Contains(err.Error(), "reuses only lanes run under") {
		t.Fatalf("registered run reusing another effort's lanes: %v", err)
	}
	if _, _, err := runTrial(ctx, append(arguments("control", past), "--output", output, "--resume")); err == nil || !strings.Contains(err.Error(), "reuses only lanes run under") {
		t.Fatalf("registered run resuming another effort's checkpoint: %v", err)
	}
	// A checkpoint that no longer decodes is refused and kept, not silently
	// replaced by rerunning the lanes it holds.
	document.Effort = notObserved
	broken := filepath.Join(directory, "broken.json")
	var stdout, stderr bytes.Buffer
	if code := write(document, checkpointPath(broken), &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	data, err := os.ReadFile(checkpointPath(broken))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), `"primary_metric"`, `"secondary_metric": "spans", "primary_metric"`, 1)
	if err := os.WriteFile(checkpointPath(broken), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runTrial(ctx, append(arguments("control", past), "--output", broken, "--resume")); err == nil || !strings.Contains(err.Error(), "secondary_metric") {
		t.Fatalf("registered run resuming an undecodable checkpoint: %v", err)
	}
	if kept, err := os.ReadFile(checkpointPath(broken)); err != nil || string(kept) != edited {
		t.Fatalf("the undecodable checkpoint was not kept: %v", err)
	}
	future := commitRegistration(t, "registration.json", validRegistration("heldout", notObserved, notObserved, 1), "2099-01-01T00:00:00Z")
	document, _, err = runTrial(ctx, arguments("control", future))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document.Invalid, "is newer than the first lane") {
		t.Fatalf("a registration committed after the first lane left invalid = %q", document.Invalid)
	}
}
