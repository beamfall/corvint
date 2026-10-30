package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--selfuse-cli" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		main()
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "--root" {
		fakeCorvint(os.Args[2:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fakePacket(v object, tree string) object {
	verb := str(v["verb"])
	request := object{}
	if n, ok := v["limit"]; ok {
		request["limit"] = n
	}
	if verb == "context" {
		request["task_chars"] = utf8.RuneCountInString(strings.TrimSpace(str(v["task"])))
		var subject any
		if s, ok := v["subject"]; ok {
			subject = object{"path": s}
		}
		return object{"schema_version": 1, "results": []any{}, "request": request, "revision": tree, "tool": "context", "ok": true, "subject": subject}
	}
	if verb == "query" {
		request["text"] = strings.TrimSpace(str(v["task"]))
		if budget, ok := v["budget_bytes"]; ok {
			request["budget_bytes"] = budget
		}
	} else {
		request["paths"] = v["paths"]
	}
	return object{"schema_version": 1, "results": []any{}, "request": request, "revision": tree, "mode": verb, "freshness": object{"revision": tree}}
}
func fakeCorvint(args []string) {
	root, verb := args[0], args[1]
	mode := strings.TrimSpace(string(must(os.ReadFile(filepath.Join(root, "mode")))))
	if verb == "batch" && (mode == "hang" || mode == "flood") {
		fakeProcessFault(root, mode)
		return
	}
	git := func(arg string) string {
		return strings.TrimSpace(string(must(exec.Command("git", "-C", root, "rev-parse", arg).Output())))
	}
	tree, commit := git("HEAD^{tree}"), git("HEAD")
	if verb == "batch" {
		if mode == "refuse" {
			os.Stderr.WriteString(`{"code":"missing-index","message":"batch unavailable"}`)
			os.Exit(2)
		}
		request := obj(parse(must(ioReadStdin())))
		operations := []any{}
		for i, value := range request["operations"].([]any) {
			v := obj(value)
			row := object{"id": v["id"], "verb": v["verb"], "ok": true, "context": fakePacket(v, tree)}
			if mode == "partial" && i == 1 {
				row = object{"id": v["id"], "verb": v["verb"], "ok": false, "error": object{"message": "controlled operation refusal"}}
			}
			operations = append(operations, row)
		}
		d := object{"ok": true, "mutates": false, "tool": "batch", "profile": batchProfile, "snapshot": object{"tree": tree, "commit": commit}, "operations": operations}
		if mode == "malformed" {
			d["delta"] = object{}
		}
		os.Stdout.Write(serialize(d))
		return
	}
	v := object{"verb": verb}
	for i := 2; i < len(args); i++ {
		if args[i] == "--task" || args[i] == "--subject" {
			v[strings.TrimPrefix(args[i], "--")] = args[i+1]
			i++
		} else if args[i] == "--limit" || args[i] == "--budget-bytes" {
			var n int
			fmt.Sscan(args[i+1], &n)
			v[strings.ReplaceAll(strings.TrimPrefix(args[i], "--"), "-", "_")] = n
			i++
		} else {
			paths, _ := v["paths"].([]any)
			v["paths"] = append(paths, args[i])
		}
	}
	if mode == "standalone-fail" {
		os.Stderr.WriteString(`{"code":"controlled","message":"standalone failed"}`)
		os.Exit(2)
	}
	packet := fakePacket(v, tree)
	if mode == "drift" {
		packet["revision"] = strings.Repeat("f", 40)
	}
	if verb == "context" {
		os.Stdout.Write(serialize(packet))
	} else {
		os.Stdout.Write(serialize(object{"ok": true, "tool": verb, "context": packet}))
	}
}
func ioReadStdin() ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(os.Stdin)
	return b.Bytes(), err
}
func fixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	root := t.TempDir()
	must(true, os.WriteFile(filepath.Join(root, "mode"), []byte(mode), 0600))
	must(true, os.WriteFile(filepath.Join(root, "code.go"), []byte("package fixture\nfunc Run() {}\n"), 0600))
	must(true, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/fixture\n\ngo 1.27\n"), 0600))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if raw, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", raw, err)
		}
	}
	plan := filepath.Join(t.TempDir(), "plan.json")
	must(true, os.WriteFile(plan, serialize(object{"task": "investigation", "views": []any{object{"id": "../unsafe-label", "verb": "query", "task": "Run", "limit": 3}, object{"id": "context", "verb": "context", "task": "Run", "subject": "code.go"}, object{"id": "impact", "verb": "impact", "paths": []any{"code.go"}}}}), 0600))
	return root, plan
}
func runFixture(t *testing.T, mode, route string) (object, string) {
	t.Helper()
	root, plan := fixture(t, mode)
	out := filepath.Join(t.TempDir(), "result")
	receipt := investigate(context.Background(), options{root: root, plan: plan, out: out, corvint: must(os.Executable()), mode: route, timeout: raceScale * time.Second, limit: captureLimit})
	return receipt, out
}
func TestBatchAndStandaloneRoutes(t *testing.T) {
	for _, tc := range []struct {
		mode, route string
		count       int
		ok          bool
	}{{"normal", "batch", 1, true}, {"normal", "standalone", 3, true}, {"refuse", "batch", 4, true}, {"malformed", "batch", 4, true}, {"partial", "batch", 2, true}, {"standalone-fail", "standalone", 3, false}, {"drift", "standalone", 3, false}} {
		t.Run(tc.mode+"/"+tc.route, func(t *testing.T) {
			r, out := runFixture(t, tc.mode, tc.route)
			if r["ok"] != tc.ok || obj(r["measurement"])["corvint_processes"] != tc.count {
				t.Fatal(r)
			}
			if obj(r["mutation_check"])["unchanged"] != true {
				t.Fatal(r["mutation_check"])
			}
			entries := must(os.ReadDir(filepath.Join(out, "raw")))
			for _, e := range entries {
				if strings.Contains(e.Name(), "unsafe-label") {
					t.Fatal("caller id used as filename")
				}
				info := must(e.Info())
				if info.Mode().Perm() != 0600 {
					t.Fatal("raw output privacy")
				}
			}
			if tc.mode == "partial" {
				ops := r["operations"].([]any)
				if obj(ops[1])["route"] != "fallback" || obj(ops[1])["batch_error"] == nil {
					t.Fatal(ops)
				}
			}
		})
	}
}
func TestPlanRefusesBeforeOutputOrRepositoryCommands(t *testing.T) {
	_, source := fixture(t, "normal")
	base := must(os.ReadFile(source))
	for _, tc := range []struct {
		name   string
		mutate func(object)
	}{{"unknown", func(p object) { p["unexpected"] = true }}, {"duplicate-id", func(p object) { views := p["views"].([]any); obj(views[1])["id"] = obj(views[0])["id"] }}, {"possessed", func(p object) { obj(p["views"].([]any)[0])["possessed"] = []any{} }}, {"range", func(p object) { obj(p["views"].([]any)[0])["base"] = "HEAD" }}, {"bool-limit", func(p object) { obj(p["views"].([]any)[0])["limit"] = true }}, {"wrong-verb-field", func(p object) { obj(p["views"].([]any)[2])["task"] = "Run" }}, {"null-path", func(p object) { obj(p["views"].([]any)[2])["paths"] = nil }}, {"few-views", func(p object) { p["views"] = p["views"].([]any)[:2] }}, {"large-id", func(p object) { obj(p["views"].([]any)[0])["id"] = strings.Repeat("é", 33) }}} {
		t.Run(tc.name, func(t *testing.T) {
			p := obj(parse(base))
			tc.mutate(p)
			dir := t.TempDir()
			plan, out := filepath.Join(dir, "plan"), filepath.Join(dir, "out")
			must(true, os.WriteFile(plan, serialize(p), 0600))
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"--plan", plan, "--root", "/does-not-exist", "--out", out}, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 {
				t.Fatalf("code=%d %s %s", code, &stdout, &stderr)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("invalid plan wrote output")
			}
		})
	}
}
func TestPacketAndBatchBindings(t *testing.T) {
	tree := strings.Repeat("a", 40)
	v := object{"id": "q", "verb": "query", "task": "inspect", "limit": 3}
	good := fakePacket(v, tree)
	roundtrip := func(v any) any { return parse(serialize(v)) }
	for _, tc := range []struct {
		name, want string
		mutate     func(object)
	}{{"schema", "packet-schema", func(p object) { p["schema_version"] = true }}, {"revision", "revision-drift", func(p object) { p["revision"] = strings.Repeat("b", 40) }}, {"request", "request-binding", func(p object) { obj(p["request"])["text"] = "other" }}, {"limit", "request-binding", func(p object) { obj(p["request"])["limit"] = 4 }}, {"envelope", "wrong-verb-envelope", func(p object) { p["tool"] = "query" }}, {"results", "packet-members", func(p object) { p["results"] = object{} }}} {
		t.Run(tc.name, func(t *testing.T) {
			p := obj(roundtrip(good))
			tc.mutate(p)
			if got := checkPacket(v, p, tree); got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	repository := object{"tree": tree, "commit": strings.Repeat("c", 40)}
	d := object{"ok": true, "mutates": false, "tool": "batch", "profile": batchProfile, "snapshot": repository, "operations": []any{object{"id": "q", "verb": "query", "ok": true, "context": good}}}
	if got := checkBatch(roundtrip(d), []any{v}, repository); got != "" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		key   string
		value any
	}{{"delta", object{}}, {"mutates", true}, {"snapshot", object{"tree": tree}}, {"operations", []any{}}, {"profile", "other"}} {
		copy := obj(roundtrip(d))
		copy[tc.key] = tc.value
		if checkBatch(copy, []any{v}, repository) == "" {
			t.Fatal("forged batch accepted", tc)
		}
	}
	contextView := object{"verb": "context", "task": " \té \n", "subject": "code.go"}
	p := obj(roundtrip(fakePacket(contextView, tree)))
	if got := checkPacket(contextView, p, tree); got != "" {
		t.Fatal(got)
	}
	p["request"] = object{"task_chars": 2}
	if got := checkPacket(object{"verb": "context", "task": "\x1cé🙂\x1f", "subject": "code.go"}, p, tree); got != "" {
		t.Fatal(got)
	}
	query := object{"verb": "query", "task": "\x1cinspect\x1f"}
	p = obj(roundtrip(fakePacket(object{"verb": "query", "task": "inspect"}, tree)))
	if got := checkPacket(query, p, tree); got != "" {
		t.Fatal(got)
	}
}
func TestOutputAndDirtyTruth(t *testing.T) {
	root, plan := fixture(t, "normal")
	for _, out := range []string{root, filepath.Join(root, "new"), t.TempDir()} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{"--root", root, "--plan", plan, "--out", out, "--corvint", must(os.Executable())}, &stdout, &stderr); code != 2 {
			t.Fatal(code)
		}
	}
	before := object{"head": "a", "tree": "b", "status_sha256": "c", "clean": false, "corvint_listing_sha256": "d"}
	check := mutationCheck(before, before, 0, 0)
	if check["unchanged"] != nil || obj(check["working_tree"])["bytes"] != "NOT_OBSERVED" {
		t.Fatal(check)
	}
	a := standaloneArgv("corvint", "root", object{"verb": "impact", "paths": []any{"code.go"}, "limit": 5})
	if !reflect.DeepEqual(a, []string{"corvint", "--root", "root", "impact", "--limit", "5", "code.go"}) {
		t.Fatal(a)
	}
}

