// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-984 (V1-3751): narrow.jq samples a run that would narrow when the head's first eight hex
// digits plus the seed are divisible by k; the sampled shard runs FULL, says why and keeps the
// driver's selection. An unsampled seed, an empty k and a run already FULL for another reason
// carry no sample.
func TestAFPV0984SampledRunIsFullAndKeepsTheSelection(t *testing.T) {
	const (
		base   = "142f0e3e7bfb153f803fb6c4bad7989f87ee324a"
		head   = "460fc31ae6d0134fa27f7a66336b99db88fc91c6" // 0x460fc31a = 1175438106
		target = "5163d1ed4b61084fc32cf79355848899c3e8a5bb"
		a, b   = "example.org/m/a", "example.org/m/b"
	)
	decide := func(t *testing.T, github, k, seed string) doc {
		dir := t.TempDir()
		write := func(name string, v any) string {
			p := filepath.Join(dir, name)
			if s, ok := v.(string); ok {
				writeFile(t, p, []byte(s))
			} else {
				writeDoc(t, p, v)
			}
			return p
		}
		args := []string{"-n", "-c",
			"--slurpfile", "driver", write("driver.json", doc{"Base": base, "Head": head, "Target": target, "Packages": []string{a}}),
			"--slurpfile", "shadow", write("shadow.json", doc{"scope": "NARROWED", "invocations": []any{doc{"units": []any{doc{"adapter": "go", "unit": "go:" + a}}}}}),
			"--arg", "shadowExit", "0", "--rawfile", "packages", write("packages", a+"\n"+b+"\n"), "--rawfile", "slices", write("slices", ""),
			"--rawfile", "selected", write("selected", a+"\n"), "--rawfile", "balancedPackages", write("placed", a+"\n"),
			"--rawfile", "balancedSlices", write("placed-slices", ""), "--arg", "balanced", "1",
			"--rawfile", "universe", write("universe", a+"\n"+b+"\n"), "--arg", "selectedOk", "1",
			"--arg", "github", github, "--arg", "mergeQueue", "1", "--arg", "baseRef", "main", "--arg", "base", base,
			"--arg", "head", head, "--arg", "target", target, "--arg", "pr", "868", "--arg", "runId", prRun, "--arg", "runAttempt", "1",
			"--arg", "shard", "3", "--arg", "shards", "8", "--arg", "toolSource", target, "--arg", "verdictRun", "4242",
			"--arg", "sampleK", k, "--arg", "sampleSeed", seed, "-f", script(t, "narrow.jq")}
		out, err := exec.Command("jq", args...).Output()
		if err != nil {
			t.Fatalf("narrow.jq: %v", err)
		}
		var v doc
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	// (1175438106 + 4) % 10 == 0: sampled.
	v := decide(t, "0", "10", "4")
	if v["mode"] != "FULL" || fmt.Sprint(v["reasons"]) != "[sampled for the AFP-V0-984 miss-rate estimate (1 in 10)]" ||
		fmt.Sprint(sub(v, "sample")) != fmt.Sprint(doc{"k": float64(10), "seed": float64(4), "selected": []any{a}}) ||
		v["placement"] != "PARTITION" || fmt.Sprint(v["executed"]) != fmt.Sprint([]any{a, b}) {
		t.Fatalf("sampled decision = %v", v)
	}
	for name, c := range map[string][3]string{
		"another seed": {"0", "10", "3751"}, "sampling off": {"0", "", "4"}, "k below 2": {"0", "1", "0"},
	} {
		if v := decide(t, c[0], c[1], c[2]); v["mode"] != "NARROWED" || v["sample"] != nil {
			t.Errorf("%s: decision = %v", name, v)
		}
	}
	if v := decide(t, "1", "10", "4"); v["mode"] != "FULL" || v["sample"] != nil || len(v["reasons"].([]any)) != 1 {
		t.Fatalf("a run FULL for another reason was sampled: %v", v)
	}
}

// AFP-V0-984: a pull-request row whose every shard record was sampled names the failed packages
// outside the driver's selection; one inside it, or a run with an unsampled shard, has none.
func TestAFPV0984SampledRowNamesFailuresOutsideTheSelection(t *testing.T) {
	pr := map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun}
	sampled := func(t *testing.T, selected []string, every bool) doc {
		f := prFixture(t)
		p := strs(f.sel[0]["selected"])[0]
		fail(f.out[0], p, "fail", "RAN", "fail")
		f.nar = map[int]doc{}
		for k := 0; k < shards; k++ {
			r := narrowingRecord(k, "FULL", prRun, nil, nil)
			if every || k != 5 {
				r["sample"] = doc{"k": 10, "seed": 4, "selected": selected}
			}
			f.nar[k] = r
		}
		row, _ := f.classify(t, pr)
		if row["class"] == "NARROWED" {
			t.Fatalf("a sampled run is no NARROWED row: %v", row)
		}
		return row
	}
	f := prFixture(t)
	p := strs(f.sel[0]["selected"])[0]
	row := sampled(t, []string{"github.com/Beamfall/corvint/elsewhere"}, true)
	s := sub(row, "sample")
	if num(s["k"]) != 10 || num(s["selected"]) != 1 || fmt.Sprint(s["failingOutside"]) != "["+p+"]" || s["missed"] != true {
		t.Fatalf("sample outside the selection = %v", s)
	}
	if s := sub(sampled(t, []string{p}, true), "sample"); len(s["failingOutside"].([]any)) != 0 || s["missed"] != false {
		t.Fatalf("sample inside the selection = %v", s)
	}
	if row := sampled(t, []string{p}, false); row["sample"] != nil {
		t.Fatalf("a run with an unsampled shard has a sample: %v", row["sample"])
	}
	if row, _ := prFixture(t).classify(t, pr); row["sample"] != nil {
		t.Fatalf("an unsampled run has a sample: %v", row["sample"])
	}
}

