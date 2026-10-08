// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/intake"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/stepverify"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func candidateHostProfile() HostProfile {
	h := strings.Repeat("a", 64)
	oid := strings.Repeat("b", 40)
	return HostProfile{Profile: "corvint-postmerge-author-host/0", Mode: "candidate-qualification", Implementation: HostImplementation{oid, oid, h, h}, Engine: HostEngine{"colima", h, "29.5.2", "linux/amd64"}, Image: HostImage{"sha256:" + h, "sha256:" + h, []HostEnvironment{}, []HostEnvironment{}}, Admission: HostAdmission{oid, oid, oid, h, h, h}, Command: HostCommand{"/tools/author", []string{"/admitted/author-input.json", "/product"}, "/product", []HostEnvironment{{"HOME", "/private/home", "NONE"}, {"TMPDIR", "/private/tmp", "NONE"}, {"PATH", "/usr/local/go/bin:/usr/bin:/bin", "NONE"}}}, Limits: HostLimits{2000000000, 8 << 30, 8 << 30, 256, 1 << 20, 16 << 20, 30, 30, 60, 16 << 20}}
}
func TestHostProfileClosedWire(t *testing.T) {
	p := candidateHostProfile()
	good, _ := json.Marshal(p)
	if _, err := DecodeHostProfile(good); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"PCH-V0-009-duplicate":   strings.Replace(string(good), `"mode":`, `"mode":"candidate-qualification","mode":`, 1),
		"PCH-V0-009-case-alias":  strings.Replace(string(good), `"mode":`, `"Mode":`, 1),
		"PCH-V0-009-unknown":     strings.Replace(string(good), `"mode":`, `"verified":true,"mode":`, 1),
		"PCH-V0-009-missing":     strings.Replace(string(good), `"environment":[],`, "", 1),
		"PCH-V0-009-null":        strings.Replace(string(good), `"environment":[]`, `"environment":null`, 1),
		"PCH-V0-011-environment": strings.Replace(string(good), `"value":"/private/home"`, `"value":"/runner/home"`, 1),
		"PCH-V0-014-limits":      strings.Replace(string(good), `"private_tmpfs_bytes":16777216`, `"private_tmpfs_bytes":15032385536`, 1),
		"PCH-V0-009-hash":        strings.Replace(string(good), strings.Repeat("a", 64), strings.Repeat("A", 64), 1),
		"PCH-V0-009-surrogate":   strings.Replace(string(good), `"context":"colima"`, `"context":"\ud800"`, 1),
		"PCH-V0-009-number":      strings.Replace(string(good), `"pids":256`, `"pids":256.0`, 1),
	}
	for name, data := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeHostProfile([]byte(data)); err == nil {
				t.Fatal("accepted invalid candidate")
			}
		})
	}
}

func candidateHostRequest(p HostProfile) HostRequest {
	ref := HostFileRef{"/private/tmp/host-input", p.Admission.AuthorSHA256, 0}
	return HostRequest{Profile: "corvint-postmerge-author-request/0", RunID: strings.Repeat("1", 32), OperatorManifest: ref, Product: HostProduct{"/private/tmp/product", p.Admission.ProductBase, p.Admission.ProductMerge, p.Admission.ProductTree}, Declaration: ref, Observer: HostObserver{"observer_" + strings.Repeat("1", 32), ref}, AdmittedInput: ref, Author: ref, ExecShim: ref, OutputRoot: "/private/tmp/output"}
}
func TestHostRequestIdentityRefusals(t *testing.T) {
	p := candidateHostProfile()
	r := candidateHostRequest(p)
	data, _ := json.Marshal(r)
	if _, err := DecodeHostRequest(data, p); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*HostRequest){
		"PCH-V0-009-observer-id":     func(r *HostRequest) { r.Observer.ID = r.RunID },
		"PCH-V0-009-run-id":          func(r *HostRequest) { r.RunID = "../other" },
		"PCH-V0-009-source-pin":      func(r *HostRequest) { r.Product.Tree = strings.Repeat("c", 40) },
		"PCH-V0-009-input-pin":       func(r *HostRequest) { r.AdmittedInput.SHA256 = strings.Repeat("c", 64) },
		"PCH-V0-009-tool-pin":        func(r *HostRequest) { r.ExecShim.SHA256 = strings.Repeat("c", 64) },
		"PCH-V0-009-alternate-path":  func(r *HostRequest) { r.OutputRoot = "/private/tmp/../output" },
		"PCH-V0-009-mount-delimiter": func(r *HostRequest) { r.Author.Path = "/private/tmp/author,readonly=false" },
		"PCH-V0-009-intake-bound":    func(r *HostRequest) { r.AdmittedInput.Bytes = (1 << 20) + 1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			x := r
			mutate(&x)
			data, _ := json.Marshal(x)
			if _, err := DecodeHostRequest(data, p); err == nil {
				t.Fatal("accepted identity drift")
			}
		})
	}
}
func TestHostControlBindingRefusals(t *testing.T) {
	p := candidateHostProfile()
	r := candidateHostRequest(p)
	sha := strings.Repeat("a", 64)
	ref := HostFileRef{r.OutputRoot + "/native/record.json", sha, 2}
	good := HostControl{Profile: "corvint-postmerge-host-control/0", Operation: "execute", RunID: r.RunID, RequestSHA256: sha, ProfileSHA256: sha, Host: &ref, Before: &ref, Preflight: &ref}
	data, _ := json.Marshal(good)
	if _, err := DecodeHostControl(data, r, sha, sha); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*HostControl){
		"PCH-V0-012-stale-profile":            func(c *HostControl) { c.ProfileSHA256 = strings.Repeat("b", 64) },
		"PCH-V0-012-stale-run":                func(c *HostControl) { c.RunID = strings.Repeat("2", 32) },
		"PCH-V0-012-missing-preflight":        func(c *HostControl) { c.Preflight = nil },
		"PCH-V0-012-outside-evidence":         func(c *HostControl) { x := ref; x.Path = "/private/tmp/another/record.json"; c.Host = &x },
		"PCH-V0-012-cancel-with-execute-refs": func(c *HostControl) { c.Operation = "cancel"; s := "supervisor-cancelled"; c.Reason = &s },
	} {
		t.Run(name, func(t *testing.T) {
			c := good
			mutate(&c)
			b, _ := json.Marshal(c)
			if _, err := DecodeHostControl(b, r, sha, sha); err == nil {
				t.Fatal("accepted stale or unbound control")
			}
		})
	}
}
func TestHostFileSnapshotsRejectAliases(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "input.json")
	data := []byte("{}\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ref := HostFileRef{path, hostDigest(data), int64(len(data))}
	if _, err := readHostFile(ref, 1024); err != nil {
		t.Fatal(err)
	}
	t.Run("PCH-V0-010-symlink", func(t *testing.T) {
		link := filepath.Join(root, "alias")
		if err := os.Symlink(path, link); err != nil {
			t.Fatal(err)
		}
		r := ref
		r.Path = link
		if _, err := readHostFile(r, 1024); err == nil {
			t.Fatal("accepted symlink")
		}
	})
	t.Run("PCH-V0-010-hardlink", func(t *testing.T) {
		link := filepath.Join(root, "hardlink")
		if err := os.Link(path, link); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(link)
		if _, err := readHostFile(ref, 1024); err == nil {
			t.Fatal("accepted aliased original")
		}
	})
	t.Run("PCH-V0-009-digest-drift", func(t *testing.T) {
		r := ref
		r.SHA256 = strings.Repeat("0", 64)
		if _, err := readHostFile(r, 1024); err == nil {
			t.Fatal("accepted wrong digest")
		}
	})
	t.Run("PCH-V0-009-size-drift", func(t *testing.T) {
		r := ref
		r.Bytes++
		if _, err := readHostFile(r, 1024); err == nil {
			t.Fatal("accepted wrong size")
		}
	})
}

