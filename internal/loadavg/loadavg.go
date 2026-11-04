// Package loadavg reads the machine's load average.
package loadavg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ErrUnavailable means this platform has no load average to read.
var ErrUnavailable = errors.New("load average not available on this platform")

// Parse returns the 1-minute load average from the text of /proc/loadavg or
// of `sysctl -n vm.loadavg`. The second one starts with a brace.
func Parse(s string) (float64, error) {
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(s), "{"))
	if len(fields) == 0 {
		return 0, fmt.Errorf("no load average in %q", s)
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("no load average in %q: %w", s, err)
	}
	return load, nil
}

// Read returns the current 1-minute load average.
func Read() (float64, error) {
	switch runtime.GOOS {
	case "linux":
		b, err := os.ReadFile("/proc/loadavg")
		if err != nil {
			return 0, err
		}
		return Parse(string(b))
	case "darwin":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// The absolute path works when PATH has no /usr/sbin. LC_ALL=C makes the
		// decimal mark a point in every locale.
		cmd := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "vm.loadavg")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		b, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		return Parse(string(b))
	}
	return 0, ErrUnavailable
}
