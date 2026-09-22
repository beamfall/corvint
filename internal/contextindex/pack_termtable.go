package contextindex

import (
	"encoding/binary"
	"errors"
	"io"
	"sort"
	"unsafe"
)

// A term table section is termPostings' five-slice layout (termtable.go)
// stored so it can be viewed in place: [u32 x 5 element counts: key bytes,
// key offsets, offsets, sources, counts][pad to 64][key bytes][pad to 4]
// [u32 key offsets][u32 offsets][u32 sources][u32 counts]. The reader aliases
// each array from the verified mapping; nothing is copied on a little-endian
// host.
const packTermHeader = packAlignment

func termTableSection(name string, postings termPostings) sectionWriter {
	return sectionWriter{name, func(w io.Writer) error {
		arrays := [][]uint32{postings.KeyOffsets, postings.Offsets, postings.Sources, postings.Counts}
		header := make([]byte, 0, packTermHeader)
		header = binary.LittleEndian.AppendUint32(header, uint32(len(postings.KeyBytes)))
		for _, array := range arrays {
			header = binary.LittleEndian.AppendUint32(header, uint32(len(array)))
		}
		header = append(header, make([]byte, packTermHeader-len(header))...)
		if _, err := w.Write(header); err != nil {
			return err
		}
		if _, err := io.WriteString(w, postings.KeyBytes); err != nil {
			return err
		}
		if _, err := padTo(w, uint64(len(postings.KeyBytes)), 4); err != nil {
			return err
		}
		size := 0
		for _, array := range arrays {
			size += 4 * len(array)
		}
		data := make([]byte, 0, size)
		for _, array := range arrays {
			for _, value := range array {
				data = binary.LittleEndian.AppendUint32(data, value)
			}
		}
		_, err := w.Write(data)
		return err
	}}
}

// packTermView is one inverted table viewed over its verified section: the
// same five slices termPostings holds, aliasing the mapping instead of the
// heap. find is termPostings.find's algorithm against the view; postings
// hands the slices to a TermTable without copying, so the ranking code that
// reads Sources and Counts by index is unchanged.
type packTermView struct {
	keyBytes                             string
	keyOffsets, offsets, sources, counts []uint32
}

func viewTermTable(data []byte) (packTermView, error) {
	if len(data) < packTermHeader {
		return packTermView{}, errors.New("pack term table header is truncated")
	}
	keyLength := uint64(binary.LittleEndian.Uint32(data))
	var counts [4]uint64
	for index := range counts {
		counts[index] = uint64(binary.LittleEndian.Uint32(data[4+4*index:]))
	}
	offset := uint64(packTermHeader)
	if offset+keyLength > uint64(len(data)) {
		return packTermView{}, errors.New("pack term table keys lie outside the section")
	}
	view := packTermView{keyBytes: stringView(data[offset : offset+keyLength])}
	offset += keyLength + (4-keyLength%4)%4
	arrays := [4]*[]uint32{&view.keyOffsets, &view.offsets, &view.sources, &view.counts}
	for index, target := range arrays {
		array, next, err := u32Array(data, offset, counts[index])
		if err != nil {
			return packTermView{}, err
		}
		*target, offset = array, next
	}
	if offset != uint64(len(data)) {
		return packTermView{}, errors.New("pack term table has trailing bytes")
	}
	if view.keyCount() < 0 || len(view.offsets) != len(view.keyOffsets) {
		return packTermView{}, errors.New("pack term table offsets do not match its keys")
	}
	return view, nil
}

// u32Array views count little-endian u32 values at offset and returns the
// offset after them; count zero views nil exactly as the gob decode does.
func u32Array(data []byte, offset, count uint64) ([]uint32, uint64, error) {
	end := offset + 4*count
	if end > uint64(len(data)) || end < offset {
		return nil, 0, errors.New("pack u32 array lies outside its section")
	}
	if count == 0 {
		return nil, end, nil
	}
	return u32View(data[offset:end]), end, nil
}

var nativeLittleEndian = binary.NativeEndian.Uint32([]byte{1, 0, 0, 0}) == 1

// u32View aliases data as u32 values where the host is little-endian and the
// bytes are aligned, and decodes a copy elsewhere.
func u32View(data []byte) []uint32 {
	count := len(data) / 4
	if count == 0 {
		return nil
	}
	if nativeLittleEndian && uintptr(unsafe.Pointer(&data[0]))%4 == 0 {
		return unsafe.Slice((*uint32)(unsafe.Pointer(&data[0])), count)
	}
	values := make([]uint32, count)
	for index := range values {
		values[index] = binary.LittleEndian.Uint32(data[4*index:])
	}
	return values
}

func u64View(data []byte) []uint64 {
	count := len(data) / 8
	if count == 0 {
		return nil
	}
	if nativeLittleEndian && uintptr(unsafe.Pointer(&data[0]))%8 == 0 {
		return unsafe.Slice((*uint64)(unsafe.Pointer(&data[0])), count)
	}
	values := make([]uint64, count)
	for index := range values {
		values[index] = binary.LittleEndian.Uint64(data[8*index:])
	}
	return values
}

// stringView aliases data as a string. The mapping is read-only and lives
// for the process, so the alias is never written and never dangles.
func stringView(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return unsafe.String(&data[0], len(data))
}

func (view packTermView) keyCount() int { return len(view.keyOffsets) - 1 }

func (view packTermView) key(index int) string {
	return view.keyBytes[view.keyOffsets[index]:view.keyOffsets[index+1]]
}

// find returns the posting range of key, or false: termPostings.find over
// the view.
func (view packTermView) find(key string) (int, int, bool) {
	count := view.keyCount()
	if count <= 0 {
		return 0, 0, false
	}
	index := sort.Search(count, func(candidate int) bool { return view.key(candidate) >= key })
	if index == count || view.key(index) != key {
		return 0, 0, false
	}
	return int(view.offsets[index]), int(view.offsets[index+1]), true
}

// postings is the view as the five-slice table the ranking reads; the
// slices are shared with the mapping, not copied.
func (view packTermView) postings() termPostings {
	return termPostings{KeyBytes: view.keyBytes, KeyOffsets: view.keyOffsets, Offsets: view.offsets, Sources: view.sources, Counts: view.counts}
}

// check is termPostings.check over the view, so a consistently signed but
// malformed pack cannot make the ranking panic.
func (view packTermView) check(sourceCount int, counted bool) error {
	return view.postings().check(sourceCount, counted)
}
