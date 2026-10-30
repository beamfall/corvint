//go:build darwin || linux

package analyzerexec

import (
	"debug/elf"
	"debug/macho"
	"encoding/binary"
	"errors"
	"os"
	"sort"
	"strconv"
	"sync"
)

const (
	lcVersionMinMacOS = 0x24
	lcVersionMinIOS   = 0x25
	lcMain            = 0x80000028
	lcVersionMinTVOS  = 0x2f
	lcVersionMinWatch = 0x30
	lcBuildVersion    = 0x32
	platformMacOS     = 1
	vmRead            = 1
	vmWrite           = 2
	vmExecute         = 4
	maxMachSegments   = 128
)

// NativePlatform is the executable-header tuple required by a contained
// native launch. It deliberately names the native object format ABI rather
// than a caller-selected compatibility label.
type NativePlatform struct {
	OS           string
	Architecture string
	ABI          string
}

var hostPlatform struct {
	once  sync.Once
	value NativePlatform
	err   error
}

func ActualHostPlatform() (NativePlatform, error) {
	hostPlatform.once.Do(func() {
		path, err := os.Executable()
		if err != nil {
			hostPlatform.err = err
			return
		}
		file, err := os.Open(path)
		if err != nil {
			hostPlatform.err = err
			return
		}
		defer file.Close()
		hostPlatform.value, hostPlatform.err = NativeExecutablePlatform(file)
		if hostPlatform.err == nil {
			hostPlatform.value, hostPlatform.err = bindActualHostPlatform(hostPlatform.value)
		}
	})
	return hostPlatform.value, hostPlatform.err
}

// NativeExecutablePlatform parses a retained descriptor, never a mutable
// pathname. A valid magic number alone is intentionally insufficient.
func NativeExecutablePlatform(file *os.File) (NativePlatform, error) {
	if file == nil {
		return NativePlatform{}, errors.New("missing executable descriptor")
	}
	stat, err := file.Stat()
	if err != nil || stat.Size() <= 0 || stat.Size() > MaxExecutableBytes {
		return NativePlatform{}, errors.New("invalid executable size")
	}
	var magic [4]byte
	if count, err := file.ReadAt(magic[:], 0); err != nil || count != len(magic) {
		return NativePlatform{}, errors.New("truncated executable")
	}
	var platform NativePlatform
	if string(magic[:]) == "\x7fELF" {
		object, err := elf.NewFile(file)
		if err != nil {
			return NativePlatform{}, err
		}
		architecture := elfArchitectureName(object.Machine)
		if architecture == "" {
			return NativePlatform{}, errors.New("unsupported ELF architecture")
		}
		bits, endian := "32", "b"
		if object.Class == elf.ELFCLASS64 {
			bits = "64"
		} else if object.Class != elf.ELFCLASS32 {
			return NativePlatform{}, errors.New("unsupported ELF class")
		}
		if object.Data == elf.ELFDATA2LSB {
			endian = "l"
		} else if object.Data != elf.ELFDATA2MSB {
			return NativePlatform{}, errors.New("unsupported ELF byte order")
		}
		platform = NativePlatform{OS: "linux", Architecture: architecture, ABI: "e" + bits + endian + decimalByte(byte(object.OSABI))}
	} else {
		object, err := macho.NewFile(file)
		if err != nil {
			return NativePlatform{}, err
		}
		architecture := machArchitectureName(object.Cpu, object.SubCpu)
		if architecture == "" {
			return NativePlatform{}, errors.New("unsupported Mach-O architecture")
		}
		if object.ByteOrder != binary.LittleEndian || object.Magic != macho.Magic32 && object.Magic != macho.Magic64 {
			return NativePlatform{}, errors.New("unsupported Mach-O class")
		}
		wideArchitecture := architecture == "amd64" || architecture == "arm64" || architecture == "arm64e"
		if wideArchitecture != (object.Magic == macho.Magic64) {
			return NativePlatform{}, errors.New("Mach-O architecture and class mismatch")
		}
		platform = NativePlatform{OS: "darwin", Architecture: architecture}
	}
	abi, err := validateNativeObject(file, platform, uint64(stat.Size()))
	if err != nil {
		return NativePlatform{}, err
	}
	platform.ABI += abi
	return platform, nil
}

