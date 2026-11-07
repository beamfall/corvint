package t5

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"testing"
	"text/template"
	"time"
)

var mu sync.Mutex

func TestHome(t *testing.T) { _, _ = os.UserHomeDir() }

func TestZip(t *testing.T) { zip.RegisterCompressor(65000, nil) }

func TestCrypto(t *testing.T) { _ = crypto.MD4.Available() }

func TestRuntime(t *testing.T) { _ = runtime.SetMutexProfileFraction(-1) }

func TestTemplate(t *testing.T) { _, _ = template.ParseFiles("x.tmpl") }

func TestParser(t *testing.T) { _, _ = parser.ParseFile(token.NewFileSet(), "x.go", nil, 0) }

func TestLocation(t *testing.T) { _, _ = time.LoadLocation("Europe/Paris") }

func TestSlog(t *testing.T) { slog.SetDefault(slog.Default()) }

func TestNotifyContext(t *testing.T) {
	_, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	stop()
}

func TestWriterInterface(t *testing.T) {
	var w io.Writer = os.Stdout
	_, _ = w.Write(nil)
}

func TestFprintFile(t *testing.T) { fmt.Fprintln(os.Stderr, "x") }

func TestFprintBuffer(t *testing.T) {
	var b bytes.Buffer
	fmt.Fprintln(&b, "x")
}

func TestPrintValue(t *testing.T) {
	f := fmt.Println
	f("x")
}

func TestSymlink(t *testing.T) { _ = os.Symlink("/", t.TempDir()+"/link") }

func TestLock(t *testing.T) { mu.Lock(); mu.Unlock() }
