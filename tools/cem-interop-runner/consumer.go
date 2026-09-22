package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func copyFixtureTree(p packetSnapshot, source, destination string) error {
	prefix, err := relativePath(source, "repository-root")
	if err != nil {
		return err
	}
	prefix += "/"
	selected := []string{}
	for path := range p.artifacts {
		if strings.HasPrefix(path, prefix) {
			selected = append(selected, path)
		}
	}
	sortStrings(selected)
	if len(selected) == 0 || len(selected) > 50 {
		return fail("repository-fixture-set")
	}
	if err = os.Mkdir(destination, 0o700); err != nil {
		return fail("repository-fixture-set")
	}
	for _, path := range selected {
		relative := strings.TrimPrefix(path, prefix)
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fail("repository-fixture-set")
		}
		if err = os.WriteFile(target, p.artifacts[path], 0o600); err != nil {
			return fail("repository-fixture-set")
		}
	}
	return nil
}
func initializeRepository(ctx context.Context, root, home string, p packetSnapshot, objectFormat string) (string, error) {
	repo := p.manifest["repository"].(map[string]any)
	if err := copyFixtureTree(p, repo["root"].(string), root); err != nil {
		return "", err
	}
	if _, err := gitCommand(ctx, root, home, nil, "init", "-q", "--object-format="+objectFormat); err != nil {
		if objectFormat == "sha256" {
			return "", fail("sha256-unsupported")
		}
		return "", err
	}
	for _, pair := range [][2]string{{"user.name", "CEM Interop"}, {"user.email", "cem@example.invalid"}, {"core.autocrlf", "false"}, {"core.fileMode", "false"}} {
		if _, err := gitCommand(ctx, root, home, nil, "config", pair[0], pair[1]); err != nil {
			return "", err
		}
	}
	if _, err := gitCommand(ctx, root, home, nil, "add", "."); err != nil {
		return "", err
	}
	date := repo["timestamp"].(string)
	if _, err := gitCommand(ctx, root, home, &date, "commit", "--no-gpg-sign", "-qm", repo["message"].(string)); err != nil {
		return "", err
	}
	return root, nil
}
func makeRepository(ctx context.Context, root, home string, p packetSnapshot, objectFormat string) (string, error) {
	repo, err := initializeRepository(ctx, filepath.Join(root, "repo-"+objectFormat), home, p, objectFormat)
	if err != nil {
		return "", err
	}
	actual, err := gitCommand(ctx, repo, home, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	want := p.manifest["repository"].(map[string]any)["revisions"].(map[string]any)[objectFormat]
	if actual != want {
		return "", fail("repository-identity")
	}
	return repo, nil
}
func makeDriftRepository(ctx context.Context, root, home string, p packetSnapshot, c map[string]any, index int) (string, error) {
	repo, err := initializeRepository(ctx, filepath.Join(root, fmt.Sprintf("drift-%d", index)), home, p, "sha1")
	if err != nil {
		return "", err
	}
	if c["targetPatch"] != nil {
		raw, err := artifactBytes(p, c["targetPatch"], "target-patch")
		if err != nil {
			return "", err
		}
		patch := filepath.Join(root, fmt.Sprintf("target-%d.patch", index))
		if err = os.WriteFile(patch, raw, 0o400); err != nil {
			return "", fail("target-patch")
		}
		if _, err = gitCommand(ctx, repo, home, nil, "apply", "--whitespace=nowarn", patch); err != nil {
			return "", err
		}
		if _, err = gitCommand(ctx, repo, home, nil, "add", "-A"); err != nil {
			return "", err
		}
		date := c["timestamp"].(string)
		if _, err = gitCommand(ctx, repo, home, &date, "commit", "--no-gpg-sign", "-qm", c["message"].(string)); err != nil {
			return "", err
		}
	}
	actual, err := gitCommand(ctx, repo, home, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if actual != c["targetRevision"] {
		return "", fail("target-identity")
	}
	return repo, nil
}
func patchBytes(p packetSnapshot, c map[string]any) ([]byte, error) {
	patch, hasPatch := c["patch"]
	recipe, hasRecipe := c["patchRecipe"]
	if hasPatch == hasRecipe {
		return nil, fail("patch-selector")
	}
	if hasPatch {
		return artifactBytes(p, patch, "patch")
	}
	if recipe != lfOverflowRecipe {
		return nil, fail("patch-recipe")
	}
	payload := bytes.Repeat([]byte("x\n"), lfOverflowLines)
	if len(payload) != lfOverflowBytes || bytes.Count(payload, []byte{'\n'}) != lfOverflowLines || sha(payload) != lfOverflowSHA256 {
		return nil, fail("patch-recipe")
	}
	return payload, nil
}

func implementation(path string) ([]byte, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", fail("implementation-not-absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", fail("implementation-unreadable")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fail("implementation-not-regular")
	}
	if info.Size() > implementationLimit {
		return nil, "", fail("implementation-too-large")
	}
	if info.Mode().Perm()&0o111 == 0 {
		return nil, "", fail("implementation-not-executable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fail("implementation-unreadable")
	}
	if len(raw) > implementationLimit {
		return nil, "", fail("implementation-too-large")
	}
	return raw, sha(raw), nil
}
func privateImplementation(root string, data []byte) (string, error) {
	path := filepath.Join(root, "implementation")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
	if err != nil {
		return "", fail("implementation-copy")
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", fail("implementation-copy")
	}
	if err = f.Chmod(0o500); err != nil {
		f.Close()
		return "", fail("implementation-copy")
	}
	if err = f.Close(); err != nil {
		return "", fail("implementation-copy")
	}
	return path, nil
}

func caseResult(group, name string, process processResult, output map[string]any, expectedAccept bool, expectedDrift []any) map[string]any {
	failure := process.ErrorCode
	state := "PASS"
	exit := asInt(process.ExitCode)
	if exit == 2 && failure == nil {
		state = "UNSUPPORTED"
		failure = "implementation-unsupported"
	} else if failure != nil {
		state = "FAIL"
	} else if exit != map[bool]int{true: 0, false: 1}[expectedAccept] {
		state = "FAIL"
		failure = "exit-mismatch"
	} else if output == nil {
		state = "FAIL"
		failure = "protocol-output"
	} else if output["spec"] != "cem/0.1" || output["accept"] != expectedAccept {
		state = "FAIL"
		failure = "decision-mismatch"
	} else if !equalJSON(output["drift"], expectedDrift) {
		state = "FAIL"
		failure = "drift-mismatch"
	}
	return map[string]any{"durationNs": process.DurationNS, "exitCode": process.ExitCode, "failureCode": failure, "group": group, "name": name, "state": state}
}
func equalJSON(a, b any) bool { x, _ := canonical(a); y, _ := canonical(b); return bytes.Equal(x, y) }

func invokeCase(ctx context.Context, implementationBytes []byte, implementationDigest, repository string, mapBytes, patch []byte, home, group string, c map[string]any, target *string) (map[string]any, error) {
	inputRoot, err := os.MkdirTemp(filepath.Dir(home), "case-input-")
	if err != nil {
		return nil, fail("case-input")
	}
	defer os.RemoveAll(inputRoot)
	executable, err := privateImplementation(inputRoot, implementationBytes)
	if err != nil {
		return nil, err
	}
	privateMap, privatePatch := filepath.Join(inputRoot, "change.cem.json"), filepath.Join(inputRoot, "change.patch")
	if err = os.WriteFile(privateMap, mapBytes, 0o400); err != nil {
		return nil, fail("case-input")
	}
	if err = os.WriteFile(privatePatch, patch, 0o400); err != nil {
		return nil, fail("case-input")
	}
	argv := []string{executable, "verify", "--repository", repository, "--map", privateMap, "--patch", privatePatch}
	if target != nil {
		argv = append(argv, "--target", *target)
	}
	process, stdout, _, err := runProcess(ctx, argv, repository, minimalEnvironment(home, nil), caseTimeoutSeconds*time.Second, captureLimit)
	if err != nil {
		return nil, err
	}
	exeAfter, _ := os.ReadFile(executable)
	mapAfter, _ := os.ReadFile(privateMap)
	patchAfter, _ := os.ReadFile(privatePatch)
	if sha(exeAfter) != implementationDigest || !bytes.Equal(mapAfter, mapBytes) || !bytes.Equal(patchAfter, patch) {
		process.ErrorCode = "input-mutated"
	}
	var output map[string]any
	if process.ErrorCode == nil {
		output, err = strictObject(stdout, captureLimit, "protocol-output")
		if err != nil {
			if code, ok := err.(runnerError); ok {
				process.ErrorCode = string(code)
			} else {
				process.ErrorCode = "protocol-output-invalid-json"
			}
			output = nil
		}
	}
	expectedAccept := group == "valid" || (group == "drift" && c["accept"] == true)
	expectedDrift := []any{}
	if group == "drift" {
		doc, err := strictObject(mapBytes, manifestLimit, "map")
		if err != nil {
			return nil, err
		}
		evidence, ok := doc["evidence"].([]any)
		if !ok || len(evidence) != 1 {
			return nil, fail("drift-fixture")
		}
		e := evidence[0].(map[string]any)
		expectedDrift = append(expectedDrift, map[string]any{"evidenceId": e["id"], "path": e["path"], "status": c["status"], "targetBlobOid": c["targetBlobOid"], "targetSpan": c["targetSpan"]})
	}
	return caseResult(group, c["name"].(string), process, output, expectedAccept, expectedDrift), nil
}

func consumerRun(ctx context.Context, implementationPath string) (map[string]any, error) {
	started := time.Now()
	p, err := loadPacket()
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "cem-interop-consumer-")
	if err != nil {
		return nil, fail("temporary-directory")
	}
	defer os.RemoveAll(root)
	home := filepath.Join(root, "home")
	if err = os.Mkdir(home, 0o700); err != nil {
		return nil, fail("temporary-directory")
	}
	implementationBytes, implementationDigest, err := implementation(implementationPath)
	if err != nil {
		return nil, err
	}
	repos := map[string]string{}
	for _, format := range []string{"sha1", "sha256"} {
		repos[format], err = makeRepository(ctx, root, home, p, format)
		if err != nil {
			return nil, err
		}
	}
	driftCases := manifestArray(p.manifest, "drift")
	driftRepos := make([]string, len(driftCases))
	for i, c := range driftCases {
		driftRepos[i], err = makeDriftRepository(ctx, root, home, p, c, i)
		if err != nil {
			return nil, err
		}
	}
	cases := []any{}
	for _, group := range []string{"valid", "invalid"} {
		for _, c := range manifestArray(p.manifest, group) {
			patch, err := patchBytes(p, c)
			if err != nil {
				return nil, err
			}
			mapRaw, err := artifactBytes(p, c["map"], "map")
			if err != nil {
				return nil, err
			}
			format := "sha1"
			if v, ok := c["objectFormat"].(string); ok {
				format = v
			}
			result, err := invokeCase(ctx, implementationBytes, implementationDigest, repos[format], mapRaw, patch, home, group, c, nil)
			if err != nil {
				return nil, err
			}
			cases = append(cases, result)
			if time.Since(started) > totalTimeoutSeconds*time.Second {
				return nil, fail("total-timeout")
			}
		}
	}
	for i, c := range driftCases {
		patch, err := patchBytes(p, c)
		if err != nil {
			return nil, err
		}
		mapRaw, err := artifactBytes(p, c["map"], "map")
		if err != nil {
			return nil, err
		}
		target := c["targetRevision"].(string)
		result, err := invokeCase(ctx, implementationBytes, implementationDigest, driftRepos[i], mapRaw, patch, home, "drift", c, &target)
		if err != nil {
			return nil, err
		}
		cases = append(cases, result)
		if time.Since(started) > totalTimeoutSeconds*time.Second {
			return nil, fail("total-timeout")
		}
	}
	counts := map[string]int{"pass": 0, "fail": 0, "unsupported": 0}
	for _, raw := range cases {
		counts[strings.ToLower(raw.(map[string]any)["state"].(string))]++
	}
	state := "FAIL"
	if counts["pass"] == 32 && counts["fail"] == 0 && counts["unsupported"] == 0 {
		state = "PASS"
	}
	return map[string]any{"cases": cases, "counts": counts, "durationNs": time.Since(started).Nanoseconds(), "implementationSha256": implementationDigest, "manifestSha256": p.manifestDigest, "packetSha256": p.packetDigest, "sequence": 0, "state": state}, nil
}

func corvintPacketSource(ctx context.Context, home string) (any, string) {
	result, stdout, _, _ := runProcess(ctx, []string{"git", "-C", kitRoot, "rev-parse", "--show-toplevel", "HEAD"}, kitRoot, minimalEnvironment(home, nil), 20*time.Second, captureLimit)
	if result.ErrorCode != nil || asInt(result.ExitCode) != 0 {
		return nil, "standalone"
	}
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) != 2 || !filepath.IsAbs(lines[0]) || !lowerHex(lines[1]) || (len(lines[1]) != 40 && len(lines[1]) != 64) {
		return nil, "standalone"
	}
	relative, err := filepath.Rel(lines[0], kitRoot)
	if err != nil || strings.HasPrefix(relative, "..") {
		return nil, "standalone"
	}
	status, statusOut, _, _ := runProcess(ctx, []string{"git", "-C", lines[0], "status", "--porcelain=v1", "--untracked-files=all", "--", filepath.ToSlash(relative)}, lines[0], minimalEnvironment(home, nil), 20*time.Second, captureLimit)
	if status.ErrorCode != nil || asInt(status.ExitCode) != 0 {
		return lines[1], "modified"
	}
	if len(statusOut) == 0 {
		return lines[1], "clean"
	}
	return lines[1], "modified"
}
func doctor(ctx context.Context) (map[string]any, error) {
	started := time.Now()
	p, err := loadPacket()
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp("", "cem-interop-doctor-")
	if err != nil {
		return nil, fail("temporary-directory")
	}
	defer os.RemoveAll(root)
	home := filepath.Join(root, "home")
	if err = os.Mkdir(home, 0o700); err != nil {
		return nil, fail("temporary-directory")
	}
	commit, state := corvintPacketSource(ctx, home)
	for _, format := range []string{"sha1", "sha256"} {
		if _, err = makeRepository(ctx, root, home, p, format); err != nil {
			return nil, err
		}
	}
	for i, c := range manifestArray(p.manifest, "drift") {
		if _, err = makeDriftRepository(ctx, root, home, p, c, i); err != nil {
			return nil, err
		}
	}
	return map[string]any{"artifactCount": len(p.artifacts), "corvintCommit": commit, "corvintTreeState": state, "durationNs": time.Since(started).Nanoseconds(), "gitObjectFormats": map[string]any{"sha1": "PASS", "sha256": "PASS"}, "manifestSha256": p.manifestDigest, "packetSha256": p.packetDigest, "required": expectedCounts, "state": "PASS"}, nil
}
