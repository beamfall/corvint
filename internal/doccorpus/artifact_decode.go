package doccorpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// artifactMember is one JSON member of Artifact: its name, struct field index and whether
// its elements decode and verify in parallel. Only slice members the encoder always emits
// (no omitempty) are split, so the skeleton comparison can stand in for them with `[]`.
type artifactMember struct {
	field int
	split bool
}

var artifactMembers = func() map[string]artifactMember {
	members := map[string]artifactMember{}
	t := reflect.TypeFor[Artifact]()
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if !f.IsExported() || tag == "-" || tag == "" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		members[name] = artifactMember{field: i, split: f.Type.Kind() == reflect.Slice && !strings.Contains(options, "omitempty")}
	}
	return members
}()

// byteSpan is one value's byte range in the artifact.
type byteSpan struct{ start, end int64 }

// splitArray is one split member: its field, the offsets of its brackets and its elements.
type splitArray struct {
	field       int
	open, close int64 // offsets of '[' and of the byte after ']'
	elements    []byteSpan
	chunks      []reflect.Value // decoded elements, in order, one slice per dispatched chunk
}

// splitChunk is the number of split elements one decode job takes at most; a job also ends at
// the end of a window.
const splitChunk = 512

// artifactWindow is the number of artifact bytes a streamed read holds beyond the value being
// scanned (DCR-V1-431).
const artifactWindow = 1 << 20

// decodeArtifactParallel is decodeArtifactStream over bytes already in memory; their digest is
// hashArtifactTail's.
func decodeArtifactParallel(data []byte, header func(*Artifact)) (*Artifact, error) {
	return decodeArtifactBytes(data, header, nil)
}

// decodeArtifactBytes is decodeArtifactParallel with the claims hook of decodeArtifactStream.
func decodeArtifactBytes(data []byte, header func(*Artifact), claims func(lo int, chunk []Claim)) (*Artifact, error) {
	if len(data) > MaxCorpusBytes {
		return nil, fail("input bound exceeded")
	}
	a, _, err := decodeArtifactStream(&artifactStream{buf: data, eof: true}, header, false, claims)
	return a, err
}

// decodeArtifactReader is decodeArtifactStream over r, read ahead in blocks of artifactWindow
// bytes on its own goroutine and hashed as it is read, so the artifact is never held whole
// (DCR-V1-431). A read error is returned as it is, before any refusal of the bytes read so far.
func decodeArtifactReader(r io.Reader, header func(*Artifact), claims func(lo int, chunk []Claim)) (*Artifact, artifactTail, error) {
	return decodeArtifactBlocks(r, artifactWindow, blockHeadroom, header, claims, nil)
}

// blockStats counts the blocks a streamed read read and the buffers it allocated for them.
type blockStats struct{ blocks, allocated int }

// decodeArtifactBlocks is decodeArtifactReader with blocks of window bytes after headroom bytes
// of room for a carry; tests use small blocks to cross every window boundary.
func decodeArtifactBlocks(r io.Reader, window, headroom int, header func(*Artifact), claims func(lo int, chunk []Claim), stats *blockStats) (*Artifact, artifactTail, error) {
	return decodeArtifactBlocksScan(r, window, headroom, boundaryScan{}, header, claims, stats)
}

