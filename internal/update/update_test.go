package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureEngine(t *testing.T) engine {
	t.Helper()
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := Config{Component: "core", BinDir: filepath.Join(dir, "bin"), StateDir: filepath.Join(dir, "state"), AllowNetwork: true}
	if err = os.Mkdir(c.BinDir, 0700); err != nil {
		t.Fatal(err)
	}
	return engine{config: c, api: "https://api.github.com/releases", platform: runtime.GOOS + "_" + runtime.GOARCH, inspect: func(string, string) error { return nil }}
}
func writeScript(t *testing.T, p string, build int) {
	t.Helper()
	if err := os.WriteFile(p, []byte(fmt.Sprintf("#!/bin/sh\necho 'Corvint 1.0.0-rc.1 (build %d)'\n", build)), 0700); err != nil {
		t.Fatal(err)
	}
}
func archiveBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}
func coreArchive(t *testing.T, platform string, build int) []byte {
	t.Helper()
	binary := []byte(fmt.Sprintf("#!/bin/sh\necho 'Corvint 1.0.0-rc.1 (build %d)'\n", build))
	prefix := "corvint_" + platform + "/"
	files := map[string][]byte{prefix + "corvint": binary, prefix + "SHA256SUMS": []byte(hash(binary) + "  corvint\n")}
	for _, n := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
		files[prefix+n] = []byte("notice")
	}
	return archiveBytes(t, files)
}
func releaseClient(t *testing.T, e *engine, build int) {
	t.Helper()
	a := coreArchive(t, e.platform, build)
	name := "corvint_" + e.platform + ".tar.gz"
	r := release{Tag: "v1.0.0-rc.1", Published: time.Now(), Prerelease: true, Body: "experimental", Assets: []asset{{name, "https://github.com/archive"}, {"SHA256SUMS", "https://github.com/sums"}, {"verification-report.json", "https://github.com/qualification"}}}
	rows, _ := json.Marshal([]release{r})
	e.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		var b []byte
		switch req.URL.Path {
		case "/archive":
			b = a
		case "/qualification":
			binary := []byte(fmt.Sprintf("#!/bin/sh\necho 'Corvint 1.0.0-rc.1 (build %d)'\n", build))
			parts := strings.Split(e.platform, "_")
			b = []byte(fmt.Sprintf(`{"schema":"corvint.release-go-archive-report.v2","verdict":"PASS","targets":[{"goos":%q,"goarch":%q,"archiveName":%q,"retainedBinary":{"sha256":%q},"retainedArchive":{"sha256":%q}}]}`, parts[0], parts[1], name, hash(binary), hash(a)))
		case "/sums":
			b = []byte(hash(a) + "  " + name + "\n")
		default:
			b = rows
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
}
func TestUPDV0001OfflineReadOnlyAndChannels(t *testing.T) {
	e := fixtureEngine(t)
	e.config.AllowNetwork = false
	_, err := e.run(context.Background(), "check")
	if err == nil {
		t.Fatal("offline accepted")
	}
	if _, err = os.Stat(e.config.StateDir); !os.IsNotExist(err) {
		t.Fatal("offline check created state")
	}
	e.config.AllowNetwork = true
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 163)
	releaseClient(t, &e, 163)
	r, err := e.run(context.Background(), "check")
	if err != nil || r.AvailableTag != "v1.0.0-rc.1" {
		t.Fatalf("check: %+v %v", r, err)
	}
	if _, err = os.Stat(e.config.StateDir); !os.IsNotExist(err) {
		t.Fatal("check created state")
	}
	e.config.Component = "tasks"
	if _, err = e.discover(context.Background()); err == nil {
		t.Fatal("Core release selected for Tasks")
	}
}
func TestUPDV0001PaginationCapUnknown(t *testing.T) {
	e := fixtureEngine(t)
	rows := make([]release, 100)
	rows[0] = release{Tag: "v1", Published: time.Now(), Assets: []asset{{"corvint_" + e.platform + ".tar.gz", "https://github.com/archive"}}}
	b, _ := json.Marshal(rows)
	calls := 0
	e.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
	_, err := e.discover(context.Background())
	if err == nil || calls != 5 {
		t.Fatalf("cap: calls=%d err=%v", calls, err)
	}
}
func TestUPDV0002ChecksumsAndArchive(t *testing.T) {
	for _, s := range []string{"bad", strings.Repeat("0", 64) + "  ../escape", strings.Repeat("0", 64) + "  x\n" + strings.Repeat("0", 64) + "  x"} {
		if _, err := parseSums([]byte(s)); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	e := fixtureEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 162)
	releaseClient(t, &e, 163)
	original := e.client.Transport
	e.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		res, err := original.RoundTrip(r)
		if r.URL.Path == "/sums" {
			res.Body = io.NopCloser(strings.NewReader(strings.Repeat("0", 64) + "  corvint_" + e.platform + ".tar.gz\n"))
		}
		return res, err
	})
	if _, err := e.run(context.Background(), "apply"); err == nil {
		t.Fatal("bad checksum applied")
	}
}
func TestUPDV0003ArchivePathsAndLocks(t *testing.T) {
	for _, n := range []string{"../outside", "/absolute", "a/../../escape", "a\\b"} {
		t.Run(n, func(t *testing.T) {
			dir := t.TempDir()
			a := filepath.Join(dir, "a.tgz")
			os.WriteFile(a, archiveBytes(t, map[string][]byte{n: []byte("x")}), 0600)
			if err := extract(a, filepath.Join(dir, "out")); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	e := fixtureEngine(t)
	if err := validateDirs(e.config.BinDir, e.config.BinDir+"/state"); err == nil {
		t.Fatal("overlap allowed")
	}
	unlock, err := lockDirectory(e.config.BinDir)
	if err != nil {
		t.Fatal(err)
	}
	if u, err := lockDirectory(e.config.BinDir); err == nil {
		u()
		t.Fatal("concurrent lock allowed")
	}
	unlock()
	u, err := lockDirectory(e.config.BinDir)
	if err != nil {
		t.Fatal(err)
	}
	u()
}
func TestUPDV0004ActivationDowngradeCancelRace(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 164)
	releaseClient(t, &e, 163)
	before, _ := digest(dest)
	if _, err := e.run(context.Background(), "apply"); err == nil {
		t.Fatal("downgrade accepted")
	}
	after, _ := digest(dest)
	if before != after {
		t.Fatal("downgrade changed destination")
	}
	source := filepath.Join(filepath.Dir(e.config.BinDir), "candidate")
	writeScript(t, source, 165)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := activate(ctx, source, dest, before); err == nil {
		t.Fatal("cancelled activation")
	}
	if err := activate(context.Background(), source, dest, "foreign"); err == nil {
		t.Fatal("digest conflict accepted")
	}
	after, _ = digest(dest)
	if before != after {
		t.Fatal("failed switch changed destination")
	}
}
func TestUPDV0005ApplyRollbackAndPreparedReceipt(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	before, _ := digest(dest)
	releaseClient(t, &e, 163)
	r, err := e.run(context.Background(), "apply")
	if err != nil {
		t.Fatal(err)
	}
	if r.Receipt == "" || r.Action != "applied" {
		t.Fatalf("missing receipt %+v", r)
	}
	e.config.AllowNetwork = false
	if _, err = e.run(context.Background(), "rollback"); err != nil {
		t.Fatal(err)
	}
	after, _ := digest(dest)
	if before != after {
		t.Fatal("rollback bytes differ")
	}
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("rollback guessed nonmatching receipt")
	}
	e.config.AllowNetwork = true
	if _, err = e.run(context.Background(), "apply"); err != nil {
		t.Fatal(err)
	}
	e.config.AllowNetwork = false
	if _, err = e.run(context.Background(), "rollback"); err != nil {
		t.Fatalf("repeated lifecycle: %v", err)
	}
	writeScript(t, dest, 163)
	os.WriteFile(filepath.Join(filepath.Dir(r.Receipt), "previous"), []byte("tampered"), 0700)
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("tampered rollback accepted")
	}
}
func TestUPDV0006SubprocessCancellationCleanup(t *testing.T) {
	e := fixtureEngine(t)
	binary := filepath.Join(e.config.BinDir, "hang")
	script := "#!/bin/sh\nsleep 60 &\nwait\n"
	os.WriteFile(binary, []byte(script), 0700)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	o := runProcess(ctx, binary, e.config.BinDir, "--version")
	if !o.Cancelled || !o.OwnedProcessGroupCleanup || !o.WaitCompleted || !o.PipesDrained {
		t.Fatalf("cleanup incomplete: %+v", o)
	}
}

