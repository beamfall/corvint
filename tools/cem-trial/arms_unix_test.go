//go:build unix

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CRT-V0-005, CRT-V0-012, CRT-V0-015 (V1-2648): `run` drives all three arms
// end to end on one fixture pair with a built corvint. Each arm's prompt
// carries exactly its declared artefact set: control carries no map and never
// names Corvint or CEM; treatment carries the prepared cem/1.0 map, its
// `cem status` worklist and its `cem report`; seeded carries the same three
// plus the suggested evidence. The map-carrying lanes record a cem/1.0
// measurement and control records none.
func TestRunDrivesAllThreeArmsEndToEnd(t *testing.T) {
	repo := fixtureRepo(t)
	directory := t.TempDir()
	item := selectPilot(t, repo, directory).Tasks[0]
	corvintGo := buildCorvint(t)
	fakeAgent, err := filepath.Abs(filepath.Join("testdata", "fake-agent.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// The recording agent keeps every prompt it is given, outside the lane
	// clone, then replies as the fake agent does.
	prompts := t.TempDir()
	recorder := filepath.Join(t.TempDir(), "recording-agent.sh")
	script := "#!/bin/sh\nset -eu\nprintf '%s' \"$1\" > \"$(mktemp \"" + prompts + "/prompt.XXXXXX\")\"\nexec \"" + fakeAgent + "\" \"$1\"\n"
	if err := os.WriteFile(recorder, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	document, _, err := runTrial(context.Background(), []string{
		"--tasks", filepath.Join(directory, "tasks.json"), "--history", repo,
		"--arms", "control,treatment,seeded", "--corvint", corvintGo,
		"--agent", "script", "--agent-command", recorder, "--seed", "fixture",
		"--output", filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.Invalid != "" || len(document.Lanes) != 3 {
		t.Fatalf("invalid %q with %d lanes", document.Invalid, len(document.Lanes))
	}
	entries, err := os.ReadDir(prompts)
	if err != nil {
		t.Fatal(err)
	}
	byDigest := map[string]string{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(prompts, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		byDigest[digestOf(string(data))] = string(data)
	}
	if len(byDigest) != 3 {
		t.Fatalf("the agent saw %d distinct prompts, want one per arm", len(byDigest))
	}
	const (
		mapHeading       = "## Change Evidence Map"
		statusHeading    = "### Status worklist"
		reportHeading    = "### Report"
		suggestedHeading = "### Suggested evidence"
	)
	for _, record := range document.Lanes {
		if record.Error != "" {
			t.Fatalf("%s lane errored: %s", record.Arm, record.Error)
		}
		prompt, ok := byDigest[record.PromptSHA256]
		if !ok {
			t.Fatalf("%s lane's prompt digest matches no prompt the agent saw", record.Arm)
		}
		switch record.Arm {
		case "control":
			for _, leak := range []string{"Corvint", "corvint", "CEM", "cem/", "Change Evidence Map", mapHeading, suggestedHeading} {
				if strings.Contains(prompt, leak) {
					t.Fatalf("the control prompt names %q", leak)
				}
			}
			if record.CEM != nil || record.Suggested != nil {
				t.Fatalf("the control lane recorded map state %v or suggestions %v", record.CEM, record.Suggested)
			}
			continue
		case "treatment":
			if strings.Contains(prompt, suggestedHeading) || record.Suggested != nil {
				t.Fatal("the treatment arm carries the seeded arm's suggestions")
			}
		case "seeded":
			if !strings.Contains(prompt, suggestedHeading) || len(record.Suggested) == 0 {
				t.Fatalf("the seeded arm carries no suggested evidence: %v", record.Suggested)
			}
		default:
			t.Fatalf("unexpected arm %q", record.Arm)
		}
		for _, heading := range []string{mapHeading, statusHeading, reportHeading} {
			if strings.Count(prompt, heading) != 1 {
				t.Fatalf("the %s prompt carries %q %d times, want once", record.Arm, heading, strings.Count(prompt, heading))
			}
		}
		var carried struct {
			Spec  string `json:"spec"`
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(promptBlock(t, prompt, "### Map\n\n```json\n")), &carried); err != nil || carried.Spec != "cem/1.0" {
			t.Fatalf("the %s arm's map is %q, want cem/1.0: %v", record.Arm, carried.Spec, err)
		}
		if err := json.Unmarshal([]byte(promptBlock(t, prompt, statusHeading+"\n\n```json\n")), &carried); err != nil || carried.State != "accepted" {
			t.Fatalf("the %s arm's status worklist is %q: %v", record.Arm, carried.State, err)
		}
		if strings.TrimSpace(promptBlock(t, prompt, reportHeading+"\n\n```\n")) == "" {
			t.Fatalf("the %s arm's report is empty", record.Arm)
		}
		// The measurement must have run: an unobserved verdict or no hunk
		// count is a lane that lost its map target.
		counts, _ := record.CEM["counts"].(map[string]any)
		if _, observed := record.CEM["verify_ok"].(bool); !observed || counts["total"] != item.HunkCount || item.HunkCount < 2 {
			t.Fatalf("the %s lane recorded no observed cem/1.0 measurement: %v", record.Arm, record.CEM)
		}
	}
}

// promptBlock returns the fenced block that follows opening in a prompt.
func promptBlock(t *testing.T, prompt, opening string) string {
	t.Helper()
	_, rest, ok := strings.Cut(prompt, opening)
	if !ok {
		t.Fatalf("the prompt carries no block opened by %q", opening)
	}
	block, _, ok := strings.Cut(rest, "\n```")
	if !ok {
		t.Fatalf("the block opened by %q is not closed", opening)
	}
	return block
}
