package affected

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestSourceReadBoundsAndStickyFailure(t *testing.T) {
	t.Run("DLT-V0-002 sticky read failure", testSourceReadBoundsAndStickyFailure)
}

func TestSourceReadBoundedCapabilityAndStickyErrors_DLT_V0_002(t *testing.T) {
	t.Run("DLT-V0-002 smaller capture before allocating open", func(t *testing.T) {
		const limit = 1 << 20
		readFailure := errors.New("injected read failure")
		closeFailure := errors.New("injected close failure")
		for _, tc := range []struct {
			name     string
			size     int64
			length   int
			mode     fs.FileMode
			openErr  error
			statErr  error
			readErr  error
			closeErr error
			wantErr  error
			wantRead int
		}{
			{name: "exact limit", size: limit, length: limit, wantRead: limit},
			{name: "reported oversize", size: limit + 1, length: limit + 1, wantErr: ErrWalkLimit},
			{name: "growth after stat", size: 1, length: limit + 20, wantErr: ErrWalkLimit, wantRead: limit + 1},
			{name: "nonregular", mode: fs.ModeSymlink, wantErr: ErrInvalidUnit},
			{name: "open error", openErr: fs.ErrPermission, wantErr: fs.ErrPermission},
			{name: "optional missing", openErr: fs.ErrNotExist, wantErr: fs.ErrNotExist},
			{name: "stat error", statErr: fs.ErrPermission, wantErr: fs.ErrPermission},
			{name: "read error", readErr: readFailure, wantErr: readFailure},
			{name: "close error", closeErr: closeFailure, wantErr: closeFailure},
			{name: "read and close errors", readErr: readFailure, closeErr: closeFailure, wantErr: readFailure},
		} {
			t.Run(tc.name, func(t *testing.T) {
				file := &boundedCaptureFile{Reader: strings.NewReader(strings.Repeat("x", tc.length)), size: tc.size, mode: tc.mode, statErr: tc.statErr, readErr: tc.readErr, closeErr: tc.closeErr}
				backend := &boundedCaptureFS{file: file, err: tc.openErr}
				source := FSSource(backend)
				body, err := source.ReadBounded(ReadScopesPath, limit)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil && body != nil || tc.wantErr == nil && len(body) != limit {
					t.Fatalf("returned %d bytes with %v", len(body), err)
				}
				if backend.limit != limit || backend.opens != 0 || backend.stats != 0 || file.readBytes != tc.wantRead {
					t.Fatalf("admission: limit=%d generic opens=%d stats=%d body reads=%d", backend.limit, backend.opens, backend.stats, file.readBytes)
				}
				if file.closed != (tc.openErr == nil) {
					t.Fatalf("closed=%v, open error=%v", file.closed, tc.openErr)
				}
				wantSticky := tc.wantErr
				if errors.Is(wantSticky, fs.ErrNotExist) {
					wantSticky = nil
				}
				if !errors.Is(source.Err(), wantSticky) {
					t.Fatalf("sticky error = %v, want %v", source.Err(), wantSticky)
				}
			})
		}
	})
	t.Run("DLT-V0-002 opaque filesystem cannot claim optional absence", func(t *testing.T) {
		source := FSSource(fstest.MapFS{})
		if _, err := source.ReadBounded(ReadScopesPath, 1<<20); !errors.Is(err, ErrWalkUnrepresentable) || !errors.Is(source.Err(), ErrWalkUnrepresentable) {
			t.Fatalf("opaque filesystem admitted: %v, retained %v", err, source.Err())
		}
	})
}

