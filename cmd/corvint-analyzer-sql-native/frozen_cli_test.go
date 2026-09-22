package main

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func buildFrozenCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-sql-native")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, ".")
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	return binary
}

func runFrozenCLI(t *testing.T, binary, frame string, dir string, env []string) []byte {
	t.Helper()
	command := exec.Command(binary)
	command.Dir = dir
	command.Env = env
	command.Stdin = bytes.NewBufferString(frame)
	got, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCLIFrozenLiteralGoldensAndInputOrder(t *testing.T) {
	binary := buildFrozenCLI(t)
	const successFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	const successGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a"}],"facts":[{"kind":"sqlite.query.read","input_handle":"input-1","related_handle":"-","subject":"query","predicate":"uses-statement","value":"select","instance_id":"root:1","evidence_sha256":"sha256:fbb3585f6c2e93b21fee27e8b6a23a20bf77eabb380e4728b3a589a3d7a20496"}]}` + "\n"
	const invalidIdentifierFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"bad/id","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	const unknownFamilyFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"postgres","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	const unknownFieldFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[],"extra":"x"},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	const orderedFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-2","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/a.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"},{"handle":"input-2","family":"sqlite.query","path":"internal/store/b.sql","sha256":"sha256:8e7003d62f9d8cbd28da2f243bb0d215bfd4622c716be09be89a8764d9f4c7cb","content_base64":"U0VMRUNUIDI7"}]}` + "\n"
	const orderedGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-2","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a"},{"handle":"input-2","sha256":"sha256:8e7003d62f9d8cbd28da2f243bb0d215bfd4622c716be09be89a8764d9f4c7cb"}],"facts":[{"kind":"sqlite.query.read","input_handle":"input-1","related_handle":"-","subject":"query","predicate":"uses-statement","value":"select","instance_id":"root:1","evidence_sha256":"sha256:73a4ae2b50710dcdb411aff7ea0ba1cca6f65d2b245d0b4bb68e888d988e9c85"},{"kind":"sqlite.query.read","input_handle":"input-2","related_handle":"-","subject":"query","predicate":"uses-statement","value":"select","instance_id":"root:1","evidence_sha256":"sha256:5c3a77ca3fc4c571900ae53e7437222933836a0d743d573a0af91a87d261d1f7"}]}` + "\n"
	const reversedFrame = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-2","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-2","family":"sqlite.query","path":"internal/store/b.sql","sha256":"sha256:8e7003d62f9d8cbd28da2f243bb0d215bfd4622c716be09be89a8764d9f4c7cb","content_base64":"U0VMRUNUIDI7"},{"handle":"input-1","family":"sqlite.query","path":"internal/store/a.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	const noncanonicalGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	const invalidIdentifierGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"INVALID_IDENTIFIER"}` + "\n"
	const unknownFamilyGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNKNOWN_FAMILY"}` + "\n"
	const unknownFieldGolden = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNKNOWN_FIELD"}` + "\n"

	env := []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir()}
	for _, tc := range []struct{ name, frame, want string }{
		{"success", successFrame, successGolden},
		{"invalid-identifier", invalidIdentifierFrame, invalidIdentifierGolden},
		{"unknown-family", unknownFamilyFrame, unknownFamilyGolden},
		{"unknown-field", unknownFieldFrame, unknownFieldGolden},
		{"ordered-inputs", orderedFrame, orderedGolden},
		{"reversed-inputs", reversedFrame, noncanonicalGolden},
		{"malformed-envelope", "{}\n", noncanonicalGolden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runFrozenCLI(t, binary, tc.frame, t.TempDir(), env)
			if string(got) != tc.want || got[len(got)-1] != '\n' || bytes.Count(got, []byte{'\n'}) != 1 {
				t.Fatalf("cli bytes %q want %q", got, tc.want)
			}
		})
	}
}

