package touchsurprise

import (
	"math"
	"sort"
)

// Report is the set arithmetic of one touch-set surprise (TSS-V0-004). It is
// pure: the same two path lists always produce the same report, in path order.
type Report struct {
	Predicted           []string `json:"predicted"`
	Actual              []string `json:"actual"`
	PredictedOnly       []string `json:"predicted_only"`
	ActualOnly          []string `json:"actual_only"`
	Intersection        []string `json:"intersection"`
	SymmetricDifference int      `json:"symmetric_difference"`
	Surprise            float64  `json:"surprise"`
	EmptyActual         bool     `json:"empty_actual"`
}

// Compute is the pure surprise computation over one predicted and one actual
// path set. `surprise` is the share of actually touched paths the prediction
// never named; an empty actual set scores 0 and says so rather than dividing.
func Compute(predicted, actual []string) Report {
	predictedSet, actualSet := pathSet(predicted), pathSet(actual)
	report := Report{
		Predicted:     sortedKeys(predictedSet),
		Actual:        sortedKeys(actualSet),
		PredictedOnly: difference(predictedSet, actualSet),
		ActualOnly:    difference(actualSet, predictedSet),
		Intersection:  intersection(predictedSet, actualSet),
		EmptyActual:   len(actualSet) == 0,
	}
	report.SymmetricDifference = len(report.PredictedOnly) + len(report.ActualOnly)
	if len(actualSet) != 0 {
		report.Surprise = round4(float64(len(report.ActualOnly)) / float64(len(actualSet)))
	}
	return report
}

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func difference(left, right map[string]struct{}) []string {
	paths := make([]string, 0)
	for value := range left {
		if _, shared := right[value]; !shared {
			paths = append(paths, value)
		}
	}
	sort.Strings(paths)
	return paths
}

func intersection(left, right map[string]struct{}) []string {
	paths := make([]string, 0)
	for value := range left {
		if _, shared := right[value]; shared {
			paths = append(paths, value)
		}
	}
	sort.Strings(paths)
	return paths
}

func sortedKeys(set map[string]struct{}) []string {
	paths := make([]string, 0, len(set))
	for value := range set {
		paths = append(paths, value)
	}
	sort.Strings(paths)
	return paths
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