func TestSourceReadBoundedOversizeRetainsReadError(t *testing.T) {
	t.Run("DLT-V0-002 oversized capture preserves simultaneous read failure", func(t *testing.T) {
		readFailure := errors.New("injected failure with bytes")
		closeFailure := errors.New("injected close failure")
		for _, tc := range []struct {
			name              string
			readErr, closeErr error
			limit             int
		}{
			{"read failure", readFailure, nil, 1},
			{"not-exist is not optional after oversized bytes", fs.ErrNotExist, nil, 1},
			{"read and close failures", readFailure, closeFailure, 1},
			{"not-exist and close failure with oversize", fs.ErrNotExist, closeFailure, 1},
			{"not-exist and close failure without oversize", fs.ErrNotExist, closeFailure, 2},
		} {
			t.Run(tc.name, func(t *testing.T) {
				file := &boundedCaptureFile{Reader: strings.NewReader("xx"), size: 1, readErr: tc.readErr, closeErr: tc.closeErr, readErrWithData: true}
				backend := &boundedCaptureFS{file: file}
				source := FSSource(backend)
				body, err := source.ReadBounded(ReadScopesPath, tc.limit)
				if body != nil || errors.Is(err, ErrWalkLimit) != (tc.limit == 1) || !errors.Is(err, tc.readErr) {
					t.Fatalf("oversize/read failure lost: body=%q err=%v", body, err)
				}
				if errors.Is(source.Err(), ErrWalkLimit) != (tc.limit == 1) || !errors.Is(source.Err(), tc.readErr) {
					t.Fatalf("sticky oversize/read failure lost: %v", source.Err())
				}
				if tc.closeErr != nil && (!errors.Is(err, tc.closeErr) || !errors.Is(source.Err(), tc.closeErr)) {
					t.Fatalf("close failure lost: returned=%v sticky=%v", err, source.Err())
				}
				if backend.limit != tc.limit || backend.opens != 0 || backend.stats != 0 || file.readBytes != 2 || !file.closed {
					t.Fatalf("capture escaped bound: limit=%d opens=%d stats=%d bytes=%d closed=%v", backend.limit, backend.opens, backend.stats, file.readBytes, file.closed)
				}
			})
		}
	})
}

type boundedCaptureFS struct {
	file                fs.File
	err                 error
	limit, opens, stats int
}

func (s *boundedCaptureFS) Open(string) (fs.File, error) {
	s.opens++
	return nil, errors.New("generic allocating open must not run")
}
func (s *boundedCaptureFS) Stat(string) (fs.FileInfo, error) {
	s.stats++
	return nil, errors.New("generic allocating stat must not run")
}
func (s *boundedCaptureFS) OpenBounded(_ string, limit int) (fs.File, error) {
	s.limit = limit
	return s.file, s.err
}

type boundedCaptureFile struct {
	*strings.Reader
	size                       int64
	mode                       fs.FileMode
	statErr, readErr, closeErr error
	readBytes                  int
	closed                     bool
	readErrWithData            bool
}

func (f *boundedCaptureFile) Stat() (fs.FileInfo, error) { return f, f.statErr }
func (f *boundedCaptureFile) Read(p []byte) (int, error) {
	if f.readErr != nil && !f.readErrWithData {
		return 0, f.readErr
	}
	n, err := f.Reader.Read(p)
	f.readBytes += n
	if n > 0 && f.readErrWithData {
		return n, f.readErr
	}
	return n, err
}
func (f *boundedCaptureFile) Close() error       { f.closed = true; return f.closeErr }
func (f *boundedCaptureFile) Name() string       { return ReadScopesPath }
func (f *boundedCaptureFile) Size() int64        { return f.size }
func (f *boundedCaptureFile) Mode() fs.FileMode  { return f.mode }
func (f *boundedCaptureFile) ModTime() time.Time { return time.Time{} }
func (f *boundedCaptureFile) IsDir() bool        { return f.mode.IsDir() }
func (f *boundedCaptureFile) Sys() any           { return nil }

func testSourceReadBoundsAndStickyFailure(t *testing.T) {
	source := FSSource(fstest.MapFS{"ok": {Data: []byte(strings.Repeat("x", MaxSourceBytes))}, "big": {Data: []byte(strings.Repeat("x", MaxSourceBytes+1))}})
	if body, err := source.Read("ok"); err != nil || len(body) != MaxSourceBytes {
		t.Fatalf("boundary: %d %v", len(body), err)
	}
	if _, err := source.Read("missing"); !errors.Is(err, fs.ErrNotExist) || source.Err() != nil {
		t.Fatalf("optional missing: %v %v", err, source.Err())
	}
	if _, err := source.Read("big"); !errors.Is(err, ErrWalkLimit) || !errors.Is(source.Err(), ErrWalkLimit) {
		t.Fatalf("bound: %v %v", err, source.Err())
	}
	source.Read("missing")
	if !errors.Is(source.Err(), ErrWalkLimit) {
		t.Fatal("lost first failure")
	}
}

func TestSourceHiddenWalkRetainsIgnoredFailure(t *testing.T) {
	t.Run("DLT-V0-002 hidden walk failure retained", testSourceHiddenWalkRetainsIgnoredFailure)
}
func testSourceHiddenWalkRetainsIgnoredFailure(t *testing.T) {
	source := FSSource(fstest.MapFS{".maestro/escape": {Mode: fs.ModeSymlink}})
	_ = source.Walk(".maestro", func(string, fs.DirEntry, error) error { return nil })
	if !errors.Is(source.Err(), ErrWalkUnrepresentable) {
		t.Fatalf("hidden mode: %v", source.Err())
	}
}
