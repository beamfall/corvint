// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
)

func TestBaselineReplayHelper(t *testing.T) {
	if os.Getenv("SB_BASELINE_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	args = args[1:]
	switch args[0] {
	case "--version":
		fmt.Println("fixture planner")
	case "affected":
		id, _ := os.ReadFile("id")
		fifo := filepath.Join(os.Getenv("SB_BASELINE_ORDER_DIR"), "release-first")
		if string(id) == "first" && os.Getenv("SELECTION_BASELINE_JOBS") == "3" {
			if _, err := os.ReadFile(fifo); err != nil {
				panic(err)
			}
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("SB_BASELINE_ORDER_DIR"), string(id)), []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0o600); err != nil {
			panic(err)
		}
		if string(id) == "second" && os.Getenv("SELECTION_BASELINE_JOBS") == "3" {
			if err := os.WriteFile(fifo, []byte("done"), 0o600); err != nil {
				panic(err)
			}
		}
		fmt.Println(`{"plan":{"scope":"BOUNDED"}}`)
	case "test":
		for i, arg := range args {
			if arg == "--manifest" {
				if err := os.WriteFile(args[i+1], []byte(`{"scope":"NOTHING"}`), 0o600); err != nil {
					panic(err)
				}
			}
		}
	default:
		fmt.Println("NOTHING no packages")
	}
	os.Exit(0)
}

// ASD-V0-925: force the first change to finish after the second. Compare the
// complete JSON and progress bytes, rather than just comparing a sorted list.
func TestBaselineReplayParallelBytesASDV0925(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	if _, err := exec.LookPath("mkfifo"); err != nil {
		t.Skip("mkfifo unavailable")
	}
	root := t.TempDir()
	baselineFile(t, root, "go.mod", "module m\n")
	baselineFile(t, root, ".github/cishards/package-costs.json", `{"milliseconds":{"m/a":1}}`)
	baselineGit(t, root, "init", "-q")
	baselineGit(t, root, "add", ".")
	baselineGit(t, root, "commit", "-qm", "base")
	base := baselineGit(t, root, "rev-parse", "HEAD")
	set := changeSet{Profile: changesProfile}
	for _, id := range []string{"first", "second", "third"} {
		baselineFile(t, root, "id", id)
		baselineGit(t, root, "add", "id")
		baselineGit(t, root, "commit", "-qm", id)
		set.Changes = append(set.Changes, change{ID: id, Kind: "merged", Base: base, Head: baselineGit(t, root, "rev-parse", "HEAD")})
	}
	bin := filepath.Join(t.TempDir(), "planner")
	testbin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nexec '" + strings.ReplaceAll(testbin, "'", "'\\''") + "' -test.run=^TestBaselineReplayHelper$ -- \"$@\"\n"
	if err := fixture.WriteExecutable(bin, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SB_BASELINE_HELPER", "1")
	order := t.TempDir()
	if out, err := exec.Command("mkfifo", filepath.Join(order, "release-first")).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v: %s", err, out)
	}
	t.Setenv("SB_BASELINE_ORDER_DIR", order)
	var reports, progress [][]byte
	for _, jobs := range []string{"1", "3"} {
		t.Setenv("SELECTION_BASELINE_JOBS", jobs)
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		previous := os.Stderr
		os.Stderr = w
		result, replayErr := replay(context.Background(), root, set, "changes.json", bin, bin)
		os.Stderr = previous
		w.Close()
		lines, readErr := io.ReadAll(r)
		r.Close()
		if replayErr != nil || readErr != nil {
			t.Fatalf("replay: %v / %v", replayErr, readErr)
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		reports = append(reports, data)
		progress = append(progress, lines)
		for i, res := range result.Results {
			if res.ID != set.Changes[i].ID {
				t.Fatalf("result %d=%s", i, res.ID)
			}
		}
	}
	if !bytes.Equal(reports[0], reports[1]) || !bytes.Equal(progress[0], progress[1]) {
		t.Fatalf("parallel replay changed bytes\n%s\n%s\n%s\n%s", reports[0], reports[1], progress[0], progress[1])
	}
	finished := func(id string) int64 {
		data, err := os.ReadFile(filepath.Join(order, id))
		if err != nil {
			t.Fatal(err)
		}
		n, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if finished("second") >= finished("first") {
		t.Fatal("fixture did not complete out of order")
	}
}
