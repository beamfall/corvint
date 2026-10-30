package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const profile = "corvint-human-documentation-compiler-conformance/0"
const observationProfile = "corvint-human-documentation-compiler-observation/0"
const planProfile = "corvint-human-documentation-plan/0"
const buildProfile = "corvint-human-documentation-build-receipt/0"
const environmentProfile = "corvint-doccompiler-environment-trust/0"
const denialProfile = "corvint-doccompiler-network-denial/0"
const resultProfile = "corvint-human-documentation-compiler-conformance-result/0"
const maxFileBytes = 1048576
const maxStreamBytes = 262144

var signalPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var caseIDs = []string{"corvint-material-plan", "build-environment-unavailable", "material-version-mismatch", "plugins-replace-default-search", "config-inherit-path-escape", "markdown-include-path-escape", "unknown-plugin-canary", "unknown-hook-canary", "privacy-remote-assets-canary", "offline-plugin-is-config-owned", "docs-dir-symlink-escape", "configured-site-dir-escape", "stale-plan-source", "hallucinated-supported-claim", "false-strict-build-pass", "subprocess-timeout-cleanup", "build-output-bound", "strict-build-project-environment", "offline-qualification-external-harness"}

type conformanceError string

func (e conformanceError) Error() string { return string(e) }
func fail(label, reason string)          { panic(conformanceError(label + ":" + reason)) }
func must[T any](value T, err error) T {
	if err != nil {
		panic(conformanceError(err.Error()))
	}
	return value
}
func str(value any) string { s, _ := value.(string); return s }
func has(value string, values ...string) bool {
	for _, s := range values {
		if value == s {
			return true
		}
	}
	return false
}
func canonical(value any, lf bool) []byte {
	raw := must(contextindex.CanonicalJSON(value))
	if lf {
		raw = append(raw, '\n')
	}
	return raw
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func digest(kind string, raw []byte) string {
	return hash(append([]byte(profile+"\x00"+kind+"\x00"), raw...))
}
func closed(value any, fields, label string) map[string]any {
	m, ok := value.(map[string]any)
	if !ok {
		fail(label, "not-object")
	}
	wanted := strings.Fields(fields)
	missing, extra := []string{}, []string{}
	for _, k := range wanted {
		if _, ok := m[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range m {
		if !has(k, wanted...) {
			extra = append(extra, k)
		}
	}
	if len(missing)+len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		fail(label, "closed-shape:missing="+strings.Join(missing, ",")+":extra="+strings.Join(extra, ","))
	}
	return m
}
func parseJSON(raw []byte, label string) any {
	if len(raw) > maxFileBytes {
		fail(label, "too-large")
	}
	if !utf8.Valid(raw) {
		fail(label, "invalid-json")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func() any
	read = func() any {
		token, err := d.Token()
		if err != nil {
			fail(label, "invalid-json")
		}
		switch token {
		case json.Delim('{'):
			m := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					fail(label, "invalid-json")
				}
				s, ok := key.(string)
				if !ok {
					fail(label, "invalid-json")
				}
				if _, ok = m[s]; ok {
					fail(label, "duplicate-key:"+s)
				}
				m[s] = read()
			}
			if _, err = d.Token(); err != nil {
				fail(label, "invalid-json")
			}
			return m
		case json.Delim('['):
			a := []any{}
			for d.More() {
				a = append(a, read())
			}
			if _, err = d.Token(); err != nil {
				fail(label, "invalid-json")
			}
			return a
		default:
			if n, ok := token.(json.Number); ok {
				i, err := n.Int64()
				if err != nil {
					fail(label, "invalid-integer")
				}
				return i
			}
			return token
		}
	}
	value := read()
	if _, err := d.Token(); err != io.EOF {
		fail(label, "invalid-json")
	}
	return value
}
func safeRelative(value any, label string) string {
	s, ok := value.(string)
	if !ok || s == "" || utf8.RuneCountInString(s) > 512 || strings.ContainsAny(s, "\\\x00") || path.IsAbs(s) || path.Clean(s) != s || s == "." {
		fail(label, "invalid-path")
	}
	for _, p := range strings.Split(s, "/") {
		if p == ".." {
			fail(label, "invalid-path")
		}
	}
	return s
}
func containedFile(root string, value any, label string) string {
	s := safeRelative(value, label)
	current := root
	for _, p := range strings.Split(s, "/") {
		current = filepath.Join(current, p)
		info, err := os.Lstat(current)
		if err != nil {
			fail(label, "missing")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			fail(label, "symlink")
		}
	}
	r := must(filepath.EvalSymlinks(root))
	r = must(filepath.Abs(r))
	resolved := must(filepath.EvalSymlinks(current))
	resolved = must(filepath.Abs(resolved))
	rel, err := filepath.Rel(r, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		fail(label, "escape")
	}
	info := must(os.Stat(resolved))
	if !info.Mode().IsRegular() {
		fail(label, "not-file")
	}
	if info.Size() > maxFileBytes {
		fail(label, "too-large")
	}
	return resolved
}
func readBounded(p, label string) []byte {
	f := must(os.Open(p))
	defer f.Close()
	raw := must(io.ReadAll(io.LimitReader(f, maxFileBytes+1)))
	if len(raw) > maxFileBytes {
		fail(label, "too-large")
	}
	return raw
}
func fileHash(p string) string { return hash(readBounded(p, "file")) }
func treeHash(root string) string {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fail("tree", "not-directory:"+filepath.Base(root))
	}
	paths := []string{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p != root {
			paths = append(paths, p)
		}
		return nil
	})
	must(true, err)
	sort.Slice(paths, func(i, j int) bool { return filepath.ToSlash(paths[i]) < filepath.ToSlash(paths[j]) })
	var b bytes.Buffer
	for _, p := range paths {
		rel := filepath.ToSlash(must(filepath.Rel(root, p)))
		info := must(os.Lstat(p))
		switch {
		case info.IsDir():
			b.WriteString("d\x00" + rel + "\x00")
		case info.Mode()&os.ModeSymlink != 0:
			b.WriteString("l\x00" + rel + "\x00" + must(os.Readlink(p)) + "\x00")
		case info.Mode().IsRegular():
			if info.Size() > maxFileBytes {
				fail("tree", "file-too-large:"+rel)
			}
			b.WriteString("f\x00" + rel + "\x00")
			b.Write(readBounded(p, "tree:file"))
			b.WriteByte(0)
		default:
			fail("tree", "unsupported-file-type:"+rel)
		}
	}
	return digest("tree", b.Bytes())
}
func array(value any, label string, min, max int) []any {
	a, ok := value.([]any)
	if !ok || len(a) < min || len(a) > max {
		fail(label, "invalid-list")
	}
	return a
}
func stringList(value any, label string, empty bool) []string {
	a := array(value, label, 0, 128)
	if !empty && len(a) == 0 {
		fail(label, "empty")
	}
	out := []string{}
	for _, item := range a {
		s := str(item)
		if !signalPattern.MatchString(s) {
			fail(label, "invalid-item")
		}
		if has(s, out...) {
			fail(label, "duplicate")
		}
		out = append(out, s)
	}
	return out
}
func loadCases(root string) []map[string]any {
	m := closed(parseJSON(readBounded(filepath.Join(root, "cases.json"), "cases"), "cases"), "cases profile", "cases")
	if m["profile"] != profile {
		fail("cases", "profile-mismatch")
	}
	values, ok := m["cases"].([]any)
	if !ok || len(values) != len(caseIDs) {
		fail("cases", "wrong-count")
	}
	out := []map[string]any{}
	for i, value := range values {
		label := fmt.Sprintf("case[%d]", i)
		c := closed(value, "allowed fixture id operation requiredSignals", label)
		id := str(c["id"])
		if !signalPattern.MatchString(id) {
			fail(label, "invalid-id")
		}
		if str(c["operation"]) == "" {
			fail(id, "invalid-operation")
		}
		fixture := filepath.Join(root, "fixtures", safeRelative(c["fixture"], id+":fixture"))
		info, err := os.Stat(fixture)
		_, configErr := os.Stat(filepath.Join(fixture, "mkdocs.yml"))
		if err != nil || !info.IsDir() || configErr != nil {
			fail(id, "missing-fixture")
		}
		allowed := array(c["allowed"], id+":allowed", 1, 2)
		seen := map[string]bool{}
		for j, value := range allowed {
			a := closed(value, "code decision", fmt.Sprintf("%s:allowed[%d]", id, j))
			decision, code := str(a["decision"]), str(a["code"])
			if !has(decision, "ABSTAINED", "BUILT", "PLANNED", "REJECTED") || !signalPattern.MatchString(code) {
				fail(id, "invalid-allowed-result")
			}
			key := decision + "\x00" + code
			if seen[key] {
				fail(id, "duplicate-allowed-result")
			}
			seen[key] = true
		}
		stringList(c["requiredSignals"], id+":required-signals", true)
		treeHash(fixture)
		if id != caseIDs[i] {
			fail("cases", "id-order-mismatch")
		}
		out = append(out, c)
	}
	return out
}
func corpusHash(root string, cases []map[string]any) string {
	fixtures := map[string]any{}
	for _, c := range cases {
		name := str(c["fixture"])
		fixtures[name] = treeHash(filepath.Join(root, "fixtures", name))
	}
	return digest("corpus", canonical(map[string]any{"cases": parseJSON(readBounded(filepath.Join(root, "cases.json"), "cases-digest"), "cases-digest"), "fixtures": fixtures}, false))
}
func materializeFixture(source, destination string) {
	must(true, os.Mkdir(destination, 0700))
	err := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == source {
			return nil
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		target := filepath.Join(destination, rel)
		info, e := d.Info()
		if e != nil {
			return e
		}
		switch {
		case d.Type()&os.ModeSymlink != 0:
			link, e := os.Readlink(p)
			if e != nil {
				return e
			}
			return os.Symlink(link, target)
		case d.IsDir():
			return os.Mkdir(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			return os.WriteFile(target, readBounded(p, "fixture"), info.Mode().Perm())
		default:
			return fmt.Errorf("fixture:unsupported-file-type")
		}
	})
	must(true, err)
	setupPath := filepath.Join(destination, ".fixture.json")
	if _, err := os.Stat(setupPath); os.IsNotExist(err) {
		return
	}
	setup := closed(parseJSON(readBounded(setupPath, "fixture-setup"), "fixture-setup"), "symlinks", "fixture-setup")
	links := array(setup["symlinks"], "fixture-setup:symlinks", 1, 128)
	for i, value := range links {
		link := closed(value, "path target targetSource", fmt.Sprintf("fixture-setup:symlink[%d]", i))
		linkPath := filepath.Join(destination, safeRelative(link["path"], "fixture-setup:path"))
		source := filepath.Join(destination, safeRelative(link["targetSource"], "fixture-setup:source"))
		target := str(link["target"])
		if target != "../"+filepath.Base(source) {
			fail("fixture-setup", "invalid-external-target")
		}
		external := filepath.Join(filepath.Dir(destination), filepath.Base(source))
		if _, err := os.Lstat(external); err == nil {
			fail("fixture-setup", "target-collision")
		}
		if _, err := os.Lstat(linkPath); err == nil {
			fail("fixture-setup", "target-collision")
		}
		must(true, os.Rename(source, external))
		must(true, os.Symlink(target, linkPath))
	}
}
func integer(value any, label string) int64 {
	switch n := value.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	}
	fail(label, "invalid-byte-range")
	return 0
}
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// Preserve blank lines, including consecutive Python splitlines separators.
	out := []string{}
	start := 0
	for i, r := range s {
		if r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x1c || r == 0x1d || r == 0x1e || r == 0x85 || r == 0x2028 || r == 0x2029 {
			out = append(out, s[start:i])
			start = i + utf8.RuneLen(r)
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
func validateEvidence(root string, value any, label string) string {
	m := closed(value, "lineEnd lineStart path sha256", label)
	p := containedFile(root, m["path"], label+":path")
	if m["sha256"] != fileHash(p) {
		fail(label, "digest-mismatch")
	}
	raw := readBounded(p, label)
	if !utf8.Valid(raw) {
		fail(label, "invalid-utf8")
	}
	lines := splitLines(string(raw))
	start, end := integer(m["lineStart"], label), integer(m["lineEnd"], label)
	if start < 1 || end < start || end > int64(len(lines)) {
		fail(label, "invalid-lines")
	}
	return contextindex.TrimPythonSpace(strings.Join(lines[start-1:end], "\n"))
}

func validatePlan(root, sourceDigest string, value any, label string) {
	p := closed(value, "applied claims navPatches profile reviewRequired sourceTreeSha256 writes", label)
	if p["profile"] != planProfile || p["sourceTreeSha256"] != sourceDigest {
		fail(label, "identity-mismatch")
	}
	if p["applied"] != false || p["reviewRequired"] != true {
		fail(label, "mutation-or-review-bypass")
	}
	for i, value := range array(p["writes"], label+":writes", 1, 64) {
		wl := fmt.Sprintf("%s:write[%d]", label, i)
		w := closed(value, "beforeSha256 endByte operation originalSha256 path replacement replacementSha256 startByte", wl)
		relative := safeRelative(w["path"], wl+":path")
		if !has(strings.Split(relative, "/")[0], "docs", "mkdocs.yml") {
			fail(wl, "path-outside-doc-boundary")
		}
		replacement := str(w["replacement"])
		if replacement == "" || len(replacement) > maxFileBytes {
			fail(wl, "invalid-replacement")
		}
		if w["replacementSha256"] != hash([]byte(replacement)) {
			fail(wl, "replacement-digest-mismatch")
		}
		start, end := integer(w["startByte"], wl), integer(w["endByte"], wl)
		switch w["operation"] {
		case "create_file":
			_, err := os.Stat(filepath.Join(root, relative))
			if w["beforeSha256"] != nil || err == nil || start != 0 || end != 0 || w["originalSha256"] != hash(nil) {
				fail(wl, "invalid-create")
			}
		case "insert_after":
			existing := containedFile(root, w["path"], wl+":existing")
			if w["beforeSha256"] != fileHash(existing) {
				fail(wl, "stale-before-digest")
			}
			original := readBounded(existing, wl)
			if start < 0 || end < start || end > int64(len(original)) {
				fail(wl, "invalid-byte-range")
			}
			if start != end {
				fail(wl, "insert-replaces-prose")
			}
			if w["originalSha256"] != hash(original[start:end]) {
				fail(wl, "original-range-digest-mismatch")
			}
		default:
			fail(wl, "invalid-kind")
		}
	}
	patches := array(p["navPatches"], label+":nav-patches", 1, 32)
	config := containedFile(root, "mkdocs.yml", label+":config")
	configDigest := fileHash(config)
	configRaw := readBounded(config, label+":config")
	for i, value := range patches {
		pl := fmt.Sprintf("%s:nav[%d]", label, i)
		patch := closed(value, "anchorSha256 authorityNavSha256 endByte navOwner operation originalSha256 replacement replacementSha256 startByte", pl)
		if patch["operation"] != "edit_nav" {
			fail(pl, "invalid-operation")
		}
		if patch["anchorSha256"] != configDigest || patch["navOwner"] != "mkdocs.yml" {
			fail(pl, "invalid-owner")
		}
		if !hexPattern.MatchString(str(patch["authorityNavSha256"])) {
			fail(pl, "invalid-authority-nav-digest")
		}
		start, end := integer(patch["startByte"], pl), integer(patch["endByte"], pl)
		replacement := str(patch["replacement"])
		if start < 0 || end < start || end > int64(len(configRaw)) || replacement == "" {
			fail(pl, "invalid-byte-range")
		}
		if patch["originalSha256"] != hash(configRaw[start:end]) {
			fail(pl, "original-range-digest-mismatch")
		}
		if patch["replacementSha256"] != hash([]byte(replacement)) {
			fail(pl, "replacement-digest-mismatch")
		}
	}
	for i, value := range array(p["claims"], label+":claims", 1, 128) {
		cl := fmt.Sprintf("%s:claim[%d]", label, i)
		claim := closed(value, "evidence state text", cl)
		state := str(claim["state"])
		if !has(state, "CONFLICTED", "SUPPORTED", "UNKNOWN") {
			fail(cl, "invalid-state")
		}
		text := str(claim["text"])
		if text == "" {
			fail(cl, "invalid-text")
		}
		evidence := array(claim["evidence"], cl+":evidence", 0, 32)
		if state != "UNKNOWN" && len(evidence) == 0 {
			fail(cl, "unsupported-claim")
		}
		excerpts := []string{}
		for j, e := range evidence {
			excerpts = append(excerpts, validateEvidence(root, e, fmt.Sprintf("%s:evidence[%d]", cl, j)))
		}
		if state == "SUPPORTED" && !has(contextindex.TrimPythonSpace(text), excerpts...) {
			fail(cl, "claim-not-exactly-supported")
		}
	}
}
func buildFlagValue(argv []string, flag, label string) string {
	count, index := 0, 0
	for i, s := range argv {
		if s == flag {
			count++
			index = i
		}
	}
	if count != 1 {
		fail(label, "missing-or-duplicate:"+flag)
	}
	if index+1 >= len(argv) || strings.HasPrefix(argv[index+1], "-") {
		fail(label, "missing-value:"+flag)
	}
	return argv[index+1]
}
func generatedFetchAsset(output string) string {
	found := ""
	err := filepath.WalkDir(output, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !has(strings.ToLower(filepath.Ext(p)), ".css", ".html", ".js", ".svg", ".xml") {
			return nil
		}
		raw := bytes.ToLower(readBounded(p, "build:generated-asset-scan-bound"))
		for _, marker := range []string{`src="http://`, `src="https://`, `src="//`, "src='http://", "src='https://", "src='//", "url(http://", "url(https://", "url(//", `fetch("http://`, `fetch("https://`} {
			if bytes.Contains(raw, []byte(marker)) {
				if found == "" {
					found = filepath.ToSlash(must(filepath.Rel(output, p)))
				}
				break
			}
		}
		return nil
	})
	must(true, err)
	return found
}

type bindings struct {
	environmentPath, environmentHash, denialPath, denialHash, denialHarness string
	environment, denial                                                     map[string]any
}

func resolvedPath(p string) string {
	absolute := must(filepath.Abs(p))
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return absolute
}
func validateBuild(root, output string, value any, label string, offline bool, b bindings) {
	build := closed(value, "argv configSha256 environmentSha256 exitCode materialVersion markdownVersion mkdocsExecutable mkdocsVersion networkDenial outputTreeSha256 pymdownVersion pythonExecutable profile status strict", label)
	if build["profile"] != buildProfile || build["status"] != "PASS" {
		fail(label, "not-pass")
	}
	if build["strict"] != true || integer(build["exitCode"], label) != 0 {
		fail(label, "false-strict-pass")
	}
	for _, field := range []string{"environmentSha256", "outputTreeSha256"} {
		if !hexPattern.MatchString(str(build[field])) {
			fail(label, "invalid-"+field)
		}
	}
	if b.environmentHash == "" || b.environment == nil || build["environmentSha256"] != b.environmentHash {
		fail(label, "unbound-environment")
	}
	for _, check := range []struct{ field, reason string }{{"mkdocsExecutable", "executable"}, {"mkdocsVersion", "mkdocs-version"}, {"materialVersion", "material-version"}, {"markdownVersion", "markdown-version"}, {"pymdownVersion", "markdown-version"}, {"pythonExecutable", "python-executable"}} {
		if build[check.field] != b.environment[check.field] {
			fail(label, "environment-"+check.reason+"-mismatch")
		}
	}
	found := false
	err := filepath.WalkDir(output, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			info, err := os.Stat(p)
			if err == nil && info.Mode().IsRegular() {
				found = true
			}
		}
		return nil
	})
	if err != nil || !found {
		fail(label, "expected-output-missing")
	}
	if build["outputTreeSha256"] != treeHash(output) {
		fail(label, "output-digest-mismatch")
	}
	for _, field := range []string{"mkdocsVersion", "materialVersion", "markdownVersion", "pymdownVersion"} {
		s := str(build[field])
		if s == "" || utf8.RuneCountInString(s) > 128 {
			fail(label, "invalid-"+field)
		}
	}
	executable := str(build["mkdocsExecutable"])
	if !filepath.IsAbs(executable) {
		fail(label, "executable-not-absolute")
	}
	config := containedFile(root, "mkdocs.yml", label+":config")
	if build["configSha256"] != fileHash(config) {
		fail(label, "config-digest-mismatch")
	}
	argv := []string{}
	for _, a := range array(build["argv"], label+":argv", 1, 1024) {
		if str(a) == "" {
			fail(label, "invalid-argv")
		}
		argv = append(argv, str(a))
	}
	if !has("build", argv...) || !has("--strict", argv...) || !has("--clean", argv...) || has("gh-deploy", argv...) {
		fail(label, "invalid-build-boundary")
	}
	if argv[0] != executable {
		fail(label, "argv-executable-mismatch")
	}
	configArg := resolvedPath(buildFlagValue(argv, "--config-file", label))
	outputArg := resolvedPath(buildFlagValue(argv, "--site-dir", label))
	if configArg != resolvedPath(config) || outputArg != resolvedPath(output) {
		fail(label, "nonisolated-config-or-output")
	}
	denial := closed(build["networkDenial"], "harness receiptSha256 status", label+":network")
	if !has(str(denial["status"]), "NOT_OBSERVED", "PASS", "UNKNOWN") {
		fail(label, "invalid-network-status")
	}
	if offline {
		if denial["status"] != "PASS" || str(denial["harness"]) == "" {
			fail(label, "offline-without-external-harness")
		}
		if !hexPattern.MatchString(str(denial["receiptSha256"])) {
			fail(label, "offline-without-denial-receipt")
		}
		if denial["harness"] != b.denialHarness || denial["receiptSha256"] != b.denialHash {
			fail(label, "unbound-network-denial")
		}
		if b.denial == nil || b.denial["harness"] != denial["harness"] {
			fail(label, "invalid-network-denial-receipt")
		}
		if b.denial["environmentSha256"] != b.environmentHash {
			fail(label, "network-denial-environment-mismatch")
		}
		if asset := generatedFetchAsset(output); asset != "" {
			fail(label, "fetch-bearing-generated-asset:"+asset)
		}
		fail(label, "offline-observer-not-executed-by-runner")
	} else if denial["status"] == "PASS" {
		fail(label, "unexpected-offline-qualification")
	}
	fail(label, "build-not-observed-by-runner")
}
func validateObservation(c map[string]any, root, output, sourceDigest string, raw []byte, b bindings) map[string]any {
	id := str(c["id"])
	value := parseJSON(raw, id+":observation")
	if !bytes.Equal(raw, canonical(value, true)) {
		fail(id+":observation", "not-canonical")
	}
	o := closed(value, "build code decision id plan profile repositoryMutated signals sourceTreeSha256 truth", id+":observation")
	if o["profile"] != observationProfile || o["id"] != id {
		fail(id+":observation", "identity-mismatch")
	}
	if o["sourceTreeSha256"] != sourceDigest {
		fail(id+":observation", "stale-source")
	}
	if o["repositoryMutated"] != false {
		fail(id+":observation", "repository-mutation")
	}
	allowed := false
	for _, a := range c["allowed"].([]any) {
		m := a.(map[string]any)
		if o["decision"] == m["decision"] && o["code"] == m["code"] {
			allowed = true
		}
	}
	if !allowed {
		fail(id+":observation", "unexpected-result")
	}
	signals := stringList(o["signals"], id+":signals", true)
	missing := []string{}
	for _, s := range c["requiredSignals"].([]any) {
		if !has(str(s), signals...) {
			missing = append(missing, str(s))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fail(id+":observation", "missing-signals:"+strings.Join(missing, ","))
	}
	truth := closed(o["truth"], "buildStrict offline", id+":truth")
	if !has(str(truth["buildStrict"]), "FAIL", "NOT_RUN", "PASS", "UNKNOWN") || !has(str(truth["offline"]), "NOT_OBSERVED", "QUALIFIED", "UNKNOWN") {
		fail(id+":truth", "invalid")
	}
	if o["decision"] == "PLANNED" {
		validatePlan(root, sourceDigest, o["plan"], id+":plan")
		if o["build"] != nil || truth["buildStrict"] != "NOT_RUN" {
			fail(id+":plan", "unexpected-build")
		}
	} else if o["plan"] != nil {
		fail(id, "unexpected-plan")
	}
	pass := truth["buildStrict"] == "PASS"
	if pass && !has(id, "strict-build-project-environment", "offline-qualification-external-harness") {
		fail(id, "strict-pass-on-hostile-case")
	}
	if pass && id == "strict-build-project-environment" && o["decision"] != "BUILT" {
		fail(id, "strict-pass-without-built-decision")
	}
	if pass {
		validateBuild(root, output, o["build"], id+":build", truth["offline"] == "QUALIFIED", b)
	} else if o["build"] != nil {
		fail(id, "build-without-pass-truth")
	}
	if o["decision"] == "BUILT" && !pass {
		fail(id, "built-without-strict-pass")
	}
	if truth["offline"] == "QUALIFIED" && o["code"] != "offline-qualified" {
		fail(id, "offline-qualified-on-wrong-case")
	}
	if o["code"] == "offline-qualified" && truth["offline"] != "QUALIFIED" {
		fail(id, "offline-code-without-proof")
	}
	if o["code"] == "network-denial-not-observed" && truth["offline"] == "QUALIFIED" {
		fail(id, "contradictory-offline-truth")
	}
	return o
}
func externalReceipt(p, label string) (string, string) {
	if p == "" {
		return "", ""
	}
	info, err := os.Lstat(p)
	if err != nil {
		fail(label, "missing")
	}
	if !info.Mode().IsRegular() {
		fail(label, "not-regular-file")
	}
	if info.Size() > maxFileBytes {
		fail(label, "too-large")
	}
	resolved := resolvedPath(p)
	return resolved, fileHash(resolved)
}
func loadBindings(environmentPath, denialPath, harness string) bindings {
	b := bindings{denialHarness: harness}
	b.environmentPath, b.environmentHash = externalReceipt(environmentPath, "environment-manifest")
	if b.environmentPath != "" {
		m := closed(parseJSON(readBounded(b.environmentPath, "environment-manifest"), "environment-manifest"), "authority lockFile lockFileSha256 markdownVersion materialVersion mkdocsExecutable mkdocsVersion profile pymdownVersion pythonExecutable trusted", "environment-manifest")
		if m["profile"] != environmentProfile {
			fail("environment-manifest", "profile-mismatch")
		}
		if m["authority"] != "project-owned" || m["trusted"] != true {
			fail("environment-manifest", "untrusted")
		}
		for _, field := range []string{"mkdocsVersion", "materialVersion", "markdownVersion", "pymdownVersion"} {
			s := str(m[field])
			if s == "" || utf8.RuneCountInString(s) > 128 {
				fail("environment-manifest", "invalid-"+field)
			}
		}
		for _, field := range []string{"mkdocsExecutable", "pythonExecutable", "lockFile"} {
			p := str(m[field])
			info, err := os.Lstat(p)
			if !filepath.IsAbs(p) || err != nil || !info.Mode().IsRegular() {
				fail("environment-manifest", "invalid-"+field)
			}
			m[field] = resolvedPath(p)
		}
		d := str(m["lockFileSha256"])
		if !hexPattern.MatchString(d) {
			fail("environment-manifest", "invalid-lock-digest")
		}
		if d != fileHash(str(m["lockFile"])) {
			fail("environment-manifest", "lock-digest-mismatch")
		}
		b.environment = m
	}
	b.denialPath, b.denialHash = externalReceipt(denialPath, "network-denial-receipt")
	if (harness == "") != (b.denialPath == "") {
		fail("network-denial", "incomplete-binding")
	}
	if b.denialPath != "" {
		if !signalPattern.MatchString(harness) {
			fail("network-denial", "invalid-harness")
		}
		m := closed(parseJSON(readBounded(b.denialPath, "network-denial-receipt"), "network-denial-receipt"), "environmentSha256 harness network profile scope status", "network-denial-receipt")
		if m["profile"] != denialProfile || m["status"] != "PASS" {
			fail("network-denial", "invalid-profile-or-status")
		}
		if m["network"] != "DENIED" || m["scope"] != "PROCESS_TREE" {
			fail("network-denial", "invalid-enforcement")
		}
		if m["harness"] != harness {
			fail("network-denial", "harness-mismatch")
		}
		if b.environmentHash == "" || m["environmentSha256"] != b.environmentHash {
			fail("network-denial", "environment-mismatch")
		}
		b.denial = m
	}
	return b
}
func scrubbedEnvironment(c map[string]any, workspace, output, sourceDigest, tempRoot string, b bindings) []string {
	pathValue := os.Getenv("PATH")
	if pathValue == "" {
		pathValue = "/usr/bin:/bin"
	}
	env := []string{"CORVINT_HDC_CASE=" + str(c["id"]), "CORVINT_HDC_FIXTURE_SHA256=" + sourceDigest, "CORVINT_HDC_OPERATION=" + str(c["operation"]), "CORVINT_HDC_OUTPUT=" + output, "CORVINT_HDC_WORKSPACE=" + workspace, "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "PATH=" + pathValue, "TMPDIR=" + tempRoot}
	if runtime.GOOS == "windows" && os.Getenv("SystemRoot") != "" {
		env = append(env, "SystemRoot="+os.Getenv("SystemRoot"))
	}
	if b.environmentPath != "" {
		env = append(env, "CORVINT_HDC_ENVIRONMENT_MANIFEST="+b.environmentPath, "CORVINT_HDC_ENVIRONMENT_SHA256="+b.environmentHash)
	}
	if b.denialPath != "" {
		env = append(env, "CORVINT_HDC_NETWORK_DENIAL_HARNESS="+b.denialHarness, "CORVINT_HDC_NETWORK_DENIAL_RECEIPT="+b.denialPath, "CORVINT_HDC_NETWORK_DENIAL_RECEIPT_SHA256="+b.denialHash)
	}
	return env
}
func runProcess(ctx context.Context, command []string, cwd string, environment []string, timeout time.Duration, limit int) procgroup.Observation {
	r := procgroup.Run(ctx, procgroup.Spec{Argv: command, Dir: cwd, Env: environment, Timeout: timeout, ShutdownTimeout: 2 * time.Second, OutputLimit: limit})
	if r.OutputOverflow {
		fail("command", "output-bound-exceeded")
	}
	if r.TimedOut {
		fail("command", "timeout")
	}
	if r.Cancelled {
		fail("command", "interrupted")
	}
	if r.Err != nil {
		fail("command", r.Err.Error())
	}
	return r
}
func executeCase(ctx context.Context, corpusRoot string, c map[string]any, command []string, b bindings) map[string]any {
	temp := must(os.MkdirTemp("", "corvint-hdc-conformance-"))
	defer os.RemoveAll(temp)
	temp = resolvedPath(temp)
	workspace, output := filepath.Join(temp, "workspace"), filepath.Join(temp, "site")
	must(true, os.Mkdir(output, 0700))
	materializeFixture(filepath.Join(corpusRoot, "fixtures", str(c["fixture"])), workspace)
	before := treeHash(workspace)
	r := runProcess(ctx, command, workspace, scrubbedEnvironment(c, workspace, output, before, temp, b), 10*time.Second, maxStreamBytes)
	if treeHash(workspace) != before {
		fail(str(c["id"]), "workspace-mutated")
	}
	if r.ExitStatus != 0 {
		fail(str(c["id"]), fmt.Sprintf("command-exit:%d", r.ExitStatus))
	}
	if len(r.Stderr) > 0 {
		fail(str(c["id"]), "unexpected-stderr")
	}
	return validateObservation(c, workspace, output, before, r.Stdout, b)
}
func run(ctx context.Context, args []string) (result map[string]any, status int) {
	defer func() {
		if value := recover(); value != nil {
			if e, ok := value.(conformanceError); ok {
				result = map[string]any{"error": e.Error(), "ok": false, "profile": resultProfile}
				status = 2
			} else {
				panic(value)
			}
		}
	}()
	flags := flag.NewFlagSet("human-documentation-conformance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	environment := flags.String("environment-manifest", "", "project-owned environment manifest")
	denial := flags.String("network-denial-receipt", "", "external denial receipt")
	harness := flags.String("network-denial-harness", "", "external denial harness")
	root := flags.String("corpus", "conformance/human-documentation-compiler-v0", "corpus directory")
	command := []string{}
	for i, s := range args {
		if s == "--command" {
			command = args[i+1:]
			args = args[:i]
			break
		}
	}
	if err := flags.Parse(args); err != nil {
		fail("arguments", err.Error())
	}
	if flags.NArg() != 0 {
		fail("arguments", "unexpected-positional")
	}
	cases := loadCases(*root)
	corpus := corpusHash(*root, cases)
	b := loadBindings(*environment, *denial, *harness)
	execution := "NOT_RUN"
	if len(command) > 0 {
		executable, err := exec.LookPath(command[0])
		if err != nil {
			fail("command", "not-found")
		}
		command[0] = resolvedPath(executable)
		for _, c := range cases {
			executeCase(ctx, *root, c, command, b)
		}
		execution = "PASS"
	}
	return map[string]any{"cases": len(cases), "corpusSha256": corpus, "execution": execution, "ok": true, "profile": resultProfile}, 0
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, status := run(ctx, os.Args[1:])
	if _, err := os.Stdout.Write(canonical(result, true)); err != nil {
		status = 2
	}
	os.Exit(status)
}
