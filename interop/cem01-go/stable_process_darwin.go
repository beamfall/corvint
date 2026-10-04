//go:build darwin

package main

// Darwin owner/keeper process containment for canonical Stable transactions.
// The owner starts a private keeper (this executable) as a new process-group
// leader; the keeper starts exactly one Git child in that group and reports
// its status over a control socket. Retirement follows the OQ-12 two-step
// rule as amended by OQ-12b: poll the group after SIGKILL with the leader
// unreaped, reap, then poll signal 0 briefly until ESRCH proves absence.

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	stableStatusStarted       = 1
	stableStatusLaunchFailed  = 2
	stableStatusExited        = 3
	stableStatusSignaled      = 4
	stableStatusProtocolError = 5
	stableFrameMax            = 8404
)

func stableOwnerSupported() bool { return true }

// stableKeeperControlPresent reports whether fd 3 is a socket, as only an
// owner-started keeper receives one.
func stableKeeperControlPresent() bool {
	var st syscall.Stat_t
	if syscall.Fstat(3, &st) != nil {
		return false
	}
	return st.Mode&syscall.S_IFMT == syscall.S_IFSOCK
}

func stableEncodeFrame(op byte, oidLen int, fields []string) []byte {
	b := []byte{'C', 'E', 'M', 'K', 1, op, 1, 0, 0, 0}
	if oidLen == 64 {
		b[6] = 2
	}
	binary.BigEndian.PutUint16(b[8:], uint16(len(fields)))
	for _, f := range fields {
		b = binary.BigEndian.AppendUint16(b, uint16(len(f)))
		b = append(b, f...)
	}
	return b
}

func stableRawRead(fd int, n int) ([]byte, bool) {
	b := make([]byte, n)
	for got := 0; got < n; {
		k, err := syscall.Read(fd, b[got:])
		if err == syscall.EINTR {
			continue
		}
		if err != nil || k <= 0 {
			return nil, false
		}
		got += k
	}
	return b, true
}

func stableKeeperFrame() ([]string, byte, bool) {
	h, ok := stableRawRead(3, 10)
	if !ok || string(h[:4]) != "CEMK" || h[4] != 1 || h[7] != 0 || (h[6] != 1 && h[6] != 2) {
		return nil, 0, false
	}
	oidLen := 40
	if h[6] == 2 {
		oidLen = 64
	}
	count := int(binary.BigEndian.Uint16(h[8:]))
	total := 10
	fields := []string{}
	for i := 0; i < count; i++ {
		l, ok := stableRawRead(3, 2)
		if !ok {
			return nil, 0, false
		}
		n := int(binary.BigEndian.Uint16(l))
		total += 2 + n
		if total > stableFrameMax || n > 4096 || (i >= 2 && n != oidLen) {
			return nil, 0, false
		}
		f, ok := stableRawRead(3, n)
		if !ok {
			return nil, 0, false
		}
		fields = append(fields, string(f))
	}
	argv, ok := stableGitArgv(h[5], fields)
	if !ok {
		return nil, 0, false
	}
	return argv, h[5], true
}

func stableKeeperStatus(kind byte, value uint32) {
	b := []byte{'C', 'E', 'M', 'S', 1, kind, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(b[8:], value)
	for len(b) > 0 {
		n, err := syscall.Write(3, b)
		if err == syscall.EINTR {
			continue
		}
		if err != nil || n <= 0 {
			return
		}
		b = b[n:]
	}
}

// stableKeeperMain is the private keeper. It never returns.
func stableKeeperMain() {
	syscall.CloseOnExec(3)
	argv, _, ok := stableKeeperFrame()
	if !ok {
		stableKeeperStatus(stableStatusProtocolError, 0)
		os.Exit(2)
	} else if p, err := os.StartProcess(argv[0], argv, &os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}); err != nil {
		stableKeeperStatus(stableStatusLaunchFailed, 0)
	} else {
		stableKeeperStatus(stableStatusStarted, uint32(p.Pid))
		os.Stdin.Close()
		os.Stdout.Close()
		os.Stderr.Close()
		st, err := p.Wait()
		ws, _ := st.Sys().(syscall.WaitStatus)
		switch {
		case err == nil && ws.Exited():
			stableKeeperStatus(stableStatusExited, uint32(ws.ExitStatus()))
		case err == nil && ws.Signaled():
			stableKeeperStatus(stableStatusSignaled, uint32(ws.Signal()))
		}
	}
	buf := make([]byte, 64)
	for {
		n, err := syscall.Read(3, buf)
		if err == syscall.EINTR {
			continue
		}
		if err != nil || n <= 0 {
			break
		}
	}
	_ = syscall.Kill(0, syscall.SIGKILL)
	os.Exit(2)
}