// decodeArtifactBlocksScan is decodeArtifactBlocks with the boundary scan's segmentation; tests
// use tiny segments so a split falls at every kind of byte, and one segment for the sequential scan.
func decodeArtifactBlocksScan(r io.Reader, window, headroom int, scan boundaryScan, header func(*Artifact), claims func(lo int, chunk []Claim), stats *blockStats) (*Artifact, artifactTail, error) {
	if stats == nil {
		stats = &blockStats{}
	}
	blocks := make(chan streamBlock, 4)
	done := make(chan struct{})
	pool := make(chan []byte, 8)
	// The blocks read ahead are pre-scanned for split elements unless the boundary scan is
	// sequential (DCR-V1-559); the scanners end after the reader, once every block is scanned.
	var scanner *blockScanner
	if _, segments := scan.plan(); segments > 1 {
		scanner = newBlockScanner(headroom)
		defer scanner.close()
	}
	var reader sync.WaitGroup
	reader.Go(func() { readBlocks(r, blocks, done, pool, window, headroom, scanner, stats) })
	defer func() {
		close(done)
		reader.Wait()
	}()
	s := &artifactStream{blocks: blocks, headroom: headroom, boundary: scan, scanner: scanner}
	a, tail, err := decodeArtifactStream(s, header, true, claims)
	if err != nil && !s.eof {
		// A refusal of the bytes read so far does not hide a later read error: the rest of the
		// input is read, bounded as before, before the refusal is reported (DCR-V1-431).
		var total int64 = s.base + int64(len(s.buf))
		for b := range blocks {
			total += int64(b.n)
			b.ref.release()
			b.ref.release()
			if b.err != nil && b.err != io.EOF && s.readErr == nil {
				s.readErr = b.err
			}
			if total > MaxCorpusBytes {
				s.overBound = true
				break
			}
		}
	}
	switch {
	case s.readErr != nil:
		return nil, artifactTail{}, s.readErr
	case s.overBound:
		return nil, artifactTail{}, fail("input bound exceeded")
	}
	return a, tail, err
}

// streamBlock is one block read ahead: data starts blockHeadroom bytes into its buffer, so a
// short carry from the previous window can be placed right before it without copying the block.
type streamBlock struct {
	ref  *blockRef
	n    int
	err  error
	scan *blockScan // the block's pre-scan, or nil when it is not pre-scanned (DCR-V1-559)
}

// blockRef counts the holders of one block buffer: the window it backs, the hash of its data
// and every decode job or deferred member decoding from it. The last release returns the buffer
// to the pool for a later block, so a streamed read reuses a few buffers instead of allocating
// one per block (DCR-V1-431). A nil blockRef is a buffer outside the pool.
type blockRef struct {
	buf  []byte
	refs atomic.Int32
	pool chan []byte
}

func (b *blockRef) hold() {
	if b != nil {
		b.refs.Add(1)
	}
}

func (b *blockRef) release() {
	if b != nil && b.refs.Add(-1) == 0 {
		select {
		case b.pool <- b.buf:
		default:
		}
	}
}

// blockHeadroom is the room before each block's data for the carry of the previous window.
const blockHeadroom = 64 << 10

// readBlocks reads r into blocks until the end, an error or done, taking each block's buffer
// from pool when one is free. A block starts with two holders: its window and its hash, and a
// third, its pre-scan, when scanner is set; it is submitted to the scanner before it is sent.
func readBlocks(r io.Reader, blocks chan<- streamBlock, done <-chan struct{}, pool chan []byte, window, headroom int, scanner *blockScanner, stats *blockStats) {
	defer close(blocks)
	for {
		var buf []byte
		select {
		case buf = <-pool:
		default:
			buf = make([]byte, headroom+window)
			stats.allocated++
		}
		stats.blocks++
		n, err := io.ReadFull(r, buf[headroom:])
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		ref := &blockRef{buf: buf, pool: pool}
		ref.refs.Store(2)
		b := streamBlock{ref: ref, n: n, err: err}
		if scanner != nil {
			ref.refs.Add(1)
			b.scan = &blockScan{done: make(chan struct{})}
			scanner.submit(b)
		}
		select {
		case blocks <- b:
		case <-done:
			return
		}
		if err != nil {
			return
		}
	}
}

