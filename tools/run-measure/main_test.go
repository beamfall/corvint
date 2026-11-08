package main

import (
	"bytes"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

// The test binary doubles as the measured child: RUN_MEASURE_CHILD=alloc touches 64 MiB, and
// RUN_MEASURE_CHILD=exit3 exits 3.
func TestMain(m *testing.M) {
	switch os.Getenv("RUN_MEASURE_CHILD") {
	case "alloc":
		b := make([]byte, 64<<20)
		for i := range b {
			b[i] = byte(i)
		}
		os.Stdout.WriteString("touched " + strconv.Itoa(int(b[len(b)-1])) + "\n")
		os.Exit(0)
	case "exit3":
		os.Exit(3)
	}
	os.Exit(m.Run())
}

var report = regexp.MustCompile(`^run-measure: wall=[0-9]+\.[0-9]{3}s peak_rss_kib=([0-9]+|unknown) rc=([0-9]+) cmd=\S+ -test.run=none\n$`)

func measure(t *testing.T, child string) (rss string, rc int, out string) {
	t.Helper()
	t.Setenv("RUN_MEASURE_CHILD", child)
	var stdout, stderr bytes.Buffer
	rc = run([]string{"--", os.Args[0], "-test.run=none"}, nil, &stdout, &stderr)
	m := report.FindStringSubmatch(stderr.String())
	if m == nil {
		t.Fatalf("report line %q does not match %s", stderr.String(), report)
	}
	if got, _ := strconv.Atoi(m[2]); got != rc {
		t.Fatalf("reported rc=%s, returned %d", m[2], rc)
	}
	return m[1], rc, stdout.String()
}

// ASD-V0-570: the helper reports wall time and a normalized peak RSS, passes the child's output
// through, and exits with the child's code.
func TestASDV0570ReportsWallPeakRSSAndExitCode(t *testing.T) {
	rss, rc, out := measure(t, "alloc")
	if rc != 0 || out == "" {
		t.Fatalf("alloc child: rc=%d stdout=%q", rc, out)
	}
	if runtime.GOOS == "windows" {
		if rss != "unknown" {
			t.Fatalf("peak RSS on windows = %s, want unknown", rss)
		}
	} else if kib, _ := strconv.Atoi(rss); kib < 64<<10 || kib > 8<<20 {
		t.Fatalf("peak RSS of a 64 MiB child = %s KiB, want between 64 MiB and 8 GiB (a unit error shows as 1024x)", rss)
	}
	if _, rc, _ := measure(t, "exit3"); rc != 3 {
		t.Fatalf("exit3 child: rc=%d, want 3", rc)
	}
	var stderr bytes.Buffer
	if rc := run(nil, nil, nil, &stderr); rc != 2 {
		t.Fatalf("no command: rc=%d, want usage 2", rc)
	}
	if rc := run([]string{"/nonexistent/run-measure-child"}, nil, nil, &stderr); rc != 127 {
		t.Fatalf("missing command: rc=%d, want 127", rc)
	}
}
