package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type packetSnapshot struct {
	manifest       map[string]any
	manifestDigest string
	artifacts      map[string][]byte
	packetDigest   string
}

var kitRoot = defaultKitRoot()

func defaultKitRoot() string {
	if v := os.Getenv("CORVINT_CEM_KIT_ROOT"); v != "" {
		return v
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Clean(filepath.Join(filepath.Dir(file), "../../interop/cem-0.1"))
	}
	return filepath.Join("interop", "cem-0.1")
}
func sha(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func relativePath(v any, code string) (string, error) {
	s, ok := v.(string)
	if !ok || s == "" || len([]byte(s)) > 512 || filepath.IsAbs(s) || strings.Contains(s, "\\") {
		return "", fail(code)
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return "", fail(code)
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return "", fail(code)
		}
	}
	return s, nil
}
func readRegular(path string, limit int, code string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fail(code + "-unreadable")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fail(code + "-not-regular")
	}
	if info.Size() > int64(limit) {
		return nil, fail(code + "-too-large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fail(code + "-unreadable")
	}
	if len(data) > limit {
		return nil, fail(code + "-too-large")
	}
	return data, nil
}
func readPacketPath(root, relative string, limit int, code string) ([]byte, error) {
	value, err := relativePath(relative, code+"-path")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(value, "/")
	if len(parts) > 16 {
		return nil, fail(code + "-path")
	}
	current := root
	for i, p := range parts {
		current = filepath.Join(current, p)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fail(code + "-unreadable")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fail(code + "-unreadable")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, fail(code + "-unreadable")
		}
	}
	return readRegular(current, limit, code)
}
func framedPacketDigest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	var width [8]byte
	for _, name := range names {
		binary.BigEndian.PutUint32(width[:4], uint32(len([]byte(name))))
		_, _ = h.Write(width[:4])
		_, _ = h.Write([]byte(name))
		binary.BigEndian.PutUint64(width[:], uint64(len(files[name])))
		_, _ = h.Write(width[:])
		_, _ = h.Write(files[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func loadPacket() (packetSnapshot, error) {
	manifestRaw, err := readPacketPath(kitRoot, "manifest.json", manifestLimit, "manifest")
	if err != nil {
		return packetSnapshot{}, err
	}
	manifestDigest := sha(manifestRaw)
	if manifestDigest != expectedManifestSHA256 {
		return packetSnapshot{}, fail("manifest-digest")
	}
	manifest, err := strictObject(manifestRaw, manifestLimit, "manifest")
	if err != nil {
		return packetSnapshot{}, err
	}
	if !exactKeys(manifest, "suite", "repository", "valid", "invalid", "drift", "producerJobs", "artifactSha256") || manifest["suite"] != "cem/0.1-interop" {
		return packetSnapshot{}, fail("manifest-shape")
	}
	if err = validateManifest(manifest); err != nil {
		return packetSnapshot{}, err
	}
	declared, ok := manifest["artifactSha256"].(map[string]any)
	if !ok || len(declared) != 51 {
		return packetSnapshot{}, fail("manifest-artifact-set")
	}
	names := make([]string, 0, len(declared))
	for n := range declared {
		names = append(names, n)
	}
	sort.Strings(names)
	artifacts := map[string][]byte{}
	total := 0
	for _, name := range names {
		path, err := relativePath(name, "manifest-artifact-path")
		if err != nil {
			return packetSnapshot{}, err
		}
		head := strings.SplitN(path, "/", 2)[0]
		if head != "maps" && head != "patches" && head != "repository" && head != "targets" {
			return packetSnapshot{}, fail("manifest-artifact-path")
		}
		digest, ok := declared[name].(string)
		if !ok {
			return packetSnapshot{}, fail("artifact-digest")
		}
		data, err := readPacketPath(kitRoot, path, artifactLimit, "artifact")
		if err != nil {
			return packetSnapshot{}, err
		}
		total += len(data)
		if total > maxPacketBytes {
			return packetSnapshot{}, fail("packet-too-large")
		}
		if sha(data) != digest {
			return packetSnapshot{}, fail("artifact-digest")
		}
		artifacts[path] = data
	}
	packetFiles := map[string][]byte{}
	for _, name := range packetIdentityFiles {
		data := manifestRaw
		if name != "manifest.json" {
			data, err = readPacketPath(kitRoot, name, manifestLimit, "packet-file")
			if err != nil {
				return packetSnapshot{}, err
			}
		}
		packetFiles[name] = data
		total += len(data)
	}
	if total > maxPacketBytes {
		return packetSnapshot{}, fail("packet-too-large")
	}
	return packetSnapshot{manifest, manifestDigest, artifacts, framedPacketDigest(packetFiles)}, nil
}

func validateManifest(m map[string]any) error {
	repo, ok := m["repository"].(map[string]any)
	if !ok || !exactKeys(repo, "author", "message", "revisions", "root", "timestamp") {
		return fail("manifest-repository")
	}
	for _, k := range []string{"author", "message", "root", "timestamp"} {
		if _, ok := repo[k].(string); !ok {
			return fail("manifest-repository")
		}
	}
	if _, err := relativePath(repo["root"], "repository-root"); err != nil {
		return err
	}
	revisions, ok := repo["revisions"].(map[string]any)
	if !ok || !exactKeys(revisions, "sha1", "sha256") {
		return fail("manifest-repository")
	}
	for k, n := range map[string]int{"sha1": 40, "sha256": 64} {
		s, ok := revisions[k].(string)
		if !ok || len(s) != n || !lowerHex(s) {
			return fail("manifest-repository")
		}
	}
	observed := [][2]string{}
	names := map[string]bool{}
	for _, group := range []string{"valid", "invalid"} {
		cases, ok := m[group].([]any)
		if !ok || len(cases) != expectedCounts[group] {
			return fail("manifest-case-count")
		}
		for _, raw := range cases {
			c, ok := raw.(map[string]any)
			if !ok {
				return fail("manifest-case")
			}
			name, ok := c["name"].(string)
			if !ok || names[name] {
				return fail("manifest-case")
			}
			names[name] = true
			observed = append(observed, [2]string{group, name})
			if _, err := relativePath(c["map"], "map-path"); err != nil {
				return err
			}
			_, hasPatch := c["patch"]
			_, hasRecipe := c["patchRecipe"]
			if hasPatch == hasRecipe {
				return fail("manifest-case")
			}
			if hasPatch {
				if _, err := relativePath(c["patch"], "patch-path"); err != nil {
					return err
				}
			} else if c["patchRecipe"] != lfOverflowRecipe {
				return fail("manifest-case")
			}
			if group == "valid" && c["objectFormat"] != "sha1" && c["objectFormat"] != "sha256" {
				return fail("manifest-case")
			}
		}
	}
	drift, ok := m["drift"].([]any)
	if !ok || len(drift) != 6 {
		return fail("manifest-case-count")
	}
	driftNames := map[string]bool{}
	for _, raw := range drift {
		c, ok := raw.(map[string]any)
		if !ok || !exactKeys(c, "accept", "map", "message", "name", "patch", "status", "targetBlobOid", "targetPatch", "targetRevision", "targetSpan", "timestamp") {
			return fail("manifest-drift")
		}
		name, ok := c["name"].(string)
		if !ok || driftNames[name] {
			return fail("manifest-drift")
		}
		driftNames[name] = true
		observed = append(observed, [2]string{"drift", name})
		for _, k := range []string{"map", "patch"} {
			if _, err := relativePath(c[k], k+"-path"); err != nil {
				return err
			}
		}
		if c["targetPatch"] != nil {
			if _, err := relativePath(c["targetPatch"], "target-patch"); err != nil {
				return err
			}
		}
		accept, ok := c["accept"].(bool)
		_ = accept
		if !ok {
			return fail("manifest-drift")
		}
		status, ok := c["status"].(string)
		if !ok || !contains([]string{"stable", "relocated", "stale", "ambiguous", "deleted"}, status) {
			return fail("manifest-drift")
		}
		rev, ok := c["targetRevision"].(string)
		if !ok || len(rev) != 40 || !lowerHex(rev) {
			return fail("manifest-drift")
		}
	}
	if len(observed) != len(expectedMatrix) {
		return fail("manifest-matrix-identity")
	}
	for i := range observed {
		if observed[i] != expectedMatrix[i] {
			return fail("manifest-matrix-identity")
		}
	}
	jobs, ok := m["producerJobs"].([]any)
	if !ok || len(jobs) != 5 {
		return fail("manifest-producer")
	}
	return nil
}
func lowerHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func artifactBytes(p packetSnapshot, v any, code string) ([]byte, error) {
	path, err := relativePath(v, code+"-path")
	if err != nil {
		return nil, err
	}
	data, ok := p.artifacts[path]
	if !ok {
		return nil, fail(code + "-missing")
	}
	return append([]byte(nil), data...), nil
}
func manifestArray(m map[string]any, key string) []map[string]any {
	raw := m[key].([]any)
	out := make([]map[string]any, len(raw))
	for i, v := range raw {
		out[i] = v.(map[string]any)
	}
	return out
}
func debugJSON(v any) string { b, _ := canonical(v); return string(b) }
