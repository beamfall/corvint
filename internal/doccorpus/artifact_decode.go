package doccorpus

import (
	"bytes"
	json "encoding/json/v2"
	"reflect"
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

// splitChunk is the number of split elements one decode job takes.
const splitChunk = 128

// decodeArtifactParallel is decodeArtifact plus verifyCanonical's canonical-form check, with
// the elements of every split member decoded and compared with their canonical encoding in
// parallel (DCP-V1-414). The input is validated as JSON in one sequential scan; then:
//   - every split element must be exactly the canonical encoding of the element it decodes to,
//     and the elements must be separated by exactly one comma inside their brackets;
//   - the input with every split member's brackets emptied must equal the canonical encoding
//     of the artifact with those members empty.
//
// Together these are byte equality with the canonical encoding of the whole artifact. Unknown or
// duplicate members refuse as the closed decoder does. header, when set, receives a copy of the
// members that precede the first split member (in canonical order the schema, builder, manifest,
// its digests and the tree) as soon as they are decoded, and runs while the records are scanned
// and decoded; decodeArtifactParallel returns only after it does.
func decodeArtifactParallel(data []byte, header func(*Artifact)) (*Artifact, error) {
	if len(data) > MaxCorpusBytes {
		return nil, fail("input bound exceeded")
	}
	invalid := fail("invalid closed JSON input")
	var a Artifact
	value := reflect.ValueOf(&a).Elem()
	sc := &boundaryScanner{data: data}
	if !sc.consume('{') {
		return nil, invalid
	}
	type plain struct {
		field int
		raw   byteSpan
	}
	plains := []plain{}
	splits := []*splitArray{}
	headerSent := header == nil
	var headerWG sync.WaitGroup
	sendHeader := func() {
		headerSent = true
		h := a
		headerWG.Go(func() { header(&h) })
	}
	defer headerWG.Wait()
	decodePlain := func(p plain) error {
		if err := json.Unmarshal(data[p.raw.start:p.raw.end], value.Field(p.field).Addr().Interface(), json.RejectUnknownMembers(true)); err != nil {
			return invalid
		}
		return nil
	}
	// Split elements are decoded and compared with their canonical encoding in chunks while the
	// scan continues (DCP-V1-414, DCP-V1-419). A decode failure refuses as the closed decoder
	// does, before any canonical mismatch, so the refusal does not depend on which worker
	// finishes first.
	var undecodable, mismatched, stop atomic.Bool
	var wg sync.WaitGroup
	defer func() {
		stop.Store(true)
		wg.Wait()
	}()
	dispatch := func(s *splitArray, from int) {
		spans := s.elements[from:]
		chunk := reflect.MakeSlice(value.Field(s.field).Type(), len(spans), len(spans))
		s.chunks = append(s.chunks, chunk)
		wg.Go(func() {
			for i, span := range spans {
				if stop.Load() {
					return
				}
				raw := data[span.start:span.end]
				if json.Unmarshal(raw, chunk.Index(i).Addr().Interface(), json.RejectUnknownMembers(true)) != nil {
					undecodable.Store(true)
					return
				}
				c := encodingComparer{want: raw}
				if _, err := EncodeTo(&c, chunk.Index(i).Interface()); err != nil || c.differ || c.n != int64(len(raw)) {
					mismatched.Store(true)
					return
				}
			}
		})
	}
	seen := map[string]bool{}
	for more := sc.peek() != '}'; more; more = sc.consume(',') {
		if sc.peek() != '"' {
			return nil, invalid
		}
		from := sc.pos
		if !sc.skipString() {
			return nil, invalid
		}
		name, ok := memberName(data[from:sc.pos])
		member, known := artifactMembers[name]
		if !ok || !known || seen[name] || !sc.consume(':') {
			return nil, invalid
		}
		seen[name] = true
		if member.split && !headerSent {
			sendHeader()
		}
		if member.split && sc.peek() == '[' {
			s := &splitArray{field: member.field, open: int64(sc.pos)}
			sc.pos++
			flushed := 0
			for more := sc.peek() != ']'; more; more = sc.consume(',') {
				begin := sc.pos
				if !sc.skipValue() {
					return nil, invalid
				}
				s.elements = append(s.elements, byteSpan{int64(begin), int64(sc.pos)})
				if len(s.elements)-flushed == splitChunk {
					dispatch(s, flushed)
					flushed = len(s.elements)
				}
			}
			if !sc.consume(']') {
				return nil, invalid
			}
			if flushed < len(s.elements) {
				dispatch(s, flushed)
			}
			s.close = int64(sc.pos)
			splits = append(splits, s)
			continue
		}
		sc.peek()
		begin := sc.pos
		if !sc.skipValue() {
			return nil, invalid
		}
		p := plain{member.field, byteSpan{int64(begin), int64(sc.pos)}}
		if !headerSent {
			// The header members precede the records in the canonical order, so the header work
			// starts while the records are still being scanned.
			if err := decodePlain(p); err != nil {
				return nil, err
			}
			continue
		}
		plains = append(plains, p)
	}
	if !sc.consume('}') || sc.peek() != 0 || sc.pos != len(data) {
		return nil, invalid
	}
	if !headerSent {
		sendHeader()
	}
	if int64(len(data)) > int64(encodingLimit(a)) {
		return nil, fail("output bound exceeded")
	}
	noncanonical := fail("noncanonical artifact")
	// The scan consumed exactly one comma between neighbours, so an element starting one byte
	// after its predecessor, right after '[' and ending right before ']' leaves no whitespace.
	for _, s := range splits {
		next := s.open + 1
		for _, e := range s.elements {
			if e.start != next {
				return nil, noncanonical
			}
			next = e.end + 1
		}
		if len(s.elements) > 0 && s.close-1 != s.elements[len(s.elements)-1].end || len(s.elements) == 0 && s.close != s.open+2 {
			return nil, noncanonical
		}
	}
	for _, p := range plains {
		wg.Go(func() {
			if decodePlain(p) != nil {
				undecodable.Store(true)
			}
		})
	}
	wg.Wait()
	if undecodable.Load() {
		return nil, invalid
	}
	if mismatched.Load() {
		return nil, noncanonical
	}
	for _, s := range splits {
		slice := value.Field(s.field)
		slice.Set(reflect.MakeSlice(slice.Type(), len(s.elements), len(s.elements)))
		at := 0
		for _, chunk := range s.chunks {
			at += reflect.Copy(slice.Slice(at, len(s.elements)), chunk)
		}
	}
	// The skeleton: the artifact with every split member empty must encode to the input with
	// those members' brackets emptied.
	skeleton := a
	var want bytes.Buffer
	from := int64(0)
	for _, s := range splits {
		field := reflect.ValueOf(&skeleton).Elem().Field(s.field)
		field.Set(reflect.Zero(field.Type()))
		want.Write(data[from : s.open+1])
		from = s.close - 1
	}
	want.Write(data[from:])
	c := encodingComparer{want: want.Bytes()}
	if _, err := EncodeTo(&c, skeleton); err != nil {
		return nil, err
	}
	if !c.equal() {
		return nil, noncanonical
	}
	return &a, nil
}

// boundaryScanner finds the byte spans of the artifact's members and of the
// elements of its split members without validating their contents: every
// byte it passes over is validated afterwards, by the closed decode of the
// value that holds it or by the byte comparison of the skeleton with its
// canonical encoding (DCP-V1-419). Strings are skipped with a vectorized
// search for their closing quote.
type boundaryScanner struct {
	data []byte
	pos  int
}

// peek skips whitespace and returns the next byte, or 0 at the end.
func (s *boundaryScanner) peek() byte {
	for s.pos < len(s.data) {
		switch c := s.data[s.pos]; c {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return c
		}
	}
	return 0
}

// consume skips whitespace and then c, reporting whether c was next.
func (s *boundaryScanner) consume(c byte) bool {
	if s.peek() != c || c == 0 {
		return false
	}
	s.pos++
	return true
}

// skipString skips the string whose opening quote is at pos.
func (s *boundaryScanner) skipString() bool {
	open := s.pos
	for i := open + 1; ; i++ {
		j := bytes.IndexByte(s.data[i:], '"')
		if j < 0 {
			return false
		}
		i += j
		k := i
		for k > open+1 && s.data[k-1] == '\\' {
			k--
		}
		if (i-k)%2 == 0 {
			s.pos = i + 1
			return true
		}
	}
}

// skipValue skips whitespace and one value: a string, a bracketed value
// (matching only the bracket depth), or a scalar up to its delimiter.
func (s *boundaryScanner) skipValue() bool {
	switch s.peek() {
	case 0:
		return false
	case '"':
		return s.skipString()
	case '{', '[':
		depth := 0
		for s.pos < len(s.data) {
			switch s.data[s.pos] {
			case '"':
				if !s.skipString() {
					return false
				}
				continue
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					s.pos++
					return true
				}
			}
			s.pos++
		}
		return false
	}
	begin := s.pos
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			return s.pos > begin
		}
		s.pos++
	}
	return s.pos > begin
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
