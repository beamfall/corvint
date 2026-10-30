//go:build linux

package analyzerexec

func bindActualHostPlatform(executable NativePlatform) (NativePlatform, error) {
	return executable, nil
}

func nativeExecutableMatchesHost(executable, host NativePlatform) bool {
	return executable == host
}