func validateNativeObject(file *os.File, platform NativePlatform, fileSize uint64) (string, error) {
	switch platform.OS {
	case "linux":
		object, err := elf.NewFile(file)
		if err != nil || object.Entry == 0 || object.Type != elf.ET_EXEC && object.Type != elf.ET_DYN {
			return "", errors.New("invalid ELF object")
		}
		loadContainsEntry := false
		for _, program := range object.Progs {
			if program.Type != elf.PT_LOAD || program.Memsz == 0 || program.Flags&elf.PF_X == 0 {
				continue
			}
			if program.Flags&elf.PF_W != 0 {
				return "", errors.New("writable executable ELF segment")
			}
			if object.Entry >= program.Vaddr && object.Entry-program.Vaddr < program.Memsz {
				loadContainsEntry = true
			}
		}
		if !loadContainsEntry || elfArchitectureName(object.Machine) != platform.Architecture {
			return "", errors.New("ELF entry or architecture mismatch")
		}
		return "", nil
	case "darwin":
		return validateMachO(file, platform, fileSize)
	default:
		return "", errors.New("unsupported native object")
	}
}

type machRange struct{ start, end uint64 }

func validateMachO(file *os.File, platform NativePlatform, fileSize uint64) (string, error) {
	object, err := macho.NewFile(file)
	if err != nil || object.Type != macho.TypeExec || machArchitectureName(object.Cpu, object.SubCpu) != platform.Architecture || len(object.Loads) == 0 {
		return "", errors.New("invalid Mach-O object")
	}
	minOS, entry, err := machMetadata(object)
	if err != nil {
		return "", err
	}
	var mapped []machRange
	var fileBacked []machRange
	var executable []*macho.Segment
	segments := 0
	for _, load := range object.Loads {
		segment, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		segments++
		if segments > maxMachSegments {
			return "", errors.New("too many Mach-O segments")
		}
		if segment.Prot & ^uint32(vmRead|vmWrite|vmExecute) != 0 || segment.Maxprot & ^uint32(vmRead|vmWrite|vmExecute) != 0 || segment.Prot & ^segment.Maxprot != 0 || segment.Prot&vmWrite != 0 && segment.Prot&vmExecute != 0 {
			return "", errors.New("invalid Mach-O segment protection")
		}
		if segment.Offset > fileSize || segment.Filesz > fileSize-segment.Offset {
			return "", errors.New("invalid Mach-O segment file range")
		}
		if segment.Filesz > 0 {
			fileBacked = append(fileBacked, machRange{segment.Offset, segment.Offset + segment.Filesz})
		}
		if segment.Memsz == 0 {
			if segment.Prot != 0 || segment.Maxprot != 0 {
				return "", errors.New("invalid unmapped Mach-O segment")
			}
			continue
		}
		if segment.Filesz > segment.Memsz || segment.Addr+segment.Memsz < segment.Addr {
			return "", errors.New("invalid Mach-O segment mapping")
		}
		mapped = append(mapped, machRange{segment.Addr, segment.Addr + segment.Memsz})
		if segment.Prot&vmExecute != 0 {
			executable = append(executable, segment)
		}
	}
	if len(executable) == 0 || rangesOverlap(mapped) || rangesOverlap(fileBacked) {
		return "", errors.New("invalid Mach-O executable mapping")
	}
	text := object.Section("__text")
	if text == nil || text.Size == 0 || text.Flags&0x80000000 == 0 || uint64(text.Offset) > fileSize || text.Size > fileSize-uint64(text.Offset) {
		return "", errors.New("missing Mach-O executable text")
	}
	entryValid := false
	for _, segment := range executable {
		segmentEnd := segment.Addr + segment.Memsz
		fileEnd := segment.Offset + segment.Filesz
		textOffset := uint64(text.Offset)
		if text.Seg != segment.Name || text.Addr < segment.Addr || text.Addr >= segmentEnd || text.Size > segmentEnd-text.Addr || textOffset < segment.Offset || textOffset >= fileEnd || text.Size > fileEnd-textOffset {
			continue
		}
		if entry >= uint64(text.Offset) && entry-uint64(text.Offset) < text.Size {
			entryValid = true
		}
	}
	if !entryValid {
		return "", errors.New("invalid Mach-O entry point")
	}
	return "v" + strconv.FormatUint(uint64(minOS), 36), nil
}