// decodeArtifactStream is decodeArtifact plus verifyCanonical's canonical-form check, with
// the elements of every split member decoded and compared with their canonical encoding in
// parallel (DCR-V1-414). The input is scanned once for member and element boundaries; then:
//   - every split element must be exactly the canonical encoding of the element it decodes to,
//     and the elements must be separated by exactly one comma inside their brackets;
//   - the input with every split member's brackets emptied must equal the canonical encoding
//     of the artifact with those members empty.
//
// Together these are byte equality with the canonical encoding of the whole artifact. Unknown or
// duplicate members refuse as the closed decoder does. header, when set, receives a copy of the
// members that precede the first split member (in canonical order the schema, builder, manifest,
// its digests and the tree) as soon as they are decoded, and runs while the records are scanned
// and decoded; decodeArtifactStream returns only after it does.
//
// The scan holds only the window being scanned: an element is decoded from the window it was
// found in, the bytes outside every split member (the skeleton) are copied once, and a window is
// released when its last element is decoded (DCR-V1-431). When hash is set the bytes are hashed
// in read order and the tail is returned. claims, when set, runs on the decode worker for every
// batch of claims once each of them is decoded and verified, with the offset of its first claim
// (DCR-V1-434); its work must not be used unless the decode succeeds.
func decodeArtifactStream(s *artifactStream, header func(*Artifact), hash bool, claims func(lo int, chunk []Claim)) (*Artifact, artifactTail, error) {
	invalid := fail("invalid closed JSON input")
	var hasher *tailHasher
	if hash {
		hasher = newTailHasher()
		s.onRead = hasher.write
		defer hasher.stop()
	}
	if s.blocks == nil && s.onRead != nil {
		s.onRead(s.buf, nil)
	}
	d := &artifactDecoder{artifactStream: s}
	var a Artifact
	value := reflect.ValueOf(&a).Elem()
	if d.peek() != '{' {
		return nil, artifactTail{}, invalid
	}
	d.pos++
	type plain struct {
		field int
		raw   []byte
	}
	plains := []plain{}
	splits := []*splitArray{}
	// The header members are decoded during the scan even without a header callback: the
	// output bound below depends on the decoded schema (DCR-V1-461). Each is decoded on its own
	// goroutine while the scan goes on to the records, and the header callback starts once all
	// of them are decoded; an undecodable header member refuses as the inline decode did, and
	// the callback never sees it (DCR-V1-554).
	headerSent := false
	var headerWG, headerDecodes sync.WaitGroup
	var headerUndecodable atomic.Bool
	headerFields := []int{}
	sendHeader := func() {
		headerSent = true
		if header == nil {
			return
		}
		fields := headerFields
		headerWG.Go(func() {
			headerDecodes.Wait()
			if headerUndecodable.Load() {
				return
			}
			var h Artifact
			hv := reflect.ValueOf(&h).Elem()
			for _, f := range fields {
				hv.Field(f).Set(value.Field(f))
			}
			header(&h)
		})
	}
	defer headerWG.Wait()
	defer headerDecodes.Wait()
	decodePlain := func(p plain) error {
		if err := json.Unmarshal(p.raw, value.Field(p.field).Addr().Interface(), json.RejectUnknownMembers(true)); err != nil {
			return invalid
		}
		return nil
	}
	// headerDecoded waits for the header members and reports an undecodable one.
	headerDecoded := func() error {
		headerDecodes.Wait()
		if headerUndecodable.Load() {
			return invalid
		}
		return nil
	}
	// Split elements are decoded and compared with their canonical encoding by a pool of workers
	// while the scan continues (DCR-V1-414, DCR-V1-419). A decode failure refuses as the closed
	// decoder does, before any canonical mismatch, so the refusal does not depend on which worker
	// finishes first.
	var undecodable, mismatched, stop atomic.Bool
	jobs := make(chan decodeJob, runtime.GOMAXPROCS(0))
	var workers sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		workers.Go(func() {
			options := json.RejectUnknownMembers(true)
			for job := range jobs {
				verified := true
				for i, raw := range job.raws {
					if stop.Load() {
						verified = false
						break
					}
					element := job.chunk.Index(i)
					if json.Unmarshal(raw, element.Addr().Interface(), options) != nil {
						undecodable.Store(true)
						stop.Store(true)
						verified = false
						break
					}
					// A mismatch stops only the comparisons: every element is still decoded, so an
					// undecodable element anywhere refuses first whatever the batches and their
					// order (DCR-V1-431).
					if !verified {
						continue
					}
					c := encodingComparer{want: raw}
					if _, err := EncodeTo(&c, element.Addr().Interface()); err != nil || c.differ || c.n != int64(len(raw)) {
						mismatched.Store(true)
						verified = false
					}
				}
				job.ref.release()
				if verified && job.claims != nil {
					job.claims(job.lo, job.chunk.Interface().([]Claim))
				}
			}
		})
	}
	closed := false
	finish := func() {
		if !closed {
			closed = true
			close(jobs)
			workers.Wait()
		}
	}
	defer func() {
		stop.Store(true)
		finish()
	}()
	d.dispatch = func(s *splitArray, raws [][]byte) {
		chunk := reflect.MakeSlice(value.Field(s.field).Type(), len(raws), len(raws))
		s.chunks = append(s.chunks, chunk)
		d.cur.hold()
		job := decodeJob{chunk: chunk, raws: raws, lo: len(s.elements) - len(raws), ref: d.cur}
		if s.field == claimsField {
			job.claims = claims
		}
		jobs <- job
	}
	seen := map[string]bool{}
	for more := d.peek() != '}'; more; more = d.consume(',') {
		if d.peek() != '"' {
			return nil, artifactTail{}, invalid
		}
		from, to, ok := d.scan()
		if !ok {
			return nil, artifactTail{}, invalid
		}
		name, ok := memberName(d.buf[from:to])
		member, known := artifactMembers[name]
		if !ok || !known || seen[name] || !d.consume(':') {
			return nil, artifactTail{}, invalid
		}
		seen[name] = true
		if member.split && !headerSent {
			sendHeader()
		}
		if member.split && d.peek() == '[' {
			s := &splitArray{field: member.field, open: d.offset()}
			d.pos++
			d.openSplit(s)
			for more := d.peek() != ']'; more; more = d.consume(',') {
				d.peek()
				spans := d.prescanned()
				if len(spans) == 0 {
					spans = d.parallelElements()
				}
				if len(spans) > 0 {
					// The elements the pre-scan or the parallel scan found are exactly the ones
					// the scan below would find from here, so it resumes after the last
					// (DCR-V1-551, DCR-V1-559).
					for _, e := range spans {
						d.element(s, e.begin, e.end)
					}
					d.pos = spans[len(spans)-1].end
					continue
				}
				begin, end, ok := d.scan()
				if !ok {
					return nil, artifactTail{}, invalid
				}
				d.element(s, begin, end)
			}
			if d.peek() != ']' {
				return nil, artifactTail{}, invalid
			}
			d.closeSplit()
			d.pos++
			s.close = d.offset()
			splits = append(splits, s)
			continue
		}
		d.peek()
		begin, end, ok := d.scan()
		if !ok {
			return nil, artifactTail{}, invalid
		}
		p := plain{member.field, d.buf[begin:end]}
		// A header or deferred member keeps its window out of the pool while it is decoded.
		d.cur.hold()
		if !headerSent {
			// The header members precede the records in the canonical order, so the header work
			// starts while the records are still being scanned.
			headerFields = append(headerFields, p.field)
			ref := d.cur
			headerDecodes.Go(func() {
				defer ref.release()
				if decodePlain(p) != nil {
					headerUndecodable.Store(true)
				}
			})
			continue
		}
		plains = append(plains, p)
	}
	if !d.consume('}') || d.peek() != 0 || d.pos != len(d.buf) || !d.eof {
		return nil, artifactTail{}, invalid
	}
	d.skeleton.Write(d.buf[d.skelFrom-d.base:])
	size := d.base + int64(len(d.buf))
	d.release()
	if err := headerDecoded(); err != nil {
		return nil, artifactTail{}, err
	}
	if !headerSent {
		sendHeader()
	}
	if size > int64(encodingLimit(a)) {
		return nil, artifactTail{}, fail("output bound exceeded")
	}
	noncanonical := fail("noncanonical artifact")
	// The scan consumed exactly one comma between neighbours, so an element starting one byte
	// after its predecessor, right after '[' and ending right before ']' leaves no whitespace.
	for _, s := range splits {
		next := s.open + 1
		for _, e := range s.elements {
			if e.start != next {
				return nil, artifactTail{}, noncanonical
			}
			next = e.end + 1
		}
		if len(s.elements) > 0 && s.close-1 != s.elements[len(s.elements)-1].end || len(s.elements) == 0 && s.close != s.open+2 {
			return nil, artifactTail{}, noncanonical
		}
	}
	var plainWG sync.WaitGroup
	for _, p := range plains {
		plainWG.Go(func() {
			if decodePlain(p) != nil {
				undecodable.Store(true)
			}
		})
	}
	finish()
	plainWG.Wait()
	if undecodable.Load() {
		return nil, artifactTail{}, invalid
	}
	if mismatched.Load() {
		return nil, artifactTail{}, noncanonical
	}
	// The skeleton: the artifact with every split member empty must encode to the input with
	// those members' brackets emptied. It is compared while the decoded chunks are copied into
	// their members, each run of chunks on its own goroutine (DCR-V1-555).
	skeleton := a
	for _, s := range splits {
		field := reflect.ValueOf(&skeleton).Elem().Field(s.field)
		field.Set(reflect.Zero(field.Type()))
	}
	var skeletonWG sync.WaitGroup
	var skeletonErr error
	skeletonEqual := false
	skeletonWG.Go(func() {
		c := encodingComparer{want: d.skeleton.Bytes()}
		_, skeletonErr = EncodeTo(&c, skeleton)
		skeletonEqual = c.equal()
	})
	copyChunks(value, splits)
	skeletonWG.Wait()
	if skeletonErr != nil {
		return nil, artifactTail{}, skeletonErr
	}
	if !skeletonEqual {
		return nil, artifactTail{}, noncanonical
	}
	var tail artifactTail
	if hasher != nil {
		tail = hasher.finish()
	}
	return &a, tail, nil
}

