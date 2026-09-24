// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		gitTest(t, root, args...)
	}
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "fixture")
	return root, strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
}
func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git: %v: %s", err, raw)
	}
	return string(raw)
}
func helperArgs(mode string, args ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestMeasureHelper$", "--", "measure-helper", mode}, args...)
}
func TestMeasureHelper(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "measure-helper" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	mode := os.Args[marker+1]
	args := os.Args[marker+2:]
	switch mode {
	case "good":
		fmt.Println("native fixture")
	case "random":
		fmt.Println(time.Now().UnixNano())
	case "stderr":
		fmt.Fprintln(os.Stderr, "fixture")
	case "failure":
		os.Exit(7)
	case "mutate":
		os.WriteFile("source.go", []byte("changed\n"), 0600)
	case "flood":
		for {
			os.Stdout.Write(bytes.Repeat([]byte("x"), 4096))
		}
	case "sleep":
		if len(args) > 0 {
			os.WriteFile(args[0], []byte(strconv.Itoa(os.Getpid())), 0600)
		}
		time.Sleep(time.Minute)
	case "descendant", "orphan":
		command := exec.Command(os.Args[0], helperArgs("sleep", args[0])[1:]...)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if command.Start() != nil {
			os.Exit(2)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(args[0]); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(3)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if mode == "orphan" {
			os.Exit(0)
		}
		command.Wait()
	default:
		os.Exit(9)
	}
	os.Exit(0)
}
func protocolFixture(head, mode string) protocol {
	return protocol{Profile: protocolProfile, ExpectedHead: head, ExpectedDirtyPaths: []string{}, Inputs: []string{"source.go"}, Workloads: []workload{{ID: "fixture", Argv: helperArgs(mode), CacheMode: "corvint-index", ExpectedExit: 0, TimeoutSeconds: 5}}}
}
func TestDistributionNearestRank(t *testing.T) {
	got := distribution([]int64{1, 2, 3, 4, 100})
	want := object{"n": 5, "p50": int64(3), "p95": int64(100), "max": int64(100), "mad": int64(1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v != %#v", got, want)
	}
}
func TestProtocolClosedAndRejectsRetiredRuntime(t *testing.T) {
	p := protocolFixture(strings.Repeat("a", 40), "good")
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "protocol.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProtocol(path); err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		bytes.Replace(raw, []byte(`"profile":`), []byte(`"extra":true,"profile":`), 1),
		bytes.Replace(raw, []byte(`"profile":`), []byte(`"profile":"duplicate","profile":`), 1),
		bytes.Replace(raw, []byte(`"expectedExit":0`), []byte(`"expectedExit":true`), 1),
		bytes.Replace(raw, []byte(`"timeoutSeconds":5`), []byte(`"timeoutSeconds":0`), 1),
	}
	for _, bad := range cases {
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadProtocol(path); err == nil {
			t.Fatalf("invalid protocol accepted: %s", bad)
		}
	}
	if _, err := expandArgv([]string{"{python}", "-m", "corvint_cli"}, "/repository"); err == nil {
		t.Fatal("retired Python placeholder accepted")
	}
}

// BRAIN-DOG-013 and GOC-V0-008: native measurement retains correctness,
// uncertainty, ordering and sample qualification independently of latency.
func TestNativeMeasurementAndMinimumClaimSamples(t *testing.T) {
	root, head := testRepository(t)
	p := protocolFixture(head, "good")
	limit := int64(10000)
	p.Workloads[0].MaxP95WallMs = &limit
	p.Workloads = append(p.Workloads, workload{ID: "second", Argv: helperArgs("good"), CacheMode: "none", TimeoutSeconds: 5})
	artifact, err := measure(context.Background(), root, p, 2)
	if err != nil {
		t.Fatal(err)
	}
	if artifact["profile"] != profile || artifact["valid"] != true || artifact["checksState"] != "NOT_RUN" {
		t.Fatalf("artifact: %#v", artifact)
	}
	samples := artifact["samples"].([]object)
	if len(samples) != 8 {
		t.Fatal(len(samples))
	}
	for i, want := range []string{"fixture", "second", "second", "fixture", "fixture", "second", "second", "fixture"} {
		if samples[i]["workloadId"] != want {
			t.Fatalf("order[%d]=%v", i, samples[i])
		}
	}
	for _, sample := range samples {
		for _, key := range []string{"wallNs", "userCpuNs", "systemCpuNs", "maxRssBytes"} {
			if sample[key].(int64) < 0 {
				t.Fatal(sample)
			}
		}
		if sample["stdoutSha256"] != digest([]byte("native fixture\n")) {
			t.Fatal(sample)
		}
	}
	for _, value := range artifact["sessionObservability"].(object) {
		if value != "NOT_OBSERVED" {
			t.Fatal("fabricated session observability")
		}
	}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := writeOutput(path, artifact); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	canonicalRaw, _ := canonical(artifact)
	info, _ := os.Stat(path)
	if !bytes.Equal(raw, canonicalRaw) || info.Mode().Perm() != 0600 {
		t.Fatal("receipt is not canonical/private")
	}
}
func TestNativeMeasurementRejectsDriftAndBadOutput(t *testing.T) {
	for _, mode := range []string{"random", "stderr", "failure", "mutate"} {
		t.Run(mode, func(t *testing.T) {
			root, head := testRepository(t)
			artifact, err := measure(context.Background(), root, protocolFixture(head, mode), 1)
			if err != nil {
				t.Fatal(err)
			}
			if artifact["valid"] != false || len(artifact["invalidReasons"].([]string)) == 0 {
				t.Fatal("invalid run accepted")
			}
		})
	}
	root, head := testRepository(t)
	p := protocolFixture(head, "good")
	limit := int64(1)
	p.Workloads[0].MaxStdoutBytes = &limit
	artifact, err := measure(context.Background(), root, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if artifact["valid"] != true || artifact["checksState"] != "FAIL" {
		t.Fatal("stdout threshold bypassed")
	}
}
func TestProtocolInputsMustBeRegularFiles(t *testing.T) {
	root, head := testRepository(t)
	for _, kind := range []string{"missing", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(root, "input-"+kind)
			switch kind {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("source.go", path); err != nil {
					t.Fatal(err)
				}
			}
			p := protocolFixture(head, "good")
			p.Inputs = []string{filepath.Base(path)}
			identity, err := repositoryIdentity(context.Background(), root, p.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateInputs(identity, p.Inputs); err == nil {
				t.Fatal("non-regular input accepted")
			}
		})
	}
}
func assertGone(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if processGone(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("owned descendant %d survived", pid)
}
func TestTimeoutNormalExitAndCancellationReapDescendants(t *testing.T) {
	for _, mode := range []string{"descendant", "orphan"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			pid := filepath.Join(root, "pid")
			observation, err := runProcess(context.Background(), helperArgs(mode, pid), root, t.TempDir(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "descendant" && observation["timedOut"] != true {
				t.Fatal("timeout not reported")
			}
			assertGone(t, pid)
		})
	}
	t.Run("cancellation", func(t *testing.T) {
		root := t.TempDir()
		pid := filepath.Join(root, "pid")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { _, err := runProcess(ctx, helperArgs("descendant", pid), root, t.TempDir(), 30); done <- err }()
		deadline := time.Now().Add(5 * time.Second)
		for {
			if raw, err := os.ReadFile(pid); err == nil && len(raw) > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("helper did not start")
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		select {
		case err := <-done:
			if err != context.Canceled {
				t.Fatalf("cancel error=%v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cancellation hung")
		}
		assertGone(t, pid)
	})
}
func TestOutputFloodIsBounded(t *testing.T) {
	_, err := runProcess(context.Background(), helperArgs("flood"), t.TempDir(), t.TempDir(), 5)
	if err == nil || !strings.Contains(err.Error(), "capture bound") {
		t.Fatalf("flood error=%v", err)
	}
}
