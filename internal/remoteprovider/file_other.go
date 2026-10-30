//go:build !darwin && !linux

package remoteprovider

func readRegular(string, int64, bool) ([]byte, error) { return nil, ErrRefused }
