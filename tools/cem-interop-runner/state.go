package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	linkFile   = os.Link
	renameFile = os.Rename
	syncFile   = func(f *os.File) error { return f.Sync() }
)

func safeParent(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fail("output-parent")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", fail("output-parent")
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fail("output-parent")
	}
	current := parent
	for {
		info, err = os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", fail("output-parent")
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return parent, nil
}
func withObservationLock(path string, timeout time.Duration, fn func() error) error {
	parent, err := safeParent(path)
	if err != nil {
		return err
	}
	lockPath := filepath.Join(parent, ".cem-observation.lock")
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fail("observation-lock-open")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fail("observation-lock-owner")
	}
	if uid, ok := lockOwnerUID(info); !ok || uid != os.Geteuid() {
		return fail("observation-lock-owner")
	}
	if err = f.Chmod(0o600); err != nil {
		return fail("observation-lock-mode")
	}
	deadline := time.Now().Add(timeout)
	for {
		locked, lockErr := flockExclusive(int(f.Fd()))
		if lockErr != nil {
			return fail("observation-lock-acquire")
		}
		if locked {
			break
		}
		if time.Now().After(deadline) {
			return fail("observation-lock-timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer flockUnlock(int(f.Fd()))
	return fn()
}

func atomicWrite(path string, document map[string]any, mustAbsent bool, expectedDigest *string) error {
	payload, err := canonical(document)
	if err != nil {
		return fail("observation-write")
	}
	payload = append(payload, '\n')
	if len(payload) > observationLimit {
		return fail("observation-too-large")
	}
	parent, err := safeParent(path)
	if err != nil {
		return err
	}
	if mustAbsent == (expectedDigest != nil) {
		return fail("observation-write-contract")
	}
	nonce := make([]byte, 8)
	_, _ = rand.Read(nonce)
	tmp := filepath.Join(parent, ".cem-observation-"+fmt.Sprint(os.Getpid())+"-"+hex.EncodeToString(nonce))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fail("observation-write")
	}
	cleanup := func() { _ = f.Close(); _ = os.Remove(tmp) }
	defer cleanup()
	if err = f.Chmod(0o600); err != nil {
		return fail("observation-write")
	}
	if _, err = f.Write(payload); err != nil {
		return fail("observation-write")
	}
	if err = syncFile(f); err != nil {
		return fail("observation-write")
	}
	if err = f.Close(); err != nil {
		return fail("observation-write")
	}
	if mustAbsent {
		if err = linkFile(tmp, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fail("observation-exists")
			}
			if publishedMatches(path, payload) {
				return nil
			}
			return fail("observation-commit-uncertain")
		}
		if err = os.Remove(tmp); err != nil {
			return fail("observation-commit-uncertain")
		}
	} else {
		current, err := readObservationBytes(path)
		if err != nil {
			return err
		}
		if sha(current) != *expectedDigest {
			return fail("observation-changed")
		}
		if err = renameFile(tmp, path); err != nil {
			if publishedMatches(path, payload) {
				return nil
			}
			return fail("observation-commit-uncertain")
		}
	}
	directory, err := os.Open(parent)
	if err != nil {
		return fail("observation-commit-uncertain")
	}
	defer directory.Close()
	if err = syncFile(directory); err != nil {
		if publishedMatches(path, payload) {
			return nil
		}
		return fail("observation-commit-uncertain")
	}
	return nil
}

func publishedMatches(path string, payload []byte) bool {
	published, err := readObservationBytes(path)
	return err == nil && bytes.Equal(published, payload)
}

func readObservationBytes(path string) ([]byte, error) {
	parent, err := safeParent(path)
	if err != nil {
		return nil, err
	}
	_ = parent
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fail("observation-unreadable")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fail("observation-not-regular")
	}
	if info.Mode().Perm() != 0o600 {
		return nil, fail("observation-not-private")
	}
	if info.Size() > observationLimit {
		return nil, fail("observation-too-large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fail("observation-unreadable")
	}
	return data, nil
}
func loadObservation(path string) (map[string]any, string, error) {
	data, err := readObservationBytes(path)
	if err != nil {
		return nil, "", err
	}
	document, err := strictObject(data, observationLimit, "observation")
	if err != nil {
		return nil, "", err
	}
	if err = validateObservation(document); err != nil {
		return nil, "", err
	}
	return document, sha(data), nil
}