// copyChunks sets each split member of the artifact value to its decoded chunks, in order. The
// chunks are copied in contiguous runs, one goroutine per run, and each chunk is dropped once
// copied.
func copyChunks(value reflect.Value, splits []*splitArray) {
	var wg sync.WaitGroup
	runs := runtime.GOMAXPROCS(0)
	for _, s := range splits {
		slice := value.Field(s.field)
		slice.Set(reflect.MakeSlice(slice.Type(), len(s.elements), len(s.elements)))
		starts := make([]int, len(s.chunks)+1)
		for i, chunk := range s.chunks {
			starts[i+1] = starts[i] + chunk.Len()
		}
		per := (len(s.chunks) + runs - 1) / runs
		for lo := 0; lo < len(s.chunks); lo += per {
			hi := min(lo+per, len(s.chunks))
			wg.Go(func() {
				for i := lo; i < hi; i++ {
					reflect.Copy(slice.Slice(starts[i], starts[i+1]), s.chunks[i])
					s.chunks[i] = reflect.Value{}
				}
			})
		}
	}
	wg.Wait()
}

// decodeJob is one batch of consecutive split elements, the slice they decode into and the
// offset of the first; claims is set for a batch of claims.
type decodeJob struct {
	chunk  reflect.Value
	raws   [][]byte
	lo     int
	claims func(lo int, chunk []Claim)
	ref    *blockRef
}

