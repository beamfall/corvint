package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// TestPRSV1020VersionJSONNamesVerbsAndHosts (V1-0300): `corvint --version --json`
// emits the corvint-version/1 identity a plugin
// reads to detect binary skew: version, build, every dispatched verb and every
// admitted harness host. Plain --version keeps its text line, and any other
// argument after --json stays an argument error.
func TestPRSV1020VersionJSONNamesVerbsAndHosts(t *testing.T) {
	for _, args := range [][]string{{"--version", "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit %d, stderr %s", args, code, stderr.String())
		}
		var got struct {
			Profile, Tool, Version, Build string
			Commands, HarnessHosts        []string
		}
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("%v: %v in %s", args, err, stdout.String())
		}
		if got.Profile != "corvint-version/1" || got.Tool != "corvint" || got.Version != version || got.Build != build {
			t.Fatalf("%v: identity %+v", args, got)
		}
		if !slices.IsSorted(got.Commands) || len(got.Commands) != len(topLevelCommands) {
			t.Fatalf("%v: commands %v", args, got.Commands)
		}
		for _, verb := range topLevelCommands {
			if !slices.Contains(got.Commands, verb) {
				t.Fatalf("%v: commands lack %q", args, verb)
			}
		}
		if !slices.Equal(got.HarnessHosts, gokernel.HarnessHosts()) || len(got.HarnessHosts) == 0 {
			t.Fatalf("%v: harnessHosts %v", args, got.HarnessHosts)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.String() != "Corvint "+version+" (build "+build+")\n" {
		t.Fatalf("plain --version: exit %d, %q", code, stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"--version", "--json", "extra"}, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("--version --json extra: exit %d, stdout %q", code, stdout.String())
	}
}
