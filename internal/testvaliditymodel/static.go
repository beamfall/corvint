package testvaliditymodel

// Static identifies the committed content a static document read (LPCV-V0-150).
type Static struct {
	Revision string   `json:"revision"`
	Files    []string `json:"files"`
}