// claimsField is the Artifact field index of the claims.
var claimsField = artifactMembers["claims"].field

// artifactStream is the artifact input: the window being scanned, its offset in the artifact
// and, for a streamed read, the blocks read ahead that refill it. An in-memory artifact is one
// window.
type artifactStream struct {
	blocks    <-chan streamBlock
	cur       *blockRef // the pooled block backing buf, if any
	headroom  int
	buf       []byte
	base      int64
	pos       int
	eof       bool
	overBound bool
	readErr   error
	onRead    func(run []byte, ref *blockRef) // releases ref once run is hashed
	boundary  boundaryScan
	scanner   *blockScanner // pre-scans the blocks read ahead, if any (DCR-V1-559)
	pre       []elementRun  // the pre-scanned runs of the window's block not yet passed
	preOff    int           // the window offset of the block's first byte
}

// offset is the artifact offset of the scan position.
func (s *artifactStream) offset() int64 { return s.base + int64(s.pos) }

// refill drops the window before keep and appends the next block read ahead after the rest,
// in a new window, so every slice of an earlier window that a decode job holds stays intact.
// A carry longer than the room before a block (a value longer than a window) is copied into an
// unpooled window that takes blocks until it is at least twice the carry, so a value of any
// length is rescanned only a logarithmic number of times and copied a bounded number of times
// per byte. It reports whether any byte was read.
func (s *artifactStream) refill(keep int) bool {
	if s.eof {
		return false
	}
	b, ok := <-s.blocks
	if !ok {
		s.eof = true
		return false
	}
	kept := len(s.buf) - keep
	read := b.n
	previous := s.cur
	s.pre = nil
	if b.scan != nil {
		<-b.scan.done
	}
	if kept <= s.headroom {
		next := b.ref.buf[s.headroom-kept : s.headroom+b.n]
		copy(next, s.buf[keep:])
		s.cur = b.ref
		s.buf = next
		s.pre, s.preOff = b.scan.elementRuns(), kept
		s.take(b, false)
	} else {
		next := make([]byte, kept, max(2*kept, kept+b.n))
		copy(next, s.buf[keep:])
		s.cur = nil
		s.buf = next
		s.take(b, true)
		for !s.eof && len(s.buf) < 2*kept {
			more, ok := <-s.blocks
			if !ok {
				s.eof = true
				break
			}
			if more.scan != nil {
				<-more.scan.done
			}
			read += more.n
			s.take(more, true)
		}
	}
	previous.release()
	s.base += int64(keep)
	s.pos -= keep
	if s.base+int64(len(s.buf)) > MaxCorpusBytes {
		// Past the bound the scan stops as at the end of the input, and the read refuses.
		s.eof, s.overBound, s.pre = true, true, nil
		s.buf = s.buf[:MaxCorpusBytes+1-s.base]
	}
	return read > 0
}