// AFP-V0-984: the bar counts the epoch's sample names once per pull request and head and reports
// the estimated miss rate with its Wilson 95% upper bound, outside the status and the kill switch.
func TestAFPV0984BarEstimatesTheMissRate(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	names := strings.Join([]string{
		"ci-forward-SAMPLE-pr1-" + a + "-e2-k10-m1", "ci-forward-SAMPLE-pr1-" + a + "-e2-k10-m0", // the newer name wins
		"ci-forward-SAMPLE-pr2-" + b + "-e2-k10-m0", "ci-forward-SAMPLE-pr3-" + b + "-e2-k10-m0",
		"ci-forward-SAMPLE-pr4-" + b + "-e1-k10-m1", "ci-forward-CLEAN-pr2-" + b + "-e2-f0"}, "\n") + "\n"
	bar := func(format string) string {
		args := []string{"-c", `printf '%s' "$1" | jq -R -s -r --arg epoch 2 ` + format + ` -f "$2"`, "sh", names, script(t, "bar.jq")}
		return run(t, nil, "sh", args...)
	}
	var verdict doc
	if err := json.Unmarshal([]byte(bar("--arg format json")), &verdict); err != nil {
		t.Fatal(err)
	}
	if m := sub(verdict, "sampledMissRate"); num(m["sampled"]) != 3 || num(m["misses"]) != 1 || m["estimate"] != 0.333 || m["upper95"] != 0.793 {
		t.Fatalf("sampledMissRate = %v", m)
	}
	if sub(verdict, "epochRows")["miss"] != float64(0) || verdict["status"] != "NOT MET" {
		t.Fatalf("a sample name fed the bar: %v", verdict)
	}
	if md := bar(""); !strings.Contains(md, "1 of 3 sampled narrowing pull request(s) failed a package outside the driver's selection; estimated miss rate 0.333, Wilson 95% upper bound 0.793.") {
		t.Fatalf("bar markdown:\n%s", md)
	}
}
