package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckCPU(t *testing.T) {
	cases := []struct {
		name       string
		numCPU     int
		env        string
		configured int
		want       Severity
		wantDetail string
	}{
		{"not pinned", 10, "", 0, SeverityWarn, "not pinned"},
		{"pinned in the config", 10, "", 1, SeverityOK, "gomaxprocs: 1 in the config"},
		{"pinned in the environment", 10, "4", 0, SeverityOK, "GOMAXPROCS=4"},
		// The harness sets the config value last, so the config value wins.
		{"pinned in both", 10, "4", 2, SeverityOK, "gomaxprocs: 2 in the config"},
		{"one core", 1, "", 0, SeverityOK, "1 logical CPU cores"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := checkCPU(c.numCPU, c.env, c.configured)
			if f.Severity != c.want {
				t.Errorf("severity = %v, want %v (%s)", f.Severity, c.want, f.Detail)
			}
			if !strings.Contains(f.Detail, c.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", f.Detail, c.wantDetail)
			}
		})
	}
}

// A run from a subdirectory must find the config at the repository root.
func TestCheckFindsThePinInTheConfig(t *testing.T) {
	if runtime.NumCPU() == 1 {
		t.Skip("one core never warns")
	}
	t.Setenv("GOMAXPROCS", "")
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.Mkdir(filepath.Join(root, ".autor3search"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".autor3search", "config.yaml"), []byte("gomaxprocs: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range Check(sub) {
		if f.Name == "gomaxprocs" {
			if f.Severity != SeverityOK {
				t.Errorf("severity = %v (%s), want OK", f.Severity, f.Detail)
			}
			return
		}
	}
	t.Fatal("no gomaxprocs finding")
}

func TestCheckWarnsWithNoConfig(t *testing.T) {
	if runtime.NumCPU() == 1 {
		t.Skip("one core never warns")
	}
	t.Setenv("GOMAXPROCS", "")
	for _, f := range Check(t.TempDir()) {
		if f.Name == "gomaxprocs" && f.Severity != SeverityWarn {
			t.Errorf("severity = %v (%s), want a warning", f.Severity, f.Detail)
		}
	}
}

func TestLoadFinding(t *testing.T) {
	cases := []struct {
		name   string
		load   float64
		numCPU int
		want   Severity
		detail string
	}{
		{"low", 1, 10, SeverityOK, "1.00 (threshold: 5.00)"},
		{"at the threshold", 5, 10, SeverityOK, "5.00 (threshold: 5.00)"},
		{"above the threshold", 5.01, 10, SeverityWarn, "5.01 (threshold: 5.00)"},
	}
	for _, c := range cases {
		f := loadFinding(c.load, c.numCPU)
		if f.Severity != c.want || f.Name != "load" || !strings.Contains(f.Detail, c.detail) {
			t.Errorf("%s: got %+v, want severity %v and detail with %q", c.name, f, c.want, c.detail)
		}
	}
}

// macOS has no /proc/loadavg, and the check once reported "not available" there.
func TestCheckLoadAverageReadsARealValueOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	f := checkLoadAverage()
	if f.Severity == SeverityNotApplicable {
		t.Errorf("load was not read: %s", f.Detail)
	}
}
