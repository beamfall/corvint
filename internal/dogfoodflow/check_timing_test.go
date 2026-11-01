package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// DCW-V0-942 (V1-3625): a check writes one row per phase, the phases are
// contiguous so the rows sum to the wall time, every verifier command has its
// own row, and the wrapper's build rows are prepended only when its file names
// this process.
func TestDCWV0942CheckWritesPhaseTimings(t *testing.T) {
	evidence := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sleeper := func(delay time.Duration) Runner {
		return Runner{Path: filepath.Join(t.TempDir(), "verifier"), Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			time.Sleep(delay)
			return 0
		}}
	}
	base, tree := sleeper(30*time.Millisecond), sleeper(10*time.Millisecond)
	newRun := func() (*check, *bytes.Buffer) {
		var stderr bytes.Buffer
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{BaseVerifier: base, TreeVerifier: tree}, io.Discard, &stderr)
		c.evidence = evidence
		c.operationUnlock = func() {}
		return c, &stderr
	}
	buildFile := buildTimingsPath(evidence, os.Getpid())
	started := time.Now().Add(-2 * time.Second)
	buildRows := fmt.Sprintf("%s\npid %%d\nstarted %d.%06d\nsetup\t-\t0.250\nbuild-tree\treuse\t0.250\nbuild-base\tbuild\t1.000\n",
		checkBuildProfile, started.Unix(), started.Nanosecond()/1000)

	// A concurrent wrapper's build file, and a file at this process's name
	// headed with another PID, are not counted and stay.
	other := buildTimingsPath(evidence, os.Getpid()+1)
	write(other, fmt.Sprintf(buildRows, os.Getpid()+1))
	write(buildFile, fmt.Sprintf(buildRows, os.Getpid()+1))
	c, _ := newRun()
	c.adoptBuildTimings()
	c.writeTimings()
	if rows := readTimingRows(t, evidence); len(rows) != 1 || rows[0][0] != "startup" {
		t.Fatalf("a foreign build file was counted: %q", rows)
	}
	if !isRegular(buildFile) || !isRegular(other) {
		t.Fatal("a foreign build file was removed")
	}
	// A run that does not hold the operation lock leaves the previous file.
	before := readFile(evidence + "/" + checkTimings)
	c, _ = newRun()
	c.operationUnlock = nil
	if _, _, detail := c.writeTimings(); detail != "" || !bytes.Equal(readFile(evidence+"/"+checkTimings), before) {
		t.Fatalf("a run without the lock wrote timings: %q", detail)
	}

	// This process's build file is prepended and consumed; each phase and
	// verifier command is its own row, and the rows sum to the total.
	write(buildFile, fmt.Sprintf(buildRows, os.Getpid()))
	c, stderr := newRun()
	c.adoptBuildTimings()
	if isRegular(buildFile) {
		t.Fatal("the matching build file was not consumed")
	}
	c.clock.to("lock-wait")
	time.Sleep(5 * time.Millisecond)
	c.clock.to("binding")
	c.verifyAll("cem", "status", "--map", "x")
	c.verifyAll("cem", "status", "--map", "x")
	c.verify(tree, "dogfood-ocm", "status")
	want := []string{"setup\t-", "build-tree\treuse", "build-base\tbuild", "startup\t-", "lock-wait\t-", "binding\t-",
		"verifier-base\tcem status", "verifier-tree\tcem status", "verifier-tree\tdogfood-ocm status"}
	total, phases, detail := c.writeTimings()
	if phases != len(want) || detail != evidence+"/"+checkTimings {
		t.Fatalf("writeTimings: phases=%d detail=%q", phases, detail)
	}
	rows := readTimingRows(t, evidence)
	var got []string
	sum := 0.0
	seconds := map[string]float64{}
	for _, row := range rows {
		got = append(got, row[0]+"\t"+row[1])
		value, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			t.Fatalf("row %q: %v", row, err)
		}
		sum += value
		seconds[row[0]+"\t"+row[1]] = value
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if seconds["verifier-base\tcem status"] < 0.06 || seconds["build-base\tbuild"] != 1 {
		t.Fatalf("verifier or build seconds lost: %v", seconds)
	}
	if stderr.Len() != 0 {
		t.Fatalf("the timings reached stderr: %q", stderr)
	}
	// The total runs from the wrapper's start; the rows cover it within 5%
	// (the wrapper rows are rounded to the millisecond).
	if total < 2 || math.Abs(sum-total) > 0.05*total {
		t.Fatalf("rows sum %.3f, total %.3f", sum, total)
	}

	// The timing files never enter the DCW-V0-347 inputs digest.
	if !isTimingFile(checkTimings) || !isTimingFile(filepath.Base(other)) || isTimingFile("local-outcome.json") {
		t.Fatal("a timing file is a reuse input")
	}
	// An unknown evidence directory writes nothing.
	c, _ = newRun()
	c.evidence = ""
	if _, _, detail := c.writeTimings(); detail != "" {
		t.Fatalf("wrote timings without an evidence directory: %s", detail)
	}
}

func readTimingRows(t *testing.T, evidence string) [][]string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(readFile(evidence+"/"+checkTimings)), "\n"), "\n")
	if len(lines) == 0 || lines[0] != "phase\tdetail\tseconds" {
		t.Fatalf("timings header: %q", lines)
	}
	var rows [][]string
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("timings row %q", line)
		}
		rows = append(rows, fields)
	}
	return rows
}
