//go:build !darwin && !linux

package authority

func newChangeWatch() (changeWatch, error) { return nil, fsErr("change guard", "unsupported platform") }