func validateObservation(d map[string]any) error {
	if !exactKeys(d, "doctor", "lane", "profile", "publication", "runs", "state") || d["profile"] != "cem-external-interop-observation/1" || d["publication"] != "COMMITTED" || d["lane"] != "consumer" || !oneOf(d["state"], "STARTED", "PASS", "FAIL") {
		return fail("observation-shape")
	}
	doctor, ok := d["doctor"].(map[string]any)
	if !ok || !exactKeys(doctor, "artifactCount", "corvintCommit", "corvintTreeState", "durationNs", "gitObjectFormats", "manifestSha256", "packetSha256", "required", "state") {
		return fail("observation-shape")
	}
	if n, ok := integer(doctor["artifactCount"], nil); !ok || n != 51 {
		return fail("observation-shape")
	}
	if doctor["state"] != "PASS" || !oneOf(doctor["corvintTreeState"], "clean", "modified", "standalone") || !validDigest(doctor["manifestSha256"]) || !validDigest(doctor["packetSha256"]) {
		return fail("observation-shape")
	}
	if _, ok := integer(doctor["durationNs"], nil); !ok {
		return fail("observation-shape")
	}
	required, ok := doctor["required"].(map[string]any)
	if !ok || !countsEqual(required, expectedCounts) {
		return fail("observation-shape")
	}
	formats, ok := doctor["gitObjectFormats"].(map[string]any)
	if !ok || !exactKeys(formats, "sha1", "sha256") || formats["sha1"] != "PASS" || formats["sha256"] != "PASS" {
		return fail("observation-shape")
	}
	commit := doctor["corvintCommit"]
	if commit != nil {
		v, ok := commit.(string)
		if !ok || (len(v) != 40 && len(v) != 64) || !lowerHex(v) {
			return fail("observation-shape")
		}
	}
	if (doctor["corvintTreeState"] == "standalone") != (commit == nil) {
		return fail("observation-shape")
	}
	runs, ok := d["runs"].([]any)
	if !ok || len(runs) > maxRuns {
		return fail("observation-shape")
	}
	for i, raw := range runs {
		run, ok := raw.(map[string]any)
		if !ok || !exactKeys(run, "cases", "counts", "durationNs", "implementationSha256", "manifestSha256", "packetSha256", "sequence", "state") {
			return fail("observation-shape")
		}
		seq, ok := integer(run["sequence"], nil)
		if !ok || seq != int64(i+1) || !oneOf(run["state"], "PASS", "FAIL") || run["manifestSha256"] != doctor["manifestSha256"] || run["packetSha256"] != doctor["packetSha256"] || !validDigest(run["implementationSha256"]) {
			return fail("observation-shape")
		}
		if _, ok = integer(run["durationNs"], nil); !ok {
			return fail("observation-shape")
		}
		cases, ok := run["cases"].([]any)
		if !ok || len(cases) != 32 {
			return fail("observation-shape")
		}
		groups := map[string]int{"valid": 0, "invalid": 0, "drift": 0}
		states := map[string]int{"pass": 0, "fail": 0, "unsupported": 0}
		for j, rawCase := range cases {
			c, ok := rawCase.(map[string]any)
			if !ok || !exactKeys(c, "durationNs", "exitCode", "failureCode", "group", "name", "state") {
				return fail("observation-shape")
			}
			group, name := fmt.Sprint(c["group"]), fmt.Sprint(c["name"])
			if [2]string{group, name} != expectedMatrix[j] || !oneOf(c["state"], "PASS", "FAIL", "UNSUPPORTED") || !validToken(name, 80) {
				return fail("observation-shape")
			}
			if _, ok = integer(c["durationNs"], nil); !ok {
				return fail("observation-shape")
			}
			if c["exitCode"] != nil {
				if _, ok := c["exitCode"].(json.Number); !ok {
					return fail("observation-shape")
				}
			}
			if c["failureCode"] != nil && !validToken(fmt.Sprint(c["failureCode"]), 64) {
				return fail("observation-shape")
			}
			if (c["state"] == "PASS") != (c["failureCode"] == nil) {
				return fail("observation-shape")
			}
			groups[group]++
			states[strings.ToLower(fmt.Sprint(c["state"]))]++
		}
		if !intMapEqual(groups, expectedCounts) {
			return fail("observation-shape")
		}
		counts, ok := run["counts"].(map[string]any)
		if !ok || !countsEqual(counts, states) {
			return fail("observation-shape")
		}
		want := "FAIL"
		if states["pass"] == 32 && states["fail"] == 0 && states["unsupported"] == 0 {
			want = "PASS"
		}
		if run["state"] != want {
			return fail("observation-shape")
		}
	}
	want := "STARTED"
	if len(runs) > 0 {
		want = runs[len(runs)-1].(map[string]any)["state"].(string)
	}
	if d["state"] != want {
		return fail("observation-shape")
	}
	return nil
}
func validDigest(v any) bool { s, ok := v.(string); return ok && len(s) == 64 && lowerHex(s) }
func validToken(s string, max int) bool {
	if len(s) < 1 || len(s) > max {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func oneOf(v any, values ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, x := range values {
		if s == x {
			return true
		}
	}
	return false
}
func countsEqual(v map[string]any, want map[string]int) bool {
	if len(v) != len(want) {
		return false
	}
	for k, n := range want {
		x, ok := integer(v[k], nil)
		if !ok || x != int64(n) {
			return false
		}
	}
	return true
}
func intMapEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
