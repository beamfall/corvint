package service

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Service runtime logging (SERVICE500-007). Each managed unit (the main and
// every helper) owns logs/<unit>/ under the private state root. Every stream
// rotates between <stream>.log and <stream>.log.1, each at most logFileMax
// bytes, so a stream retains at most 2*logFileMax bytes. Writers append to a
// bounded logBufferMax pending buffer and never block: bytes that do not fit
// are dropped and counted. One writer goroutine per stream performs the file
// I/O, so a slow or broken filesystem cannot block supervision; an I/O error
// drops that chunk, counts it, and is reported as LOG_IO_ERROR.
const (
	logFileMax    = 8 << 20
	logBufferMax  = 64 << 10
	logExcerptMax = 4 << 10
	logStatusFile = "status.json"
	logStatusMax  = 4096
	logDir        = "logs"
	// logCloseBound bounds the final flush on Close.
	logCloseBound = 2 * time.Second
)

// LogStats is one stream's counters. IOError is the latest LOG_IO_ERROR
// diagnostic; it stays set once observed.
type LogStats struct {
	Written uint64 `json:"written"`
	Dropped uint64 `json:"dropped"`
	IOError string `json:"ioError,omitempty"`
}

// logSink is one stream's bounded, non-blocking sink.
type logSink struct {
	dir, stream string

	mu      sync.Mutex
	wake    chan struct{}
	pending []byte
	stats   LogStats
	closed  bool
	done    chan struct{}
	// inflight is the chunk the writer has taken but not yet settled;
	// settled marks a Close that reached its bound and counted every
	// unsettled byte as dropped, so a late completion changes no counter.
	inflight int
	settled  bool

	// file I/O, owned by the writer goroutine
	f    *os.File
	size int64
	// write is the file write; tests replace it to inject I/O failure.
	write func(f *os.File, b []byte) (int, error)
}

// openLogSink starts a sink for dir/<stream>.log. It never fails: a
// directory or open error is reported as LOG_IO_ERROR and every byte is
// then dropped and counted.
func openLogSink(dir, stream string) *logSink {
	s := &logSink{dir: dir, stream: stream, wake: make(chan struct{}, 1), done: make(chan struct{}), write: func(f *os.File, b []byte) (int, error) { return f.Write(b) }}
	go s.run()
	return s
}

func (s *logSink) path() string { return filepath.Join(s.dir, s.stream+".log") }

// Write never blocks on I/O and always reports len(p): bytes beyond the
// pending bound are dropped and counted.
func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	room := logBufferMax - len(s.pending)
	if s.closed {
		room = 0
	}
	n := len(p)
	if n > room {
		s.stats.Dropped += uint64(n - room)
		n = room
	}
	s.pending = append(s.pending, p[:n]...)
	s.mu.Unlock()
	if n > 0 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return len(p), nil
}

// Stats returns the current counters.
func (s *logSink) Stats() LogStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Close stops accepting bytes and waits at most bound for the final flush;
// bytes still pending at the bound are counted as dropped.
func (s *logSink) Close(bound time.Duration) LogStats {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	select {
	case <-s.done:
	case <-time.After(bound):
		s.mu.Lock()
		if !s.settled {
			s.settled = true
			s.stats.Dropped += uint64(len(s.pending) + s.inflight)
			s.pending = nil
		}
		s.mu.Unlock()
	}
	return s.Stats()
}

func (s *logSink) run() {
	defer close(s.done)
	defer func() {
		if s.f != nil {
			s.f.Close()
		}
	}()
	for {
		s.mu.Lock()
		chunk := s.pending
		s.pending = nil
		s.inflight = len(chunk)
		closed := s.closed
		s.mu.Unlock()
		if len(chunk) > 0 {
			err := s.append(chunk)
			s.mu.Lock()
			s.inflight = 0
			switch {
			case s.settled:
				// Close already counted this chunk as dropped.
			case err != nil:
				s.stats.Dropped += uint64(len(chunk))
				s.stats.IOError = "LOG_IO_ERROR: " + truncate(describe(err), 512)
			default:
				s.stats.Written += uint64(len(chunk))
			}
			s.mu.Unlock()
			continue
		}
		if closed {
			return
		}
		<-s.wake
	}
}

// append writes one chunk (at most logBufferMax bytes), rotating first when
// the chunk would take the current file past logFileMax.
func (s *logSink) append(chunk []byte) error {
	if s.f == nil {
		if err := s.open(); err != nil {
			return err
		}
	}
	if s.size+int64(len(chunk)) > logFileMax {
		s.f.Close()
		s.f = nil
		if err := os.Rename(s.path(), s.path()+".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := s.open(); err != nil {
			return err
		}
	}
	n, err := s.write(s.f, chunk)
	s.size += int64(n)
	if err != nil {
		// The file position is unknown after a failed write; reopen next time.
		s.f.Close()
		s.f = nil
	}
	return err
}

