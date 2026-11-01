package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
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
	if err := execfile.WriteFile(p, []byte(fmt.Sprintf("#!/bin/sh\necho 'Corvint 1.0.0-rc.1 (build %d)'\n", build)), 0700); err != nil {
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

// scriptBinary is the fixture executable for build; pad appends that many
// bytes of incompressible comment after exit, so a fixture can weigh like a
// real binary.
func scriptBinary(build, pad int) []byte {
	b := []byte(fmt.Sprintf("#!/bin/sh\necho 'Corvint 1.0.0-rc.1 (build %d)'\n", build))
	if pad == 0 {
		return b
	}
	noise := make([]byte, pad/2)
	rand.New(rand.NewSource(int64(build) * int64(pad))).Read(noise)
	return append(append(b, "exit 0\n#"...), hex.EncodeToString(noise)...)
}
func coreArchive(t *testing.T, platform string, build int) []byte {
	return coreArchiveOf(t, platform, scriptBinary(build, 0))
}
func coreArchiveOf(t *testing.T, platform string, binary []byte) []byte {
	t.Helper()
	prefix := "corvint_" + platform + "/"
	files := map[string][]byte{prefix + "corvint": binary, prefix + "SHA256SUMS": []byte(hash(binary) + "  corvint\n")}
	for _, n := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
		files[prefix+n] = []byte("notice")
	}
	return archiveBytes(t, files)
}
func releaseClient(t *testing.T, e *engine, build int) { t.Helper(); releasePadded(t, e, build, 0) }
func releasePadded(t *testing.T, e *engine, build, pad int) {
	t.Helper()
	binary := scriptBinary(build, pad)
	a := coreArchiveOf(t, e.platform, binary)
	name := "corvint_" + e.platform + ".tar.gz"
	r := release{Tag: "v1.0.0-rc.1", Published: time.Now(), Prerelease: true, Body: "experimental", Assets: []asset{{name, "https://github.com/archive"}, {"SHA256SUMS", "https://github.com/sums"}, {"verification-report.json", "https://github.com/qualification"}}}
	rows, _ := json.Marshal([]release{r})
	e.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		var b []byte
		switch req.URL.Path {
		case "/archive":
			b = a
		case "/qualification":
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
	if _, _, err = e.discover(context.Background()); err == nil {
		t.Fatal("Core release selected for Tasks")
	}
}
func TestUPDV0001TasksInstalledVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "corvint-tasks")
	for out, want := range map[string]string{
		`{"codes":[],"command":["version"],"items":[{"formats":["taskman-receipt/0"],"version":"0.0.0-tcp01-unverified+build.407"}],"outcome":"OK","profile":"taskman-command-result/0","warnings":[]}`: "0.0.0-tcp01-unverified+build.407",
		`taskman-command-result/0 build 407`:                                                                   "",
		`{"items":[{"version":"x+build.407"}],"profile":"taskman-command-result/1"}`:                           "",
		`{"items":[{"version":"a+build.407"},{"version":"b+build.407"}],"profile":"taskman-command-result/0"}`: "",
		`{"items":[{"version":""}],"profile":"taskman-command-result/0","note":"build.407"}`:                   "",
		`{"items":[],"profile":"taskman-command-result/0","note":"build.407"}`:                                 "",
	} {
		if err := execfile.WriteFile(bin, []byte("#!/bin/sh\ncat <<'EOF'\n"+out+"\nEOF\n"), 0700); err != nil {
			t.Fatal(err)
		}
		v, n, err := version(context.Background(), bin, "", "tasks")
		if want == "" {
			if err == nil {
				t.Fatalf("%s: accepted as %q build %d", out, v, n)
			}
			continue
		}
		if err != nil || v != want || n != 407 {
			t.Fatalf("%s: %q %d %v", out, v, n, err)
		}
	}
}