func TestUPDV0002PlatformAndQualificationIdentity(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = inspectPlatform(binary, runtime.GOOS+"_"+runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	if err = inspectPlatform(binary, "wrong_arch"); err == nil {
		t.Fatal("wrong platform accepted")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "candidate")
	os.WriteFile(p, []byte("candidate"), 0700)
	if err = verifyQualification([]byte(`{"schema":"corvint.release-go-archive-report.v2","verdict":"PASS","targets":[]}`), dir, "core", "darwin_arm64", "archive", p); err == nil {
		t.Fatal("missing target accepted")
	}
}
func TestUPDV0003ArchiveLinksCaseAndBounds(t *testing.T) {
	for _, headers := range [][]tar.Header{{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}}, {{Name: "link", Typeflag: tar.TypeLink, Linkname: "outside"}}, {{Name: "A/one", Mode: 0600}, {Name: "a/two", Mode: 0600}}, {{Name: "large", Size: extractedLimit + 1}}} {
		dir := t.TempDir()
		var data bytes.Buffer
		gz := gzip.NewWriter(&data)
		tw := tar.NewWriter(gz)
		for i := range headers {
			tw.WriteHeader(&headers[i])
		}
		tw.Close()
		gz.Close()
		p := filepath.Join(dir, "archive")
		os.WriteFile(p, data.Bytes(), 0600)
		if err := extract(p, filepath.Join(dir, "out")); err == nil {
			t.Fatalf("unsafe headers accepted: %+v", headers)
		}
	}
	e := fixtureEngine(t)
	link := filepath.Join(filepath.Dir(e.config.BinDir), "linked")
	os.Symlink(e.config.BinDir, link)
	if err := validateDirs(link, e.config.StateDir); err == nil {
		t.Fatal("symlink bin accepted")
	}
}
func TestUPDV0004EqualBuildDivergenceAndDownloadCancellation(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 163)
	releaseClient(t, &e, 163)
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("# differing bytes\n")
	f.Close()
	if _, err = e.run(context.Background(), "apply"); err == nil {
		t.Fatal("equal build divergence accepted")
	}
	before, _ := digest(dest)
	e.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if _, err = e.run(ctx, "apply"); err == nil {
		t.Fatal("cancelled request accepted")
	}
	after, _ := digest(dest)
	if before != after {
		t.Fatal("download cancellation changed binary")
	}
}

