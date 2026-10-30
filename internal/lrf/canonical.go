package lrf

// CanonicalBytes returns sorted-key compact UTF-8 JSON with one terminal LF.
func CanonicalBytes(result Result) []byte {
	return result.CanonicalBytes()
}
