//go:build !linux

package perfbudget

import "os/exec"

// relay is a no-op off Linux, where a child's maxrss is its own (PRB-V0-060).
type relay struct{ target string }

type relayReport struct {
	StartError string
	WallUS     int64
	MaxRSSKiB  int64
	RelayKiB   int64
}

func startRelayed(*exec.Cmd) (*relay, error) { return nil, nil }
func (rl *relay) started()                   {}
func (rl *relay) result() *relayReport       { return nil }
