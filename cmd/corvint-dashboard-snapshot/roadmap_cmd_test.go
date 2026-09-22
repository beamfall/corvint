package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseRoadmapArgumentsRequiresCoreFlags(t *testing.T) {
	root := t.TempDir()
	store := t.TempDir()
	arguments := []string{
		"roadmap", "--root", root, "--tasks", "/bin/true", "--store", store,
		"--generated-at", testTime,
	}
	got, err := parseRoadmapArguments(arguments)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != root || got.AtmBinary != "/bin/true" || got.StoreRoot != store || got.GeneratedAt != testTime {
		t.Fatalf("options = %+v", got)
	}
	if got.RequirementsTSV != filepath.Join(root, "docs", "specs", "REQUIREMENTS.tsv") {
		t.Fatalf("default RequirementsTSV = %q", got.RequirementsTSV)
	}
}

func TestParseRoadmapArgumentsRejectsMissingRequired(t *testing.T) {
	cases := [][]string{
		{"roadmap", "--atm", "/bin/true", "--store", t.TempDir(), "--generated-at", testTime},
		{"roadmap", "--root", t.TempDir(), "--store", t.TempDir(), "--generated-at", testTime},
		{"roadmap", "--root", t.TempDir(), "--atm", "/bin/true", "--generated-at", testTime},
		{"roadmap", "--root", t.TempDir(), "--atm", "/bin/true", "--store", t.TempDir()},
	}
	for _, arguments := range cases {
		if _, err := parseRoadmapArguments(arguments); err == nil {
			t.Fatalf("arguments %v: want error", arguments)
		}
	}
}

func TestParseRoadmapArgumentsSelectsTaskCompatibilityFlags(t *testing.T) {
	root := t.TempDir()
	store := t.TempDir()
	base := []string{"roadmap", "--root", root, "--store", store, "--generated-at", testTime}
	tests := []struct {
		name      string
		flags     []string
		want      string
		wantError bool
	}{
		{name: "CRB-V0-015 primary", flags: []string{"--tasks", "/current"}, want: "/current"},
		{name: "CRB-V0-015 legacy fallback", flags: []string{"--atm", "/legacy"}, want: "/legacy"},
		{name: "CRB-V0-015 equal", flags: []string{"--tasks", "/same", "--atm", "/same"}, want: "/same"},
		{name: "CRB-V0-015 neither", wantError: true},
		{name: "CRB-V0-015 conflict", flags: []string{"--tasks", "/current", "--atm", "/legacy"}, wantError: true},
		{name: "CRB-V0-015 empty primary", flags: []string{"--tasks", ""}, wantError: true},
		{name: "CRB-V0-015 empty legacy", flags: []string{"--atm", ""}, wantError: true},
		{name: "CRB-V0-015 equal empty", flags: []string{"--tasks", "", "--atm", ""}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arguments := append(append([]string{}, base...), test.flags...)
			got, err := parseRoadmapArguments(arguments)
			if test.wantError {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.AtmBinary != test.want {
				t.Fatalf("tasks binary = %q, want %q", got.AtmBinary, test.want)
			}
		})
	}
}