// relationClient serves one release with tag and the given assets by name.
func relationClient(e *engine, tag string, assets map[string][]byte) {
	r := release{Tag: tag, Published: time.Now()}
	for name := range assets {
		r.Assets = append(r.Assets, asset{name, "https://github.com/" + name})
	}
	rows, _ := json.Marshal([]release{r})
	e.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		b, ok := assets[strings.TrimPrefix(req.URL.Path, "/")]
		if !ok {
			b = rows
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
}
func TestUPDV0009CheckStatesRelation(t *testing.T) {
	e := fixtureEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 407)
	for tag, want := range map[string]string{"v1.0.0-rc.1": "CURRENT", "v1.0.0-rc.0": "LOCAL_NEWER", "v1.0.0-rc.10": "UPDATE_AVAILABLE", "v1.0.0": "UPDATE_AVAILABLE", "v1.0.0-rc.1+meta.7": "CURRENT", "vnext": "UNKNOWN", "v01.0.0": "UNKNOWN", "v1.0.0-01": "UNKNOWN", "v1.0.0garbage": "UNKNOWN"} {
		relationClient(&e, tag, map[string][]byte{"corvint_" + e.platform + ".tar.gz": nil})
		r, err := e.run(context.Background(), "check")
		if err != nil || r.Relation != want || (want != "UNKNOWN") != (r.RelationBasis == "version") || r.AvailableBuild != 0 || (want == "LOCAL_NEWER") != strings.Contains(r.RelationReason, "only when the verified candidate build is lower") {
			t.Fatalf("core %s: %+v %v", tag, r, err)
		}
	}
	// A label carrying a malformed version is not a semantic version.
	label := filepath.Join(e.config.BinDir, "corvint")
	if err := execfile.WriteFile(label, []byte("#!/bin/sh\necho 'Corvint 1.0.0garbage (build 407)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	relationClient(&e, "v1.0.0", map[string][]byte{"corvint_" + e.platform + ".tar.gz": nil})
	if r, err := e.run(context.Background(), "check"); err != nil || r.Relation != "UNKNOWN" || r.RelationReason == "" {
		t.Fatalf("malformed installed label: %+v %v", r, err)
	}
	e.config.Component = "tasks"
	envelope := `{"items":[{"version":"0.0.0-tcp01-unverified+build.407"}],"profile":"taskman-command-result/0"}`
	if err := execfile.WriteFile(filepath.Join(e.config.BinDir, "corvint-tasks"), []byte("#!/bin/sh\necho '"+envelope+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	archive := "corvint-tasks_" + e.platform + ".tar.gz"
	target := strings.ReplaceAll(e.platform, "_", "/")
	declare := func(build, digest string) []byte {
		return []byte(fmt.Sprintf(`{"profile":"corvint-tasks-archive/0","target":%q,"build":%q,"archiveSha256":%q}`, target, build, digest))
	}
	sums := []byte(hash([]byte("archive")) + "  " + archive + "\n")
	for name, c := range map[string]struct {
		declaration []byte
		relation    string
		build       int
	}{
		"older":      {declare("202", hash([]byte("archive"))), "LOCAL_NEWER", 202},
		"same":       {declare("407", hash([]byte("archive"))), "CURRENT", 407},
		"newer":      {declare("500", hash([]byte("archive"))), "UPDATE_AVAILABLE", 500},
		"unbound":    {declare("500", hash([]byte("other"))), "UNKNOWN", 0},
		"undeclared": {nil, "UNKNOWN", 0},
	} {
		assets := map[string][]byte{archive: []byte("archive"), "SHA256SUMS": sums}
		if c.declaration != nil {
			assets["build-verification.json"] = c.declaration
		}
		relationClient(&e, "tasks-dev-20260929.2", assets)
		r, err := e.run(context.Background(), "check")
		if err != nil || r.InstalledBuild != 407 || r.Relation != c.relation || r.AvailableBuild != c.build {
			t.Fatalf("tasks %s: %+v %v", name, r, err)
		}
		if (c.relation == "UNKNOWN") != (r.RelationReason != "" && r.RelationBasis == "") || (c.relation == "LOCAL_NEWER") != strings.Contains(r.RelationReason, "downgrade") {
			t.Fatalf("tasks %s reason: %+v", name, r)
		}
	}
	if _, err := os.Stat(e.config.StateDir); !os.IsNotExist(err) {
		t.Fatal("check created state")
	}
	for _, c := range [][2]string{{"1.0.0-rc.10", "1.0.0-rc.2"}, {"1.0.0-alpha.1", "1.0.0-alpha"}, {"1.0.0-alpha.beta", "1.0.0-alpha.1"}, {"1.0.0", "1.0.0-rc.1"}, {"1.10.0", "1.9.0"}, {"1.0.0-rc.a", "1.0.0-rc.-1"}, {"1.0.0-100000000000000000000", "1.0.0-99999999999999999999"}} {
		if compareSemver(c[0], c[1]) <= 0 || compareSemver(c[1], c[0]) >= 0 {
			t.Fatalf("semver %s > %s", c[0], c[1])
		}
	}
	if compareSemver("1.0.0+a", "1.0.0+b") != 0 {
		t.Fatal("build metadata affected precedence")
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
	_, _, err := e.discover(context.Background())
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
	if r, err = e.run(context.Background(), "apply"); err != nil {
		t.Fatal(err)
	}
	e.config.AllowNetwork = false
	if _, err = e.run(context.Background(), "rollback"); err != nil {
		t.Fatalf("repeated lifecycle: %v", err)
	}
	writeScript(t, dest, 163)
	// The second apply superseded the first transaction (proposed UPD-V0-007),
	// so the retained one is the one to tamper with.
	execfile.WriteFile(filepath.Join(filepath.Dir(r.Receipt), "previous"), []byte("tampered"), 0700)
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("tampered rollback accepted")
	}
}
func TestUPDV0006SubprocessCancellationCleanup(t *testing.T) {
	e := fixtureEngine(t)
	binary := filepath.Join(e.config.BinDir, "hang")
	script := "#!/bin/sh\nsleep 60 &\nwait\n"
	execfile.WriteFile(binary, []byte(script), 0700)
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
	execfile.WriteFile(p, []byte("candidate"), 0700)
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
	cancel context.CancelFunc
	first  bool
	read   int
	closed bool
}

func (b *interruptedBody) Read(p []byte) (int, error) {
	if !b.first {
		b.first = true
		n := copy(p, []byte("partial archive bytes"))
		b.read += n
		b.cancel() // cancel only after the first chunk was read, not on a wall-clock deadline
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
	// No deadline: cancellation is driven only by interruptedBody.Read; the test timeout bounds a hang.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var body *interruptedBody
	e.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/archive" {
			body = &interruptedBody{ctx: req.Context(), cancel: cancel}
			return &http.Response{StatusCode: 200, Body: body}, nil
		}
		return original.RoundTrip(req)
	})
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
	if err = execfile.WriteFile(filepath.Join(stage, "previous"), previous, 0700); err != nil {
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

// proposed UPD-V0-007: apply keeps one rollback-capable transaction per
// component and destination, drops its download, smoke home and candidate copy,
// sweeps a transaction a killed apply left without receipt.json, leaves what it
// cannot classify, names every removal, and rollback still restores the bytes.
func TestUPDV0007RetentionBoundInterruptedSweepAndRollback(t *testing.T) {
	const pad = 4 << 20
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	if err := execfile.WriteFile(dest, scriptBinary(161, pad), 0700); err != nil {
		t.Fatal(err)
	}
	state := e.config.StateDir
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	// An apply killed before its receipt, long enough ago that no run can
	// still own it; a fresh one a live run may still be staging; and an
	// operator directory that only looks like a transaction.
	stale := time.Now().Add(-2 * incompleteStaleAfter)
	killed := filepath.Join(state, "transaction-100")
	fresh := filepath.Join(state, "transaction-400")
	notes := filepath.Join(state, "transaction-notes")
	for _, dir := range []string{killed, fresh, notes} {
		if err := os.MkdirAll(filepath.Join(dir, "extracted"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "archive.tar.gz"), scriptBinary(1, pad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, stale, stale); err != nil && dir != fresh {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(fresh, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(state, "transaction-200")
	malformed := filepath.Join(state, "transaction-300")
	for dir, body := range map[string]string{other: `{"Component":"tasks","Destination":"/elsewhere/corvint-tasks"}`, malformed: "not json"} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(state, "operator-notes")
	if err := os.WriteFile(unrelated, []byte("kept"), 0600); err != nil {
		t.Fatal(err)
	}
	var applied []Result
	for _, build := range []int{162, 163, 164} {
		releasePadded(t, &e, build, pad)
		r, err := e.run(context.Background(), "apply")
		if err != nil || r.Action != "applied" {
			t.Fatalf("apply %d: %+v %v", build, r, err)
		}
		applied = append(applied, r)
		t.Logf("after apply %d: state holds %d bytes", build, treeBytes(t, state))
	}
	first, second, third := applied[0], applied[1], applied[2]
	stage := filepath.Dir(first.Receipt)
	want := []string{killed, filepath.Join(stage, "archive.tar.gz"), filepath.Join(stage, "smoke-home"), filepath.Join(stage, "extracted", "corvint_"+e.platform, "corvint")}
	if strings.Join(first.Removed, "\n") != strings.Join(want, "\n") {
		t.Fatalf("first apply removed %q, want %q", first.Removed, want)
	}
	if len(first.Left) != 2 || first.Left[0] != fresh+": incomplete transaction younger than 30m0s" || !strings.HasPrefix(first.Left[1], malformed+": ") {
		t.Fatalf("first apply left %q", first.Left)
	}
	if !slices.Contains(second.Removed, stage) || slices.Contains(second.Removed, other) {
		t.Fatalf("second apply removed %q", second.Removed)
	}
	for _, kept := range []string{other, malformed, unrelated, fresh, filepath.Join(notes, "archive.tar.gz"), third.Receipt, filepath.Join(filepath.Dir(third.Receipt), "previous"), filepath.Join(filepath.Dir(third.Receipt), "verification-report.json")} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("%s was removed: %v", kept, err)
		}
	}
	transactions, _ := filepath.Glob(filepath.Join(state, "transaction-*"))
	if len(transactions) != 5 {
		t.Fatalf("transactions after three applies: %q", transactions)
	}
	// An apply killed after its stage existed and before its receipt: the
	// next run sweeps it and rollback still restores build 163's bytes.
	interrupted, err := os.MkdirTemp(state, "transaction-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(interrupted, "archive.tar.gz"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(interrupted, stale, stale); err != nil {
		t.Fatal(err)
	}
	// Rollback refuses any unreadable receipt (UPD-V0-005), so the operator
	// clears the one retention left and named before rolling back.
	if err = os.RemoveAll(malformed); err != nil {
		t.Fatal(err)
	}
	e.config.AllowNetwork = false
	r, err := e.run(context.Background(), "rollback")
	if err != nil || !slices.Contains(r.Removed, interrupted) {
		t.Fatalf("rollback after interruption: %+v %v", r, err)
	}
	if got, _ := digest(dest); got != hash(scriptBinary(163, pad)) {
		t.Fatal("rollback did not restore the previous binary")
	}
	if _, err = e.run(context.Background(), "rollback"); err == nil {
		t.Fatal("rollback beyond the retained transaction succeeded")
	}
	unlock, err := lockDirectory(state)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err = e.run(context.Background(), "rollback"); err == nil || !strings.Contains(err.Error(), "state directory") {
		t.Fatalf("rollback ran while another process held the state lock: %v", err)
	}
}

func treeBytes(t *testing.T, root string) (total int64) {
	t.Helper()
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// TestUPDV0008CoreSwitchRefreshesIndexedCheckouts: after a Core apply or
// rollback the switched binary refreshes each explicit root and the working
// checkout only when it already has a snapshot store; a failed refresh is
// reported and does not fail the switch.
func TestUPDV0008CoreSwitchRefreshesIndexedCheckouts(t *testing.T) {
	e := fixtureEngine(t)
	dest := filepath.Join(e.config.BinDir, "corvint")
	writeScript(t, dest, 162)
	base := filepath.Dir(e.config.BinDir)
	indexedRoot := filepath.Join(base, "indexed")
	plainRoot := filepath.Join(base, "plain")
	explicitRoot := filepath.Join(base, "explicit")
	for _, dir := range []string{filepath.Join(indexedRoot, ".git", "corvint", "index"), filepath.Join(plainRoot, ".git"), explicitRoot} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(base, "refresh.log")
	// The release fixture's binary answers --version only; the refresh check
	// swaps in a binary that records its argv and answers `index --if-stale`.
	refresher := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'Corvint 1.0.0-rc.1 (build 163)'; exit 0; fi\necho \"$*\" >> %q\nif [ \"$2\" = %q ]; then echo broken >&2; exit 3; fi\nif [ \"$2\" = %q ]; then echo '{\"mutates\":false,\"state\":\"fresh\"}'; else echo '{\"mutates\":true}'; fi\n", log, explicitRoot, indexedRoot)

	e.config.WorkDir = filepath.Join(indexedRoot, "sub")
	os.MkdirAll(e.config.WorkDir, 0700)
	e.config.RefreshRoots = []string{indexedRoot}
	releaseClient(t, &e, 163)
	r, err := e.execute(context.Background(), "apply")
	if err != nil || r.Action != "applied" {
		t.Fatalf("apply: %+v %v", r, err)
	}
	// The fixture binary is not a real Core, so its receipt is unreadable.
	if len(r.IndexRefresh) != 1 || r.IndexRefresh[0].Root != indexedRoot || r.IndexRefresh[0].State != "failed" {
		t.Fatalf("explicit and working root not refreshed once: %+v", r.IndexRefresh)
	}

	execfile.WriteFile(dest, []byte(refresher), 0700)
	got := refreshIndexes(context.Background(), dest, []string{explicitRoot, filepath.Join(plainRoot, ".")}, filepath.Join(indexedRoot, "sub"))
	want := []IndexRefresh{
		{Root: explicitRoot, State: "failed", Reason: "exit 3: broken"},
		{Root: plainRoot, State: "built"},
		{Root: indexedRoot, State: "fresh"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("refresh outcomes:\n got %+v\nwant %+v", got, want)
	}
	if got = refreshIndexes(context.Background(), dest, nil, plainRoot); !slices.Equal(got, []IndexRefresh{{Root: plainRoot, State: "skipped", Reason: "no index snapshot store"}}) {
		t.Fatalf("unindexed working checkout refreshed: %+v", got)
	}
	if got = refreshIndexes(context.Background(), dest, nil, base); len(got) != 0 {
		t.Fatalf("directory outside any checkout refreshed: %+v", got)
	}
	calls, _ := os.ReadFile(log)
	if want := fmt.Sprintf("--root %s index --if-stale\n--root %s index --if-stale\n--root %s index --if-stale\n", explicitRoot, plainRoot, indexedRoot); string(calls) != want {
		t.Fatalf("refresh argv:\n%s", calls)
	}

	// Rollback switches the binary too, so it refreshes.
	e.config.AllowNetwork = false
	writeScript(t, dest, 163)
	r, err = e.execute(context.Background(), "rollback")
	if err != nil || len(r.IndexRefresh) != 1 || r.IndexRefresh[0] != (IndexRefresh{Root: indexedRoot, State: "failed", Reason: "unreadable index receipt"}) {
		t.Fatalf("rollback did not refresh: %+v %v", r, err)
	}
	// An unchanged apply switches nothing and refreshes nothing.
	writeScript(t, dest, 163)
	e.config.AllowNetwork = true
	if r, err = e.execute(context.Background(), "apply"); err != nil || r.Action != "unchanged" || r.IndexRefresh != nil {
		t.Fatalf("unchanged apply: %+v %v", r, err)
	}
	// A successful Tasks rollback switches a binary that keys no snapshot.
	tasks := filepath.Join(e.config.BinDir, "corvint-tasks")
	writeScript(t, tasks, 2)
	current, _ := digest(tasks)
	stage := filepath.Join(e.config.StateDir, "transaction-1")
	os.Mkdir(stage, 0700)
	writeScript(t, filepath.Join(stage, "previous"), 1)
	previous, _ := digest(filepath.Join(stage, "previous"))
	encoded, _ := json.Marshal(receipt{"tasks", tasks, previous, current})
	os.WriteFile(filepath.Join(stage, "receipt.json"), encoded, 0600)
	e.config.Component, e.config.AllowNetwork = "tasks", false
	if r, err = e.execute(context.Background(), "rollback"); err != nil || r.IndexRefresh != nil {
		t.Fatalf("tasks rollback: %+v %v", r, err)
	}
	if after, _ := digest(tasks); after != previous {
		t.Fatal("tasks rollback did not switch")
	}
}

// TestUPDV0010UnnumberedLocalBuild: an installed build 0 or a label without a
// build number still checks against the available release with status
// UNNUMBERED and relation UNKNOWN, and apply refuses to replace it.
func TestUPDV0010UnnumberedLocalBuild(t *testing.T) {
	e := fixtureEngine(t)
	core := filepath.Join(e.config.BinDir, "corvint")
	tasks := filepath.Join(e.config.BinDir, "corvint-tasks")
	archive := "corvint-tasks_" + e.platform + ".tar.gz"
	declaration := []byte(fmt.Sprintf(`{"profile":"corvint-tasks-archive/0","target":%q,"build":"500","archiveSha256":%q}`, strings.ReplaceAll(e.platform, "_", "/"), hash([]byte("archive"))))
	tasksAssets := map[string][]byte{archive: []byte("archive"), "SHA256SUMS": []byte(hash([]byte("archive")) + "  " + archive + "\n"), "build-verification.json": declaration}
	for _, c := range []struct{ name, component, label string }{
		{"core build 0", "core", "Corvint 1.0.0-rc.3 (build 0)"},
		{"core no build", "core", "Corvint 1.0.0-rc.3"},
		{"core unstamped commit line", "core", "Corvint 1.0.0-rc.3.dev (commit " + strings.Repeat("f", 40) + ", modified)"},
		{"tasks build 0", "tasks", "0.0.0-tcp01-unverified+build.0"},
		{"tasks no build", "tasks", "0.0.0-tcp01-unverified"},
	} {
		e.config.Component = c.component
		e.config.StateDir = filepath.Join(filepath.Dir(e.config.BinDir), "state-"+strings.ReplaceAll(c.name, " ", "-"))
		dest, tag, wantBuild := core, "v1.0.0-rc.3", 0
		body := "echo '" + c.label + "'"
		if c.component == "tasks" {
			dest, tag, wantBuild = tasks, "tasks-dev-20260929.2", 500
			body = `echo '{"items":[{"version":"` + c.label + `"}],"profile":"taskman-command-result/0"}'`
			relationClient(&e, tag, tasksAssets)
		} else {
			releaseClient(t, &e, 163)
			tag = "v1.0.0-rc.1"
		}
		if err := execfile.WriteFile(dest, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		r, err := e.run(context.Background(), "check")
		if err != nil || r.AvailableTag != tag || r.InstalledVersion != c.label || r.InstalledBuild != 0 || r.InstalledBuildStatus != "UNNUMBERED" || r.Freshness != "RELEASE_OBSERVED" || r.Relation != "UNKNOWN" || r.RelationBasis != "" || !strings.Contains(r.RelationReason, "cannot be ordered against releases") || r.AvailableBuild != wantBuild {
			t.Fatalf("%s check: %+v %v", c.name, r, err)
		}
		if _, err = os.Stat(e.config.StateDir); !os.IsNotExist(err) {
			t.Fatalf("%s: check created state", c.name)
		}
		before, _ := digest(dest)
		r, err = e.run(context.Background(), "apply")
		if err == nil || !strings.Contains(err.Error(), "refusing to replace unnumbered installed build") || r.InstalledBuildStatus != "UNNUMBERED" || r.Action != "apply" || r.Receipt != "" {
			t.Fatalf("%s apply: %+v %v", c.name, r, err)
		}
		if after, _ := digest(dest); after != before {
			t.Fatalf("%s: refused apply changed destination", c.name)
		}
		if entries, _ := os.ReadDir(e.config.StateDir); len(entries) != 0 {
			t.Fatalf("%s: refused apply created a transaction %v", c.name, entries)
		}
	}
	// A malformed build marker is not an unnumbered build.
	e.config.Component = "core"
	if err := execfile.WriteFile(core, []byte("#!/bin/sh\necho 'Corvint 1.0.0-rc.3 (build -1)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if r, err := e.run(context.Background(), "check"); err == nil || err.Error() != "invalid numeric build" || r.InstalledBuildStatus != "" {
		t.Fatalf("malformed build marker: %+v %v", r, err)
	}
	// A numbered install is unaffected, and a candidate without a build is
	// still an unknown build that apply refuses.
	e.config.Component = "core"
	writeScript(t, core, 163)
	releaseClient(t, &e, 0)
	before, _ := digest(core)
	if _, err := e.run(context.Background(), "apply"); err == nil || !strings.Contains(err.Error(), "candidate numeric build") {
		t.Fatalf("build-0 candidate: %v", err)
	}
	if after, _ := digest(core); after != before {
		t.Fatal("refused candidate changed destination")
	}
}

// TestPUBV0053UpdateReadsTheCommitBearingLine: the current --version line, with the commit member
// after the build, yields the same numbered build as the earlier line.
func TestPUBV0053UpdateReadsTheCommitBearingLine(t *testing.T) {
	dir := t.TempDir()
	for line, want := range map[string]int{
		"Corvint 1.0.0-rc.3 (build 163)":                                                       163,
		"Corvint 1.0.0-rc.3 (build 163, commit " + strings.Repeat("a", 40) + ")":               163,
		"Corvint 1.0.0-rc.3.dev (build 164, commit " + strings.Repeat("a", 40) + ", modified)": 164,
	} {
		binary := filepath.Join(dir, "corvint")
		if err := execfile.WriteFile(binary, []byte("#!/bin/sh\necho '"+line+"'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		got, build, err := version(context.Background(), binary, dir, "core")
		if err != nil || build != want || got != line {
			t.Fatalf("%q: %q %d %v", line, got, build, err)
		}
	}
}

// TestUPDV0367CheckExplainsSourceAndUnknown checks that every check names the
// official source and channel, that an RC or developer prerelease is selected
// without any stable release, and that an empty inventory, an unreadable one
// and a missing network grant each stay UNKNOWN with an actionable reason.
func TestUPDV0367CheckExplainsSourceAndUnknown(t *testing.T) {
	e := fixtureEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 407)
	serve := func(status int, rows []release) {
		b, _ := json.Marshal(rows)
		e.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(b))}, nil
		})}
	}
	core := asset{"corvint_" + e.platform + ".tar.gz", "https://github.com/core"}
	serve(200, []release{
		{Tag: "v1.0.0-rc.3", Prerelease: true, Published: time.Now(), Assets: []asset{core}},
		{Tag: "tasks-dev-20261030.1", Prerelease: true, Published: time.Now().Add(time.Hour), Assets: []asset{{"corvint-tasks_" + e.platform + ".tar.gz", "https://github.com/tasks"}}},
		{Tag: "v9.0.0", Draft: true, Published: time.Now(), Assets: []asset{core}},
	})
	r, err := e.run(context.Background(), "check")
	if err != nil || r.AvailableTag != "v1.0.0-rc.3" || r.Source != "beamfall/corvint" || r.Channel != "v" || r.ReleasesObserved == nil || *r.ReleasesObserved != 2 || r.UnknownReason != "" || r.Freshness != "RELEASE_OBSERVED" || r.Authenticated {
		t.Fatalf("prerelease check: %+v %v", r, err)
	}
	serve(200, []release{{Tag: "tasks-dev-20261030.1", Published: time.Now(), Assets: []asset{{"corvint-tasks_" + e.platform + ".tar.gz", "https://github.com/tasks"}}}})
	r, err = e.run(context.Background(), "check")
	if err == nil || r.Freshness != "UNKNOWN" || r.InstalledVersion == "" || r.ReleasesObserved == nil || *r.ReleasesObserved != 1 || !strings.Contains(r.UnknownReason, `none has a tag starting "v"`) || !strings.Contains(r.UnknownReason, "owner-authorized") {
		t.Fatalf("no Core release: %+v %v", r, err)
	}
	serve(200, nil)
	e.config.Component = "tasks"
	envelope := `{"items":[{"version":"0.0.0-tcp01-unverified+build.7"}],"profile":"taskman-command-result/0"}`
	if err = execfile.WriteFile(filepath.Join(e.config.BinDir, "corvint-tasks"), []byte("#!/bin/sh\necho '"+envelope+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r, err = e.run(context.Background(), "check")
	if err == nil || r.Channel != "tasks-dev-" || r.InstalledBuild != 7 || !strings.Contains(r.UnknownReason, "lists 0 published release(s)") || !strings.Contains(r.UnknownReason, "tasks-dev-YYYYMMDD.N") {
		t.Fatalf("empty inventory: %+v %v", r, err)
	}
	serve(403, nil)
	r, err = e.run(context.Background(), "check")
	if err == nil || r.Freshness != "UNKNOWN" || !strings.Contains(r.UnknownReason, "status 403") || !strings.Contains(r.UnknownReason, "--github-token-env") {
		t.Fatalf("unreadable inventory: %+v %v", r, err)
	}
	e.config.AllowNetwork = false
	r, err = e.run(context.Background(), "check")
	if err == nil || !strings.Contains(r.UnknownReason, "--allow-network") || r.Source != "beamfall/corvint" {
		t.Fatalf("offline: %+v %v", r, err)
	}
	if _, err = os.Stat(e.config.StateDir); !os.IsNotExist(err) {
		t.Fatal("check created state")
	}
}

// TestUPDV0368TokenReachesOnlyTheAPIHost checks that an operator token
// authenticates api.github.com requests, never a download host, and that a
// named but unset variable refuses before any request.
func TestUPDV0368TokenReachesOnlyTheAPIHost(t *testing.T) {
	e := fixtureEngine(t)
	e.token = "secret-token"
	seen := map[string]string{}
	e.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		seen[req.URL.Host] = req.Header.Get("Authorization")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})}
	for _, address := range []string{"https://api.github.com/repos/beamfall/corvint/releases", "https://github.com/beamfall/corvint/releases/download/v1/x", "https://release-assets.githubusercontent.com/x"} {
		if _, err := e.get(context.Background(), address, metadataLimit); err != nil {
			t.Fatal(err)
		}
	}
	if seen["api.github.com"] != "Bearer secret-token" || seen["github.com"] != "" || seen["release-assets.githubusercontent.com"] != "" {
		t.Fatalf("authorization headers: %v", seen)
	}
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 407)
	r, err := e.run(context.Background(), "check")
	if err == nil || !r.Authenticated || strings.Contains(r.UnknownReason, "--github-token-env") || strings.Contains(fmt.Sprint(r), "secret-token") {
		t.Fatalf("authenticated empty check: %+v %v", r, err)
	}
	t.Setenv("CORVINT_UPDATE_TEST_TOKEN", "")
	r, err = Run(context.Background(), "check", Config{Component: "core", BinDir: e.config.BinDir, StateDir: e.config.StateDir, AllowNetwork: true, TokenEnv: "CORVINT_UPDATE_TEST_TOKEN"})
	if err == nil || !strings.Contains(r.UnknownReason, "CORVINT_UPDATE_TEST_TOKEN") {
		t.Fatalf("unset token variable: %+v %v", r, err)
	}
}

// TestUPDV0367ReleasesObservedSerialisedAndTokenRefusal checks the wire form:
// a read empty inventory serialises releasesObserved 0, an unread one omits
// it, an unset token variable still names source and channel, and rollback,
// which is offline, ignores the token option (UPD-V0-367, UPD-V0-368).
func TestUPDV0367ReleasesObservedSerialisedAndTokenRefusal(t *testing.T) {
	e := fixtureEngine(t)
	writeScript(t, filepath.Join(e.config.BinDir, "corvint"), 407)
	e.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})}
	r, err := e.run(context.Background(), "check")
	wire, _ := json.Marshal(r)
	if err == nil || !strings.Contains(string(wire), `"releasesObserved":0`) {
		t.Fatalf("read empty inventory: %s %v", wire, err)
	}
	e.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	r, _ = e.run(context.Background(), "check")
	if wire, _ = json.Marshal(r); strings.Contains(string(wire), "releasesObserved") {
		t.Fatalf("unread inventory reported a count: %s", wire)
	}
	t.Setenv("CORVINT_UPDATE_TEST_TOKEN", "")
	c := Config{Component: "tasks", BinDir: e.config.BinDir, StateDir: e.config.StateDir, AllowNetwork: true, TokenEnv: "CORVINT_UPDATE_TEST_TOKEN"}
	r, err = Run(context.Background(), "check", c)
	if err == nil || r.Source != "beamfall/corvint" || r.Channel != "tasks-dev-" {
		t.Fatalf("unset token refusal: %+v %v", r, err)
	}
	r, err = Run(context.Background(), "rollback", c)
	if err == nil || strings.Contains(err.Error(), "CORVINT_UPDATE_TEST_TOKEN") || strings.Contains(r.UnknownReason, "CORVINT_UPDATE_TEST_TOKEN") {
		t.Fatalf("rollback refused on the token option: %+v %v", r, err)
	}
}

