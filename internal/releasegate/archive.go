package releasegate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

// Owner-reviewed algorithmic ratchet: every checksum-related read crosses
// tarCandidateSource. For n >= 512 bytes, it permits at most the seed, 35
// field reads for each candidate window, and two reads for each shift.
const tarCandidateMaxChecksumFieldReads = 35

func archiveKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"), strings.HasSuffix(lower, ".tgz"), strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".gz"):
		return "tar"
	}
	return ""
}

func scanArchive(name string, parent Evidence, content []byte) ([]Finding, error) {
	return scanArchiveDepth(name, parent, content, 0)
}
func scanArchiveDepth(name string, parent Evidence, content []byte, depth int) ([]Finding, error) {
	if depth > maxArchiveDepth {
		return []Finding{wholeFinding("archive-depth", "nested archive depth exceeds bound", parent, content)}, nil
	}
	if len(content) > maxArchiveBytes {
		return []Finding{wholeFinding("archive-too-large", "archive exceeds scan bound", parent, content)}, nil
	}
	kind, claimed := archiveKind(name), archiveKind(name) != ""
	sniffed, archiveLike := classifyArchive(content)
	if sniffed == "" || (claimed && kind != sniffed) || !archiveLike {
		return []Finding{wholeFinding("archive-malformed", "archive name and container bytes disagree", parent, content)}, nil
	}
	var members []ArchiveMember
	var err error
	if sniffed == "zip" {
		members, err = ZipArchive{}.Members(content)
	} else {
		members, err = TarArchive{}.Members(content)
	}
	if err != nil {
		return []Finding{wholeFinding("archive-malformed", "archive cannot be scanned safely", parent, content)}, nil
	}
	seen := map[string]struct{}{}
	folded := map[string]struct{}{}
	var findings []Finding
	enclosing := parent.EnclosingMembers
	if parent.MemberPath != "" {
		enclosing = append(append([]string(nil), parent.EnclosingMembers...), parent.MemberPath)
	}
	for _, member := range members {
		memberEvidence := parent
		memberEvidence.EnclosingMembers = enclosing
		memberEvidence.MemberPath = member.Path
		memberEvidence.MemberSHA = sha256Hex(member.Data)
		if !safePath(member.Path) || member.Mode != 0 {
			findings = append(findings, wholeFinding("unsafe-archive-member", "archive member is unsafe, special, or non-regular", memberEvidence, member.Data))
			continue
		}
		if _, duplicate := seen[member.Path]; duplicate {
			findings = append(findings, wholeFinding("duplicate-archive-member", "archive contains duplicate normalized member path", memberEvidence, member.Data))
			continue
		}
		seen[member.Path] = struct{}{}
		fold := strings.ToLower(member.Path)
		if _, duplicate := folded[fold]; duplicate {
			findings = append(findings, wholeFinding("casefold-archive-member", "archive contains case-fold-colliding member path", memberEvidence, member.Data))
			continue
		}
		folded[fold] = struct{}{}
		findings = append(findings, detect(member.Path, member.Data, memberEvidence)...)
		if archiveKind(member.Path) != "" || archiveCandidate(member.Data) {
			nested, err := scanArchiveDepth(member.Path, memberEvidence, member.Data, depth+1)
			if err != nil {
				return nil, err
			}
			findings = append(findings, nested...)
		}
	}
	return findings, nil
}

// Archive names are advisory only.  Every candidate is classified from bytes,
// including an extensionless archive nested inside another archive.
func sniffArchiveKind(content []byte) string {
	kind, _ := classifyArchive(content)
	return kind
}

// classifyArchive recognizes only canonical whole-byte containers.  A marker
// at any other offset is deliberately archive-like but invalid: accepting a
// self-extracting prefix, a concatenated stream, or padded bytes would let an
// archive reader inspect a different byte range from the release gate.
func classifyArchive(content []byte) (kind string, archiveLike bool) {
	if bytes.HasPrefix(content, []byte("PK\x03\x04")) || bytes.HasPrefix(content, []byte("PK\x05\x06")) {
		return map[bool]string{true: "zip"}[validZIPContainer(content)], true
	}
	if bytes.HasPrefix(content, []byte{0x1f, 0x8b}) {
		return map[bool]string{true: "tar"}[validGZIPContainer(content)], true
	}
	if validTARContainer(content) {
		return "tar", true
	}
	return "", archiveCandidate(content)
}

