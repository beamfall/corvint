package contextindex

// Convention and framework-resource edges of the pinned import graph
// (NGI-V0-306 to NGI-V0-308). Each edge names the referencing site; anything
// that cannot be resolved to exactly one indexed file stays an uncertainty
// record on the referencing path, never an edge.

func webIdentifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
