package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		os.Exit(helperConsumer(os.Args[2:]))
	}
	os.Exit(m.Run())
}
func helperConsumer(args []string) int {
	values := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		values[args[i]] = args[i+1]
	}
	mapRaw, err := os.ReadFile(values["--map"])
	if err != nil {
		return 2
	}
	p, err := loadPacket()
	if err != nil {
		return 2
	}
	digest := sha(mapRaw)
	group := ""
	var matched map[string]any
	for _, g := range []string{"valid", "invalid"} {
		for _, c := range manifestArray(p.manifest, g) {
			raw, _ := artifactBytes(p, c["map"], "map")
			if sha(raw) == digest {
				group = g
				matched = c
				break
			}
		}
	}
	target := values["--target"]
	accept := group == "valid"
	drift := []any{}
	if target != "" {
		group = "drift"
		for _, c := range manifestArray(p.manifest, "drift") {
			if c["targetRevision"] == target {
				matched = c
				break
			}
		}
		if matched == nil {
			return 2
		}
		accept = matched["accept"] == true
		doc, err := strictObject(mapRaw, manifestLimit, "map")
		if err != nil {
			return 2
		}
		e := doc["evidence"].([]any)[0].(map[string]any)
		drift = append(drift, map[string]any{"evidenceId": e["id"], "path": e["path"], "status": matched["status"], "targetBlobOid": matched["targetBlobOid"], "targetSpan": matched["targetSpan"]})
	}
	if matched == nil {
		return 2
	}
	if group == "invalid" {
		accept = false
	}
	raw, _ := canonical(map[string]any{"accept": accept, "spec": "cem/0.1", "drift": drift})
	_, _ = os.Stdout.Write(append(raw, '\n'))
	if accept {
		return 0
	}
	return 1
}