func machMetadata(object *macho.File) (uint32, uint64, error) {
	var minOS uint32
	var entry uint64
	platformCommands, entryCommands := 0, 0
	for _, load := range object.Loads {
		raw := load.Raw()
		if len(raw) < 8 {
			return 0, 0, errors.New("invalid Mach-O load command")
		}
		command := object.ByteOrder.Uint32(raw)
		switch command {
		case lcBuildVersion:
			if len(raw) < 24 || object.ByteOrder.Uint32(raw[8:]) != platformMacOS {
				return 0, 0, errors.New("non-macOS Mach-O platform")
			}
			tools := uint64(object.ByteOrder.Uint32(raw[20:]))
			if tools > uint64((len(raw)-24)/8) || 24+tools*8 != uint64(len(raw)) {
				return 0, 0, errors.New("invalid Mach-O build version")
			}
			minOS = object.ByteOrder.Uint32(raw[12:])
			sdk := object.ByteOrder.Uint32(raw[16:])
			if minOS == 0 || sdk < minOS {
				return 0, 0, errors.New("invalid Mach-O minimum OS")
			}
			platformCommands++
		case lcVersionMinMacOS:
			if len(raw) != 16 {
				return 0, 0, errors.New("invalid Mach-O minimum OS")
			}
			minOS = object.ByteOrder.Uint32(raw[8:])
			if sdk := object.ByteOrder.Uint32(raw[12:]); minOS == 0 || sdk < minOS {
				return 0, 0, errors.New("invalid Mach-O minimum OS")
			}
			platformCommands++
		case lcVersionMinIOS, lcVersionMinTVOS, lcVersionMinWatch:
			return 0, 0, errors.New("non-macOS Mach-O platform")
		case lcMain:
			if len(raw) != 24 {
				return 0, 0, errors.New("invalid Mach-O entry command")
			}
			entry = object.ByteOrder.Uint64(raw[8:])
			entryCommands++
		}
	}
	if platformCommands != 1 || entryCommands != 1 {
		return 0, 0, errors.New("incomplete Mach-O execution metadata")
	}
	return minOS, entry, nil
}

func rangesOverlap(ranges []machRange) bool {
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	if len(ranges) == 0 {
		return false
	}
	end := ranges[0].end
	for index := 1; index < len(ranges); index++ {
		if ranges[index].start < end {
			return true
		}
		end = ranges[index].end
	}
	return false
}

func elfArchitectureName(machine elf.Machine) string {
	switch machine {
	case elf.EM_386:
		return "386"
	case elf.EM_ARM:
		return "arm"
	case elf.EM_X86_64:
		return "amd64"
	case elf.EM_AARCH64:
		return "arm64"
	default:
		return ""
	}
}

func machArchitectureName(cpu macho.Cpu, subtype uint32) string {
	base := subtype & 0x00ffffff
	switch cpu {
	case macho.Cpu386:
		if base == 3 {
			return "386"
		}
	case macho.CpuAmd64:
		if base == 3 {
			return "amd64"
		}
	case macho.CpuArm:
		if base == 0 {
			return "arm"
		}
	case macho.CpuArm64:
		if base == 2 {
			return "arm64e"
		}
		if base == 0 {
			return "arm64"
		}
	}
	return ""
}

func decimalByte(value byte) string {
	if value == 0 {
		return "0"
	}
	var digits [3]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = '0' + value%10
		value /= 10
	}
	return string(digits[position:])
}
