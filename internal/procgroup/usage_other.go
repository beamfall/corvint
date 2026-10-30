//go:build !darwin && !linux

package procgroup

import "os"

func processResourceUsage(_ *os.ProcessState) *ResourceUsage { return nil }