// take hashes block b, records its end or read error, and when copy is set appends its data to
// the window and releases the block's window hold.
func (s *artifactStream) take(b streamBlock, copied bool) {
	data := b.ref.buf[s.headroom : s.headroom+b.n]
	if copied {
		s.buf = append(s.buf, data...)
		b.ref.release()
	}
	if s.onRead != nil {
		s.onRead(data, b.ref)
	} else {
		b.ref.release()
	}
	switch {
	case b.err == io.EOF:
		s.eof = true
	case b.err != nil:
		s.eof, s.readErr = true, b.err
	}
}

// artifactDecoder scans an artifactStream. It copies the skeleton bytes, the bytes outside every
// split member's brackets, as they are passed, and dispatches each batch of split elements
// before the window holding them is dropped.
type artifactDecoder struct {
	*artifactStream
	skeleton bytes.Buffer
	skelFrom int64 // artifact offset of the first skeleton byte not yet copied
	split    *splitArray
	batch    [][]byte
	dispatch func(*splitArray, [][]byte)
	scanned  int64 // artifact offset up to which a parallel scan has looked
}

// element records the split element at buf[begin:end] and dispatches a full batch.
func (d *artifactDecoder) element(s *splitArray, begin, end int) {
	s.elements = append(s.elements, byteSpan{d.base + int64(begin), d.base + int64(end)})
	d.batch = append(d.batch, d.buf[begin:end])
	if len(d.batch) == splitChunk {
		d.flush()
	}
}

