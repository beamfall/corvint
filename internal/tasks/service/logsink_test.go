package service

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func (s *logSink) pendingLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

func TestSERVICE500_LogSinkRotatesWithinTwoFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs", "main")
	s := openLogSink(dir, "stderr")
	chunk := bytes.Repeat([]byte("x"), logBufferMax)
	const chunks = 2*logFileMax/logBufferMax + 40
	for i := 0; i < chunks; i++ {
		_, _ = s.Write(chunk)
		want := uint64((i + 1) * logBufferMax)
		waitFor(t, "chunk written", func() bool { st := s.Stats(); return st.Written+st.Dropped == want })
	}
	st := s.Close(time.Second)
	if st.Dropped != 0 || st.Written != chunks*logBufferMax || st.IOError != "" {
		t.Fatalf("stats %+v", st)
	}
	cur, err1 := os.Stat(filepath.Join(dir, "stderr.log"))
	old, err2 := os.Stat(filepath.Join(dir, "stderr.log.1"))
	if err1 != nil || err2 != nil || cur.Size() > logFileMax || old.Size() > logFileMax {
		t.Fatalf("rotation %v %v", err1, err2)
	}
	if cur.Mode().Perm() != 0o600 {
		t.Fatalf("log mode %v", cur.Mode())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("retained %d files, want 2", len(entries))
	}
}

func TestSERVICE500_LogSinkDropsBeyondBufferWithoutBlocking(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs", "main")
	s := openLogSink(dir, "stderr")
	gate := make(chan struct{})
	s.write = func(f *os.File, b []byte) (int, error) {
		<-gate
		return f.Write(b)
	}
	_, _ = s.Write([]byte("a"))
	waitFor(t, "writer busy", func() bool { return s.pendingLen() == 0 })
	start := time.Now()
	if n, err := s.Write(make([]byte, logBufferMax+100)); n != logBufferMax+100 || err != nil {
		t.Fatalf("write %d %v", n, err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("a full sink blocked its writer")
	}
	if st := s.Stats(); st.Dropped != 100 {
		t.Fatalf("dropped %d, want 100", st.Dropped)
	}
	close(gate)
	st := s.Close(time.Second)
	if st.Written != 1+logBufferMax || st.Dropped != 100 {
		t.Fatalf("stats %+v", st)
	}
}

func TestSERVICE500_LogSinkIOErrorIsReportedNotBlocking(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs", "main")
	s := openLogSink(dir, "stderr")
	s.write = func(*os.File, []byte) (int, error) { return 0, errors.New("disk full") }
	_, _ = s.Write([]byte("hello"))
	waitFor(t, "LOG_IO_ERROR", func() bool { return s.Stats().IOError != "" })
	st := s.Close(time.Second)
	if !strings.HasPrefix(st.IOError, "LOG_IO_ERROR: ") || st.Dropped != 5 || st.Written != 0 {
		t.Fatalf("stats %+v", st)
	}

	// A wedged writer cannot hold Close beyond its bound; pending and
	// in-flight bytes are counted as dropped, and the late completion of the
	// in-flight write changes no counter.
	w := openLogSink(dir, "stdout")
	gate := make(chan struct{})
	defer close(gate)
	released := make(chan struct{})
	w.write = func(f *os.File, b []byte) (int, error) { <-gate; defer close(released); return len(b), nil }
	_, _ = w.Write([]byte("first"))
	waitFor(t, "writer busy", func() bool { return w.pendingLen() == 0 })
	_, _ = w.Write([]byte("second"))
	start := time.Now()
	st = w.Close(50 * time.Millisecond)
	if time.Since(start) > time.Second || st.Dropped != 11 || st.Written != 0 {
		t.Fatalf("bounded close %+v after %v", st, time.Since(start))
	}
	if _, err := w.Write([]byte("late")); err != nil || w.Stats().Dropped != 15 {
		t.Fatal("a closed sink must drop and count")
	}
	gate <- struct{}{}
	<-released
	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not exit after its late completion")
	}
	if st := w.Stats(); st.Written != 0 || st.Dropped != 15 {
		t.Fatalf("late completion changed counters %+v", st)
	}
}

// A stalled status publication never blocks the supervision loop, and
// close waits for it at most its bound.
func TestSERVICE500_LogStatusPublicationNeverBlocks(t *testing.T) {
	u := openUnitLogs(t.TempDir(), "main", "stderr")
	u.bound = 50 * time.Millisecond
	gate := make(chan struct{})
	defer close(gate)
	entered := make(chan struct{}, 8)
	u.put = func(string, []byte) error { entered <- struct{}{}; <-gate; return nil }
	_, _ = u.streams["stderr"].Write([]byte("x"))
	waitFor(t, "byte written", func() bool { return u.streams["stderr"].Stats().Written == 1 })
	u.publish()
	<-entered
	start := time.Now()
	for i := 0; i < 5; i++ {
		_, _ = u.streams["stderr"].Write([]byte("y"))
		waitFor(t, "byte written", func() bool { return u.streams["stderr"].Stats().Written == uint64(2+i) })
		u.publish()
	}
	u.close()
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("a stalled publication blocked supervision for %v", d)
	}
}

func TestSERVICE500_LogStatusAndExcerptAreBounded(t *testing.T) {
	root := t.TempDir()
	u := openUnitLogs(root, "main", "stderr")
	_, _ = u.streams["stderr"].Write([]byte("boot\n"))
	u.close()
	h := Host{UID: os.Getuid()}
	stats, err := h.readLogStatus(root, "main")
	if err != nil || stats["stderr"].Written != 5 {
		t.Fatalf("published status %+v %v", stats, err)
	}
	raw := append(bytes.Repeat([]byte("\x1b[31mred\xff\x00\n"), 1200), []byte("tail\tend\n")...)
	path := filepath.Join(root, logDir, "main", "stderr.log")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := h.logExcerpt(root, "main", "stderr")
	if err != nil || len(text) > logExcerptMax || !strings.HasSuffix(text, "tail\tend\n") {
		t.Fatalf("excerpt %d bytes, %v", len(text), err)
	}
	for _, r := range text {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			t.Fatalf("control character %q survived", r)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/hosts", path); err != nil {
		t.Fatal(err)
	}
	if _, err := h.logExcerpt(root, "main", "stderr"); err == nil {
		t.Fatal("excerpt followed a symlink")
	}
}
