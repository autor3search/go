package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autor3search/go/internal/bench"
	"github.com/autor3search/go/internal/config"
	"github.com/autor3search/go/internal/pipeline"
)

func TestPrintComparisonListsSlowdownsFirstAndMarksTheStrongOnes(t *testing.T) {
	d := func(name string, pct, p float64) bench.Delta {
		return bench.Delta{Name: name, Ratio: 1 + pct/100, PctChange: pct, P: p, Alpha: 0.05, Significant: p < 0.05}
	}
	meas := &pipeline.Measurements{
		Time: []bench.Delta{
			d("BenchmarkWeak", 9, 0.04),
			d("BenchmarkStrong", 2, 0.001),
			d("BenchmarkFast", -30, 0.001),
			d("BenchmarkNoise", 50, 0.4),
		},
		AllocsAppeared: []bench.Appeared{{Name: "BenchmarkNew", CandCenter: 2}},
	}

	var code int
	out := captureStdout(t, func() { code = printComparison(meas) })

	if code != exitOK {
		t.Errorf("code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "slower: 2 of 4") || !strings.Contains(out, "faster: 1 of 4") {
		t.Errorf("wrong counts:\n%s", out)
	}
	weak, strong, fast := strings.Index(out, "BenchmarkWeak"), strings.Index(out, "BenchmarkStrong"), strings.Index(out, "BenchmarkFast")
	if weak < 0 || strong < weak || fast < strong {
		t.Errorf("want the largest slowdown first and the wins last:\n%s", out)
	}
	if strings.Contains(out, "BenchmarkNoise") {
		t.Errorf("a non-significant change is listed:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "BenchmarkStrong") && !strings.Contains(line, "*") {
			t.Errorf("p=0.001 passes alpha/4, want a mark: %q", line)
		}
		if strings.Contains(line, "BenchmarkWeak") && strings.Contains(line, "*") {
			t.Errorf("p=0.04 does not pass alpha/4, want no mark: %q", line)
		}
	}
	if !strings.Contains(out, "allocs/op went from 0 to 2  BenchmarkNew") {
		t.Errorf("the new allocation is not listed:\n%s", out)
	}
	if !strings.Contains(out, "Other inputs can be slower") {
		t.Errorf("the limit is not stated:\n%s", out)
	}
}

func TestCompareRefusesACountTooSmallToCompare(t *testing.T) {
	if code := runCompare([]string{"-count", "1"}); code != exitUsage {
		t.Errorf("runCompare = %d, want %d", code, exitUsage)
	}
}

func TestCompareRefusesWhenItHasNothingToMeasure(t *testing.T) {
	dir := copyDemoRepo(t)
	mustInit(t, dir)
	if code := runBaseline([]string{"-C", dir, "-tag", "cmp"}); code != exitOK {
		t.Fatalf("runBaseline = %d", code)
	}

	if code := runCompare([]string{"-C", dir}); code != exitUsage {
		t.Errorf("HEAD at the baseline: runCompare = %d, want %d", code, exitUsage)
	}

	srcPath := filepath.Join(dir, "wordcount.go")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, append(src, []byte("\n// uncommitted\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runCompare([]string{"-C", dir}); code != exitUsage {
		t.Errorf("dirty tree: runCompare = %d, want %d", code, exitUsage)
	}
}

// This is the case an eval cannot see: the baseline is the run's start, even
// though no KEEP advanced anything here, and the slowdown must be measured
// against it. The worktree must not stay behind, and nothing must be recorded.
func TestCompareMeasuresHeadAgainstTheRunStart(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real benchmarks; skipped in -short")
	}
	dir := copyDemoRepo(t)
	mustInit(t, dir)
	configPath := filepath.Join(dir, config.Path)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Count = 8
	cfg.Benchtime = "50ms"
	if err := os.WriteFile(configPath, renderConfig(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "shrink count/benchtime for a fast test")
	if code := runBaseline([]string{"-C", dir, "-tag", "cmp"}); code != exitOK {
		t.Fatalf("runBaseline = %d", code)
	}

	srcPath := filepath.Join(dir, "wordcount.go")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(src), "func CountWords(s string) map[string]int {",
		"func CountWords(s string) map[string]int {\n\tfor i := 0; i < 40; i++ {\n\t\tcountWords(s)\n\t}\n\treturn countWords(s)\n}\n\nfunc countWords(s string) map[string]int {", 1)
	if err := os.WriteFile(srcPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "much slower")

	var code int
	out := captureStdout(t, func() { code = runCompare([]string{"-C", dir}) })
	if code != exitOK {
		t.Fatalf("runCompare = %d, want %d\n%s", code, exitOK, out)
	}
	if strings.Contains(out, "slower: 0 of") || !strings.Contains(out, "*") {
		t.Errorf("want the slowdown listed and marked:\n%s", out)
	}
	if list := runGit(t, dir, "worktree", "list"); strings.Contains(list, "compare-worktree") {
		t.Errorf("the temporary worktree stayed:\n%s", list)
	}
	if status := runGit(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("compare changed the tree:\n%s", status)
	}
}