func archiveCandidate(content []byte) bool {
	return bytes.Contains(content, []byte("PK\x03\x04")) ||
		bytes.Contains(content, []byte("PK\x05\x06")) ||
		bytes.Contains(content, []byte{0x1f, 0x8b}) ||
		bytes.Contains(content, []byte("ustar")) || tarHeaderCandidate(content)
}

func archiveCandidateWork(content []byte) (bool, int) {
	reads := 0
	candidate := archiveCandidateFrom(countingTARCandidateSource{content: content, reads: &reads})
	return candidate, reads
}

func archiveCandidateFrom[S tarChecksumByteSource](source S) bool {
	return tarCandidateContains(source, "PK\x03\x04") ||
		tarCandidateContains(source, "PK\x05\x06") ||
		tarCandidateContains(source, "\x1f\x8b") ||
		tarCandidateContains(source, "ustar") || tarHeaderCandidateFrom(source)
}

func tarCandidateContains[S tarChecksumByteSource](source S, marker string) bool {
	for offset := 0; offset+len(marker) <= source.len(); offset++ {
		matched := true
		for index := 0; index < len(marker); index++ {
			if source.byteAt(offset+index) != marker[index] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func tarHeaderCandidate(content []byte) bool {
	if len(content) < 512 {
		return false
	}
	var unsigned, signed int64
	for index := 0; index < 512; index++ {
		value := content[index]
		unsigned += int64(value)
		signed += int64(int8(value))
	}
	for offset := 0; ; offset++ {
		fieldOffset := offset + 148
		f0, f1, f2, f3 := content[fieldOffset], content[fieldOffset+1], content[fieldOffset+2], content[fieldOffset+3]
		f4, f5, f6, f7 := content[fieldOffset+4], content[fieldOffset+5], content[fieldOffset+6], content[fieldOffset+7]
		allZero := f0 == '0' && f1 == '0' && f2 == '0' && f3 == '0' && f4 == '0' && f5 == '0' && f6 == '0' && f7 == '0'
		if !allZero {
			allOctal := f0 >= '0' && f0 <= '7' && f1 >= '0' && f1 <= '7' && f2 >= '0' && f2 <= '7' && f3 >= '0' && f3 <= '7' && f4 >= '0' && f4 <= '7' && f5 >= '0' && f5 <= '7' && f6 >= '0' && f6 <= '7' && f7 >= '0' && f7 <= '7'
			if allOctal {
				want := int64(f0 - '0')
				want = want*8 + int64(f1-'0')
				want = want*8 + int64(f2-'0')
				want = want*8 + int64(f3-'0')
				want = want*8 + int64(f4-'0')
				want = want*8 + int64(f5-'0')
				want = want*8 + int64(f6-'0')
				want = want*8 + int64(f7-'0')
				fieldSum := int64(f0) + int64(f1) + int64(f2) + int64(f3) + int64(f4) + int64(f5) + int64(f6) + int64(f7)
				if want == unsigned-fieldSum+8*' ' || want == signed-fieldSum+8*' ' {
					return true
				}
			} else if tarChecksumFieldPlausibleDirect(content, fieldOffset) && rollingTARChecksumMatchesDirect(content, fieldOffset, unsigned, signed) {
				return true
			}
		}
		if offset+512 == len(content) {
			return false
		}
		outgoing, incoming := content[offset], content[offset+512]
		unsigned += int64(incoming) - int64(outgoing)
		signed += int64(int8(incoming)) - int64(int8(outgoing))
	}
}

func rollingTARChecksumMatchesDirect(content []byte, fieldOffset int, unsigned, signed int64) bool {
	want, ok := tarCandidateHeaderSizeDirect(content, fieldOffset)
	if !ok {
		return false
	}
	for index := 0; index < 8; index++ {
		value := content[fieldOffset+index]
		unsigned -= int64(value)
		signed -= int64(int8(value))
	}
	unsigned += 8 * ' '
	signed += 8 * ' '
	return want == unsigned || want == signed
}

func tarChecksumFieldPlausibleDirect(content []byte, fieldOffset int) bool {
	first := content[fieldOffset]
	if first&0x80 != 0 {
		return first&0x40 == 0
	}
	allZero := first == '0'
	for index := 1; index < 8; index++ {
		if content[fieldOffset+index] != '0' {
			allZero = false
			break
		}
	}
	if allZero {
		return false
	}
	end := 8
	for end > 0 {
		value := content[fieldOffset+end-1]
		if value != ' ' && value != 0 {
			break
		}
		end--
	}
	if end == 0 {
		return false
	}
	for index := 0; index < end; index++ {
		value := content[fieldOffset+index]
		if value < '0' || value > '7' {
			return false
		}
	}
	return true
}

func tarCandidateHeaderSizeDirect(content []byte, fieldOffset int) (int64, bool) {
	first := content[fieldOffset]
	if first&0x80 != 0 {
		if first&0x40 != 0 {
			return 0, false
		}
		var size int64
		for index := 0; index < 8; index++ {
			value := content[fieldOffset+index]
			if index == 0 {
				value &= 0x7f
			}
			if size > (1<<62)/256 {
				return 0, false
			}
			size = size*256 + int64(value)
		}
		return size, true
	}
	end := 8
	for end > 0 {
		value := content[fieldOffset+end-1]
		if value != ' ' && value != 0 {
			break
		}
		end--
	}
	if end == 0 {
		return 0, true
	}
	var size int64
	for index := 0; index < end; index++ {
		value := content[fieldOffset+index]
		if value < '0' || value > '7' || size > (1<<62)/8 {
			return 0, false
		}
		size = size*8 + int64(value-'0')
	}
	return size, true
}

// tarChecksumByteSource prevents the ratcheted checksum path from receiving
// raw archive bytes. Tests attach reads to prove plausibility, parsing,
// checksum adjustment, and rolling-window updates all stay within the bound.
type tarChecksumByteSource interface {
	len() int
	byteAt(offset int) byte
}

type countingTARCandidateSource struct {
	content []byte
	reads   *int
}

func (source countingTARCandidateSource) len() int { return len(source.content) }

func (source countingTARCandidateSource) byteAt(offset int) byte {
	*source.reads++
	return source.content[offset]
}

func tarCandidateChecksumReadBound(inputBytes int) int {
	if inputBytes < 512 {
		return 0
	}
	return 512 + tarCandidateMaxChecksumFieldReads*(inputBytes-511) + 2*(inputBytes-512)
}

func archiveCandidateReadBound(inputBytes int) int {
	return 15*inputBytes + tarCandidateChecksumReadBound(inputBytes)
}

func tarHeaderCandidateFrom[S tarChecksumByteSource](source S) bool {
	if source.len() < 512 {
		return false
	}
	var unsigned, signed int64
	for index := 0; index < 512; index++ {
		value := source.byteAt(index)
		unsigned += int64(value)
		signed += int64(int8(value))
	}
	for offset := 0; ; offset++ {
		fieldOffset := offset + 148
		if tarChecksumFieldPlausible(source, fieldOffset) && rollingTARChecksumMatches(source, fieldOffset, unsigned, signed) {
			return true
		}
		if offset+512 == source.len() {
			return false
		}
		outgoing, incoming := source.byteAt(offset), source.byteAt(offset+512)
		unsigned += int64(incoming) - int64(outgoing)
		signed += int64(int8(incoming)) - int64(int8(outgoing))
	}
}

func rollingTARChecksumMatches[S tarChecksumByteSource](source S, fieldOffset int, unsigned, signed int64) bool {
	want, ok := tarCandidateHeaderSize(source, fieldOffset, 8)
	if !ok {
		return false
	}
	for index := 0; index < 8; index++ {
		value := source.byteAt(fieldOffset + index)
		unsigned -= int64(value)
		signed -= int64(int8(value))
	}
	unsigned += 8 * ' '
	signed += 8 * ' '
	return want == unsigned || want == signed
}

func tarChecksumFieldPlausible[S tarChecksumByteSource](source S, fieldOffset int) bool {
	first := source.byteAt(fieldOffset)
	if first&0x80 != 0 {
		return first&0x40 == 0
	}
	allZero := first == '0'
	for index := 1; index < 8; index++ {
		if source.byteAt(fieldOffset+index) != '0' {
			allZero = false
			break
		}
	}
	if allZero {
		return false
	}
	end := 8
	for end > 0 {
		value := source.byteAt(fieldOffset + end - 1)
		if value != ' ' && value != 0 {
			break
		}
		end--
	}
	if end == 0 {
		return false
	}
	for index := 0; index < end; index++ {
		value := source.byteAt(fieldOffset + index)
		if value < '0' || value > '7' {
			return false
		}
	}
	return true
}

func tarCandidateHeaderSize[S tarChecksumByteSource](source S, fieldOffset, fieldLength int) (int64, bool) {
	first := source.byteAt(fieldOffset)
	if first&0x80 != 0 {
		if first&0x40 != 0 {
			return 0, false
		}
		var size int64
		for index := 0; index < fieldLength; index++ {
			value := source.byteAt(fieldOffset + index)
			if index == 0 {
				value &= 0x7f
			}
			if size > (1<<62)/256 {
				return 0, false
			}
			size = size*256 + int64(value)
		}
		return size, true
	}
	end := fieldLength
	for end > 0 {
		value := source.byteAt(fieldOffset + end - 1)
		if value != ' ' && value != 0 {
			break
		}
		end--
	}
	if end == 0 {
		return 0, true
	}
	var size int64
	for index := 0; index < end; index++ {
		value := source.byteAt(fieldOffset + index)
		if value < '0' || value > '7' || size > (1<<62)/8 {
			return 0, false
		}
		size = size*8 + int64(value-'0')
	}
	return size, true
}

func validArchiveContainer(kind string, content []byte) bool {
	switch kind {
	case "zip":
		return validZIPContainer(content)
	case "tar":
		if bytes.HasPrefix(content, []byte{0x1f, 0x8b}) {
			return validGZIPContainer(content)
		}
		return validTARContainer(content)
	default:
		return false
	}
}

func validGZIPContainer(content []byte) bool {
	if len(content) < 18 || content[3] != 0 {
		return false
	}
	input := bytes.NewReader(content)
	reader, err := gzip.NewReader(input)
	if err != nil {
		return false
	}
	reader.Multistream(false)
	raw, err := io.ReadAll(io.LimitReader(reader, maxArchiveTotal+1))
	closeErr := reader.Close()
	return err == nil && closeErr == nil && len(raw) <= maxArchiveTotal && input.Len() == 0 && validTARContainer(raw)
}

func validZIPContainer(content []byte) bool {
	if len(content) < 22 {
		return false
	}
	end := content[len(content)-22:]
	if !bytes.Equal(end[:4], []byte("PK\x05\x06")) || binary.LittleEndian.Uint16(end[20:22]) != 0 {
		return false
	}
	entries := int(binary.LittleEndian.Uint16(end[10:12]))
	size := int(binary.LittleEndian.Uint32(end[12:16]))
	offset := int(binary.LittleEndian.Uint32(end[16:20]))
	if offset < 0 || size < 0 || offset+size != len(content)-22 {
		return false
	}
	if entries == 0 {
		return size == 0 && offset == 0 && bytes.Equal(content[:4], []byte("PK\x05\x06"))
	}
	if size < 46 || !bytes.Equal(content[:4], []byte("PK\x03\x04")) || offset+size != len(content)-22 {
		return false
	}
	type central struct {
		offset, end, flags, method, crc, compressed, uncompressed int
		name                                                      []byte
	}
	centralEntries := make([]central, 0, entries)
	position, centralEnd := offset, offset+size
	for len(centralEntries) < entries {
		if position+46 > centralEnd || !bytes.Equal(content[position:position+4], []byte("PK\x01\x02")) {
			return false
		}
		flags, method := int(binary.LittleEndian.Uint16(content[position+8:position+10])), int(binary.LittleEndian.Uint16(content[position+10:position+12]))
		nameLen, extraLen, commentLen := int(binary.LittleEndian.Uint16(content[position+28:position+30])), int(binary.LittleEndian.Uint16(content[position+30:position+32])), int(binary.LittleEndian.Uint16(content[position+32:position+34]))
		endEntry := position + 46 + nameLen + extraLen + commentLen
		if endEntry > centralEnd || flags&1 != 0 || extraLen != 0 || commentLen != 0 {
			return false
		}
		centralEntries = append(centralEntries, central{int(binary.LittleEndian.Uint32(content[position+42 : position+46])), endEntry, flags, method, int(binary.LittleEndian.Uint32(content[position+16 : position+20])), int(binary.LittleEndian.Uint32(content[position+20 : position+24])), int(binary.LittleEndian.Uint32(content[position+24 : position+28])), append([]byte(nil), content[position+46:position+46+nameLen]...)})
		position = endEntry
	}
	if position != centralEnd {
		return false
	}
	sort.Slice(centralEntries, func(i, j int) bool { return centralEntries[i].offset < centralEntries[j].offset })
	for index, entry := range centralEntries {
		if entry.offset < 0 || entry.offset+30 > offset || (index == 0 && entry.offset != 0) || (index > 0 && entry.offset <= centralEntries[index-1].offset) || !bytes.Equal(content[entry.offset:entry.offset+4], []byte("PK\x03\x04")) {
			return false
		}
		flags, method := int(binary.LittleEndian.Uint16(content[entry.offset+6:entry.offset+8])), int(binary.LittleEndian.Uint16(content[entry.offset+8:entry.offset+10]))
		nameLen, extraLen := int(binary.LittleEndian.Uint16(content[entry.offset+26:entry.offset+28])), int(binary.LittleEndian.Uint16(content[entry.offset+28:entry.offset+30]))
		dataStart := entry.offset + 30 + nameLen + extraLen
		if dataStart > offset || flags != entry.flags || method != entry.method || extraLen != 0 || !bytes.Equal(content[entry.offset+30:entry.offset+30+nameLen], entry.name) || dataStart+entry.compressed > offset {
			return false
		}
		dataEnd := dataStart + entry.compressed
		if flags&8 == 0 {
			if int(binary.LittleEndian.Uint32(content[entry.offset+14:entry.offset+18])) != entry.crc || int(binary.LittleEndian.Uint32(content[entry.offset+18:entry.offset+22])) != entry.compressed || int(binary.LittleEndian.Uint32(content[entry.offset+22:entry.offset+26])) != entry.uncompressed {
				return false
			}
		} else {
			descriptor := dataEnd
			if descriptor+16 <= offset && bytes.Equal(content[descriptor:descriptor+4], []byte("PK\x07\x08")) {
				descriptor += 4
			}
			if descriptor+12 > offset || int(binary.LittleEndian.Uint32(content[descriptor:descriptor+4])) != entry.crc || int(binary.LittleEndian.Uint32(content[descriptor+4:descriptor+8])) != entry.compressed || int(binary.LittleEndian.Uint32(content[descriptor+8:descriptor+12])) != entry.uncompressed {
				return false
			}
			dataEnd = descriptor + 12
		}
		next := offset
		if index+1 < len(centralEntries) {
			next = centralEntries[index+1].offset
		}
		if dataEnd != next {
			return false
		}
	}
	return true
}
func validTARContainer(content []byte) bool {
	if len(content) < 1024 || len(content)%512 != 0 {
		return false
	}
	for offset := 0; offset+1024 <= len(content); {
		if bytes.Equal(content[offset:offset+512], make([]byte, 512)) {
			// Exactly two terminal zero blocks are required.  Extra zero blocks
			// hide appended bytes from archive/tar's EOF handling.
			return bytes.Equal(content[offset+512:offset+1024], make([]byte, 512)) && offset+1024 == len(content)
		}
		if offset+512 > len(content) || !validTARHeaderChecksum(content[offset:offset+512]) {
			return false
		}
		size, ok := tarHeaderSize(content[offset+124 : offset+136])
		if !ok || size > int64(len(content)) {
			return false
		}
		next := offset + 512 + int((size+511)/512)*512
		if next <= offset || next > len(content) {
			return false
		}
		offset = next
	}
	return false
}

func tarHeaderSize(field []byte) (int64, bool) {
	if len(field) > 0 && field[0]&0x80 != 0 {
		if field[0]&0x40 != 0 {
			return 0, false
		}
		var size int64
		for index, value := range field {
			if index == 0 {
				value &= 0x7f
			}
			if size > (1<<62)/256 {
				return 0, false
			}
			size = size*256 + int64(value)
		}
		return size, true
	}
	field = bytes.TrimRight(field, " \x00")
	if len(field) == 0 {
		return 0, true
	}
	var size int64
	for _, b := range field {
		if b < '0' || b > '7' || size > (1<<62)/8 {
			return 0, false
		}
		size = size*8 + int64(b-'0')
	}
	return size, true
}

func validTARHeaderChecksum(header []byte) bool {
	if len(header) != 512 {
		return false
	}
	want, ok := tarHeaderSize(header[148:156])
	if !ok {
		return false
	}
	var got, signed int64
	for index, b := range header {
		if index >= 148 && index < 156 {
			got, signed = got+' ', signed+' '
		} else {
			got += int64(b)
			signed += int64(int8(b))
		}
	}
	return got == want || signed == want
}

type ArchiveMember struct {
	Path string
	Data []byte
	Mode int
}
type ArchiveReader interface {
	Members([]byte) ([]ArchiveMember, error)
}
type ZipArchive struct{}
type TarArchive struct{}

func (ZipArchive) Members(content []byte) ([]ArchiveMember, error) {
	if !validArchiveContainer("zip", content) {
		return nil, errors.New("invalid ZIP container")
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}
	if len(reader.File) > maxArchiveFiles {
		return nil, errors.New("too many archive members")
	}
	return collectZipMembers(reader.File)
}
func collectZipMembers(files []*zip.File) ([]ArchiveMember, error) {
	members := make([]ArchiveMember, 0, len(files))
	total := int64(0)
	for _, file := range files {
		if file.Flags&1 != 0 || file.Comment != "" || len(file.Extra) > 0 || file.UncompressedSize64 > maxArchiveBytes || file.CompressedSize64 > maxArchiveBytes {
			return nil, errors.New("unsafe ZIP member")
		}
		if file.Method != zip.Store && file.Method != zip.Deflate {
			return nil, errors.New("unsupported ZIP compression")
		}
		total += int64(file.UncompressedSize64)
		if total > maxArchiveTotal {
			return nil, errors.New("archive exceeds bound")
		}
		if file.FileInfo().IsDir() {
			return nil, errors.New("directory archive member")
		}
		mode := 0
		if file.Mode()&os.ModeType != 0 || file.Mode()&os.ModeSymlink != 0 {
			mode = 1
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, maxArchiveBytes+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || len(data) > maxArchiveBytes || uint64(len(data)) != file.UncompressedSize64 {
			return nil, errors.New("cannot read ZIP member")
		}
		members = append(members, ArchiveMember{file.Name, data, mode})
	}
	return members, nil
}

func (TarArchive) Members(content []byte) ([]ArchiveMember, error) {
	if !validArchiveContainer("tar", content) {
		return nil, errors.New("invalid TAR container")
	}
	raw := content
	if bytes.HasPrefix(content, []byte{0x1f, 0x8b}) {
		input := bytes.NewReader(content)
		gzipReader, err := gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		gzipReader.Multistream(false)
		raw, err = io.ReadAll(io.LimitReader(gzipReader, maxArchiveTotal+1))
		if err != nil || len(raw) > maxArchiveTotal || gzipReader.Close() != nil || input.Len() != 0 {
			return nil, errors.New("gzip member is not exactly and fully consumed")
		}
	}
	if !validTARContainer(raw) {
		return nil, errors.New("TAR terminal padding is not exact")
	}
	tarReader := tar.NewReader(bytes.NewReader(raw))
	members := []ArchiveMember{}
	total := int64(0)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(members) >= maxArchiveFiles || header.Size < 0 || header.Size > maxArchiveBytes {
			return nil, errors.New("archive bound")
		}
		total += header.Size
		if total > maxArchiveTotal {
			return nil, errors.New("archive bound")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, errors.New("special TAR member")
		}
		data, err := io.ReadAll(io.LimitReader(tarReader, maxArchiveBytes+1))
		if err != nil || int64(len(data)) != header.Size {
			return nil, errors.New("cannot read TAR member")
		}
		members = append(members, ArchiveMember{header.Name, data, 0})
	}
	return members, nil
}
