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
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const captureLimit = 8 * 1024 * 1024
const batchProfile = "snapshot-batch/0"
const ledgerName = "self-observations.jsonl"

type object = map[string]any
type planError string

func (e planError) Error() string { return string(e) }
func fail(s string)               { panic(planError(s)) }
func must[T any](v T, e error) T {
	if e != nil {
		panic(e)
	}
	return v
}
func str(v any) string       { s, _ := v.(string); return s }
func obj(v any) object       { m, _ := v.(map[string]any); return m }
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func parse(raw []byte) any {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return v
}
func serialize(v any) []byte { raw := must(json.Marshal(v)); return append(raw, '\n') }
func member(s string, list ...string) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}
	return false
}
func sameKeys(m object, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
func number(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || n >= float64(math.MaxInt64) || n < float64(math.MinInt64) {
			return 0, false
		}
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	}
	return 0, false
}
func positive(v any, max int64) bool {
	n, ok := number(v)
	return ok && n > 0 && (max == 0 || n <= max)
}
func stringsArray(v any) ([]string, bool) {
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := []string{}
	for _, item := range a {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}
func loadPlan(path string) (object, []byte) {
	f := must(os.Open(path))
	defer f.Close()
	raw := must(io.ReadAll(io.LimitReader(f, 16385)))
	if len(raw) > 16384 {
		fail("plan exceeds 16384 bytes")
	}
	p := obj(parse(raw))
	if p == nil {
		fail("plan must be a JSON object")
	}
	for k := range p {
		if !member(k, "task", "expected_evidence", "views") {
			fail("plan member is not allowed: " + k)
		}
	}
	if value, ok := p["task"]; ok {
		if _, ok := value.(string); !ok {
			fail("plan task must be a string")
		}
	}
	if value, ok := p["expected_evidence"]; ok {
		if _, ok := stringsArray(value); !ok {
			fail("plan expected_evidence must be a list of strings")
		}
	}
	views, ok := p["views"].([]any)
	if !ok || len(views) < 3 || len(views) > 5 {
		fail("plan needs 3..5 views")
	}
	ids := map[string]bool{}
	for _, value := range views {
		v := obj(value)
		if v == nil {
			fail("each view must be an object")
		}
		id := str(v["id"])
		if id == "" || len(id) > 64 {
			fail("view id must contain 1..64 UTF-8 bytes")
		}
		if ids[id] {
			fail("view ids must be distinct")
		}
		ids[id] = true
		verb := str(v["verb"])
		if !member(verb, "query", "context", "impact") {
			fail("view verb must be query, context or impact")
		}
		allowed := []string{"id", "verb", "limit"}
		if verb == "impact" {
			allowed = append(allowed, "paths")
		} else {
			allowed = append(allowed, "task")
			if verb == "query" {
				allowed = append(allowed, "budget_bytes")
			} else {
				allowed = append(allowed, "subject")
			}
		}
		for k := range v {
			if member(k, "possessed", "base", "range") {
				fail("caller possession and range impact are not admitted batch operations")
			}
			if !member(k, allowed...) {
				fail("view member is not a " + verb + " argument: " + k)
			}
		}
		if verb != "impact" && str(v["task"]) == "" {
			fail("view task must be a non-empty string")
		}
		if s, ok := v["subject"]; ok && str(s) == "" {
			fail("view subject must be a non-empty string")
		}
		if n, ok := v["limit"]; ok && !positive(n, 50) {
			fail("view limit must be a positive integer at most 50")
		}
		if n, ok := v["budget_bytes"]; ok && !positive(n, 0) {
			fail("view budget_bytes must be a positive integer")
		}
		if verb == "impact" {
			paths, ok := stringsArray(v["paths"])
			if !ok || len(paths) == 0 || len(paths) > 100 {
				fail("view paths must list 1..100 repository paths")
			}
			for _, p := range paths {
				if p == "" {
					fail("view paths entry must be a non-empty string")
				}
			}
		}
	}
	return p, raw
}
func standaloneArgv(corvint, root string, v object) []string {
	verb := str(v["verb"])
	argv := []string{corvint, "--root", root, verb}
	if verb != "impact" {
		argv = append(argv, "--task", str(v["task"]))
		if subject, ok := v["subject"]; ok {
			argv = append(argv, "--subject", str(subject))
		}
	}
	if value, ok := v["limit"]; ok {
		n, _ := number(value)
		argv = append(argv, "--limit", strconv.FormatInt(n, 10))
	}
	if verb == "impact" {
		paths, _ := stringsArray(v["paths"])
		return append(argv, paths...)
	}
	if value, ok := v["budget_bytes"]; ok {
		n, _ := number(value)
		argv = append(argv, "--budget-bytes", strconv.FormatInt(n, 10))
	}
	return argv
}
func isRevision(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func checkPacket(v object, value any, tree string) string {
	p := obj(value)
	schema, _ := number(p["schema_version"])
	if p == nil || schema != 1 {
		return "packet-schema"
	}
	if _, ok := p["results"].([]any); !ok || obj(p["request"]) == nil {
		return "packet-members"
	}
	if !isRevision(p["revision"]) {
		return "packet-revision"
	}
	if p["revision"] != tree {
		return "revision-drift"
	}
	request, verb := obj(p["request"]), str(v["verb"])
	if verb == "context" {
		_, subjectPresent := p["subject"]
		if p["tool"] != "context" || p["ok"] != true || !subjectPresent {
			return "wrong-verb-envelope"
		}
		chars, _ := number(request["task_chars"])
		if chars != int64(utf8.RuneCountInString(contextindex.TrimPythonSpace(str(v["task"])))) {
			return "request-binding"
		}
		if subject, ok := v["subject"]; ok && obj(p["subject"])["path"] != subject {
			return "request-binding"
		}
	} else {
		_, toolPresent := p["tool"]
		fresh := obj(p["freshness"])
		if toolPresent || p["mode"] != verb || fresh == nil {
			return "wrong-verb-envelope"
		}
		if revision, ok := fresh["revision"]; ok && revision != p["revision"] {
			return "packet-revision"
		}
		if verb == "query" {
			if request["text"] != contextindex.TrimPythonSpace(str(v["task"])) {
				return "request-binding"
			}
		} else {
			paths, _ := stringsArray(v["paths"])
			sort.Strings(paths)
			unique := []string{}
			for _, p := range paths {
				if len(unique) == 0 || p != unique[len(unique)-1] {
					unique = append(unique, p)
				}
			}
			actual, ok := stringsArray(request["paths"])
			if !ok || !reflect.DeepEqual(actual, unique) {
				return "request-binding"
			}
		}
		if budget, ok := v["budget_bytes"]; ok {
			want, _ := number(budget)
			actual, ok := number(request["budget_bytes"])
			if !ok || actual != want {
				return "request-binding"
			}
		}
	}
	if limit, ok := v["limit"]; ok {
		want, _ := number(limit)
		actual, ok := number(request["limit"])
		if !ok || actual != want {
			return "request-binding"
		}
	}
	return ""
}
func checkBatch(value any, views []any, repository object) string {
	d := obj(value)
	if d == nil {
		return "not-an-object"
	}
	if d["ok"] != true || d["mutates"] != false || d["tool"] != "batch" || d["profile"] != batchProfile {
		return "header-mismatch"
	}
	if !sameKeys(d, "ok", "mutates", "tool", "profile", "snapshot", "operations") {
		return "unexpected-members"
	}
	snapshot := obj(d["snapshot"])
	if !sameKeys(snapshot, "tree", "commit") || !isRevision(snapshot["tree"]) || !isRevision(snapshot["commit"]) {
		return "snapshot-mismatch"
	}
	if snapshot["tree"] != repository["tree"] || snapshot["commit"] != repository["commit"] {
		return "snapshot-drift"
	}
	operations, ok := d["operations"].([]any)
	if !ok || len(operations) != len(views) {
		return "operation-count-mismatch"
	}
	for i, value := range operations {
		r, v := obj(value), obj(views[i])
		if r == nil || r["id"] != v["id"] || r["verb"] != v["verb"] {
			return "operation-order-mismatch"
		}
		success, ok := r["ok"].(bool)
		if !ok {
			return "operation-ok-not-boolean"
		}
		if success {
			if !sameKeys(r, "id", "verb", "ok", "context") {
				return "operation-context-mismatch"
			}
			if mismatch := checkPacket(v, r["context"], str(snapshot["tree"])); mismatch != "" {
				return "operation-context:" + mismatch
			}
		} else {
			e := obj(r["error"])
			if !sameKeys(r, "id", "verb", "ok", "error") || e == nil {
				return "operation-error-mismatch"
			}
			if _, ok := e["message"].(string); !ok {
				return "operation-error-mismatch"
			}
			for k := range e {
				if !member(k, "message", "code") {
					return "operation-error-mismatch"
				}
			}
		}
	}
	return ""
}
func errorDocument(raw []byte, fallback string) any {
	if p := parse(raw); p != nil {
		return p
	}
	text := string(bytes.ToValidUTF8(raw, []byte("�")))
	runes := []rune(text)
	if len(runes) > 2000 {
		text = string(runes[:2000])
	}
	return object{"code": fallback, "message": text}
}
func standalonePacket(v, row object, stdout, stderr []byte, tree string) object {
	entry := object{"ok": false, "context": nil, "error": nil, "exit": row["exit"], "label": row["label"]}
	if row["complete"] != true {
		entry["error"] = object{"code": "unusable-output", "message": fmt.Sprint("capture incomplete: ", row["error_code"])}
		return entry
	}
	exit, _ := number(row["exit"])
	if exit != 0 {
		entry["error"] = errorDocument(stderr, "standalone-failed")
		return entry
	}
	d := obj(parse(stdout))
	packet := d
	if v["verb"] != "context" {
		packet = obj(d["context"])
	}
	mismatch := checkPacket(v, packet, tree)
	if d == nil || d["ok"] != true || d["tool"] != v["verb"] {
		mismatch = "exit 0 without the verb's document shape"
	}
	if mismatch != "" {
		entry["error"] = object{"code": "malformed-standalone-output", "message": mismatch}
		return entry
	}
	entry["ok"] = true
	entry["context"] = packet
	entry["identity"] = object{"kind": "per-response", "revision": packet["revision"]}
	return entry
}

type runner struct {
	ctx                context.Context
	corvint, root, out string
	timeout            time.Duration
	limit              int
	commands           []any
}

func (r *runner) run(label string, argv []string, input []byte) (object, []byte, []byte) {
	for _, c := range r.commands {
		if obj(c)["label"] == label {
			fail("raw label reused")
		}
	}
	rawDir := filepath.Join(r.out, "raw")
	if input != nil {
		must(true, os.WriteFile(filepath.Join(rawDir, label+".request.json"), input, 0600))
	}
	cleanup := newDescendantCleanup()
	started := time.Now()
	result := procgroup.Run(r.ctx, procgroup.Spec{Argv: argv, Dir: r.root, Env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}, Stdin: input, Timeout: r.timeout, OutputLimit: r.limit, ShutdownTimeout: 2 * time.Second, BeforeStop: cleanup.stop, AfterStart: cleanup.start})
	must(true, os.WriteFile(filepath.Join(rawDir, label+".stdout"), result.Stdout, 0600))
	must(true, os.WriteFile(filepath.Join(rawDir, label+".stderr"), result.Stderr, 0600))
	var errorCode any
	if result.Err != nil {
		errorCode = result.Err.Error()
	}
	var rss any
	if result.Usage != nil {
		rss = result.Usage.MaxRSSBytes
	}
	row := object{"label": label, "argv": argv, "exit": result.ExitStatus, "duration_ns": time.Since(started).Nanoseconds(), "stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr), "timed_out": result.TimedOut, "error_code": errorCode, "complete": result.Err == nil && result.PipesDrained && !result.OutputOverflow, "max_rss_bytes": rss, "descendants_cleaned": cleanup.cleaned, "leaked_descendants": cleanup.leaked, "descendant_observation": cleanup.status, "cleanup_hook_allowance_ns": int64(time.Second)}
	r.commands = append(r.commands, row)
	if len(cleanup.leaked) > 0 || cleanup.failure != nil {
		fail(fmt.Sprintf("descendant cleanup failed: leaked=%v observation=%q err=%v", cleanup.leaked, cleanup.status, cleanup.failure))
	}
	if result.Cancelled {
		fail("investigation interrupted")
	}
	return row, result.Stdout, result.Stderr
}
func (r *runner) standalone(ordinal int, v object, route, tree string) object {
	row, stdout, stderr := r.run(fmt.Sprintf("%s-%02d", route, ordinal), standaloneArgv(r.corvint, r.root, v), nil)
	entry := standalonePacket(v, row, stdout, stderr, tree)
	entry["ordinal"] = ordinal
	entry["id"] = v["id"]
	entry["verb"] = v["verb"]
	entry["route"] = route
	return entry
}
func (r *runner) batch(views []any, receipt object) []any {
	repository := obj(receipt["repository"])
	tree := str(repository["tree"])
	row, stdout, stderr := r.run("batch", []string{r.corvint, "--root", r.root, "batch"}, serialize(object{"operations": views}))
	attempt := object{"attempted": true, "ok": false, "label": row["label"], "exit": row["exit"], "complete": row["complete"], "timed_out": row["timed_out"]}
	receipt["batch"] = attempt
	var document object
	exit, _ := number(row["exit"])
	switch {
	case row["complete"] != true:
		attempt["reason"] = fmt.Sprint("unusable-output:", row["error_code"])
	case exit != 0:
		attempt["reason"] = "batch-refused"
		attempt["error"] = errorDocument(stderr, "batch-refused")
	default:
		document = obj(parse(stdout))
		if mismatch := checkBatch(document, views, repository); mismatch != "" {
			attempt["reason"] = "malformed-batch-document:" + mismatch
		} else {
			attempt["ok"] = true
		}
	}
	operations := []any{}
	if attempt["ok"] != true {
		for i, value := range views {
			operations = append(operations, r.standalone(i+1, obj(value), "fallback", tree))
		}
		return operations
	}
	receipt["snapshot"] = document["snapshot"]
	for i, value := range document["operations"].([]any) {
		row, view := obj(value), obj(views[i])
		if row["ok"] == true {
			operations = append(operations, object{"ordinal": i + 1, "id": view["id"], "verb": view["verb"], "route": "batch", "ok": true, "context": row["context"], "error": nil, "label": "batch", "identity": object{"kind": "snapshot", "revision": tree}})
		} else {
			entry := r.standalone(i+1, view, "fallback", tree)
			entry["batch_error"] = row["error"]
			operations = append(operations, entry)
		}
	}
	return operations
}
func gitOutput(root string, args ...string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	argv := append([]string{absolute(must(exec.LookPath("git"))), "-C", root}, args...)
	r := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: root, Env: []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0"}, Timeout: 5 * time.Second, OutputLimit: captureLimit})
	if r.Err != nil {
		panic(r.Err)
	}
	if r.ExitStatus != 0 {
		fail("repository observation failed")
	}
	return r.Stdout
}
func ledgerBytes(root string) int64 {
	info, err := os.Stat(filepath.Join(root, ".corvint", ledgerName))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		panic(err)
	}
	return info.Size()
}
func corvintListing(root string) string {
	base := filepath.Join(root, ".corvint")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return hash(nil)
	}
	rows := []string{}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == base || filepath.Base(p) == ledgerName {
			return nil
		}
		info, e := os.Stat(p)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		rows = append(rows, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	must(true, err)
	sort.Strings(rows)
	return hash([]byte(strings.Join(rows, "\n")))
}
func repositoryState(root string) object {
	status := gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	return object{"head": strings.TrimSpace(string(gitOutput(root, "rev-parse", "HEAD"))), "tree": strings.TrimSpace(string(gitOutput(root, "rev-parse", "HEAD^{tree}"))), "status_sha256": hash(status), "clean": len(status) == 0, "corvint_listing_sha256": corvintListing(root)}
}
func mutationCheck(before, after object, ledgerBefore, ledgerAfter int64) object {
	head := before["head"] == after["head"] && before["tree"] == after["tree"]
	status := before["status_sha256"] == after["status_sha256"]
	observed := before["clean"] == true && status && after["clean"] == true
	listing := before["corvint_listing_sha256"] == after["corvint_listing_sha256"]
	var unchanged any
	truth := "NOT_OBSERVED"
	if observed {
		unchanged = head && status && listing
		truth = "OBSERVED_VIA_CLEAN_STATUS"
	}
	return object{"head_unchanged": head, "corvint_listing_unchanged": listing, "corvint_listing_note": "name/size/mtime metadata of index and trace files, not content hashing", "working_tree": object{"clean_before": before["clean"], "clean_after": after["clean"], "status_equal": status, "bytes": truth}, "unchanged": unchanged, "self_observation_ledger": object{"before_bytes": ledgerBefore, "after_bytes": ledgerAfter, "note": "AGENTS.md invariant 4 exception: a standalone verb's unsupported-* failure appends here; batch never does"}}
}
func absolute(p string) string {
	p = must(filepath.Abs(p))
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(absolute(parent), filepath.Base(p))
}

type options struct {
	root, plan, out, corvint, mode string
	allowDirty                     bool
	timeout                        time.Duration
	limit                          int
}

func investigate(ctx context.Context, args options) (receipt object) {
	plan, planRaw := loadPlan(args.plan)
	root, out := absolute(args.root), absolute(args.out)
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		fail("--root is not a Git repository")
	}
	rel := must(filepath.Rel(root, out))
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		fail("--out must lie outside the inspected repository")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		fail("--out must be a new caller-owned directory")
	}
	corvint := args.corvint
	if corvint == "" {
		corvint = must(exec.LookPath("corvint"))
	}
	corvint = absolute(corvint)
	info := must(os.Stat(corvint))
	if !info.Mode().IsRegular() {
		fail("corvint not found; pass --corvint")
	}
	before := repositoryState(root)
	if before["clean"] != true && !args.allowDirty {
		fail("worktree is dirty; pass --allow-dirty to label working bytes NOT_OBSERVED")
	}
	started := time.Now()
	receipt = object{"note": "private experimental development receipt; not a wire contract; counts/time/bytes are not token or cost evidence", "ok": false, "executed": false, "mode": args.mode, "task": plan["task"], "expected_evidence": plan["expected_evidence"], "plan_sha256": hash(planRaw), "binary": object{"path": corvint, "sha256": hash(must(os.ReadFile(corvint)))}, "repository": object{"commit": before["head"], "tree": before["tree"], "clean": before["clean"]}, "possession": "none supplied; AT-07 consumption deferred"}
	ledgerBefore := ledgerBytes(root)
	must(true, os.Mkdir(out, 0700))
	must(true, os.Mkdir(filepath.Join(out, "raw"), 0700))
	r := &runner{ctx: ctx, corvint: corvint, root: root, out: out, timeout: args.timeout, limit: args.limit, commands: []any{}}
	defer func() {
		failure := recover()
		if failure != nil {
			receipt["failure"] = object{"type": "native-investigation-failure", "detail": fmt.Sprint(failure)}
		}
		receipt["mutation_check"] = mutationCheck(before, repositoryState(root), ledgerBefore, ledgerBytes(root))
		rawBytes := int64(0)
		for _, value := range r.commands {
			row := obj(value)
			a, _ := number(row["stdout_bytes"])
			b, _ := number(row["stderr_bytes"])
			rawBytes += a + b
		}
		receipt["measurement"] = object{"corvint_processes": len(r.commands), "corvint_process_rows": r.commands, "raw_output_bytes": rawBytes, "wall_ns": time.Since(started).Nanoseconds(), "uncounted": "Git children spawned by corvint are not recorded; corvint_processes is not a total command count", "setup": "`corvint index` before this consumer is a charged setup process of the batch route, run and timed by the caller", "unknown": []string{"host load", "disk cache", "token/billed cost", "model/prompt cache", "review cost"}}
		raw := must(json.MarshalIndent(receipt, "", " "))
		must(true, os.WriteFile(filepath.Join(out, "receipt.json"), append(raw, '\n'), 0600))
		if failure != nil {
			panic(failure)
		}
	}()
	receipt["executed"] = true
	views := plan["views"].([]any)
	operations := []any{}
	if args.mode == "standalone" {
		receipt["batch"] = object{"attempted": false}
		for i, value := range views {
			operations = append(operations, r.standalone(i+1, obj(value), "standalone", str(before["tree"])))
		}
	} else {
		operations = r.batch(views, receipt)
	}
	receipt["operations"] = operations
	perResponse := []string{}
	usable := 0
	for _, value := range operations {
		o := obj(value)
		if o["ok"] == true {
			usable++
			id := obj(o["identity"])
			revision := str(id["revision"])
			if id["kind"] == "per-response" && !member(revision, perResponse...) {
				perResponse = append(perResponse, revision)
			}
		}
	}
	sort.Strings(perResponse)
	receipt["identity"] = object{"captured_tree": before["tree"], "batch_snapshot": receipt["snapshot"], "per_response_revisions": perResponse, "note": "every usable packet was checked against captured_tree; a drifted packet is unusable (revision-drift)"}
	receipt["usable_operations"] = usable
	receipt["ok"] = usable == len(views)
	return receipt
}
func run(ctx context.Context, args []string, stdout, stderr io.Writer) (status int) {
	defer func() {
		if failure := recover(); failure != nil {
			if _, ok := failure.(error); !ok {
				panic(failure)
			}
			stderr.Write(serialize(object{"ok": false, "executed": false, "error": fmt.Sprint(failure)}))
			status = 2
		}
	}()
	flags := flag.NewFlagSet("selfuse-batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	o := options{}
	flags.StringVar(&o.root, "root", "", "repository root")
	flags.StringVar(&o.plan, "plan", "", "frozen view plan")
	flags.StringVar(&o.out, "out", "", "new external receipt directory")
	flags.StringVar(&o.corvint, "corvint", "", "native Corvint executable")
	flags.StringVar(&o.mode, "mode", "batch", "batch or standalone")
	flags.BoolVar(&o.allowDirty, "allow-dirty", false, "record working bytes as NOT_OBSERVED")
	timeout := flags.Float64("timeout", 120, "seconds per process")
	flags.IntVar(&o.limit, "capture-limit", captureLimit, "bytes per stream")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if o.root == "" || o.plan == "" || o.out == "" || flags.NArg() != 0 || !member(o.mode, "batch", "standalone") || *timeout <= 0 || math.IsNaN(*timeout) || math.IsInf(*timeout, 0) || *timeout > 86400 || o.limit < 1 {
		fail("invalid investigation arguments")
	}
	o.timeout = time.Duration(*timeout * float64(time.Second))
	receipt := investigate(ctx, o)
	batch := obj(receipt["batch"])
	reason := batch["reason"]
	if reason == nil {
		reason = batch["ok"]
	}
	stdout.Write(serialize(object{"ok": receipt["ok"], "executed": true, "mode": receipt["mode"], "usable_operations": receipt["usable_operations"], "operations": len(receipt["operations"].([]any)), "corvint_processes": obj(receipt["measurement"])["corvint_processes"], "batch": reason, "mutation_unchanged": obj(receipt["mutation_check"])["unchanged"]}))
	if receipt["ok"] != true {
		return 1
	}
	return 0
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
