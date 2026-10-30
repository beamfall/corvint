//go:build darwin

package analyzerexec

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

func bindActualHostPlatform(executable NativePlatform) (NativePlatform, error) {
	if executable.OS != "darwin" || len(executable.ABI) < 2 || executable.ABI[0] != 'v' {
		return NativePlatform{}, errors.New("invalid Darwin executable ABI")
	}
	product, err := syscall.Sysctl("kern.osproductversion")
	if err != nil {
		return NativePlatform{}, err
	}
	version, ok := parseMacOSProductVersion(product)
	if !ok {
		return NativePlatform{}, errors.New("invalid host macOS version")
	}
	executable.ABI = "v" + strconv.FormatUint(uint64(version), 36)
	return executable, nil
}

func nativeExecutableMatchesHost(executable, host NativePlatform) bool {
	if executable.OS != host.OS || executable.Architecture != host.Architecture {
		return false
	}
	if len(executable.ABI) < 2 || executable.ABI[0] != 'v' || len(host.ABI) < 2 || host.ABI[0] != 'v' {
		return false
	}
	minimum, errMinimum := strconv.ParseUint(executable.ABI[1:], 36, 32)
	actual, errActual := strconv.ParseUint(host.ABI[1:], 36, 32)
	return errMinimum == nil && errActual == nil && minimum <= actual
}

func parseMacOSProductVersion(value string) (uint32, bool) {
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return 0, false
	}
	var version [3]uint64
	for index, part := range parts {
		parsed, err := strconv.ParseUint(part, 10, 16)
		if err != nil || index > 0 && parsed > 0xff {
			return 0, false
		}
		version[index] = parsed
	}
	if version[0] == 0 {
		return 0, false
	}
	return uint32(version[0]<<16 | version[1]<<8 | version[2]), true
}