// UPD-V0-910 (V1-3301): a .dev build of an RC precedes the published RC, while
// a .dev build still outranks an earlier release.
func TestUPDV0910DevelopmentBuildPrecedesItsRelease(t *testing.T) {
	e := fixtureEngine(t)
	for _, c := range []struct{ label, tag, want string }{
		{"1.0.0-rc.3.dev", "v1.0.0-rc.3", "UPDATE_AVAILABLE"},
		{"1.0.0-rc.3.dev", "v1.0.0-rc.2", "LOCAL_NEWER"},
		{"1.0.0-rc.3.dev", "v1.0.0-rc.4", "UPDATE_AVAILABLE"},
		{"1.0.0-rc.3.dev", "v1.0.0", "UPDATE_AVAILABLE"},
		{"1.0.0-dev", "v1.0.0", "UPDATE_AVAILABLE"},
		{"1.0.0-rc.3", "v1.0.0-rc.3", "CURRENT"},
		// `3-dev` is one identifier, not a trailing `dev`: ordinary precedence applies.
		{"1.0.0-rc.3-dev", "v1.0.0-rc.3", "LOCAL_NEWER"},
		{"1.0.0-rc.dev.1", "v1.0.0-rc", "LOCAL_NEWER"},
	} {
		label := filepath.Join(e.config.BinDir, "corvint")
		if err := execfile.WriteFile(label, []byte("#!/bin/sh\necho 'Corvint "+c.label+" (build 7, commit dbc1713779fc17ce0386fe95d3e0f6347f5f9aee)'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		relationClient(&e, c.tag, map[string][]byte{"corvint_" + e.platform + ".tar.gz": nil})
		r, err := e.run(context.Background(), "check")
		if err != nil || r.Relation != c.want || r.RelationBasis != "version" {
			t.Fatalf("%s against %s: %+v %v", c.label, c.tag, r, err)
		}
		if dev := c.label == "1.0.0-rc.3.dev" && c.tag == "v1.0.0-rc.3" || c.label == "1.0.0-dev"; dev != strings.Contains(r.RelationReason, "development build of this release") {
			t.Fatalf("%s against %s reason: %q", c.label, c.tag, r.RelationReason)
		}
	}
}