func TestPairedAuthorSourceConformance(t *testing.T) {
	// This runs the reviewed author against disposable trusted source fixtures.
	// Native Verify checks these same objects; asserted host booleans do not
	// establish physical confinement, engine behavior, or historical replay.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	temp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	product := filepath.Join(temp, "product")
	control := filepath.Join(temp, "control")
	for _, dir := range []string{product, control} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	fixture := filepath.Join("testdata", "paired-author")
	copyTree := func(from, to string) {
		t.Helper()
		err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(from, path)
			if err != nil {
				return err
			}
			target := filepath.Join(to, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, 0600)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "GOCACHE=" + os.Getenv("GOCACHE"), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	run := func(dir string, args ...string) procgroup.Observation {
		t.Helper()
		executable, err := exec.LookPath(args[0])
		if err != nil {
			t.Fatal(err)
		}
		args[0] = executable
		r := procgroup.Run(ctx, procgroup.Spec{Argv: args, Dir: dir, Env: env, Timeout: 60 * time.Second, OutputLimit: 1 << 20, StderrLimit: 1 << 20, ObserveDescendants: true})
		if r.Err != nil || !r.WaitCompleted || !r.OwnedProcessGroupCleanup || r.OutputOverflow {
			t.Fatalf("process incomplete %q: %+v stderr=%s", args, r, string(r.Stderr))
		}
		return r
	}
	pass := func(dir string, args ...string) string {
		t.Helper()
		r := run(dir, args...)
		if r.ExitStatus != 0 {
			t.Fatalf("%q failed: %s %s", args, r.Stdout, r.Stderr)
		}
		return strings.TrimSpace(string(r.Stdout))
	}
	copyTree(filepath.Join(fixture, "base"), product)
	pass(product, "git", "init", "--quiet")
	pass(product, "git", "add", ".")
	pass(product, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "--quiet", "-m", "base")
	base := pass(product, "git", "rev-parse", "HEAD")
	copyTree(filepath.Join(fixture, "merged"), product)
	pass(product, "git", "add", ".")
	pass(product, "git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "--quiet", "-m", "add")
	head := pass(product, "git", "rev-parse", "HEAD")
	candidate := intake.Record{Profile: "corvint-intake/0", Base: base, Head: head, Intent: "FEATURE", Behaviours: []intake.Behaviour{{Kind: "ADD", Path: "calc.go"}}, Flags: []string{}, Tests: []string{}, Comparison: "ALIGNED", Concerns: []string{}, WorkItems: []intake.WorkItem{}}
	raw, _ := json.Marshal(candidate)
	admitted, err := intake.BuildAuthorInput(ctx, product, base, head, raw)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(temp, "author-input.json")
	if err := os.WriteFile(input, admitted, 0600); err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	author := filepath.Join(temp, "author")
	pass(repo, "go", "build", "-o", author, "./internal/postmergehost/testdata/paired-author/author")
	verifyActualAuthor := hostTestNativeControlBinding(t, ctx, product, base, head, pass(product, "git", "rev-parse", "HEAD^{tree}"))
	pass(product, author, input, product)
	verifyActualAuthor()
	docs, err := os.ReadFile(filepath.Join(product, "docs/add.md"))
	if err != nil || !strings.Contains(string(docs), "returns the sum") {
		t.Fatal("substantive documentation missing", err)
	}
	test, err := os.ReadFile(filepath.Join(product, "tests/add_test.go"))
	if err != nil || !strings.Contains(string(test), "fixture.Add(2, 3)") {
		t.Fatal("substantive test missing", err)
	}
	pass(product, "go", "test", "-count=1", "-timeout", "30m", "./...")
	copyTree(filepath.Join(fixture, "merged"), control)
	b, err := os.ReadFile(filepath.Join(fixture, "control/calc.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "calc.go"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "tests/add_test.go"), test, 0600); err != nil {
		t.Fatal(err)
	}
	negative := run(control, "go", "test", "-count=1", "-timeout", "30m", "./...")
	if negative.ExitStatus == 0 || !strings.Contains(string(negative.Stdout), "Add(2,3) = -1") {
		t.Fatalf("same authored bytes did not reject subtraction: %s %s", negative.Stdout, negative.Stderr)
	}
}

func TestHostExecAttributionRefusesAmbiguity(t *testing.T) {
	h := strings.Repeat("a", 64)
	cid := strings.Repeat("b", 64)
	author := strings.Repeat("c", 64)
	x := hostExecInspection{ContainerID: cid, ID: h, OpenStdin: true, OpenStdout: true, OpenStderr: true, ProcessConfig: hostExecConfig{User: "65532:65532", Entrypoint: "/tools/exec-shim", Arguments: []string{"--internal-envelope", author}}, Running: true, PID: 42}
	raw, _ := json.Marshal(x)
	before, err := decodeHostExec(raw, h, cid, author)
	if err != nil {
		t.Fatal(err)
	}
	top := hostTop{[]string{"PID", "COMMAND"}, [][]string{{"1", "/bin/sleep infinity"}, {"42", "/tools/author /admitted/author-input.json /product"}}}
	table, _ := json.Marshal(top)
	if !hostObservedAuthorStart(before, before, table) {
		t.Fatal("bounded start observation refused")
	}
	for name, mutate := range map[string]func(*hostExecInspection){"PCH-V0-012-wrong-exec": func(x *hostExecInspection) { x.ID = cid }, "PCH-V0-012-wrong-container": func(x *hostExecInspection) { x.ContainerID = h }, "PCH-V0-012-shim-command": func(x *hostExecInspection) { x.ProcessConfig.Entrypoint = "/bin/sh" }, "PCH-V0-012-missing-pid": func(x *hostExecInspection) { x.PID = 0 }} {
		t.Run(name, func(t *testing.T) {
			bad := x
			mutate(&bad)
			b, _ := json.Marshal(bad)
			if _, err := decodeHostExec(b, h, cid, author); err == nil {
				t.Fatal("invalid exec instance accepted")
			}
		})
	}
	for name, table := range map[string]hostTop{
		"PCH-V0-012-ambiguous-titles": {[]string{"PID", "ARGS"}, top.Processes},
		"PCH-V0-012-duplicate-pid":    {top.Titles, [][]string{top.Processes[1], top.Processes[1]}},
		"PCH-V0-012-pre-exec-shim":    {top.Titles, [][]string{{"42", "/tools/exec-shim --internal-envelope " + author}}},
		"PCH-V0-012-missing":          {top.Titles, [][]string{}},
		"PCH-V0-012-malformed-row":    {top.Titles, [][]string{{"42"}}},
	} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(table)
			if hostObservedAuthorStart(before, before, raw) {
				t.Fatal("ambiguous start accepted")
			}
		})
	}
	after := before
	after.Running = false
	if hostObservedAuthorStart(before, after, table) {
		t.Fatal("fast exit relabelled observed start")
	}
	after = before
	after.PID++
	if hostObservedAuthorStart(before, after, table) {
		t.Fatal("PID drift accepted")
	}
	done := before
	done.Running = false
	done.ExitCode = 137
	if hostAuthorExitFromInspection(done, false) != nil {
		t.Fatal("shim/client status promoted without author start")
	}
	exit := hostAuthorExitFromInspection(done, true)
	if exit == nil || exit.Code == nil || *exit.Code != 137 || exit.Signal != nil {
		t.Fatal("invented signal or lost nonzero author code")
	}
}