type stableLimitReader struct {
	r    io.Reader
	n    int64
	over func()
}

var errStableOverflow = errors.New("overflow")

func (l *stableLimitReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		var one [1]byte
		k, err := l.r.Read(one[:])
		if k > 0 {
			l.over()
			return 0, errStableOverflow
		}
		return 0, err
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	k, err := l.r.Read(p)
	l.n -= int64(k)
	return k, err
}

// stableRetire applies the OQ-12b two-step rule to a group already sent
// SIGKILL. Step 1 waits, with the keeper unreaped, until signal 0 is no
// longer successful. Step 2 reaps the keeper and then polls signal 0 for a
// bounded 2 seconds, still inside the caller's retirement bound; ESRCH alone
// proves absence.
func stableRetire(proc *os.Process, pgid int, bound time.Time) bool {
	for syscall.Kill(-pgid, 0) == nil {
		if !time.Now().Before(bound) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _ = proc.Wait()
	postBound := time.Now().Add(stablePostReapPollBound)
	if bound.Before(postBound) {
		postBound = bound
	}
	for {
		if syscall.Kill(-pgid, 0) == syscall.ESRCH {
			return true
		}
		if !time.Now().Before(postBound) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *stableSession) retain(v ...any) {
	if !s.holdLine {
		s.holdLine = true
		_, _ = os.Stderr.WriteString(stableHoldLine)
	}
	s.hold = true
	s.held = append(s.held, v...)
}

func stableCloseFiles(files ...*os.File) {
	for _, f := range files {
		if f != nil {
			f.Close()
		}
	}
}

// stableRun executes one transaction under an owned process group.
func stableRun(s *stableSession, t *stableTx) *cemError {
	h := s.hooks
	setupFailed := func(step string) bool {
		return h != nil && h.setupFail != nil && h.setupFail(step)
	}
	openPipe := func(step string) (*os.File, *os.File, bool) {
		if setupFailed(step + ":before") {
			return nil, nil, false
		}
		r, w, err := os.Pipe()
		if err != nil {
			return nil, nil, false
		}
		if setupFailed(step + ":after") {
			stableCloseFiles(r, w)
			return nil, nil, false
		}
		return r, w, true
	}
	inR, inW, ok := openPipe("stdin-pipe")
	if !ok {
		return operational("unsupported-process-containment")
	}
	outR, outW, ok := openPipe("stdout-pipe")
	if !ok {
		stableCloseFiles(inR, inW)
		return operational("unsupported-process-containment")
	}
	errR, errW, ok := openPipe("stderr-pipe")
	if !ok {
		stableCloseFiles(inR, inW, outR, outW)
		return operational("unsupported-process-containment")
	}
	syscall.ForkLock.RLock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		syscall.CloseOnExec(fds[0])
		syscall.CloseOnExec(fds[1])
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		stableCloseFiles(inR, inW, outR, outW, errR, errW)
		return operational("unsupported-process-containment")
	}
	if setupFailed("socketpair:after") {
		_ = syscall.Close(fds[0])
		_ = syscall.Close(fds[1])
		stableCloseFiles(inR, inW, outR, outW, errR, errW)
		return operational("unsupported-process-containment")
	}
	_ = syscall.SetNonblock(fds[0], true)
	ctl := os.NewFile(uintptr(fds[0]), "cem01-ctl")
	ctlChild := os.NewFile(uintptr(fds[1]), "cem01-ctl-child")
	h.point(t, "before-spawn")
	proc, err := os.StartProcess(s.exe, []string{s.exe, stableKeeperProtocol}, &os.ProcAttr{
		Dir: s.admin, Env: s.env, Files: []*os.File{inR, outW, errW, ctlChild}, Sys: &syscall.SysProcAttr{Setpgid: true},
	})
	inR.Close()
	outW.Close()
	errW.Close()
	ctlChild.Close()
	if err != nil {
		inW.Close()
		outR.Close()
		errR.Close()
		ctl.Close()
		return s.arbitrate(t, &stableFlags{launch: true})
	}
	pgid := proc.Pid
	if h != nil && h.ownerFail != nil && h.ownerFail(t, "establish") {
		ctl.Close()
		s.retain(proc, inW, outR, errR)
		return operational("unsupported-process-containment")
	}

	var mu sync.Mutex
	flags := &stableFlags{}
	set := func(f func()) { mu.Lock(); f(); mu.Unlock() }
	killCh := make(chan struct{})
	var killOnce sync.Once
	kill := func() {
		killOnce.Do(func() {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			close(killCh)
		})
	}
	timer := time.AfterFunc(s.opTimeout(t), func() { set(func() { flags.deadline = true }); kill() })

	statusCh := make(chan []byte, 4)
	go func() {
		defer close(statusCh)
		for {
			b := make([]byte, 12)
			if _, err := io.ReadFull(ctl, b); err != nil {
				return
			}
			statusCh <- b
		}
	}()
	_, _ = ctl.Write(stableEncodeFrame(t.op, s.oidLen, t.fields))

	outDone := make(chan struct{})
	go func() {
		defer close(outDone)
		lr := &stableLimitReader{r: outR, n: t.limit, over: func() { set(func() { flags.overflow = true }); kill() }}
		if t.consume != nil {
			br := bufio.NewReader(lr)
			err := t.consume(inW, br)
			switch {
			case errors.Is(err, errStableOverflow):
			case err == errStableParse:
				set(func() { flags.parser = true })
				kill()
			case err != nil:
				set(func() { flags.truncated = true })
			default:
				if _, err := br.ReadByte(); err == nil {
					set(func() { flags.parser = true })
					kill()
				} else if errors.Is(err, errStableOverflow) {
					// overflow already flagged
				}
			}
		} else {
			inW.Close()
			b, err := io.ReadAll(lr)
			if err == nil {
				t.out = b
			}
		}
		_, _ = io.Copy(io.Discard, outR)
	}()
	errDone := make(chan struct{})
	go func() {
		defer close(errDone)
		lr := &stableLimitReader{r: errR, n: stableStderrLimit, over: func() { set(func() { flags.overflow = true }); kill() }}
		_, _ = io.Copy(io.Discard, lr)
		_, _ = io.Copy(io.Discard, errR)
	}()

	var bound <-chan time.Time
	kc := killCh
	ctxDone := s.ctx.Done()
	sc := statusCh
	terminal, oDone, eDone, hold := false, false, false, false
	for !(terminal && oDone && eDone) && !hold {
		select {
		case b, ok := <-sc:
			if !ok {
				sc = nil
				if !terminal {
					terminal = true
					set(func() { flags.nonzero = true })
				}
				kill()
				continue
			}
			if string(b[:4]) != "CEMS" || b[4] != 1 {
				set(func() { flags.launch = true })
				terminal = true
				kill()
				continue
			}
			v := binary.BigEndian.Uint32(b[8:])
			switch b[5] {
			case stableStatusStarted:
				h.point(t, "spawned")
			case stableStatusExited:
				terminal = true
				if v != 0 {
					set(func() { flags.nonzero = true })
				}
				if !t.held {
					kill()
				}
			case stableStatusSignaled:
				terminal = true
				set(func() { flags.nonzero = true })
				kill()
			default:
				terminal = true
				set(func() { flags.launch = true })
				kill()
			}
		case <-outDone:
			oDone = true
			outDone = nil
		case <-errDone:
			eDone = true
			errDone = nil
		case <-ctxDone:
			ctxDone = nil
			s.anchor()
			kill()
		case <-kc:
			kc = nil
			bound = time.After(time.Until(s.bound()))
		case <-bound:
			hold = true
		}
	}
	if hold {
		timer.Stop()
		s.retain(proc, ctl, inW, outR, errR)
		return operational("unsupported-process-containment")
	}
	h.point(t, "before-commit")
	mu.Lock()
	f := *flags
	mu.Unlock()
	e := s.arbitrate(t, &f)
	timer.Stop()
	h.point(t, "committed")
	if e != nil {
		h.emit("committed-return-to-stage")
	}
	if t.held && e == nil && f == (stableFlags{}) {
		s.held = append(s.held, stableHeld{proc, pgid, []*os.File{ctl, inW, outR, errR}})
		return nil
	}
	kill()
	if h != nil && h.ownerFail != nil && h.ownerFail(t, "retire") {
		s.retain(proc, ctl, inW, outR, errR)
		return operational("unsupported-process-containment")
	}
	if !stableRetire(proc, pgid, s.bound()) {
		s.retain(proc, ctl, inW, outR, errR)
		return operational("unsupported-process-containment")
	}
	ctl.Close()
	inW.Close()
	outR.Close()
	errR.Close()
	return e
}

// stableClose retires keepers parked by the hold seam at final owner close.
func stableClose(s *stableSession) {
	keep := []any{}
	for _, v := range s.held {
		k, ok := v.(stableHeld)
		if !ok {
			keep = append(keep, v)
			continue
		}
		_ = syscall.Kill(-k.pgid, syscall.SIGKILL)
		if s.hooks != nil && s.hooks.closeFail != nil && s.hooks.closeFail() || !stableRetire(k.proc, k.pgid, s.bound()) {
			keep = append(keep, k)
			s.held = keep
			s.retain()
			continue
		}
		for _, f := range k.files {
			f.Close()
		}
	}
	s.held = keep
}

// --- admission ----------------------------------------------------------------

const (
	stableKindDir = iota
	stableKindFile
	stableKindEither
)

func stableIdentOf(st *syscall.Stat_t) stableIdent {
	return stableIdent{dev: uint64(st.Dev), ino: st.Ino, mode: uint32(st.Mode)}
}

// stableProbe opens one metadata path without following its final component,
// checks its kind, reads a bounded file body and confirms the path still names
// the opened object.
func stableProbe(h *stableHooks, path string, kind int, limit int) ([]byte, stableIdent, bool, bool, *cemError) {
	fail := operational("repository-object-unavailable")
	var err error
	if h != nil && h.fs != nil {
		err = h.fs("open", path)
	}
	fd := -1
	if err == nil {
		for {
			fd, err = syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
			if err != syscall.EINTR {
				break
			}
		}
	}
	if err == syscall.ENOENT {
		return nil, stableIdent{}, false, true, nil
	}
	if err != nil {
		return nil, stableIdent{}, false, false, fail
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	if h != nil && h.fs != nil {
		if h.fs("opened", path) != nil {
			return nil, stableIdent{}, false, false, fail
		}
	}
	var st syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil {
		return nil, stableIdent{}, false, false, fail
	}
	t := st.Mode & syscall.S_IFMT
	isDir := t == syscall.S_IFDIR
	switch {
	case kind == stableKindDir && !isDir, kind == stableKindFile && t != syscall.S_IFREG, kind == stableKindEither && !isDir && t != syscall.S_IFREG:
		return nil, stableIdent{}, false, false, fail
	}
	var data []byte
	if !isDir {
		data, err = io.ReadAll(io.LimitReader(f, int64(limit)+1))
		if err != nil || len(data) > limit {
			return nil, stableIdent{}, false, false, fail
		}
	}
	var ls syscall.Stat_t
	if syscall.Lstat(path, &ls) != nil || ls.Dev != st.Dev || ls.Ino != st.Ino || ls.Mode != st.Mode {
		return nil, stableIdent{}, false, false, fail
	}
	return data, stableIdentOf(&st), isDir, false, nil
}

func stableProbeAncestry(h *stableHooks, path string) (stableIdent, *cemError) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	cur := "/"
	_, id, _, absent, e := stableProbe(h, cur, stableKindDir, 0)
	for _, p := range parts {
		if e != nil || absent {
			break
		}
		if p == "" {
			continue
		}
		cur = filepath.Join(cur, p)
		_, id, _, absent, e = stableProbe(h, cur, stableKindDir, 0)
	}
	if e == nil && absent {
		e = operational("repository-object-unavailable")
	}
	return id, e
}

// stableMetadataLine applies the gitfile/commondir grammar: valid UTF-8, no NUL
// or CR, one nonempty line and at most one final LF.
func stableMetadataLine(b []byte) (string, bool) {
	s := string(b)
	if !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\r") {
		return "", false
	}
	s = strings.TrimSuffix(s, "\n")
	if s == "" || strings.Contains(s, "\n") {
		return "", false
	}
	return s, true
}

