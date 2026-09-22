package main

import (
	"io"
	"strings"
)

// The experimental command has no caller-root, policy, signer or fixture-key
// option. An admitted deployment adapter is required before it can compute
// current authority. Legacy frontier remains byte-identical.
func parseFrontierNextInvocation(arguments []string) bool {
	return len(arguments) > 0 && arguments[0] == "frontier-next"
}
func runFrontierNext(arguments []string, stdout io.Writer) int {
	valid := len(arguments) == 2 && arguments[0] == "--enrollment" && len(arguments[1]) == 64 && strings.Trim(arguments[1], "0123456789abcdef") == ""
	if !valid {
		io.WriteString(stdout, "{\"authority\":\"NONE\",\"code\":\"invalid-protected-execution\",\"profile\":\"frontier/2-experimental\",\"state\":\"UNKNOWN\"}\n")
		return 2
	}
	io.WriteString(stdout, "{\"authority\":\"NONE\",\"code\":\"authority-unavailable\",\"profile\":\"frontier/2-experimental\",\"state\":\"UNKNOWN\"}\n")
	return 2
}