func hostTestFrame(kind byte, b []byte) []byte {
	header := make([]byte, 8)
	header[0] = kind
	binary.BigEndian.PutUint32(header[4:], uint32(len(b)))
	return append(header, b...)
}
func TestHostStreamRefusesIncompleteOrForgedReadiness(t *testing.T) {
	p := candidateHostProfile()
	env := []string{}
	for _, e := range p.Command.Environment {
		env = append(env, e.Key+"="+e.Value)
	}
	observed, _ := hostObserveEnvironment(env)
	good, _ := json.Marshal(HostEnvelopeObservation{"corvint-postmerge-internal-envelope/0", p.Admission.AuthorSHA256, observed, []HostEnvironmentDigest{}})
	good = append(good, '\n')
	for name, raw := range map[string][]byte{"valid": hostTestFrame(1, good), "truncated-header": {1, 0, 0}, "truncated-body": hostTestFrame(1, good)[:len(good)], "stderr-forgery": hostTestFrame(2, good), "multiple-frames-as-observation": hostTestFrame(1, append(append([]byte{}, good...), good...)), "zero-length": hostTestFrame(1, nil), "missing-newline": hostTestFrame(1, good[:len(good)-1])} {
		t.Run(name, func(t *testing.T) {
			s := hostExecStream{reader: bufio.NewReader(bytes.NewReader(raw)), remaining: 16 << 20}
			_, _, err := s.envelope(p)
			if (err == nil) != (name == "valid") {
				t.Fatalf("observation err=%v", err)
			}
		})
	}
	// Logs after the sole barrier observation are raw, even if they contain an
	// apparently valid observation. They cannot repair an absent start/exit join.
	stream := append(hostTestFrame(1, good), hostTestFrame(1, []byte("author-controlled\n"))...)
	s := hostExecStream{reader: bufio.NewReader(bytes.NewReader(stream)), remaining: 16 << 20}
	if _, _, err := s.envelope(p); err != nil {
		t.Fatal(err)
	}
	_, raw, err := s.frame()
	if err != nil || string(raw) != "author-controlled\n" {
		t.Fatal("author log changed", err)
	}
}

type hostTestRoundTrip func(*http.Request) (*http.Response, error)

