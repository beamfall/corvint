package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && (os.Args[1] == "worker" || os.Args[1] == "guardian") {
		if e := entry(); e != nil {
			os.Stderr.WriteString(e.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestDriverRejectsForgedVerdicts(t *testing.T) {
	t.Run("PLE-V0-005 driver rejects forged and duplicate answers", func(t *testing.T) {
		for _, b := range []string{`{"passed":true}`, `<testsuite failures="0"/>`, "{\"Nonce\":\"fake\",\"Phase\":\"completed\"}\n", "[]\n"} {
			if TestCanonicalOutput([]byte(b)) == nil {
				t.Fatal("forged verdict passed")
			}
		}
		rows := []answer{}
		for _, v := range vectors() {
			rows = append(rows, answer{v.Output, v.Rejected})
		}
		good := mustJSONLine(rows)
		if e := TestCanonicalOutput(good); e != nil {
			t.Fatal(e)
		}
		if TestCanonicalOutput(append(good, good...)) == nil {
			t.Fatal("duplicate output passed")
		}
		rows[0].Output = []byte("wrong")
		if TestCanonicalOutput(mustJSONLine(rows)) == nil {
			t.Fatal("bad output passed")
		}

	})
}
func TestSourceAdmission(t *testing.T) {
	for _, s := range []string{"package wp3codec; import \"os\"", "package wp3codec\n//go:linkname x y\n", "package wp3codec; import \"unsafe\"", "package wp3codec; import \"net\"", "package wp3codec; import \"os/exec\""} {
		if validateSource([]byte(s)) == nil {
			t.Fatal(s)
		}
	}
}
func section(id byte, p []byte) []byte {
	b := []byte{id}
	b = binary.AppendUvarint(b, uint64(len(p)))
	return append(b, p...)
}
func module(body []byte, start bool, memory bool) []byte {
	b := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	b = append(b, section(1, []byte{1, 0x60, 0, 0})...)
	b = append(b, section(3, []byte{1, 0})...)
	if memory {
		b = append(b, section(5, []byte{1, 0, 1})...)
	}
	b = append(b, section(7, []byte{1, 6, '_', 's', 't', 'a', 'r', 't', 0, 0})...)
	if start {
		b = append(b, section(8, []byte{0})...)
	}
	code := append([]byte{0}, body...)
	code = append(code, 0x0b)
	p := binary.AppendUvarint([]byte{1}, uint64(len(code)))
	p = append(p, code...)
	return append(b, section(10, p)...)
}
func TestRuntimeBounds(t *testing.T) {
	t.Run("PLE-V0-004 WASI loops and allocations remain bounded", func(t *testing.T) {
		for _, start := range []bool{false, true} {
			before := time.Now()
			_, e := executeProfile(module([]byte{3, 0x40, 0x0c, 0, 0x0b}, start, false), nil, false)
			if e == nil || time.Since(before) > 3*time.Second {
				t.Fatalf("loop start=%v %v", start, e)
			}
		}
		// memory.grow(8192) from one page must return -1; trap if it succeeds.
		grow := []byte{0x41, 0x80, 0x40, 0x40, 0, 0x41, 0x7f, 0x47, 0x04, 0x40, 0x00, 0x0b}
		if _, e := executeProfile(module(grow, false, true), nil, false); e != nil {
			t.Fatal(e)
		}
		bad := []byte{0, 97, 115, 109, 1, 0, 0, 0}
		bad = append(bad, section(0, make([]byte, (1<<20)+1))...)
		if structuralCheck(bad) == nil {
			t.Fatal("custom exhaustion")
		}
		if structuralCheck([]byte{0, 97}) == nil {
			t.Fatal("malformed")
		}
		if _, e := execute(module(nil, false, false), nil); e == nil {
			t.Fatal("missing import set")
		}

	})
}
func TestForbiddenImport(t *testing.T) {
	t.Run("PLE-V0-004 foreign process import is refused", func(t *testing.T) {
		b := []byte{0, 97, 115, 109, 1, 0, 0, 0}
		b = append(b, section(1, []byte{1, 0x60, 0, 0})...)
		p := []byte{1, 3, 'e', 'n', 'v', 4, 's', 'p', 'a', 'w', 0, 0}
		b = append(b, section(2, p)...)
		if _, e := executeProfile(b, nil, false); e == nil || !strings.Contains(e.Error(), "foreign import") {
			t.Fatalf("%v", e)
		}

	})
}
func TestOutputBound(t *testing.T) {
	var b limitedBuffer
	b.max = 2
	if _, e := b.Write([]byte("123")); e == nil {
		t.Fatal("overflow")
	}
}
func TestRealPackageAndMutant(t *testing.T) {
	if os.Getenv("CORVINT_NATIVE_CAMPAIGN") != "1" {
		t.Skip("actual native RSS/process campaign requires explicit opt-in outside sandbox")
	}
	source, e := os.ReadFile("../../internal/wp3codec/codec.go")
	if e != nil {
		t.Fatal(e)
	}
	goPath, e := exec.LookPath("go")
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []bool{false, true} {
		candidate := source
		if bad {
			candidate = bytes.Replace(source, []byte("return out.Bytes(), nil"), []byte("return []byte(`forged passing JUnit`), nil"), 1)
		}
		r := run(candidate, goPath)
		if bad && r.Status == "PASS" {
			t.Fatal("actual bad-code mutant passed")
		}
		if !bad && r.Status != "PASS" {
			t.Fatalf("real package: %+v", r)
		}
		if r.Authority != "NONE" {
			t.Fatal("prototype authority")
		}
		t.Logf("bad=%v receipt=%s", bad, mustJSONLine(r))
	}
}
func TestGuardianLossAndSignals(t *testing.T) {
	if os.Getenv("CORVINT_NATIVE_CAMPAIGN") != "1" {
		t.Skip("native process campaign")
	}
	exe, _ := os.Executable()
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		t.Run(sig.String(), func(t *testing.T) {
			lifeR, lifeW, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer lifeR.Close()
			defer lifeW.Close()
			// Build a trusted compiler process tree to exercise descendant teardown, not just wasm cancellation.
			source, _ := os.ReadFile("../../internal/wp3codec/codec.go")
			goPath, _ := exec.LookPath("go")
			w := work{Nonce: strings.Repeat("a", 64), Mode: "build", Go: goPath, Source: source}
			cmd := exec.Command(exe, "guardian")
			cmd.ExtraFiles = []*os.File{lifeR}
			cmd.Stdin = bytes.NewReader(mustJSONLine(w))
			cmd.Env = []string{}
			var errout bytes.Buffer
			cmd.Stderr = &errout
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			defer cmd.Process.Kill()
			lifeR.Close()
			pgid := 0
			until := time.Now().Add(3 * time.Second)
			for pgid == 0 && time.Now().Before(until) {
				raw, _ := exec.Command("/bin/ps", "-axo", "pid=,ppid=").Output()
				for _, line := range strings.Split(string(raw), "\n") {
					fields := strings.Fields(line)
					if len(fields) == 2 && fields[1] == jsonNumber(cmd.Process.Pid) {
						json.Unmarshal([]byte(fields[0]), &pgid)
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pgid == 0 {
				t.Fatalf("worker not observed %s", errout.String())
			}
			_ = cmd.Process.Signal(sig)
			_ = cmd.Wait()
			until = time.Now().Add(3 * time.Second)
			for syscall.Kill(-pgid, 0) != syscall.ESRCH && time.Now().Before(until) {
				time.Sleep(10 * time.Millisecond)
			}
			if syscall.Kill(-pgid, 0) != syscall.ESRCH {
				syscall.Kill(-pgid, syscall.SIGKILL)
				t.Fatalf("group survived guardian %v", sig)
			}
		})
	}
}
func jsonNumber(i int) string { b, _ := json.Marshal(i); return string(b) }
func TestSuperviseCancellation(t *testing.T) {
	if os.Getenv("CORVINT_NATIVE_CAMPAIGN") != "1" {
		t.Skip("native process campaign")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := supervise(ctx, work{Nonce: strings.Repeat("b", 64), Mode: "execute"})
	if e == nil {
		t.Fatal("cancelled run passed")
	}
}

var _ = filepath.Separator

func TestIgnoredWASIOutputErrorStillFails(t *testing.T) {
	for _, fd := range []byte{1, 2} {
		b := []byte{0, 97, 115, 109, 1, 0, 0, 0}
		b = append(b, section(1, []byte{2, 0x60, 4, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 0, 0})...)
		imports := []byte{1, 22}
		imports = append(imports, []byte("wasi_snapshot_preview1")...)
		imports = append(imports, 8)
		imports = append(imports, []byte("fd_write")...)
		imports = append(imports, 0, 0)
		b = append(b, section(2, imports)...)
		b = append(b, section(3, []byte{1, 1})...)
		b = append(b, section(5, []byte{1, 0, 17})...)
		b = append(b, section(7, []byte{1, 6, '_', 's', 't', 'a', 'r', 't', 0, 1})...)
		body := []byte{0, 0x41, fd, 0x41, 0, 0x41, 1, 0x41, 8, 0x10, 0, 0x1a, 0x0b}
		code := binary.AppendUvarint([]byte{1}, uint64(len(body)))
		code = append(code, body...)
		b = append(b, section(10, code)...)
		data := make([]byte, 16+maxOutput+1)
		binary.LittleEndian.PutUint32(data[0:4], 16)
		binary.LittleEndian.PutUint32(data[4:8], maxOutput+1)
		payload := []byte{1, 0, 0x41, 0, 0x0b}
		payload = binary.AppendUvarint(payload, uint64(len(data)))
		payload = append(payload, data...)
		b = append(b, section(11, payload)...)
		if _, e := executeProfile(b, nil, false); e == nil || !strings.Contains(e.Error(), "output overflow") {
			t.Fatalf("fd%d ignored overflow: %v", fd, e)
		}
	}
}
func TestTableAllocationBound(t *testing.T) {
	raw := []byte{1, 0x70, 0}
	raw = binary.AppendUvarint(raw, 65537)
	if tableBound(raw) == nil {
		t.Fatal("excessive table allocation")
	}
}