func stableAttributesInert(b []byte) bool {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimFunc(line, func(r rune) bool { return unicode.Is(unicode.White_Space, r) })
		if line != "" && !strings.HasPrefix(line, "#") {
			return false
		}
	}
	return true
}

func stableAdmit(repo string, h *stableHooks) (*stableAdmission, *cemError) {
	fail := operational("repository-object-unavailable")
	if !filepath.IsAbs(repo) || filepath.Clean(repo) != repo {
		return nil, fail
	}
	a := &stableAdmission{root: repo, ids: map[string]stableIdent{}}
	id, e := stableProbeAncestry(h, repo)
	if e != nil {
		return nil, e
	}
	a.ids[repo] = id
	marker := filepath.Join(repo, ".git")
	data, markerID, isDir, absent, e := stableProbe(h, marker, stableKindEither, 4096)
	if e != nil {
		return nil, e
	}
	if absent {
		return nil, fail
	}
	if isDir {
		a.admin, a.common = marker, marker
		if _, e := stableProbeAncestry(h, a.admin); e != nil {
			return nil, e
		}
	} else {
		line, ok := stableMetadataLine(data)
		if !ok || !strings.HasPrefix(line, "gitdir: ") || len(line) == len("gitdir: ") {
			return nil, fail
		}
		x := strings.TrimPrefix(line, "gitdir: ")
		if !filepath.IsAbs(x) {
			x = filepath.Join(repo, x)
		}
		a.admin = filepath.Clean(x)
		if _, e := stableProbeAncestry(h, a.admin); e != nil {
			return nil, e
		}
		bp := filepath.Join(a.admin, "gitdir")
		data, _, _, absent, e := stableProbe(h, bp, stableKindFile, 4096)
		if e != nil {
			return nil, e
		}
		if absent {
			return nil, fail
		}
		back, ok := stableMetadataLine(data)
		if !ok {
			return nil, fail
		}
		if !filepath.IsAbs(back) {
			back = filepath.Join(a.admin, back)
		}
		back = filepath.Clean(back)
		_, backID, _, absent, e := stableProbe(h, back, stableKindFile, 4096)
		if e != nil || absent || backID != markerID {
			return nil, fail
		}
		_, parentID, _, absent, e := stableProbe(h, filepath.Dir(back), stableKindDir, 0)
		if e != nil || absent || parentID != a.ids[repo] {
			return nil, fail
		}
		a.common = a.admin
		data, _, _, absent, e = stableProbe(h, filepath.Join(a.admin, "commondir"), stableKindFile, 4096)
		if e != nil {
			return nil, e
		}
		if !absent {
			c, ok := stableMetadataLine(data)
			if !ok || filepath.IsAbs(c) {
				return nil, fail
			}
			a.common = filepath.Clean(filepath.Join(a.admin, c))
		}
		if _, e := stableProbeAncestry(h, a.common); e != nil {
			return nil, e
		}
	}
	a.ids[a.admin], _ = stableProbeAncestry(h, a.admin)
	a.ids[a.common], _ = stableProbeAncestry(h, a.common)
	objects := filepath.Join(a.common, "objects")
	if _, _, _, absent, e := stableProbe(h, objects, stableKindDir, 0); e != nil || absent {
		return nil, fail
	}
	info := filepath.Join(objects, "info")
	_, _, _, infoAbsent, e := stableProbe(h, info, stableKindDir, 0)
	if e != nil {
		return nil, e
	}
	if _, _, _, _, e := stableProbe(h, filepath.Join(objects, "pack"), stableKindDir, 0); e != nil {
		return nil, e
	}
	if os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES") != "" {
		return nil, operational("unsupported-object-alternates")
	}
	if !infoAbsent {
		for _, n := range []string{"alternates", "http-alternates"} {
			var st syscall.Stat_t
			err := syscall.Lstat(filepath.Join(info, n), &st)
			if err == nil {
				return nil, operational("unsupported-object-alternates")
			}
			if err != syscall.ENOENT {
				return nil, fail
			}
		}
	}
	dirs := []string{a.common}
	if a.admin != a.common {
		dirs = append(dirs, a.admin)
	}
	for _, d := range dirs {
		infoDir := filepath.Join(d, "info")
		_, _, _, absent, e := stableProbe(h, infoDir, stableKindDir, 0)
		if e != nil {
			return nil, e
		}
		if absent {
			continue
		}
		data, _, _, absent, e := stableProbe(h, filepath.Join(infoDir, "attributes"), stableKindFile, 65536)
		if e != nil {
			return nil, e
		}
		if !absent && !stableAttributesInert(data) {
			return nil, operational("unsupported-repository-envelope")
		}
	}
	return a, nil
}

// stableRevalidate confirms root, admin and common still name the admitted
// directories after all content and artifact work.
func stableRevalidate(a *stableAdmission) *cemError {
	for _, p := range []string{a.root, a.admin, a.common} {
		_, id, _, absent, e := stableProbe(nil, p, stableKindDir, 0)
		if e != nil || absent || id != a.ids[p] {
			return operational("repository-object-unavailable")
		}
	}
	return nil
}