// more refills the window keeping buf[keep:]; it reports whether any byte was read.
func (d *artifactDecoder) more(keep int) bool {
	if d.eof {
		return false
	}
	if d.split == nil {
		d.skeleton.Write(d.buf[d.skelFrom-d.base : keep])
		d.skelFrom = d.base + int64(keep)
	}
	d.flush()
	return d.refill(keep)
}

// release drops the last window once the scan has finished.
func (d *artifactDecoder) release() {
	d.buf, d.base, d.pos = d.buf[len(d.buf):], d.base+int64(len(d.buf)), 0
	d.cur.release()
	d.cur = nil
}

// openSplit starts split member s just after its '['; the skeleton keeps the bracket.
func (d *artifactDecoder) openSplit(s *splitArray) {
	d.skeleton.Write(d.buf[d.skelFrom-d.base : d.pos])
	d.split, d.skelFrom = s, d.offset()
	if d.scanner != nil {
		// The blocks taken from here are pre-scanned under this member's separator, which the
		// first element gives once the window holds its member name (DCR-V1-559).
		d.peek()
		d.scanner.publish(elementSeparator(d.buf[d.pos:]))
	}
}

// closeSplit ends the current split member at its ']', which the skeleton keeps.
func (d *artifactDecoder) closeSplit() {
	d.flush()
	d.split, d.skelFrom = nil, d.offset()
	d.scanner.publish(nil)
}

// flush dispatches the batch of split elements scanned so far.
func (d *artifactDecoder) flush() {
	if len(d.batch) > 0 {
		d.dispatch(d.split, d.batch)
		d.batch = make([][]byte, 0, splitChunk)
	}
}

// peek skips whitespace and returns the next byte, or 0 at the end.
func (d *artifactDecoder) peek() byte {
	for {
		for d.pos < len(d.buf) {
			switch c := d.buf[d.pos]; c {
			case ' ', '\t', '\n', '\r':
				d.pos++
			default:
				return c
			}
		}
		if !d.more(d.pos) {
			return 0
		}
	}
}

// consume skips whitespace and then c, reporting whether c was next.
func (d *artifactDecoder) consume(c byte) bool {
	if d.peek() != c || c == 0 {
		return false
	}
	d.pos++
	return true
}

// scan passes over the value at the scan position, refilling the window while the value runs
// past it, and returns the value's bounds in the window.
func (d *artifactDecoder) scan() (begin, end int, ok bool) {
	for {
		begin = d.pos
		if begin >= len(d.buf) {
			return 0, 0, false
		}
		end, state := scanValue(d.buf, begin)
		switch state {
		case scanOK:
			d.pos = end
			return begin, end, true
		case scanShort:
			// A refill moves the window even when it reads nothing, so the value is rescanned
			// from the moved scan position; only once the input has ended is a short scalar whole.
			if !d.eof {
				d.more(begin)
				continue
			}
			if end > begin && scalarStart(d.buf[begin]) {
				d.pos = end
				return begin, end, true
			}
		}
		return 0, 0, false
	}
}

const (
	scanOK = iota
	scanShort
	scanInvalid
)

func scalarStart(c byte) bool { return c != '"' && c != '{' && c != '[' }

// scanValue finds the end of the value starting at data[at] without validating its contents:
// every byte it passes over is validated afterwards, by the closed decode of the value that
// holds it or by the byte comparison of the skeleton with its canonical encoding (DCR-V1-419).
// Strings are passed with a vectorized search for their closing quote, a bracketed value by its
// bracket depth and a scalar up to its delimiter. scanShort means the data ended first; for a
// scalar, end is then the end of the data.
func scanValue(data []byte, at int) (int, int) {
	switch data[at] {
	case 0:
		return 0, scanInvalid
	case '"':
		return scanString(data, at)
	case '{', '[':
		return scanNested(data, at, 0)
	}
	for i := at; i < len(data); i++ {
		switch data[i] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			if i == at {
				return 0, scanInvalid
			}
			return i, scanOK
		}
	}
	return len(data), scanShort
}