func TestUPDV0004UnchangedCleanupAndDestinationDrift(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 163)
	releaseClient(t, &e, 163)
	for i := 0; i < 3; i++ {
		r, err := e.run(context.Background(), "apply")
		if err != nil {
			t.Fatal(err)
		}
		if r.Action != "unchanged" || r.Freshness != "CURRENT" || strings.Contains(r.Qualification, e.config.StateDir) {
			t.Fatalf("invalid unchanged result: %+v", r)
		}
		entries, err := os.ReadDir(e.config.StateDir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("no-op retained staging: %v %v", entries, err)
		}
	}
	original := e.client.Transport
	e.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		res, err := original.RoundTrip(req)
		if req.URL.Path == "/archive" {
			writeScript(t, dest, 164)
		}
		return res, err
	})
	r, err := e.run(context.Background(), "apply")
	if err == nil || r.Freshness == "CURRENT" || !strings.Contains(err.Error(), "destination digest conflict") {
		t.Fatalf("drift accepted: %+v %v", r, err)
	}
}

type interruptedBody struct {
	ctx    context.Context
	first  bool
	read   int
	closed bool
}

func (b *interruptedBody) Read(p []byte) (int, error) {
	if !b.first {
		b.first = true
		n := copy(p, []byte("partial archive bytes"))
		b.read += n
		return n, nil
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *interruptedBody) Close() error { b.closed = true; return nil }
func TestUPDV0004PartialArchiveCancellation(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	before, _ := digest(dest)
	releaseClient(t, &e, 163)
	original := e.client.Transport
	var body *interruptedBody
	e.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/archive" {
			body = &interruptedBody{ctx: req.Context()}
			return &http.Response{StatusCode: 200, Body: body}, nil
		}
		return original.RoundTrip(req)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := e.run(ctx, "apply"); err == nil {
		t.Fatal("partial cancelled archive accepted")
	}
	if body == nil || body.read == 0 || !body.closed {
		t.Fatalf("archive body not partially read and closed: %+v", body)
	}
	after, _ := digest(dest)
	entries, err := os.ReadDir(e.config.StateDir)
	if before != after || err != nil || len(entries) != 0 {
		t.Fatalf("cancellation changed binary or retained staging: %v %v", entries, err)
	}
}
func TestUPDV0002InternalTamperWithValidOuterChecksum(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	before, _ := digest(dest)
	releaseClient(t, &e, 163)
	prefix := "corvint_" + e.platform + "/"
	files := map[string][]byte{prefix + "corvint": []byte("tampered"), prefix + "SHA256SUMS": []byte(hash([]byte("original")) + "  corvint\n")}
	for _, n := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
		files[prefix+n] = []byte("notice")
	}
	a := archiveBytes(t, files)
	original := e.client.Transport
	e.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/archive":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(a))}, nil
		case "/sums":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(hash(a) + "  corvint_" + e.platform + ".tar.gz\n"))}, nil
		}
		return original.RoundTrip(req)
	})
	_, err := e.run(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "internal checksum mismatch") {
		t.Fatalf("expected inner checksum refusal: %v", err)
	}
	after, _ := digest(dest)
	if before != after {
		t.Fatal("inner tamper changed destination")
	}
}
func TestUPDV0005PreparedReceiptInterruptionRecovery(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	previous, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(e.config.StateDir, "transaction-interrupted")
	if err = os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(stage, "candidate")
	writeScript(t, candidate, 163)
	candidateDigest, _ := digest(candidate)
	rec := receipt{e.config.Component, dest, hash(previous), candidateDigest}
	b, _ := json.Marshal(rec)
	if err = os.WriteFile(filepath.Join(stage, "receipt.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, "previous"), previous, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = activate(ctx, candidate, dest, rec.PreviousDigest); err == nil {
		t.Fatal("cancelled prepared transaction activated")
	}
	e.config.AllowNetwork = false
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("pre-switch receipt incorrectly matched old destination")
	}
	actual, _ := digest(dest)
	if actual != rec.PreviousDigest {
		t.Fatal("pre-switch interruption changed destination")
	}
	// Simulate interruption directly after rename: no completion record is written.
	if err = activate(context.Background(), candidate, dest, rec.PreviousDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = e.run(context.Background(), "rollback"); err != nil {
		t.Fatalf("prepared receipt did not recover post-switch interruption: %v", err)
	}
	actual, _ = digest(dest)
	if actual != rec.PreviousDigest {
		t.Fatal("post-switch recovery changed previous bytes")
	}
}