func (f hostTestRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHostTransportLimitsAndNoRedirect(t *testing.T) {
	for name, test := range map[string]struct {
		status int
		body   string
		want   bool
	}{"success": {200, `{}`, true}, "redirect": {302, `{}`, false}, "overflow": {200, strings.Repeat("x", HostWireLimit+1), false}} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			e := hostUnixEngine("/unreachable-test-only")
			defer e.close()
			e.client.Transport = hostTestRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "engine" {
					t.Fatal("changed origin")
				}
				return &http.Response{StatusCode: test.status, Header: http.Header{"Location": []string{"http://elsewhere"}}, Body: io.NopCloser(strings.NewReader(test.body)), Request: r}, nil
			})
			_, err := e.call(context.Background(), "GET", "/version", nil, 200)
			if (err == nil) != test.want || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
	for _, pair := range [][2]string{{"1.44", "1.56"}, {"1.44", "1.99"}} {
		v, e := hostNegotiateAPI(pair[0], pair[1])
		if e != nil || v != "1.56" {
			t.Fatal(v, e)
		}
	}
	for _, pair := range [][2]string{{"1.57", "1.99"}, {"1.20", "1.43"}, {"1.056", "1.56"}, {"1.56", "1.44"}} {
		if _, e := hostNegotiateAPI(pair[0], pair[1]); e == nil {
			t.Fatal("invalid API range admitted", pair)
		}
	}
	var v any
	for _, raw := range []string{`{"ID":"one","ID":"two"}`, `{"a":{"b":1,"b":2}}`, `{} {}`} {
		if hostEngineJSON([]byte(raw), &v) == nil {
			t.Fatal("ambiguous engine facts accepted")
		}
	}
}
func TestHostStreamCloseJoinsCancellation(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	s := &hostExecStream{conn: local, reader: bufio.NewReader(local), done: make(chan struct{}), joined: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(s.joined)
		select {
		case <-ctx.Done():
			local.Close()
		case <-s.done:
		}
	}()
	read := make(chan error, 1)
	go func() { _, _, err := s.frame(); read <- err }()
	cancel()
	s.close()
	select {
	case err := <-read:
		if err == nil || errors.Is(err, io.EOF) {
			t.Fatal("expected interrupted stream", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not retire read")
	}
}

func TestHostCleanupIndependentAndExact(t *testing.T) {
	in := &HostInputs{Request: candidateHostRequest(candidateHostProfile()), RequestSHA256: strings.Repeat("b", 64), ProfileSHA256: strings.Repeat("c", 64)}
	cid := strings.Repeat("d", 64)
	good, _ := json.Marshal(map[string]any{"Id": cid, "Name": "/corvint-author-" + in.Request.RunID, "Config": map[string]any{"Labels": hostLabels(in)}})
	for _, name := range []string{"cancelled-main", "wrong-owner", "uncertain-create", "unavailable-daemon", "late-survivor"} {
		t.Run(name, func(t *testing.T) {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if cancelled.Err() == nil {
				t.Fatal("setup")
			}
			engine := hostUnixEngine("/test-unused")
			defer engine.close()
			calls := []string{}
			engine.client.Transport = hostTestRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Context().Err() != nil {
					t.Fatal("cleanup inherited cancellation")
				}
				calls = append(calls, r.Method+" "+r.URL.Path)
				if name == "unavailable-daemon" {
					return nil, errors.New("offline")
				}
				status := 200
				raw := good
				if r.Method == "DELETE" {
					if r.URL.Path != "/containers/"+cid {
						t.Fatal("removed without full identity", r.URL)
					}
					status = 204
					raw = nil
				} else if len(calls) > 1 && name != "late-survivor" {
					status = 404
					raw = []byte(`{"message":"No such container"}`)
				}
				if name == "wrong-owner" {
					raw = bytes.ReplaceAll(good, []byte(in.Request.RunID), []byte(strings.Repeat("e", 32)))
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(raw)), Request: r}, nil
			})
			transcript, removed := hostCleanup(engine, in, cid, name != "uncertain-create")
			if removed != (name == "cancelled-main") {
				t.Fatalf("removed=%v calls=%v", removed, calls)
			}
			if len(transcript.Entries) == 0 {
				t.Fatal("missing raw cleanup facts")
			}
			if name == "wrong-owner" && len(calls) != 1 {
				t.Fatal("acted on conflicting ownership")
			}
		})
	}
}
func TestHostOwnedStorageBounds(t *testing.T) {
	root, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	stored := hostStored{root: root}
	ref, err := stored.write("ownership.json", []byte("actual"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readHostFile(ref, 100); err != nil {
		t.Fatal(err)
	}
	if _, err = stored.write("ownership.json", []byte("replace")); err == nil {
		t.Fatal("overwrote ownership")
	}
	if _, err = stored.write("../escape", nil); err == nil {
		t.Fatal("escaped evidence root")
	}
	stored.refs = 32
	if _, err = stored.write("next", nil); err == nil {
		t.Fatal("accepted excess references")
	}
}

func TestHostConfiguredBoundaryRejectsDrift(t *testing.T) {
	in := &HostInputs{Profile: candidateHostProfile(), Request: candidateHostRequest(candidateHostProfile()), RequestSHA256: strings.Repeat("b", 64), ProfileSHA256: strings.Repeat("c", 64), Mounts: []HostMount{{"/private/product", "/product", true}, {"/private/product/docs", "/product/docs", false}, {"/private/product/docs/.keep", "/product/docs/.keep", true}}}
	cid := strings.Repeat("d", 64)
	config := hostContainerConfig(in)
	mounts := []any{}
	for _, m := range in.Mounts {
		mounts = append(mounts, map[string]any{"Type": "bind", "Source": m.Source, "Destination": m.Destination, "RW": !m.ReadOnly, "Propagation": "rprivate"})
	}
	inspect := map[string]any{"Id": cid, "Name": "/corvint-author-" + in.Request.RunID, "Image": in.Profile.Image.ConfigDigest, "State": map[string]any{"Running": true, "Paused": false, "Restarting": false, "Dead": false, "Pid": 123}, "Config": config, "HostConfig": config["HostConfig"], "Mounts": mounts, "NetworkSettings": map[string]any{"Ports": map[string]any{}, "Networks": map[string]any{"none": map[string]any{"IPAddress": "", "GlobalIPv6Address": ""}}}}
	good, _ := json.Marshal(inspect)
	if !hostContainerMatches(good, cid, in) {
		t.Fatal("fixed boundary rejected")
	}
	mutations := map[string]func(map[string]any){
		"missing-running-fact":        func(x map[string]any) { delete(x["State"].(map[string]any), "Paused") },
		"missing-mount-readonly-fact": func(x map[string]any) { delete(x["Mounts"].([]any)[0].(map[string]any), "RW") },

		"privileged":              func(x map[string]any) { x["HostConfig"].(map[string]any)["Privileged"] = true },
		"extra-capability":        func(x map[string]any) { x["HostConfig"].(map[string]any)["CapAdd"] = []string{"SYS_ADMIN"} },
		"unmasked-admin":          func(x map[string]any) { x["HostConfig"].(map[string]any)["MaskedPaths"] = []string{} },
		"unknown-runtime-setting": func(x map[string]any) { x["HostConfig"].(map[string]any)["NewPrivilege"] = true },
		"network":                 func(x map[string]any) { x["HostConfig"].(map[string]any)["NetworkMode"] = "host" },
		"image-volume": func(x map[string]any) {
			x["Config"].(map[string]any)["Volumes"] = map[string]any{"/escape": map[string]any{}}
		},
		"effective-extra-credential": func(x map[string]any) { x["Config"].(map[string]any)["Env"] = []string{"GITHUB_TOKEN=unapproved"} },
		"writable-root":              func(x map[string]any) { x["Mounts"].([]any)[0].(map[string]any)["RW"] = true },
		"substituted-scope-object":   func(x map[string]any) { x["Mounts"].([]any)[1].(map[string]any)["Source"] = "/private/copy" },
		"writable-guard":             func(x map[string]any) { x["Mounts"].([]any)[2].(map[string]any)["RW"] = true },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var x map[string]any
			if json.Unmarshal(good, &x) != nil {
				t.Fatal("setup")
			}
			mutate(x)
			raw, _ := json.Marshal(x)
			if hostContainerMatches(raw, cid, in) {
				t.Fatal("boundary drift accepted")
			}
		})
	}
}
func TestHostCandidateMountsRefuseAuthorityAliases(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{".git", "docs", "tests"} {
		if e := os.Mkdir(filepath.Join(root, dir), 0700); e != nil {
			t.Fatal(e)
		}
	}
	for _, path := range []string{"calc.go", "go.mod", "docs/.keep", "tests/.keep"} {
		if e := os.WriteFile(filepath.Join(root, path), []byte("fixture"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r := candidateHostRequest(candidateHostProfile())
	r.Product.Root = root
	d := stepverify.Declaration{GuardPaths: []string{"calc.go", "go.mod", "docs/.keep", "tests/.keep"}}
	if _, err := hostCandidateMounts(r, d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "operator-secret"), []byte("not-author-visible"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := hostCandidateMounts(r, d); err == nil {
		t.Fatal("undeclared root content exposed")
	}
	os.Remove(filepath.Join(root, "operator-secret"))
	os.Remove(filepath.Join(root, "docs/.keep"))
	if err := os.Link(filepath.Join(root, "calc.go"), filepath.Join(root, "docs/.keep")); err != nil {
		t.Fatal(err)
	}
	if _, err := hostCandidateMounts(r, d); err == nil {
		t.Fatal("guard hardlink accepted")
	}
}

func hostTestNativeControlBinding(t *testing.T, ctx context.Context, root, base, head, tree string) func() {
	t.Helper()
	// Confinement booleans here are test assertions, not observed host facts.
	// Native receipts must retain that unauthenticated-assertion uncertainty.
	run := strings.Repeat("1", 32)
	d := stepverify.Declaration{Profile: "corvint-step-declaration/0", SessionID: "run_" + run, CapabilityID: "authoring", Author: stepverify.Checkout{ID: "author", Root: root}, ReadOnly: []stepverify.Checkout{}, WritePaths: []string{"docs/", "tests/"}, GuardPaths: []string{"calc.go", "docs/.keep", "go.mod", "tests/.keep"}, Environment: []stepverify.EnvironmentKey{{Key: "HOME", Class: "NONE"}, {Key: "PATH", Class: "NONE"}, {Key: "TMPDIR", Class: "NONE"}}}
	var err error
	d, err = stepverify.DecodeDeclaration(stepverify.Encode(d))
	if err != nil {
		t.Fatal(err)
	}
	binary, err := exec.LookPath("git")
	if runtime.GOOS == "darwin" {
		actual := procgroup.Run(ctx, procgroup.Spec{Argv: []string{"/usr/bin/xcrun", "--find", "git"}, Dir: "/", Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 10 * time.Second, OutputLimit: 4096, StderrLimit: 4096, ObserveDescendants: true})
		if actual.Err != nil || !actual.ExitObserved || actual.ExitStatus != 0 || !actual.WaitCompleted || !actual.OwnedProcessGroupCleanup {
			t.Fatalf("resolve actual native Git: %+v", actual)
		}
		binary = strings.TrimSpace(string(actual.Stdout))
	}
	if err != nil {
		t.Fatal(err)
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	gitBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	h := stepverify.Host{Profile: "corvint-step-host/0", ObserverID: "observer_" + run, SessionID: d.SessionID, CapabilityID: d.CapabilityID, DeclarationDigest: stepverify.DeclarationDigest(d), MetadataProtected: true, FilesystemConfined: true, GitBinary: binary, GitBinaryDigest: hostDigest(gitBytes), Environment: &d.Environment}
	output, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := &HostInputs{Declaration: d, Request: HostRequest{Product: HostProduct{Root: root, Base: base, Merge: head, Tree: tree}, Observer: HostObserver{ID: h.ObserverID, GitBinary: HostFileRef{Path: binary, SHA256: h.GitBinaryDigest, Bytes: int64(len(gitBytes))}}, OutputRoot: output}}
	evidenceDir := os.Getenv("CORVINT_HOSTPAIR_EVIDENCE_DIR")
	if evidenceDir != "" {
		if !hostPath(evidenceDir) {
			t.Fatal("invalid test evidence destination")
		}
		if err := os.Mkdir(evidenceDir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name string, value any) *HostFileRef {
		t.Helper()
		raw := stepverify.Encode(value)
		if evidenceDir != "" {
			file, err := os.OpenFile(filepath.Join(evidenceDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.Write(raw)
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatal(writeErr, closeErr)
			}
		}
		path := filepath.Join(output, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return &HostFileRef{Path: path, SHA256: hostDigest(raw), Bytes: int64(len(raw))}
	}
	write("declaration.json", d)
	before, err := stepverify.Snapshot(ctx, d, h)
	if err != nil || !before.Complete {
		t.Fatalf("source fixture native before incomplete: %v %+v", err, before)
	}
	receipt, code, err := stepverify.Preflight(d, h)
	if err != nil || code != 0 {
		t.Fatal("test assertion preflight", err, code)
	}
	if len(receipt.Unknowns) == 0 {
		t.Fatal("native uncertainty lost")
	}
	c := HostControl{Operation: "execute", Host: write("host.json", h), Before: write("before.json", before), Preflight: write("preflight.json", receipt)}
	if err := validateHostNativeControl(c, in); err != nil {
		t.Fatal("original complete native control refused", err)
	}
	originalHost, originalPreflight := h, receipt
	hostTestOwnedLifecycle(t, in, h, before, receipt)
	// Keep the original native records and change the independently pinned tree.
	priorTree := in.Request.Product.Tree
	in.Request.Product.Tree = strings.Repeat("0", 40)
	if validateHostNativeControl(c, in) == nil {
		t.Fatal("source binding drift admitted")
	}
	in.Request.Product.Tree = priorTree
	// Produce a genuine incomplete native observation, rather than editing its
	// digest or inventing a success record. A missing actual Git pin stays missing.
	h.GitBinaryDigest = strings.Repeat("0", 64)
	in.Request.Observer.GitBinary.SHA256 = h.GitBinaryDigest
	incomplete, err := stepverify.Snapshot(ctx, d, h)
	if (err != nil && !errors.Is(err, stepverify.ErrUnsupported)) || incomplete.Complete {
		t.Fatal("expected native incomplete observation", err)
	}
	receipt, _, err = stepverify.Preflight(d, h)
	if err != nil {
		t.Fatal(err)
	}
	c.Host = write("host-incomplete.json", h)
	c.Before = write("before-incomplete.json", incomplete)
	c.Preflight = write("preflight-incomplete.json", receipt)
	if validateHostNativeControl(c, in) == nil {
		t.Fatal("incomplete original native state admitted")
	}
	return func() {
		t.Helper()
		after, code, err := stepverify.Verify(ctx, d, originalHost, before)
		write("actual-author-verify.json", after)
		if err != nil || code != 0 || after.WriteScopeVerdict != "PASS" || after.EnvironmentVerdict != "PASS" || after.AfterDigest == nil {
			t.Fatalf("same-object actual author Verify failed: code=%d err=%v receipt=%+v", code, err, after)
		}
		if len(after.Unknowns) == 0 || len(originalPreflight.Unknowns) == 0 {
			t.Fatal("native uncertainty lost")
		}
		t.Logf("unconfined actual author native Verify unknowns: %v; original Preflight unknowns: %v", after.Unknowns, originalPreflight.Unknowns)
		guard := filepath.Join(root, "calc.go")
		original, err := os.ReadFile(guard)
		if err != nil {
			t.Fatal(err)
		}
		// Appending a comment preserves semantic Add behavior and the original host
		// object, so this control specifically discriminates native guard checking.
		f, err := os.OpenFile(guard, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.WriteString("\n// guarded mutation negative control\n")
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
		rejected, code, err := stepverify.Verify(ctx, d, originalHost, before)
		write("guard-mutation-verify.json", rejected)
		if err != nil || code != 1 || rejected.WriteScopeVerdict != "FAIL" {
			t.Fatalf("guarded mutation accepted: code=%d err=%v receipt=%+v", code, err, rejected)
		}
		guarded := false
		for _, finding := range rejected.Findings {
			if finding.Code == "GUARDED_WRITE" && finding.Path == "calc.go" {
				guarded = true
			}
		}
		if !guarded {
			t.Fatalf("missing exact guard finding: %+v", rejected.Findings)
		}
		if err := os.WriteFile(guard, original, 0600); err != nil {
			t.Fatal(err)
		}
		restored, code, err := stepverify.Verify(ctx, d, originalHost, before)
		write("guard-restored-verify.json", restored)
		if err != nil || code != 0 {
			t.Fatalf("same-object guard restore failed: %d %v", code, err)
		}
	}
}

type hostLifecycleEngine struct {
	*hostEngine
	open func(context.Context, string) (*hostExecStream, error)
}

func (e *hostLifecycleEngine) startExec(ctx context.Context, id string) (*hostExecStream, error) {
	return e.open(ctx, id)
}
func hostTestOwnedLifecycle(t *testing.T, native *HostInputs, h stepverify.Host, before stepverify.State, receipt stepverify.EnvironmentReceipt) {
	t.Helper()
	for _, mode := range []string{"success", "author-nonzero", "fast-exit", "stream-truncated", "late-completion", "cleanup-failure"} {
		t.Run("lifecycle-"+mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			profile := candidateHostProfile()
			request := candidateHostRequest(profile)
			request.Product = native.Request.Product
			request.Observer = native.Request.Observer
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			request.OutputRoot = filepath.Join(parent, "run")
			in := &HostInputs{Profile: profile, Request: request, Declaration: native.Declaration, RequestSHA256: strings.Repeat("b", 64), ProfileSHA256: strings.Repeat("c", 64), Mounts: []HostMount{{request.Product.Root, "/product", true}}}
			cid := strings.Repeat("d", 64)
			execID := strings.Repeat("e", 64)
			config := hostContainerConfig(in)
			inspect := map[string]any{"Id": cid, "Name": "/corvint-author-" + request.RunID, "Image": profile.Image.ConfigDigest, "State": map[string]any{"Running": true, "Paused": false, "Restarting": false, "Dead": false, "Pid": 42}, "Config": config, "HostConfig": config["HostConfig"], "Mounts": []any{map[string]any{"Type": "bind", "Source": request.Product.Root, "Destination": "/product", "RW": false, "Propagation": "rprivate"}}, "NetworkSettings": map[string]any{"Ports": map[string]any{}, "Networks": map[string]any{"none": map[string]any{}}}}
			release := make(chan struct{})
			serverStop := make(chan struct{})
			serverJoined := make(chan struct{})
			released := false
			removed := false
			inspections := 0
			engine := hostUnixEngine("/unused-test-endpoint")
			defer engine.close()
			engine.client.Transport = hostTestRoundTrip(func(r *http.Request) (*http.Response, error) {
				var body any
				status := 200
				path := r.URL.Path
				switch {
				case r.Method == "GET" && path == "/containers/corvint-author-"+request.RunID+"/json":
					status = 404
					body = map[string]any{"message": "absent"}
				case r.Method == "GET" && strings.HasPrefix(path, "/images/"):
					body = map[string]any{"Id": profile.Image.ConfigDigest, "RepoDigests": []string{"fixture@" + profile.Image.ManifestDigest}, "Os": "linux", "Architecture": "amd64", "Config": map[string]any{"Env": []string{}, "Volumes": map[string]any{}}}
				case r.Method == "POST" && path == "/containers/create":
					status = 201
					body = map[string]any{"Id": cid, "Warnings": []string{}}
				case r.Method == "POST" && path == "/containers/"+cid+"/start":
					status = 204
				case r.Method == "GET" && path == "/containers/"+cid+"/json":
					body = inspect
					if removed {
						status = 404
						body = map[string]any{"message": "absent"}
					}
				case r.Method == "DELETE" && path == "/containers/"+cid:
					status = 204
					if mode == "cleanup-failure" {
						return nil, errors.New("daemon unavailable")
					}
					removed = true
				case r.Method == "POST" && path == "/containers/"+cid+"/exec":
					status = 201
					body = map[string]any{"Id": execID}
				case r.Method == "GET" && path == "/exec/"+execID+"/json":
					inspections++
					running := inspections <= 2 && mode != "fast-exit"
					if mode == "late-completion" {
						running = true
					}
					code := 0
					if !running && mode == "author-nonzero" {
						code = 7
					}
					body = hostExecInspection{ContainerID: cid, ID: execID, OpenStdin: true, OpenStdout: true, OpenStderr: true, ProcessConfig: hostExecConfig{User: "65532:65532", Entrypoint: "/tools/exec-shim", Arguments: []string{"--internal-envelope", request.Author.SHA256}}, Running: running, PID: 42, ExitCode: code}
					if inspections == 2 && !released {
						close(release)
						released = true
					}
				case r.Method == "GET" && path == "/containers/"+cid+"/top":
					body = hostTop{Titles: []string{"PID", "COMMAND"}, Processes: [][]string{{"42", "/tools/author /admitted/author-input.json /product"}}}
				default:
					return nil, fmt.Errorf("unexpected test route: %s %s", r.Method, path)
				}
				raw := []byte{}
				if body != nil {
					raw, _ = json.Marshal(body)
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(raw)), Request: r}, nil
			})
			fake := &hostLifecycleEngine{hostEngine: engine, open: func(ctx context.Context, id string) (*hostExecStream, error) {
				local, remote := net.Pipe()
				stream := &hostExecStream{conn: local, reader: bufio.NewReader(local), done: make(chan struct{}), joined: make(chan struct{}), remaining: 16 << 20}
				go func() {
					defer close(stream.joined)
					select {
					case <-ctx.Done():
						local.Close()
					case <-stream.done:
					}
				}()
				go func() {
					defer close(serverJoined)
					defer remote.Close()
					env := []string{}
					for _, entry := range profile.Command.Environment {
						env = append(env, entry.Key+"="+entry.Value)
					}
					actual, _ := hostObserveEnvironment(env)
					raw, _ := json.Marshal(HostEnvelopeObservation{"corvint-postmerge-internal-envelope/0", request.Author.SHA256, actual, []HostEnvironmentDigest{}})
					raw = append(raw, '\n')
					if _, err := remote.Write(hostTestFrame(1, raw)); err != nil {
						return
					}
					barrier := make([]byte, 8)
					if _, err := io.ReadFull(remote, barrier); err != nil || string(barrier) != "execute\n" {
						return
					}
					if mode == "fast-exit" {
						return
					}
					select {
					case <-release:
					case <-serverStop:
						return
					case <-ctx.Done():
						return
					}
					frame := hostTestFrame(1, []byte("raw author log\n"))
					if mode == "stream-truncated" {
						frame = frame[:len(frame)-1]
					}
					_, _ = remote.Write(frame)
				}()
				return stream, nil
			}}
			input, send := io.Pipe()
			events, output := io.Pipe()
			observedEvents := make(chan []HostEvent, 1)
			producerError := make(chan error, 1)
			go func() {
				defer events.Close()
				defer send.Close()
				decoder := json.NewDecoder(events)
				all := []HostEvent{}
				for {
					var event HostEvent
					if err := decoder.Decode(&event); err != nil {
						producerError <- err
						observedEvents <- all
						return
					}
					all = append(all, event)
					if event.Event != "ready" {
						producerError <- nil
						observedEvents <- all
						return
					}
					write := func(name string, v any) *HostFileRef {
						raw := stepverify.Encode(v)
						path := filepath.Join(request.OutputRoot, "native", name)
						if err := os.WriteFile(path, raw, 0600); err != nil {
							return nil
						}
						return &HostFileRef{path, hostDigest(raw), int64(len(raw))}
					}
					frame := HostControl{Profile: "corvint-postmerge-host-control/0", Operation: "execute", RunID: request.RunID, RequestSHA256: in.RequestSHA256, ProfileSHA256: in.ProfileSHA256, Host: write("host.json", h), Before: write("before.json", before), Preflight: write("preflight.json", receipt)}
					if err := json.NewEncoder(send).Encode(frame); err != nil {
						producerError <- err
						observedEvents <- all
						return
					}
				}
			}()
			err = runHostOwned(ctx, in, fake, hostAPIRange{}, "test-start-identity", input, output)
			output.Close()
			close(serverStop)
			<-serverJoined
			all := <-observedEvents
			if producerErr := <-producerError; producerErr != nil {
				t.Fatal(producerErr)
			}
			if len(all) != 2 || all[0].Event != "ready" {
				t.Fatalf("missing lifecycle events: %+v err=%v", all, err)
			}
			last := all[len(all)-1]
			wantSuccess := mode == "success" || mode == "author-nonzero"
			if (err == nil) != wantSuccess || (last.Event == "finished") != wantSuccess {
				t.Fatalf("mode=%s err=%v event=%+v", mode, err, last)
			}
			if mode == "author-nonzero" && (last.AuthorExit == nil || last.AuthorExit.Code == nil || *last.AuthorExit.Code != 7 || last.AuthorExit.Signal != nil) {
				t.Fatal("author status lost or client signal invented")
			}
			if mode == "fast-exit" && last.AuthorExit != nil {
				t.Fatal("fast exit manufactured author start")
			}
			if (last.Cleanup == "REMOVED") != (mode != "cleanup-failure") {
				t.Fatal("cleanup claim mismatch", last.Cleanup)
			}
		})
	}
}

func TestHostNativeRecordsShareRetentionBudget(t *testing.T) {
	refs := []HostFileRef{{Path: "/native/host", SHA256: strings.Repeat("a", 64), Bytes: 6 << 20}, {Path: "/native/before", SHA256: strings.Repeat("b", 64), Bytes: 6 << 20}, {Path: "/native/preflight", SHA256: strings.Repeat("c", 64), Bytes: 6 << 20}}
	control := HostControl{Host: &refs[0], Before: &refs[1], Preflight: &refs[2]}
	store := hostStored{bytes: 1024, refs: 4}
	if store.reserveNative(control) == nil || store.bytes != 1024 || store.refs != 4 {
		t.Fatal("native overflow was admitted or partially counted")
	}
	for i := range refs {
		refs[i].Bytes = 1024
	}
	if store.reserveNative(control) != nil || store.bytes != 4096 || store.refs != 7 {
		t.Fatal("native records not included in shared bound")
	}
}

func TestHostDocumentedInspectSerialization(t *testing.T) {
	raw, err := os.ReadFile("testdata/paired-author/engine-inspect-v156.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Inspection json.RawMessage `json:"inspection"`
	}
	if json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("invalid documented fixture")
	}
	p := candidateHostProfile()
	r := candidateHostRequest(p)
	in := &HostInputs{Profile: p, Request: r, RequestSHA256: strings.Repeat("b", 64), ProfileSHA256: strings.Repeat("c", 64), Mounts: []HostMount{{"/private/tmp/product", "/product", true}, {"/private/tmp/product/docs", "/product/docs", false}, {"/private/tmp/product/tests", "/product/tests", false}, {"/private/tmp/product/docs/.keep", "/product/docs/.keep", true}, {"/private/tmp/product/tests/.keep", "/product/tests/.keep", true}, {"/private/tmp/admitted.json", "/admitted/author-input.json", true}, {"/private/tmp/author", "/tools/author", true}, {"/private/tmp/shim", "/tools/exec-shim", true}}}
	cid := strings.Repeat("d", 64)
	if !hostContainerMatches(fixture.Inspection, cid, in) {
		t.Fatal("documented harmless response defaults refused")
	}
	for name, mutate := range map[string]func(map[string]any){
		"null-writable-mount":     func(h map[string]any) { h["Mounts"].([]any)[1] = nil },
		"console-nonzero":         func(h map[string]any) { h["ConsoleSize"] = []int{1, 0} },
		"console-empty":           func(h map[string]any) { h["ConsoleSize"] = []int{} },
		"unknown-zero-collection": func(h map[string]any) { h["UnknownSetting"] = []int{0, 0} },
		"runtime":                 func(h map[string]any) { h["Runtime"] = "unapproved" },
		"privileged":              func(h map[string]any) { h["Privileged"] = true },
		"mount-source":            func(h map[string]any) { h["Mounts"].([]any)[0].(map[string]any)["Source"] = "/private/copied" },
		"mount-readonly-missing":  func(h map[string]any) { delete(h["Mounts"].([]any)[0].(map[string]any), "ReadOnly") },
		"mount-propagation": func(h map[string]any) {
			h["Mounts"].([]any)[0].(map[string]any)["BindOptions"].(map[string]any)["Propagation"] = "rshared"
		},
		"mount-nonrecursive": func(h map[string]any) {
			h["Mounts"].([]any)[0].(map[string]any)["BindOptions"].(map[string]any)["NonRecursive"] = true
		},
		"mount-readonly-nonrecursive": func(h map[string]any) {
			h["Mounts"].([]any)[0].(map[string]any)["BindOptions"].(map[string]any)["ReadOnlyNonRecursive"] = true
		},
		"mount-unknown-false": func(h map[string]any) {
			h["Mounts"].([]any)[0].(map[string]any)["BindOptions"].(map[string]any)["NewOption"] = false
		},
		"mount-consistency": func(h map[string]any) { h["Mounts"].([]any)[0].(map[string]any)["Consistency"] = "delegated" },
	} {
		t.Run(name, func(t *testing.T) {
			var x map[string]any
			if json.Unmarshal(fixture.Inspection, &x) != nil {
				t.Fatal("setup")
			}
			mutate(x["HostConfig"].(map[string]any))
			raw, _ := json.Marshal(x)
			if hostContainerMatches(raw, cid, in) {
				t.Fatal("inspection drift accepted")
			}
		})
	}
}
