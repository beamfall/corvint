//go:build darwin || linux

package analyzerexec

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	testLCMain         = 0x80000028
	testLCBuildVersion = 0x32
	testHeaderSize     = 32
	testPageZeroOffset = testHeaderSize
	testSegmentSize    = 72
	testTextOffset     = testPageZeroOffset + testSegmentSize
	testTextSize       = testSegmentSize + 80
	testBuildOffset    = testTextOffset + testTextSize
	testMainOffset     = testBuildOffset + 24
	testCodeOffset     = 0x200
	testImageBase      = 0x100000000
)

func syntheticMachO(t *testing.T, platform uint32) []byte {
	return syntheticMachOWithVersions(t, 1, []uint32{testLCBuildVersion, 24, platform, 0x000d0000, 0x000d0000, 0})
}

// syntheticMachOWithVersions builds the fixture with an arbitrary
// platform/version command stream so legacy, duplicate, and contradictory
// declarations reach the production parser.
func syntheticMachOWithVersions(t *testing.T, versionCommands uint32, versionWords []uint32) []byte {
	t.Helper()
	var (
		commandSize = uint32(testSegmentSize + testTextSize + 4*len(versionWords) + 24)
		imageSize   = uint64(testCodeOffset + 1)
	)
	order := binary.LittleEndian
	buffer := new(bytes.Buffer)
	write := func(value any) {
		t.Helper()
		if err := binary.Write(buffer, order, value); err != nil {
			t.Fatal(err)
		}
	}
	write(macho.FileHeader{Magic: macho.Magic64, Cpu: macho.CpuArm64, SubCpu: 0, Type: macho.TypeExec, Ncmd: 3 + versionCommands, Cmdsz: commandSize})
	write(uint32(0))
	var pageZeroName [16]byte
	copy(pageZeroName[:], "__PAGEZERO")
	write(macho.Segment64{Cmd: macho.LoadCmdSegment64, Len: testSegmentSize, Name: pageZeroName, Memsz: testImageBase})
	var segmentName [16]byte
	copy(segmentName[:], "__TEXT")
	write(macho.Segment64{Cmd: macho.LoadCmdSegment64, Len: testTextSize, Name: segmentName, Addr: testImageBase, Memsz: 0x1000, Offset: 0, Filesz: imageSize, Maxprot: 5, Prot: 5, Nsect: 1})
	var sectionName [16]byte
	copy(sectionName[:], "__text")
	write(macho.Section64{Name: sectionName, Seg: segmentName, Addr: testImageBase + testCodeOffset, Size: 1, Offset: testCodeOffset, Align: 2, Flags: 0x80000400})
	write(versionWords)
	write([2]uint32{testLCMain, 24})
	write([2]uint64{testCodeOffset, 0})
	for uint64(buffer.Len()) < imageSize {
		buffer.WriteByte(0)
	}
	return buffer.Bytes()
}

func parseMachFixture(t *testing.T, value []byte) (NativePlatform, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mach-executable")
	if err := os.WriteFile(path, value, 0o500); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	return NativeExecutablePlatform(file)
}

func TestNativeExecutablePlatformRejectsNonMacOSMachO(t *testing.T) {
	if platform, err := parseMachFixture(t, syntheticMachO(t, 2)); err == nil {
		t.Fatalf("non-macOS Mach-O accepted as %#v", platform)
	}
}

func TestNativeExecutablePlatformVersionCommandExclusivity(t *testing.T) {
	legacy := []uint32{lcVersionMinMacOS, 16, 0x000d0000, 0x000d0000}
	build := []uint32{testLCBuildVersion, 24, platformMacOS, 0x000d0000, 0x000d0000, 0}
	t.Run("legacy macOS minimum accepted", func(t *testing.T) {
		platform, err := parseMachFixture(t, syntheticMachOWithVersions(t, 1, legacy))
		want := NativePlatform{OS: "darwin", Architecture: "arm64", ABI: "v" + strconv.FormatUint(0x000d0000, 36)}
		if err != nil || platform != want {
			t.Fatalf("legacy declaration platform=%#v err=%v", platform, err)
		}
	})
	rejects := []struct {
		name     string
		commands uint32
		words    []uint32
	}{
		{"legacy iOS minimum", 1, []uint32{lcVersionMinIOS, 16, 0x000d0000, 0x000d0000}},
		{"duplicate platform declarations", 2, append(append([]uint32{}, build...), legacy...)},
		{"SDK below minimum OS", 1, []uint32{testLCBuildVersion, 24, platformMacOS, 0x000d0000, 0x000c0000, 0}},
		{"zero legacy minimum", 1, []uint32{lcVersionMinMacOS, 16, 0, 0x000d0000}},
	}
	for _, reject := range rejects {
		t.Run(reject.name, func(t *testing.T) {
			if platform, err := parseMachFixture(t, syntheticMachOWithVersions(t, reject.commands, reject.words)); err == nil {
				t.Fatalf("invalid declaration accepted as %#v", platform)
			}
		})
	}
}

func TestNativeExecutablePlatformValidatesMachOExecutionContract(t *testing.T) {
	order := binary.LittleEndian
	mutate := func(offset int, value uint32) func([]byte) {
		return func(raw []byte) { order.PutUint32(raw[offset:], value) }
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{"missing platform", mutate(testBuildOffset, 0x31)},
		{"zero minimum OS", mutate(testBuildOffset+12, 0)},
		{"missing entry", mutate(testMainOffset, 0x31)},
		{"entry outside text", func(raw []byte) { order.PutUint64(raw[testMainOffset+8:], testCodeOffset+1) }},
		{"writable executable mapping", func(raw []byte) {
			order.PutUint32(raw[testTextOffset+56:], vmRead|vmWrite|vmExecute)
			order.PutUint32(raw[testTextOffset+60:], vmRead|vmWrite|vmExecute)
		}},
		{"protection outside maximum protection", mutate(testTextOffset+60, vmRead|vmWrite|vmExecute)},
		{"file size above mapped size", func(raw []byte) { order.PutUint64(raw[testTextOffset+32:], 1) }},
		{"entry below text", func(raw []byte) { order.PutUint64(raw[testMainOffset+8:], testCodeOffset-1) }},
		{"file range outside image", func(raw []byte) { order.PutUint64(raw[testTextOffset+48:], uint64(len(raw))+1) }},
		{"overlapping file mappings", func(raw []byte) { order.PutUint64(raw[testPageZeroOffset+48:], 1) }},
		{"overlapping mapped segments", func(raw []byte) { order.PutUint64(raw[testPageZeroOffset+32:], testImageBase+1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := syntheticMachO(t, platformMacOS)
			test.mutate(value)
			if platform, err := parseMachFixture(t, value); err == nil {
				t.Fatalf("invalid Mach-O accepted as %#v", platform)
			}
		})
	}
	t.Run("valid macOS executable", func(t *testing.T) {
		platform, err := parseMachFixture(t, syntheticMachO(t, platformMacOS))
		want := NativePlatform{OS: "darwin", Architecture: "arm64", ABI: "v" + strconv.FormatUint(0x000d0000, 36)}
		if err != nil || platform != want {
			t.Fatalf("valid Mach-O platform=%#v err=%v", platform, err)
		}
	})
}