func TestCLILiteralSQLiteRejectionGoldens(t *testing.T) {
	binary := buildFrozenCLI(t)
	const prefix = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-3","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"`
	const frameTail = `","content_base64":"`
	const frameEnd = `"}]}` + "\n"
	const resultPrefix = `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-3","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"`
	const resultTail = `"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir()}
	for _, tc := range []struct{ name, sha, base64 string }{
		{"temp-virtual-table", "sha256:a2cb53ccc2ec1c44ab824c1955003f642d64beabf20fa7e5e36ee8d06f9e794a", "Q1JFQVRFIFRFTVAgVklSVFVBTCBUQUJMRSB0IFVTSU5HIGZ0czUoYSk7"},
		{"temp-index", "sha256:71029d2c2de8de263ee363faf32b793f7e59d2040196ba3acdade3cdad50713f", "Q1JFQVRFIFRFTVAgSU5ERVggaXggT04gdChhKTs="},
		{"qualified-index-target", "sha256:257a9d43ee132f953ab5a6286999b0df8f006199ba2b426f50d474f32bb660f2", "Q1JFQVRFIElOREVYIGl4IE9OIG1haW4udChhKTs="},
		{"trailing-table-constraint", "sha256:ee5ce3283310714dc5364b8d6fb916f1651dffa40fcfb2fa42c6fbe5a3890681", "Q1JFQVRFIFRBQkxFIHQgKGlkIElOVEVHRVIsIFVOSVFVRShpZCkgdHJhaWxpbmcpOw=="},
		{"empty-select-expression", "sha256:b2f6a6fe3cde0f13038c29adaea5b8e90f8710b505ca3b37877a28a433925aa2", "U0VMRUNUICgpOw=="},
		{"escape-without-like", "sha256:345a17c1a839c8e7d7152a13caf8ca6e0c0912a5e8d093f1f1fb72a0f25e4778", "U0VMRUNUIDEgRVNDQVBFIDI7"},
		{"between-missing-and", "sha256:0fbdda48e1ca6da48e37471d5316fce8a2c8ed01e718b4addad22093890b3e1c", "U0VMRUNUIDEgQkVUV0VFTiAyOw=="},
		{"case-missing-end", "sha256:88412d9fbfa19f6fd2a2e72cda0c3f42cb6c79c0133ebf4ccc9d7bc93d97e790", "U0VMRUNUIENBU0UgV0hFTiAxIFRIRU4gMjs="},
		{"numeric-collate", "sha256:be49026274e56c4b31935920f8b28fe940def806aa74c91cb7523ad86850ce0b", "U0VMRUNUIDEgQ09MTEFURSAxOw=="},
		{"type-parameter-identifier", "sha256:50238f93a3e6dad34456e0b113ad0ba0da34cb683087022ca5b4da8289c17bdf", "Q1JFQVRFIFRBQkxFIHQgKGlkIElOVChmb28pKTs="},
		{"unique-autoincrement", "sha256:5e9639ea879c2091a9933899017297e9f8db739b1e34acbfa7a52e13c829326a", "Q1JFQVRFIFRBQkxFIHQgKGlkIElOVEVHRVIgVU5JUVVFIEFVVE9JTkNSRU1FTlQpOw=="},
		{"strict-missing-second-option", "sha256:e2921e2aa3798bba8f8a20c942c32bea1b13d8b40b0fe29d9ec7476b5f454981", "Q1JFQVRFIFRBQkxFIHQoaWQgSU5URUdFUikgU1RSSUNULDs="},
		{"without-rowid-missing-second-option", "sha256:8040297388f5134f14332a4067bba5aa4c0f22a1a9311fe53f958629262b1ca7", "Q1JFQVRFIFRBQkxFIHQoaWQgSU5URUdFUikgV0lUSE9VVCBST1dJRCw7"},
		{"single-bang-expression", "sha256:a4ed034b01b2a36ac2ed21c183a0146f1265cf406594263c93e84ed141de4aed", "U0VMRUNUIDEgISAyOw=="},
		{"literal-call", "sha256:3a36b804c412959853cd3f1992c26cc982cf1e78e235f20fadaf2bcd1df7ebce", "U0VMRUNUIDEoMik7"},
		{"invalid-limit", "sha256:8e8c015ca90816120ff6aa12b6536172a5ad4419d4c122bf00c734d5d24ca182", "U0VMRUNUIDEgTElNSVQgQlk7"},
		{"open-window", "sha256:fd68ea18cfbe56822079722c455e023d1c682c6d96c7b162417918dd634f0317", "U0VMRUNUIDEgV0lORE9XIGZvbzs="},
		{"literal-from", "sha256:7524da810a144376c5bcaff1d91a29572ed13bcdcaaa80c9e0e48e4c0620aa21", "U0VMRUNUIDEgRlJPTSAxOw=="},
		{"forged-view", "sha256:fbeb7b4893c61d50b4acb84feb1b684940b970934e1fbef51a4439a2df6898b5", "Q1JFQVRFIFZJRVcgdiBBUyBTRUxFQ1QgMSAhIDI7"},
		{"forged-trigger", "sha256:fa45813809622f5b9c104719f77875acaa73a9e6a34e4f152219dfd979c79daf", "Q1JFQVRFIFRSSUdHRVIgdHIgQUZURVIgSU5TRVJUIE9OIHQgQkVHSU4gU0VMRUNUIDEgISAyOyBFTkQ7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := prefix + tc.sha + frameTail + tc.base64 + frameEnd
			want := resultPrefix + tc.sha + resultTail
			if got := runFrozenCLI(t, binary, frame, t.TempDir(), env); string(got) != want {
				t.Fatalf("cli bytes %q want %q", got, want)
			}
		})
	}
}

func networkSpy(t *testing.T) (string, <-chan struct{}, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
			close(hit)
		}
	}()
	return listener.Addr().String(), hit, func() {
		_ = listener.Close()
		<-done
	}
}

func TestCLIBuiltProcessNetworkAndAmbientSpies(t *testing.T) {
	binary := buildFrozenCLI(t)
	root := t.TempDir()
	spyDir := filepath.Join(root, "spy")
	if err := os.Mkdir(spyDir, 0700); err != nil {
		t.Fatal(err)
	}
	controlMarker := filepath.Join(root, "control-process-hit")
	candidateMarker := filepath.Join(root, "candidate-process-hit")
	spy := filepath.Join(spyDir, "sqlite3")
	if err := os.WriteFile(spy, []byte("#!/bin/sh\nprintf hit > \"$SPY_MARKER\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	control := exec.Command(spy)
	control.Env = []string{"SPY_MARKER=" + controlMarker}
	if output, err := control.CombinedOutput(); err != nil {
		t.Fatalf("process-spy positive control: %v: %s", err, output)
	}
	if _, err := os.Stat(controlMarker); err != nil {
		t.Fatalf("process-spy positive control did not record: %v", err)
	}

	ambient := filepath.Join(root, "ambient")
	if err := os.MkdirAll(filepath.Join(ambient, "internal", "store"), 0700); err != nil {
		t.Fatal(err)
	}
	ambientFile := filepath.Join(ambient, "internal", "store", "query.sql")
	const ambientContents = "ambient bytes must not be read or changed"
	if err := os.WriteFile(ambientFile, []byte(ambientContents), 0600); err != nil {
		t.Fatal(err)
	}

	address, controlHit, stopControlSpy := networkSpy(t)
	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("network-spy positive control: %v", err)
	}
	_ = connection.Close()
	select {
	case <-controlHit:
	case <-time.After(time.Second):
		t.Fatal("network-spy positive control did not record")
	}
	stopControlSpy()

	address, candidateHit, stopCandidateSpy := networkSpy(t)
	defer stopCandidateSpy()
	frame := `{"profile":"corvint-analyzer-candidate/sqlite-3.51.0-source-v1","family":"sqlite","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"sqlite.query","path":"internal/store/query.sql","sha256":"sha256:17db4fd369edb9244b9f91d9aeed145c3d04ad8ba6e95d06247f07a63527d11a","content_base64":"U0VMRUNUIDE7"}]}` + "\n"
	env := []string{
		"PATH=" + spyDir,
		"HOME=" + filepath.Join(root, "home"),
		"TMPDIR=" + filepath.Join(root, "tmp"),
		"SPY_MARKER=" + candidateMarker,
		"CORVINT_SQL_NATIVE_NETWORK_SPY=" + address,
	}
	got := runFrozenCLI(t, binary, frame, ambient, env)
	if !bytes.Contains(got, []byte(`"status":"CANDIDATE"`)) {
		t.Fatalf("candidate spy run: %q", got)
	}
	if _, err := os.Stat(candidateMarker); !os.IsNotExist(err) {
		t.Fatalf("candidate invoked ambient process: %v", err)
	}
	if got, err := os.ReadFile(ambientFile); err != nil || string(got) != ambientContents {
		t.Fatalf("candidate changed ambient file bytes=%q err=%v", got, err)
	}
	select {
	case <-candidateHit:
		t.Fatal("candidate contacted network spy")
	case <-time.After(100 * time.Millisecond):
	}
}
