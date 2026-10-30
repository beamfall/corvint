package testacceptance

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestNEAV0010TempRootValidation preserves the closed worker environment.
func TestNEAV0010TempRootValidation(t *testing.T) {
	root := canonicalTemp(t)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	write(t, file, "x")
	for _, bad := range []string{"", "relative", root + "/../", alias, alias + "/file", file, root + "/absent", root + "\n"} {
		if safeTempRoot(bad, false) {
			t.Fatalf("unsafe root admitted: %q", bad)
		}
	}
	if !safeTempRoot(root, true) {
		t.Fatal("private root refused")
	}
	saved := os.Environ()
	os.Clearenv()
	defer func() {
		os.Clearenv()
		for _, entry := range saved {
			p := strings.SplitN(entry, "=", 2)
			os.Setenv(p[0], p[1])
		}
	}()
	for _, entry := range SafeEnvironment() {
		p := strings.SplitN(entry, "=", 2)
		os.Setenv(p[0], p[1])
	}
	if !WorkerEnvironmentSafe() {
		t.Fatal("default changed")
	}
	os.Setenv("TMPDIR", root)
	if !WorkerEnvironmentSafe() {
		t.Fatal("scoped worker refused")
	}
	for key, value := range map[string]string{"GITHUB_TOKEN": "sentinel", "NODE_OPTIONS": "--inspect", "PATH": "/other", "LANG": "other", "LC_ALL": "other", "TMPDIR": alias} {
		old, present := os.LookupEnv(key)
		os.Setenv(key, value)
		if WorkerEnvironmentSafe() {
			t.Fatalf("unsafe %s admitted", key)
		}
		if present {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if WorkerEnvironmentSafe() {
		t.Fatal("public scope admitted")
	}
}

// These are real subprocess fixtures; source/approval validation stays enabled.
func tempScopeHelper() bool {
	if len(os.Args) < 2 {
		return false
	}
	if filepath.Base(os.Args[1]) == "server.fixture" {
		http.ListenAndServe(os.Args[2], http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ready") }))
		return true
	}
	switch os.Args[1] {
	case "test":
		if !WorkerEnvironmentSafe() {
			os.Exit(8)
		}
		i := slices.Index(os.Args, "--output")
		if i < 0 || i+1 >= len(os.Args) {
			os.Exit(9)
		}
		out := os.Args[i+1]
		if filepath.Dir(out) != os.Getenv("TMPDIR") {
			os.Exit(10)
		}
		// Leave a residue intentionally: interrupted workers cannot defer its removal.
		if os.WriteFile(filepath.Join(out, "residue"), []byte("owned"), 0600) != nil {
			os.Exit(11)
		}
		snapshot, _ := json.Marshal([]string{os.Getenv("TMPDIR"), out})
		if os.WriteFile(filepath.Join(filepath.Dir(os.Getenv("TMPDIR")), "observed.json"), snapshot, 0600) != nil {
			os.Exit(12)
		}
		if _, err := os.Stat("wait"); err == nil {
			time.Sleep(30 * time.Second)
		}
		fmt.Print(`{"suites":[],"errors":[]}`)
		return true
	}
	return false
}

func scopeRequest(t *testing.T, wait bool) Request {
	t.Helper()
	repo := canonicalTemp(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json", "server.fixture"} {
		write(t, filepath.Join(repo, name), "{}\n")
	}
	if wait {
		write(t, filepath.Join(repo, "wait"), "wait")
	}
	command(t, repo, "git", "init", "-q")
	command(t, repo, "git", "add", ".")
	command(t, repo, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	pin := Repository{Root: repo, Commit: command(t, repo, "git", "rev-parse", "HEAD"), Tree: command(t, repo, "git", "rev-parse", "HEAD^{tree}")}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	r := Request{Schema: RequestSchema, Product: pin, TestRepository: pin, Config: filepath.Join(repo, "config.js"), Package: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"), Runner: Command{Argv: []string{exe}, ExecutableSHA256: hashFile(t, exe)}, Server: Command{Argv: []string{exe, filepath.Join(repo, "server.fixture"), address}, ExecutableSHA256: hashFile(t, exe)}, ReadyURL: "http://" + address + "/", AppBuildDir: repo, RunnerVersion: "fixture", Environment: "scope-fixture", Repeat: 2, TimeoutSeconds: 10, Tests: []Test{{ID: "counter", File: filepath.Join(repo, "test.js"), Line: 1, Title: "counter"}}}
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json", "server.fixture"} {
		p := filepath.Join(repo, name)
		r.Inputs = append(r.Inputs, File{Path: p, SHA256: hashFile(t, p)})
	}
	if err := Validate(r); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestNEAV0010StandaloneScope runs the built companion, including actual worker
// allocation and SIGINT interruption; it asserts the fixture reached the runner.
func TestNEAV0010StandaloneScope(t *testing.T) {
	exe := filepath.Join(canonicalTemp(t), "accept")
	build := exec.Command("go", "build", "-race", "-o", exe, "../../cmd/corvint-tests-accept")
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	for _, wait := range []bool{false, true} {
		t.Run(fmt.Sprint("interrupt=", wait), func(t *testing.T) {
			r := scopeRequest(t, wait)
			root := canonicalTemp(t)
			input := filepath.Join(canonicalTemp(t), "request.json")
			b, _ := json.Marshal(r)
			if err := os.WriteFile(input, b, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "accept", "--plan", input, "--approve-plan", Digest(r), "--new", r.Tests[0].File, "--repeat", "2", "--environment", r.Environment, "--temp-root", root)
			var output strings.Builder
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			joined := false
			defer func() {
				if joined {
					return
				}
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-done:
				case <-time.After(15 * time.Second):
					_ = cmd.Process.Kill()
					<-done
				}
			}()
			deadline := time.Now().Add(2 * time.Minute)
			var paths []string
			for time.Now().Before(deadline) {
				select {
				case err := <-done:
					joined = true
					t.Fatalf("companion finished before runner: %v %s", err, output.String())
				default:
				}
				raw, err := os.ReadFile(filepath.Join(root, "observed.json"))
				if err == nil && json.Unmarshal(raw, &paths) == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if len(paths) != 2 {
				cmd.Process.Kill()
				<-done
				joined = true
				t.Fatalf("runner path not reached: %s", output.String())
			}
			if filepath.Dir(paths[0]) != root || !strings.HasPrefix(filepath.Base(paths[0]), "corvint-acceptance-run-") || filepath.Dir(paths[1]) != paths[0] {
				t.Fatalf("escaped paths: %v", paths)
			}
			if wait {
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				joined = true
				if err != nil {
					t.Fatalf("companion: %v %s", err, output.String())
				}
			case <-time.After(2 * time.Minute):
				cmd.Process.Kill()
				<-done
				joined = true
				t.Fatal("companion did not join")
			}
			if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
				t.Fatalf("scope residue: %v", err)
			}
			var report Report
			if err := json.Unmarshal([]byte(output.String()), &report); err != nil {
				t.Fatal(err)
			}
			if wait && (len(report.Runs) != 1 || !report.Runs[0].Cleanup.Cancelled || !report.Runs[0].Cleanup.OwnedGroup) {
				t.Fatalf("interruption not observed: %+v", report.Runs)
			}
		})
	}
}
