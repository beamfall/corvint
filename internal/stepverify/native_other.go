//go:build !darwin && !linux

package stepverify

import "os"

func nativeInfo(os.FileInfo) (string, string, uint64, bool) { return "", "", 0, false }