func withKit(t *testing.T, root string) {
	t.Helper()
	old := kitRoot
	kitRoot = root
	t.Cleanup(func() { kitRoot = old })
}
func copyKit(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "kit")
	if err := filepath.Walk(kitRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(kitRoot, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, info.Mode().Perm())
	}); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestDoctorAndPacketIdentity(t *testing.T) {
	p, err := loadPacket()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.artifacts) != 51 || p.manifestDigest != expectedManifestSHA256 {
		t.Fatal("frozen packet closure")
	}
	files := map[string][]byte{}
	for _, name := range packetIdentityFiles {
		raw, err := os.ReadFile(filepath.Join(kitRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = raw
	}
	if framedPacketDigest(files) != p.packetDigest {
		t.Fatal("framed packet identity")
	}
	copied := copyKit(t)
	withKit(t, copied)
	before, err := loadPacket()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(copied, "START-HERE.md")
	raw, _ := os.ReadFile(path)
	_ = os.WriteFile(path, append(raw, []byte("\nidentity change\n")...), 0o600)
	after, err := loadPacket()
	if err != nil {
		t.Fatal(err)
	}
	if before.manifestDigest != after.manifestDigest || before.packetDigest == after.packetDigest {
		t.Fatal("public file was not independently packet-bound")
	}
	if _, err = readPacketPath(copied, "/absolute", 1, "packet-file"); err == nil || err.Error() != "packet-file-path" {
		t.Fatalf("absolute packet path: %v", err)
	}
}

func TestManifestAndArtifactTamperBeforeImplementation(t *testing.T) {
	for _, combined := range []bool{false, true} {
		copied := copyKit(t)
		if combined {
			artifact := filepath.Join(copied, "patches/supported.patch")
			raw, _ := os.ReadFile(artifact)
			raw = append(raw, []byte("tamper\n")...)
			_ = os.WriteFile(artifact, raw, 0o600)
			manifestPath := filepath.Join(copied, "manifest.json")
			manifestRaw, _ := os.ReadFile(manifestPath)
			var manifest map[string]any
			_ = json.Unmarshal(manifestRaw, &manifest)
			manifest["artifactSha256"].(map[string]any)["patches/supported.patch"] = sha(raw)
			changed, _ := canonical(manifest)
			_ = os.WriteFile(manifestPath, changed, 0o600)
		} else {
			manifest := filepath.Join(copied, "manifest.json")
			raw, _ := os.ReadFile(manifest)
			_ = os.WriteFile(manifest, append(raw, ' '), 0o600)
		}
		withKit(t, copied)
		if _, err := loadPacket(); err == nil || err.Error() != "manifest-digest" {
			t.Fatalf("combined=%v: %v", combined, err)
		}
	}
}

func TestArtifactTamperRefused(t *testing.T) {
	copied := copyKit(t)
	path := filepath.Join(copied, "patches/supported.patch")
	raw, _ := os.ReadFile(path)
	_ = os.WriteFile(path, append(raw, []byte("tamper\n")...), 0o600)
	withKit(t, copied)
	if _, err := loadPacket(); err == nil || err.Error() != "artifact-digest" {
		t.Fatalf("artifact tamper: %v", err)
	}
}

func TestNativeConsumerExactMatrixAndPrivateObservation(t *testing.T) {
	ctx := context.Background()
	observation := filepath.Join(t.TempDir(), "observation.json")
	started, err := startObservation(ctx, observation)
	if err != nil {
		t.Fatal(err)
	}
	if started["state"] != "STARTED" {
		t.Fatal("start state")
	}
	implementationPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := consume(ctx, observation, implementationPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "PASS" {
		t.Fatalf("consumer: %s", debugJSON(result))
	}
	runs := result["runs"].([]any)
	run := runs[0].(map[string]any)
	counts := run["counts"].(map[string]int)
	if counts["pass"] != 32 || counts["fail"] != 0 || counts["unsupported"] != 0 {
		t.Fatalf("counts: %v", counts)
	}
	cases := run["cases"].([]any)
	for i, raw := range cases {
		c := raw.(map[string]any)
		if [2]string{fmt.Sprint(c["group"]), fmt.Sprint(c["name"])} != expectedMatrix[i] {
			t.Fatalf("case %d identity", i)
		}
	}
	swapped := append([]any(nil), cases...)
	swapped[0], swapped[1] = swapped[1], swapped[0]
	run["cases"] = swapped
	if err := validateObservation(result); err == nil || err.Error() != "observation-shape" {
		t.Fatalf("wrong ordered matrix: %v", err)
	}
	run["cases"] = cases
	first := cases[0].(map[string]any)
	oldFailure := first["failureCode"]
	first["failureCode"] = "impossible-pass"
	if err := validateObservation(result); err == nil || err.Error() != "observation-shape" {
		t.Fatalf("PASS/failure incoherence: %v", err)
	}
	first["failureCode"] = oldFailure
	info, _ := os.Stat(observation)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("observation mode %o", info.Mode().Perm())
	}
	serialized, _ := os.ReadFile(observation)
	for _, forbidden := range []string{filepath.Dir(observation), implementationPath, "PRIVATE_STDOUT_SENTINEL", "HTTP_PROXY", "PYTHONPATH"} {
		if bytes.Contains(serialized, []byte(forbidden)) {
			t.Fatalf("private content retained: %s", forbidden)
		}
	}
	if _, err = consume(ctx, observation, implementationPath, false); err == nil || err.Error() != "retry-required" {
		t.Fatalf("missing retry gate: %v", err)
	}
	second, err := consume(ctx, observation, implementationPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(second["runs"].([]any)) != 2 {
		t.Fatal("retry missing")
	}
	if _, err = consume(ctx, observation, implementationPath, true); err == nil || err.Error() != "run-limit" {
		t.Fatalf("run limit: %v", err)
	}
}

func TestEachCaseUsesFreshCopyOfCapturedImplementation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "consumer")
	marker := filepath.Join(root, "invocations")
	original := []byte("#!/bin/sh\nprintf '%s\\n' \"$0\" >> " + marker + "\nprintf '{\"accept\":true,\"spec\":\"cem/0.1\",\"drift\":[]}\\n'\n")
	if err := os.WriteFile(source, original, 0o700); err != nil {
		t.Fatal(err)
	}
	captured, digest, err := implementation(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, []byte("#!/bin/sh\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err = os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		result, err := invokeCase(context.Background(), captured, digest, root, []byte("{}"), nil, home, "valid", map[string]any{"name": name}, nil)
		if err != nil || result["state"] != "PASS" {
			t.Fatalf("%s: %v %v", name, result, err)
		}
	}
	lines := strings.Fields(string(mustRead(t, marker)))
	if len(lines) != 2 || lines[0] == lines[1] || lines[0] == source || lines[1] == source {
		t.Fatalf("case executables were not fresh private copies: %v", lines)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestObservationCASAndCreateRace(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "observation.json")
	results := make(chan string, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := startObservation(ctx, path)
			if err != nil {
				results <- err.Error()
			} else {
				results <- "created"
			}
		}()
	}
	wg.Wait()
	close(results)
	seen := map[string]int{}
	for v := range results {
		seen[v]++
	}
	// The start lock covers doctor, so the loser's stable failure depends on
	// how long the winner's doctor holds the lock against the five-second
	// acquisition deadline (CEM-EXT-007): either result is fail-closed.
	losers := seen["observation-exists"] + seen["observation-lock-timeout"]
	if seen["created"] != 1 || losers != 1 {
		t.Fatalf("create race: %v", seen)
	}
	if _, err := startObservation(ctx, path); err == nil || err.Error() != "observation-exists" {
		t.Fatalf("start over committed observation: %v", err)
	}
	document, digest, err := loadObservation(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	raw = append(raw, ' ')
	_ = os.WriteFile(path, raw, 0o600)
	if err = atomicWrite(path, document, false, &digest); err == nil || err.Error() != "observation-changed" {
		t.Fatalf("CAS: %v", err)
	}
}

func TestAtomicWriteNoReplacePreservesExistingDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	existing := []byte("{\"existing\":true}\n")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	document := map[string]any{"value": "replacement"}
	if err := atomicWrite(path, document, true, nil); err == nil || err.Error() != "observation-exists" {
		t.Fatalf("no-replace write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, existing) {
		t.Fatalf("existing destination changed: got %q, want %q", got, existing)
	}
}

func TestAtomicPublicationReconcilesEveryCommitPoint(t *testing.T) {
	basePath := t.TempDir()
	document := map[string]any{"value": "base"}
	originalLink, originalRename, originalSync := linkFile, renameFile, syncFile
	t.Cleanup(func() { linkFile = originalLink; renameFile = originalRename; syncFile = originalSync })
	t.Run("post-link", func(t *testing.T) {
		path := filepath.Join(basePath, "link.json")
		linkFile = func(old, new string) error {
			if err := originalLink(old, new); err != nil {
				return err
			}
			return syscall.EIO
		}
		defer func() { linkFile = originalLink }()
		if err := atomicWrite(path, document, true, nil); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		want, _ := canonical(document)
		if !bytes.Equal(raw, append(want, '\n')) {
			t.Fatal("link reconciliation bytes")
		}
	})
	t.Run("post-rename", func(t *testing.T) {
		path := filepath.Join(basePath, "rename.json")
		if err := atomicWrite(path, document, true, nil); err != nil {
			t.Fatal(err)
		}
		_, digest, err := loadRawObject(path)
		if err != nil {
			t.Fatal(err)
		}
		replacement := map[string]any{"value": "replacement"}
		renameFile = func(old, new string) error {
			if err := originalRename(old, new); err != nil {
				return err
			}
			return syscall.EIO
		}
		defer func() { renameFile = originalRename }()
		if err = atomicWrite(path, replacement, false, &digest); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("directory-fsync", func(t *testing.T) {
		path := filepath.Join(basePath, "fsync.json")
		syncFile = func(f *os.File) error {
			info, _ := f.Stat()
			if info.IsDir() {
				return syscall.EIO
			}
			return originalSync(f)
		}
		defer func() { syncFile = originalSync }()
		if err := atomicWrite(path, document, true, nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("uncertain", func(t *testing.T) {
		path := filepath.Join(basePath, "uncertain.json")
		linkFile = func(string, string) error { return syscall.EINTR }
		defer func() { linkFile = originalLink }()
		if err := atomicWrite(path, document, true, nil); err == nil || err.Error() != "observation-commit-uncertain" {
			t.Fatalf("uncertain publication: %v", err)
		}
	})
}
func loadRawObject(path string) (map[string]any, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	value, err := strictObject(raw, observationLimit, "raw")
	return value, sha(raw), err
}

func TestRunnerAbortLeavesObservationUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	if _, err := startObservation(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := consume(context.Background(), path, missing, false); err == nil {
		t.Fatal("missing implementation accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("runner abort changed observation")
	}
}

func TestVerifiedSnapshotIsOnlyFixtureInput(t *testing.T) {
	p, err := loadPacket()
	if err != nil {
		t.Fatal(err)
	}
	copied := copyKit(t)
	withKit(t, copied)
	for _, name := range []string{"maps/valid/supported-sha1.json", "patches/supported.patch", "repository/base/src/app.py"} {
		path := filepath.Join(copied, name)
		_ = os.WriteFile(path, []byte("tamper\n"), 0o600)
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	_ = os.Mkdir(home, 0o700)
	repo, err := makeRepository(context.Background(), root, home, p, "sha1")
	if err != nil {
		t.Fatal(err)
	}
	case0 := manifestArray(p.manifest, "valid")[0]
	mapRaw, _ := artifactBytes(p, case0["map"], "map")
	patch, _ := patchBytes(p, case0)
	exe, _ := os.Executable()
	implementationRaw, digest, err := implementation(exe)
	if err != nil {
		t.Fatal(err)
	}
	result, err := invokeCase(context.Background(), implementationRaw, digest, repo, mapRaw, patch, home, "valid", case0, nil)
	if err != nil || result["state"] != "PASS" {
		t.Fatalf("verified snapshot: %v %v", result, err)
	}
}

func TestObservationLockTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- withObservationLock(path, time.Second, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	err := withObservationLock(path, 50*time.Millisecond, func() error { return nil })
	close(release)
	<-done
	if err == nil || err.Error() != "observation-lock-timeout" {
		t.Fatalf("lock timeout: %v", err)
	}
}

func TestProcessBoundsTimeoutAndGroupCleanup(t *testing.T) {
	ctx := context.Background()
	for fd, want := range map[int]string{1: "stdout-bound-exceeded", 2: "stderr-bound-exceeded"} {
		script := fmt.Sprintf("head -c 70000 /dev/zero >&%d", fd)
		result, _, _, err := runProcess(ctx, []string{"/bin/sh", "-c", script}, t.TempDir(), minimalEnvironment(t.TempDir(), nil), 2*time.Second, captureLimit)
		if err != nil || result.ErrorCode != want {
			t.Fatalf("fd %d: %+v %v", fd, result, err)
		}
	}
	marker := filepath.Join(t.TempDir(), "pid")
	script := "sleep 60 & echo $! > " + marker + "; wait"
	result, _, _, err := runProcess(ctx, []string{"/bin/sh", "-c", script}, filepath.Dir(marker), minimalEnvironment(filepath.Dir(marker), nil), 100*time.Millisecond, captureLimit)
	if err != nil || result.ErrorCode != "timeout" {
		t.Fatalf("timeout: %+v %v", result, err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	_, _ = fmt.Sscan(string(raw), &pid)
	// The killed descendant is reparented to the host's subreaper, which may not
	// have reaped it yet: poll, and count an unreaped zombie as gone.
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) && !zombie(pid) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processAlive(pid) && !zombie(pid) {
		t.Fatalf("group descendant %d survived", pid)
	}
}

func zombie(pid int) bool {
	out, _ := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
	return strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

func TestJSONResourceCodes(t *testing.T) {
	deep := []byte(`{"value":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}")
	if _, err := strictObject(deep, captureLimit, "protocol-output"); err == nil || err.Error() != "protocol-output-nesting" {
		t.Fatalf("nesting: %v", err)
	}
	integer := []byte(`{"value":` + strings.Repeat("9", 129) + "}")
	if _, err := strictObject(integer, captureLimit, "protocol-output"); err == nil || err.Error() != "protocol-output-integer-too-long" {
		t.Fatalf("integer: %v", err)
	}
	if _, err := strictObject([]byte(`{"a":1,"a":2}`), captureLimit, "protocol-output"); err == nil || err.Error() != "protocol-output-invalid-json" {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestCaseResultHonesty(t *testing.T) {
	cases := []struct {
		group          string
		process        processResult
		output         map[string]any
		accept         bool
		drift          []any
		state, failure string
	}{{"invalid", processResult{DurationNS: 1, ExitCode: 0}, map[string]any{"accept": true, "spec": "cem/0.1", "drift": []any{}}, false, []any{}, "FAIL", "exit-mismatch"}, {"drift", processResult{DurationNS: 1, ExitCode: 0}, map[string]any{"accept": true, "spec": "cem/0.1", "drift": []any{}}, true, []any{map[string]any{"status": "stable"}}, "FAIL", "drift-mismatch"}, {"valid", processResult{DurationNS: 1, ExitCode: 2}, map[string]any{"accept": false, "spec": "cem/0.1", "drift": []any{}}, true, []any{}, "UNSUPPORTED", "implementation-unsupported"}}
	for _, tc := range cases {
		got := caseResult(tc.group, "synthetic-case", tc.process, tc.output, tc.accept, tc.drift)
		if got["state"] != tc.state || got["failureCode"] != tc.failure {
			t.Fatalf("%s: %v", tc.group, got)
		}
	}
}

func TestObservationRejectsMatrixAndUnknownContent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "observation.json")
	document, err := startObservation(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	document["invented"] = true
	raw, _ := canonical(document)
	_ = os.WriteFile(path, append(raw, '\n'), 0o600)
	if _, _, err = loadObservation(path); err == nil || err.Error() != "observation-shape" {
		t.Fatalf("unknown content: %v", err)
	}
}

func TestMinimalEnvironmentHasNoNetworkOrLanguageInjection(t *testing.T) {
	env := minimalEnvironment(t.TempDir(), nil)
	joined := strings.Join(env, "\n")
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "PYTHONPATH", "PYTHONHOME", "VIRTUAL_ENV", "PIP_"} {
		if strings.Contains(joined, name) {
			t.Fatalf("environment leaks %s", name)
		}
	}
}
