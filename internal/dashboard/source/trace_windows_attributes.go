package source

const windowsFileAttributeReparsePoint uint32 = 0x00000400

func windowsFileAttributesReparse(attributes uint32) bool {
	return attributes&windowsFileAttributeReparsePoint != 0
}
