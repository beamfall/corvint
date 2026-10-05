//go:build !(darwin || linux)

package service

import (
	"fmt"
	"os"
)

const platformSupported = false

const noFollow = 0

const nonBlock = 0

func fileOwner(os.FileInfo) (uint32, bool) { return 0, false }

func tryLock(*os.File) error { return fmt.Errorf("user service unsupported on this platform") }

func fileID(os.FileInfo) (uint64, uint64) { return 0, 0 }