// scanNested passes over bracketed bytes from data[at], at bracket depth depth there, to the
// bracket that closes depth 0, skipping strings; it returns the end after that bracket.
func scanNested(data []byte, at, depth int) (int, int) {
	for i := at; i < len(data); {
		switch data[i] {
		case '"':
			end, state := scanString(data, i)
			if state != scanOK {
				return 0, state
			}
			i = end
			continue
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1, scanOK
			}
		}
		i++
	}
	return 0, scanShort
}

// scanString finds the end of the string whose opening quote is at data[open].
func scanString(data []byte, open int) (int, int) {
	for i := open + 1; ; i++ {
		j := bytes.IndexByte(data[i:], '"')
		if j < 0 {
			return 0, scanShort
		}
		i += j
		k := i
		for k > open+1 && data[k-1] == '\\' {
			k--
		}
		if (i-k)%2 == 0 {
			return i + 1, scanOK
		}
	}
}

// memberName is the name a quoted member key denotes.
func memberName(raw []byte) (string, bool) {
	if bytes.IndexByte(raw, '\\') < 0 {
		return string(raw[1 : len(raw)-1]), true
	}
	var name string
	if json.Unmarshal(raw, &name) != nil {
		return "", false
	}
	return name, true
}

// artifactTail is the digest of the artifact bytes before the sha256 member that ends canonical
// artifact bytes, followed by an empty sha256 member: exactly the empty-field encoding hashValue
// hashes, because sha256 is the last member of the canonical object. tail holds the bytes from
// that member on; nil when the bytes hold no such member.
type artifactTail struct {
	tail   []byte
	digest string
}

var tailMember = []byte(`,"sha256":`)

func hashArtifactTail(data []byte) artifactTail {
	at := bytes.LastIndex(data, tailMember)
	if at < 0 {
		return artifactTail{}
	}
	h := sha256.New()
	h.Write(data[:at])
	h.Write([]byte(`,"sha256":""}` + "\n"))
	return artifactTail{data[at:], hex.EncodeToString(h.Sum(nil))}
}

// tailLength is the number of final bytes a streamed hash holds back until the end. The
// canonical tail, `,"sha256":"<64 hex>"}` and a newline, is 78 bytes, so the streamed hash is
// hashArtifactTail's for every artifact the verified read can admit; bytes whose last sha256
// member starts earlier still refuse, as noncanonical.
const tailLength = 4096

// tailHasher hashes streamed artifact bytes on its own goroutine in read order, holding back
// the last tailLength bytes until the end.
type tailHasher struct {
	runs chan hashRun
	done chan artifactTail
	once sync.Once
}

func newTailHasher() *tailHasher {
	t := &tailHasher{runs: make(chan hashRun, 64), done: make(chan artifactTail, 1)}
	go func() {
		h := sha256.New()
		held := make([]byte, 0, 2*tailLength)
		for r := range t.runs {
			run := r.data
			if len(run) >= tailLength {
				h.Write(held)
				h.Write(run[:len(run)-tailLength])
				held = append(held[:0], run[len(run)-tailLength:]...)
			} else {
				held = append(held, run...)
				if over := len(held) - tailLength; over > 0 {
					h.Write(held[:over])
					held = append(held[:0], held[over:]...)
				}
			}
			r.ref.release()
		}
		at := bytes.LastIndex(held, tailMember)
		if at < 0 {
			t.done <- artifactTail{}
			return
		}
		h.Write(held[:at])
		h.Write([]byte(`,"sha256":""}` + "\n"))
		t.done <- artifactTail{held[at:], hex.EncodeToString(h.Sum(nil))}
	}()
	return t
}

// hashRun is one run of artifact bytes to hash and the block holding it.
type hashRun struct {
	data []byte
	ref  *blockRef
}

// write hashes run, which is not written again before ref is released.
func (t *tailHasher) write(run []byte, ref *blockRef) { t.runs <- hashRun{run, ref} }

func (t *tailHasher) finish() artifactTail {
	t.stop()
	return <-t.done
}

func (t *tailHasher) stop() { t.once.Do(func() { close(t.runs) }) }