func TestRoadmapCompatibilityRefusesBeforeTaskChild(t *testing.T) {
	markerDir := t.TempDir()
	taskMarker := filepath.Join(markerDir, "task-ran")
	gitMarker := filepath.Join(markerDir, "git-ran")
	tasks := filepath.Join(t.TempDir(), "corvint-tasks")
	if err := os.WriteFile(tasks, []byte("#!/bin/sh\n: > \""+taskMarker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte("#!/bin/sh\n: > \""+gitMarker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	base := []string{
		"roadmap", "--root", t.TempDir(), "--store", t.TempDir(), "--generated-at", testTime,
	}
	tests := [][]string{
		{"--tasks", tasks, "--atm", "/different"},
		{"--tasks", ""},
		{"--atm", ""},
	}
	for _, flags := range tests {
		var stdout, stderr bytes.Buffer
		arguments := append(append([]string{}, base...), flags...)
		if exit := runRoadmapContext(context.Background(), arguments, &stdout, &stderr); exit != 2 {
			t.Fatalf("flags %v: exit = %d", flags, exit)
		}
		for _, marker := range []string{taskMarker, gitMarker} {
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("flags %v launched child %s: %v", flags, marker, err)
			}
		}
	}
}

func TestParseRoadmapArgumentsRejectsUnknownFlag(t *testing.T) {
	root := t.TempDir()
	arguments := []string{
		"roadmap", "--root", root, "--atm", "/bin/true", "--store", t.TempDir(),
		"--generated-at", testTime, "--unknown", "x",
	}
	if _, err := parseRoadmapArguments(arguments); err == nil {
		t.Fatal("want error for unknown flag")
	}
}

func TestRunDispatchesRoadmapSubcommand(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := run([]string{
		"roadmap", "--root", root, "--atm", filepath.Join(t.TempDir(), "missing-atm"),
		"--store", t.TempDir(), "--generated-at", testTime,
	}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	var snapshot struct {
		Schema  string `json:"schema"`
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil {
		t.Fatalf("stdout = %s: %v", stdout.String(), err)
	}
	if snapshot.Schema != "corvint-dashboard-roadmap/0" || snapshot.Outcome != "NOT_OBSERVED" || snapshot.Reason == "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

// TestRoadmapCancelledDuringCompileIsInterrupted: a cancellation that lands
// while `atm` runs must end as pre-output DASHBOARD_INTERRUPTED, not as a
// NOT_OBSERVED snapshot on exit 0 or an internal error.
func TestRoadmapCancelledDuringCompileIsInterrupted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub atm is a POSIX shell script")
	}
	atm := filepath.Join(t.TempDir(), "atm")
	if err := os.WriteFile(atm, []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()
	var stdout, stderr bytes.Buffer
	exit := runRoadmapContext(ctx, []string{
		"roadmap", "--root", t.TempDir(), "--atm", atm, "--store", t.TempDir(), "--generated-at", testTime,
	}, &stdout, &stderr)
	want := `{"code":"DASHBOARD_INTERRUPTED","profile":"corvint-dashboard-error/0"}` + "\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

// TestRoadmapAgainstSeededPlanningStore is the real, end-to-end read this
// join exists for: the actual `atm` binary against the seeded fixture
// planning store from the IPR-10 dogfood pass. It skips (never fails) when
// that fixture is absent, since it is scratch state outside the repository
// (CORVINT_DOGFOOD_ATM / CORVINT_DOGFOOD_STORE point at it).
func TestRoadmapAgainstSeededPlanningStore(t *testing.T) {
	atmBinary := os.Getenv("CORVINT_DOGFOOD_ATM")
	storeRoot := os.Getenv("CORVINT_DOGFOOD_STORE")
	if atmBinary == "" {
		atmBinary = "/private/tmp/corvint-public-release-20260912/bin/atm"
	}
	if storeRoot == "" {
		storeRoot = "/private/tmp/corvint-public-release-20260912/planning-store"
	}
	if _, err := os.Stat(atmBinary); err != nil {
		t.Skipf("seeded atm binary not present: %v", err)
	}
	if info, err := os.Stat(storeRoot); err != nil || !info.IsDir() {
		t.Skipf("seeded planning store not present: %v", err)
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot = filepath.Dir(filepath.Dir(repoRoot)) // cmd/corvint-dashboard-snapshot -> repo root

	var stdout, stderr bytes.Buffer
	exit := runRoadmapContext(context.Background(), []string{
		"roadmap", "--root", repoRoot, "--atm", atmBinary, "--store", storeRoot,
		"--generated-at", testTime,
	}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	var snapshot struct {
		Schema     string `json:"schema"`
		Outcome    string `json:"outcome"`
		Milestones []struct {
			Milestone string `json:"milestone"`
			Tickets   []struct {
				LocalID string `json:"localId"`
			} `json:"tickets"`
		} `json:"milestones"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil {
		t.Fatalf("stdout = %s: %v", stdout.String(), err)
	}
	if snapshot.Schema != "corvint-dashboard-roadmap/0" || snapshot.Outcome != "OK" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	found := false
	for _, milestone := range snapshot.Milestones {
		for _, ticket := range milestone.Tickets {
			if ticket.LocalID == "IPR-10" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("IPR-10 not found in seeded roadmap output: %+v", snapshot.Milestones)
	}
}