func TestRealNativeBatchFallbackAndReadOnlyParity(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "corvint")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/corvint")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, raw)
	}
	for _, state := range []string{"indexed", "absent", "stale"} {
		t.Run(state, func(t *testing.T) {
			root, plan := fixture(t, "normal")
			command := func(args ...string) []byte {
				cmd := exec.Command(bin, append([]string{"--root", root}, args...)...)
				raw, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%v: %v %s", args, err, raw)
				}
				return raw
			}
			if state != "absent" {
				command("index")
			}
			if state == "stale" {
				must(true, os.WriteFile(filepath.Join(root, "extra.go"), []byte("package fixture\nfunc Extra() {}\n"), 0600))
				for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "-qm", "move"}} {
					if raw, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
						t.Fatalf("git: %v %s", err, raw)
					}
				}
			}
			out := filepath.Join(t.TempDir(), "result")
			r := investigate(context.Background(), options{root: root, plan: plan, out: out, corvint: bin, mode: "batch", timeout: 15 * time.Second, limit: captureLimit})
			if r["ok"] != true || obj(r["mutation_check"])["unchanged"] != true {
				t.Fatal(r)
			}
			wantRoute, wantProcesses := "batch", 1
			if state != "indexed" {
				wantRoute, wantProcesses = "fallback", 4
			}
			if obj(r["measurement"])["corvint_processes"] != wantProcesses {
				t.Fatal(r)
			}
			views := obj(parse(must(os.ReadFile(plan))))["views"].([]any)
			for i, op := range r["operations"].([]any) {
				v := obj(views[i])
				argv := standaloneArgv(bin, root, v)
				raw, err := exec.Command(argv[0], argv[1:]...).Output()
				if err != nil {
					t.Fatal(err)
				}
				packet := obj(parse(raw))
				if str(v["verb"]) != "context" {
					packet = obj(packet["context"])
				}
				if obj(op)["route"] != wantRoute || !reflect.DeepEqual(obj(parse(serialize(obj(op)["context"]))), packet) {
					t.Fatalf("route/parity: %v", op)
				}
			}
			if state == "absent" {
				if _, err := os.Stat(contextindex.SnapshotDirectory(root)); !os.IsNotExist(err) {
					t.Fatal("read created index", err)
				}
			}
		})
	}
}
