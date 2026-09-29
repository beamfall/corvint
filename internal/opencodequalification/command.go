package opencodequalification

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func Run(ctx context.Context, args []string) error {
	f := flag.NewFlagSet("qualify-opencode", flag.ContinueOnError)
	var c Config
	var native, inspector bool
	var probe, terminalConfig string
	f.StringVar(&c.Source, "source", "", "clean Corvint checkout (default: current Git root)")
	f.StringVar(&c.Host, "host", "", "actual OpenCode 2.0.x executable")
	f.StringVar(&c.Corvint, "corvint", "", "Corvint executable")
	f.StringVar(&c.Output, "output", "", "evidence directory outside the checkout")
	f.StringVar(&c.Theme, "theme", "dark", "inspector theme: dark or light")
	f.BoolVar(&native, "native-only", false, "collect native campaign evidence without qualifying installation")
	f.BoolVar(&inspector, "inspector", false, "verify native terminal UI without qualifying installation")
	f.BoolVar(&c.InterruptProbe, "interrupt-probe", false, "run the interruption regression fixture")
	f.StringVar(&terminalConfig, "terminal-config", "", "internal terminal driver configuration")
	f.StringVar(&probe, "probe-config", "", "internal capture probe configuration")
	if e := f.Parse(args); e != nil {
		return e
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return errors.New("unsupported process cleanup platform")
	}
	if terminalConfig != "" {
		return runTerminal(ctx, terminalConfig)
	}
	if probe != "" {
		return Probe(ctx, probe, f.Args())
	}
	if len(f.Args()) != 0 {
		return errors.New("unexpected positional arguments")
	}
	if c.Host == "" || c.Corvint == "" || c.Output == "" {
		return errors.New("--host, --corvint and --output are required")
	}
	if c.Theme != "dark" && c.Theme != "light" {
		return errors.New("invalid theme")
	}
	if native && inspector {
		return errors.New("choose one campaign")
	}
	var e error
	if c.Source == "" {
		c.Source, e = rootPath()
	} else {
		c.Source, e = resolve(c.Source)
	}
	if e != nil {
		return e
	}
	c.Self, e = os.Executable()
	if e != nil {
		return e
	}
	c.Self, e = resolve(c.Self)
	if e != nil {
		return e
	}
	c.Output, e = filepath.Abs(c.Output)
	if e != nil {
		return e
	}
	if e = mkdir(c.Output); e != nil {
		return e
	}
	c.Output, e = resolve(c.Output)
	if e != nil {
		return e
	}
	relative, e := filepath.Rel(c.Source, c.Output)
	if e != nil {
		return e
	}
	if relative == "." || (!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != "..") {
		return errors.New("evidence output must be outside the checkout")
	}
	if !native && !inspector {
		return qualify(ctx, c)
	}
	if e = validateExecutables(&c); e != nil {
		return e
	}
	if inspector {
		return Inspector(ctx, c)
	}
	if native {
		if e = detectHostVersion(ctx, &c, c.Output); e != nil {
			return e
		}
		report, e := Native(ctx, c)
		if report != nil {
			b, _ := jsonBytes(Object{"result": report["result"], "report": c.Output + "/report.json"})
			fmt.Println(string(b))
		}
		return e
	}
	return nil
}
func validateExecutables(c *Config) error {
	var e error
	c.Host, e = resolve(c.Host)
	if e != nil {
		return e
	}
	c.Corvint, e = resolve(c.Corvint)
	if e != nil {
		return e
	}
	host, e := os.Open(c.Host)
	if e != nil {
		return e
	}
	prefix := make([]byte, 2)
	_, readErr := host.Read(prefix)
	host.Close()
	if readErr != nil {
		return readErr
	}
	if string(prefix) == "#!" {
		return errors.New("--host must name the actual executable, not a shell launcher")
	}
	return nil
}

func detectHostVersion(ctx context.Context, c *Config, evidenceDir string) error {
	home, e := os.MkdirTemp(evidenceDir, "host-version-")
	if e != nil {
		return e
	}
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"XDG_DATA_HOME=" + filepath.Join(home, "data"),
		"XDG_CACHE_HOME=" + filepath.Join(home, "cache"),
		"XDG_STATE_HOME=" + filepath.Join(home, "state"),
		"TMPDIR=" + home,
		"NO_COLOR=1",
	}
	result, e := runCommand(ctx, c.Source, env, []string{c.Host, "--version"}, filepath.Join(evidenceDir, "host-version"), time.Minute)
	if e != nil {
		return e
	}
	version, e := ParseSupportedHostVersion(str(result["stdout"]))
	if e != nil {
		return e
	}
	c.HostVersion = version
	return nil
}
func qualify(ctx context.Context, c Config) (failure error) {
	run, e := os.MkdirTemp(c.Output, "qualification-")
	if e != nil {
		return e
	}
	record := filepath.Join(c.Source, "integrations/opencode-qualification.json")
	if e = beginRecord(ctx, record, filepath.Join(run, "previous-record.json")); e != nil {
		return e
	}
	defer func() {
		if failure != nil {
			_ = writeRecord(context.Background(), record, Object{"profile": Profile, "result": "FAIL", "reason": failure.Error(), "evidenceDirectory": run}, nil)
		}
	}()
	if e = validateExecutables(&c); e != nil {
		return e
	}
	if e = detectHostVersion(ctx, &c, run); e != nil {
		return e
	}
	before, e := identities(ctx, c, true)
	if e != nil {
		return e
	}
	env := cleanEnvironment()
	version, e := runCommand(ctx, c.Source, env, []string{"go", "env", "GOVERSION"}, run+"/go-version", time.Minute)
	if e != nil {
		return e
	}
	if strings.TrimSpace(str(version["stdout"])) != "go1.27.1" {
		return errors.New("Go 1.27.1 is required")
	}
	focused, e := runCommand(ctx, c.Source, env, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", "./cmd/corvint", "-run", "^TestHostAdapterJavaScript(Hosts|HarnessInterruption)$"}, run+"/focused", 30*time.Minute)
	if e != nil {
		return e
	}
	nativeConfig := c
	nativeConfig.Output = filepath.Join(run, "native")
	report, e := Native(ctx, nativeConfig)
	if e != nil {
		return e
	}
	after, e := identities(ctx, c, true)
	if e != nil {
		return e
	}
	result, e := BuildRecord(report, focused, before, after)
	if e != nil {
		return e
	}
	result["evidenceDirectory"] = run
	for _, p := range []string{run + "/report.json", c.Output + "/report.json", record} {
		if e = writeRecord(ctx, p, result, nil); e != nil {
			return e
		}
	}
	b, _ := jsonBytes(Object{"result": "PASS", "record": record, "evidence": run})
	fmt.Println(string(b))
	return nil
}
