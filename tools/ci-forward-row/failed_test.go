// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// runFailed runs failed.sh on STREAM with exit STATUS for shard 7 of 8, returning its stdout,
// the job summary it appended and its exit status.
func runFailed(t *testing.T, stream, status string) (string, string, int) {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "failed.sh is a POSIX shell script run by sh")
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatalf("failed.sh needs jq on PATH: %v", err)
	}
	summary := filepath.Join(t.TempDir(), "summary.md")
	cmd := exec.Command("sh", script(t, "failed.sh"), stream, status)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "SHARD=7", "SHARDS=8", "GITHUB_STEP_SUMMARY=" + summary}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	rc := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		rc = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("failed.sh: %v\n%s", err, stderr.String())
	}
	b, _ := os.ReadFile(summary)
	return string(out), string(b), rc
}

func TestAFPV0531FailedShardNamesTypedReasons(t *testing.T) {
	t.Run("the recorded failing shard names its failed tests", func(t *testing.T) {
		stream, err := filepath.Abs(filepath.Join("testdata", "streams", "pr38476173623-shard7-attempt1.json"))
		if err != nil {
			t.Fatal(err)
		}
		out, summary, rc := runFailed(t, stream, "1")
		if rc != 0 {
			t.Fatalf("failed.sh exited %d:\n%s", rc, out)
		}
		for _, want := range []string{
			"### FAILED: go-product-shard 7 of 8 exited 1\n",
			"Typed reasons (AFP-V0-531, V1-2689):\n",
			"- `github.com/Beamfall/corvint/internal/contextindex`: test-failures: `TestTaskContextCancellationNeverReturnsAPacket`, `TestTaskContextCancellationNeverReturnsAPacket/recency-on`, `TestTaskContextCancellationNeverReturnsAPacket/recency-on/lexical-term`\n",
		} {
			if !strings.Contains(summary, want) {
				t.Errorf("summary lacks %q:\n%s", want, summary)
			}
		}
		// The recorded stream keeps only its start, pass and fail events: no FAIL line to quote and
		// every package finished.
		if strings.Contains(summary, "Unfinished packages") || strings.Contains(summary, "FAIL lines") {
			t.Errorf("the recorded stream has no unfinished package and no output line:\n%s", summary)
		}
		if !strings.HasPrefix(out, "::error title=go-product-shard 7 failed::exit 1; github.com/Beamfall/corvint/internal/contextindex: test-failures") {
			t.Errorf("annotation: %q", out)
		}
	})
	cases := []struct {
		name, status string
		stream       []byte
		want, absent []string
	}{
		{
			name: "a build failure", status: "1",
			stream: lines(
				`{"Action":"start","Package":"example.com/m/a"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"# example.com/m/a\n"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"a.go:3:2: undefined: x\n"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"FAIL\texample.com/m/a [build failed]\n"}`,
				`{"Action":"fail","Package":"example.com/m/a","Elapsed":0}`,
				`{"Action":"start","Package":"example.com/m/b"}`,
				`{"Action":"pass","Package":"example.com/m/b","Elapsed":0.1}`),
			want: []string{"- `example.com/m/a`: build-failed\n", "FAIL\texample.com/m/a [build failed]"},
		},
		{
			name: "a Go 1.24 build-fail event", status: "1",
			stream: lines(
				`{"Action":"build-output","ImportPath":"example.com/m/a","Output":"# example.com/m/a\n"}`,
				`{"Action":"build-fail","ImportPath":"example.com/m/a"}`),
			want: []string{"- `example.com/m/a`: build-failed\n"},
		},
		{
			name: "a test timeout", status: "2",
			stream: lines(
				`{"Action":"start","Package":"example.com/m/a"}`,
				`{"Action":"run","Package":"example.com/m/a","Test":"TestSlow"}`,
				`{"Action":"output","Package":"example.com/m/a","Test":"TestSlow","Output":"panic: test timed out after 30m0s\n"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"FAIL\texample.com/m/a\t1800.1s\n"}`,
				`{"Action":"fail","Package":"example.com/m/a","Elapsed":1800.1}`),
			want:   []string{"exited 2\n", "- `example.com/m/a`: timeout\n", "panic: test timed out after 30m0s"},
			absent: []string{"test-failures"},
		},
		{
			name: "a runtime panic without a failed test", status: "1",
			stream: lines(
				`{"Action":"start","Package":"example.com/m/a"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"panic: runtime error: index out of range [3] with length 3\n"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"FAIL\texample.com/m/a\t0.5s\n"}`,
				`{"Action":"fail","Package":"example.com/m/a","Elapsed":0.5}`),
			want: []string{"- `example.com/m/a`: panic\n"},
		},
		{
			name: "a killed test binary", status: "1",
			stream: lines(
				`{"Action":"start","Package":"example.com/m/a"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"signal: killed\n"}`,
				`{"Action":"output","Package":"example.com/m/a","Output":"FAIL\texample.com/m/a\t12.0s\n"}`,
				`{"Action":"fail","Package":"example.com/m/a","Elapsed":12}`),
			want: []string{"- `example.com/m/a`: signal\n", "signal: killed"},
		},
		{
			name: "no failure report at all (V1-2689)", status: "1",
			stream: lines(
				`{"Action":"start","Package":"example.com/m/a"}`,
				`{"Action":"pass","Package":"example.com/m/a","Elapsed":0.1}`,
				`{"Action":"start","Package":"example.com/m/b"}`,
				`{"Action":"run","Package":"example.com/m/b","Test":"TestCut"}`),
			want: []string{
				"- no package-level fail event: go test ended without a failure report (killed, out of memory, a runner signal or a truncated stream)\n",
				"Unfinished packages (1): `example.com/m/b`\n",
			},
			absent: []string{"FAIL lines"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stream := filepath.Join(t.TempDir(), "stream.json")
			writeFile(t, stream, c.stream)
			out, summary, rc := runFailed(t, stream, c.status)
			if rc != 0 {
				t.Fatalf("failed.sh exited %d:\n%s", rc, out)
			}
			for _, want := range c.want {
				if !strings.Contains(summary, want) {
					t.Errorf("summary lacks %q:\n%s", want, summary)
				}
			}
			for _, absent := range c.absent {
				if strings.Contains(summary, absent) {
					t.Errorf("summary has %q:\n%s", absent, summary)
				}
			}
			if !strings.HasPrefix(out, "::error title=go-product-shard 7 failed::exit "+c.status+"; ") {
				t.Errorf("annotation: %q", out)
			}
		})
	}
	t.Run("a missing stream is advisory and never fails the step", func(t *testing.T) {
		out, summary, rc := runFailed(t, filepath.Join(t.TempDir(), "absent.json"), "1")
		if rc != 0 || !strings.Contains(summary, "- the go test -json stream could not be read\n") {
			t.Fatalf("exit %d\n%s\n%s", rc, out, summary)
		}
	})
}

func TestAFPV0531WorkflowBinding(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	m := regexp.MustCompile(`(?ms)^  go-product-shard:\n(.*?)\n  [a-z][a-z0-9-]*:\n`).FindStringSubmatch(ci)
	if m == nil {
		t.Fatal("ci.yml has no go-product-shard job")
	}
	job := m[1]
	if strings.Count(ci, "failed.sh") != 1 || strings.Count(job, "failed.sh") != 1 {
		t.Fatal("failed.sh must be called exactly once, inside the go-product-shard job")
	}
	// The typed-reason summary runs after the FLAKY decision, before the shard exits, and can
	// neither fail the step nor rescue it.
	order := []string{
		`sh .github/forward-rows/flaky.sh "$outcomes_dir" "$capture" &&`,
		`[ "$status" = 0 ] || sh .github/forward-rows/failed.sh "$capture" "$status" || true`,
		`[ "$status" = 0 ] || exit "$status"`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(job[at:], want)
		if i < 0 {
			t.Fatalf("go-product-shard lacks %q after offset %d", want, at)
		}
		at += i + len(want)
	}
}
