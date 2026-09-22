//go:build windows

package analyzerswift

func readDescriptor(path string) ([]byte, error) {
	return nil, errDescriptorUnsafe
}
