package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
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
}

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
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
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
	for dec.PeekKind() != '}' {
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, invalid
		}
		member, ok := artifactMembers[tok.String()]
		if !ok {
			return nil, invalid
		}
		if member.split && !headerSent {
			sendHeader()
		}
		if member.split && dec.PeekKind() == '[' {
			if _, err := dec.ReadToken(); err != nil {
				return nil, invalid
			}
			s := &splitArray{field: member.field, open: dec.InputOffset() - 1}
			for dec.PeekKind() != ']' {
				raw, err := dec.ReadValue()
				if err != nil {
					return nil, invalid
				}
				end := dec.InputOffset()
				s.elements = append(s.elements, byteSpan{end - int64(len(raw)), end})
			}
			if _, err := dec.ReadToken(); err != nil {
				return nil, invalid
			}
			s.close = dec.InputOffset()
			splits = append(splits, s)
			continue
		}
		raw, err := dec.ReadValue()
		if err != nil {
			return nil, invalid
		}
		end := dec.InputOffset()
		p := plain{member.field, byteSpan{end - int64(len(raw)), end}}
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
	if _, err := dec.ReadToken(); err != nil {
		return nil, invalid
	}
	if _, err := dec.ReadToken(); err != io.EOF {
		return nil, invalid
	}
	if !headerSent {
		sendHeader()
	}
	if int64(len(data)) > int64(encodingLimit(a)) {
		return nil, fail("output bound exceeded")
	}
	noncanonical := fail("noncanonical artifact")
	// The scan validated the syntax, so one byte between neighbours is exactly the comma, and an
	// element starting right after '[' or ending right before ']' leaves no whitespace there.
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
	// A decode failure refuses as the closed decoder does, before any canonical mismatch, so the
	// refusal does not depend on which worker finishes first.
	var undecodable, mismatched atomic.Bool
	var wg sync.WaitGroup
	for _, p := range plains {
		wg.Go(func() {
			if decodePlain(p) != nil {
				undecodable.Store(true)
			}
		})
	}
	workers := runtime.GOMAXPROCS(0)
	for _, s := range splits {
		slice := value.Field(s.field)
		slice.Set(reflect.MakeSlice(slice.Type(), len(s.elements), len(s.elements)))
		chunk := max(64, (len(s.elements)+workers*4-1)/(workers*4))
		for from := 0; from < len(s.elements); from += chunk {
			to := min(from+chunk, len(s.elements))
			wg.Go(func() {
				for i := from; i < to; i++ {
					raw := data[s.elements[i].start:s.elements[i].end]
					if json.Unmarshal(raw, slice.Index(i).Addr().Interface(), json.RejectUnknownMembers(true)) != nil {
						undecodable.Store(true)
						return
					}
					c := encodingComparer{want: raw}
					if _, err := EncodeTo(&c, slice.Index(i).Interface()); err != nil || c.differ || c.n != int64(len(raw)) {
						mismatched.Store(true)
						return
					}
				}
			})
		}
	}
	wg.Wait()
	if undecodable.Load() {
		return nil, invalid
	}
	if mismatched.Load() {
		return nil, noncanonical
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
