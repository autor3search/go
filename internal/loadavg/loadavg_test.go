package loadavg

import (
	"errors"
	"runtime"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name, in string
		want     float64
	}{
		{"proc", "1.50 2.00 3.00 1/234 5678\n", 1.5},
		{"sysctl", "{ 1.85 3.74 4.68 }\n", 1.85},
		{"zero", "{ 0.00 0.00 0.00 }\n", 0},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil || got != c.want {
			t.Errorf("%s: Parse(%q) = %v, %v, want %v", c.name, c.in, got, err, c.want)
		}
	}
}

func TestParseRejectsTextWithNoNumber(t *testing.T) {
	for _, in := range []string{"", "\n", "{ }", "abc 1.0", "{ abc }"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %v, want an error", in, got)
		}
	}
}

func TestReadOnThisPlatform(t *testing.T) {
	got, err := Read()
	switch runtime.GOOS {
	case "linux", "darwin":
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if got < 0 {
			t.Errorf("Read = %v, want at least 0", got)
		}
	default:
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("Read error = %v, want ErrUnavailable", err)
		}
	}
}
