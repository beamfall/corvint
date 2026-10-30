//go:build !darwin && !linux && !windows

package analyzerswift

func readDescriptor(string) ([]byte, error) {
	return nil, errDescriptorUnsafe
}
