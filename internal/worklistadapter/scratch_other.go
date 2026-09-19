//go:build !darwin && !linux

package worklistadapter

import "os"

func producerReadFlags() int { return os.O_RDONLY }
