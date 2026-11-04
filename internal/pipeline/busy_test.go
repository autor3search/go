package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/autor3search/go/internal/verdict"
)

func TestBusyWarning(t *testing.T) {
	cases := []struct {
		name   string
		load   float64
		err    error
		numCPU int
		want   []string // Empty means no warning.
	}{
		{"above the CPU count", 17, nil, 10, []string{"17.00", "10 CPUs"}},
		{"at the CPU count", 10, nil, 10, nil},
		{"below the CPU count", 2, nil, 10, nil},
		{"no reading", 99, errors.New("no load"), 10, nil},
	}
	for _, c := range cases {
		got := busyWarning(c.load, c.err, c.numCPU)
		if len(c.want) == 0 && got != "" {
			t.Errorf("%s: got %q, want no warning", c.name, got)
		}
		if len(c.want) > 0 && got == "" {
			t.Errorf("%s: got no warning", c.name)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: %q does not contain %q", c.name, got, w)
			}
		}
	}
}

func TestEvalWarnsWhenTheMachineIsBusyAndKeepsTheVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("measures real benchmarks; skipped in -short")
	}
	env := setupRun(t)
	writeOptimizedWordCount(t, env.Root)
	commit(t, env.Root, "use strings.Builder")

	o := env.Options()
	o.ReadLoad = func() (float64, error) { return 1e6, nil }
	res, meas, err := Eval(context.Background(), o)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if res.Status != verdict.StatusKeep {
		t.Fatalf("status = %s (%s), want KEEP: a warning must not change the verdict", res.Status, res.Message)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "1000000.00") {
		t.Errorf("Warnings = %q, want the load named", res.Warnings)
	}
	if meas == nil || len(meas.Warnings) == 0 {
		t.Errorf("Measurements.Warnings is empty")
	}
}

func TestEvalIsQuietOnAQuietMachine(t *testing.T) {
	if testing.Short() {
		t.Skip("measures real benchmarks; skipped in -short")
	}
	env := setupRun(t)
	writeOptimizedWordCount(t, env.Root)
	commit(t, env.Root, "use strings.Builder")

	o := env.Options()
	o.ReadLoad = func() (float64, error) { return 0, nil }
	res, _, err := Eval(context.Background(), o)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s := strings.Join(res.Warnings, "\n"); strings.Contains(s, "busy") {
		t.Errorf("Warnings = %q, want no load warning", res.Warnings)
	}
}

func TestCompareWarnsWhenTheMachineIsBusy(t *testing.T) {
	if testing.Short() {
		t.Skip("measures real benchmarks; skipped in -short")
	}
	env := setupRun(t)
	writeOptimizedWordCount(t, env.Root)
	commit(t, env.Root, "use strings.Builder")

	o := env.Options()
	o.ReadLoad = func() (float64, error) { return 1e6, nil }
	meas, err := Compare(context.Background(), o)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !strings.Contains(strings.Join(meas.Warnings, "\n"), "1000000.00") {
		t.Errorf("Warnings = %q, want the load named", meas.Warnings)
	}
}
