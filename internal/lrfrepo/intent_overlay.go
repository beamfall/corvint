package lrfrepo

// IntentRequirements enumerates the requirement IDs of one Markdown intent
// document by the OCM-V0-001 rule and returns them with the byte span of its
// Requirements section and that span's SHA-256. It reads no repository and
// binds no blob: the reviewer overlay of `corvint obligations` (CEM-V1-642)
// uses it to read an intent the target tree does not carry.
func IntentRequirements(data []byte) ([]string, int64, int64, string, error) {
	intent, requirements, _, err := requirementsFromBlob("", "", data)
	if err != nil {
		return nil, 0, 0, "", err
	}
	return requirements, intent.start, intent.end, intent.spanSHA256, nil
}
