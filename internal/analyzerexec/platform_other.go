//go:build !darwin && !linux

package analyzerexec

import (
	"errors"
	"os"
)

type NativePlatform struct {
	OS           string
	Architecture string
	ABI          string
}

func ActualHostPlatform() (NativePlatform, error) {
	return NativePlatform{}, errors.New("unsupported executable platform")
}

func NativeExecutablePlatform(*os.File) (NativePlatform, error) {
	return NativePlatform{}, errors.New("unsupported executable platform")
}