func (s *logSink) open() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path(), os.O_WRONLY|os.O_APPEND|os.O_CREATE|noFollow, 0o600)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o077 != 0 {
		f.Close()
		if err == nil {
			err = errors.New("log file is not a private regular file")
		}
		return err
	}
	s.f, s.size = f, fi.Size()
	return nil
}

// unitLogs is one unit's set of stream sinks plus its counter publisher.
type unitLogs struct {
	dir     string
	streams map[string]*logSink
	names   []string

	// One publisher goroutine writes status.json, so a stalled filesystem
	// never blocks the supervision loop that calls publish. want is the
	// latest requested counters, last the latest written.
	mu         sync.Mutex
	want, last string
	kick, stop chan struct{}
	done       chan struct{}
	// put writes status.json; tests replace it to stall publication.
	put   func(dir string, raw []byte) error
	bound time.Duration
}

func openUnitLogs(root, unit string, streams ...string) *unitLogs {
	u := &unitLogs{dir: filepath.Join(root, logDir, unit), streams: map[string]*logSink{}, names: streams, kick: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), bound: logCloseBound, put: func(dir string, raw []byte) error {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return writeAtomic(dir, logStatusFile, raw)
	}}
	for _, s := range streams {
		u.streams[s] = openLogSink(u.dir, s)
	}
	go u.publisher()
	return u
}

func (u *unitLogs) stats() map[string]LogStats {
	out := map[string]LogStats{}
	for _, n := range u.names {
		out[n] = u.streams[n].Stats()
	}
	return out
}

// publish requests that the counters be written to logs/<unit>/status.json
// when they changed. It never blocks: the publisher goroutine writes the
// latest request. Publication is best effort; a failure leaves the older
// status, retried on the next request.
func (u *unitLogs) publish() {
	raw, err := json.Marshal(u.stats())
	if err != nil || len(raw) > logStatusMax {
		return
	}
	u.mu.Lock()
	changed := string(raw) != u.want
	u.want = string(raw)
	u.mu.Unlock()
	if changed {
		select {
		case u.kick <- struct{}{}:
		default:
		}
	}
}

func (u *unitLogs) publisher() {
	defer close(u.done)
	for {
		select {
		case <-u.kick:
			u.write()
		case <-u.stop:
			u.write()
			return
		}
	}
}

func (u *unitLogs) write() {
	u.mu.Lock()
	raw := u.want
	stale := raw != u.last
	u.mu.Unlock()
	if !stale {
		return
	}
	err := u.put(u.dir, []byte(raw))
	u.mu.Lock()
	if err == nil {
		u.last = raw
	} else if u.want == raw {
		// Forget the request so the next publish retries it.
		u.want = ""
	}
	u.mu.Unlock()
}

// close closes every stream concurrently, each within the close bound,
// then requests the final counters and waits at most the close bound for
// the publisher.
func (u *unitLogs) close() {
	var wg sync.WaitGroup
	for _, n := range u.names {
		wg.Add(1)
		go func(s *logSink) { defer wg.Done(); s.Close(u.bound) }(u.streams[n])
	}
	wg.Wait()
	u.publish()
	close(u.stop)
	select {
	case <-u.done:
	case <-time.After(u.bound):
	}
}

// readLogStatus reads one unit's published counters for Status.
func (h Host) readLogStatus(root, unit string) (map[string]LogStats, error) {
	raw, err := h.readPrivate(filepath.Join(root, logDir, unit, logStatusFile), logStatusMax)
	if err != nil {
		return nil, err
	}
	out := map[string]LogStats{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// logExcerpt is the sanitized tail of one stream, at most logExcerptMax
// bytes: the final symlink is not followed, the file must be the user's
// private regular file, control characters other than newline and tab are
// replaced and invalid UTF-8 is replaced. It is user-private diagnostic
// text and is not secret-screened.
func (h Host) logExcerpt(root, unit, stream string) (string, error) {
	path := filepath.Join(root, logDir, unit, stream+".log")
	f, err := os.OpenFile(path, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if uid, ok := fileOwner(fi); !fi.Mode().IsRegular() || !ok || int(uid) != h.UID {
		return "", errors.New("log file is not the user's regular file")
	}
	off := fi.Size() - logExcerptMax
	if off < 0 {
		off = 0
	}
	raw, err := io.ReadAll(io.LimitReader(io.NewSectionReader(f, off, logExcerptMax), logExcerptMax))
	if err != nil {
		return "", err
	}
	return sanitizeExcerpt(raw), nil
}

// sanitizeExcerpt keeps the result within logExcerptMax bytes after
// replacement by trimming from the front.
func sanitizeExcerpt(raw []byte) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(string(raw), "�") {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			b.WriteString("\\x" + strconv.FormatInt(int64(r), 16))
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	for len(out) > logExcerptMax {
		_, size := utf8.DecodeRuneInString(out)
		out = out[size:]
	}
	return out
}
