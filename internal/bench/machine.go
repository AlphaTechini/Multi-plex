package bench

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

// Machine describes where a benchmark ran, so results can be compared.
type Machine struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	CPUModel  string `json:"cpu_model,omitempty"`
	GoVersion string `json:"go_version"`
	Term      string `json:"term,omitempty"`
}

// describeMachine reads the current machine's description.
func describeMachine() Machine {
	return Machine{
		OS: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(),
		CPUModel: cpuModel(), GoVersion: runtime.Version(),
		Term: os.Getenv("TERM_PROGRAM") + termSuffix(),
	}
}

func termSuffix() string {
	if t := os.Getenv("TERM"); t != "" {
		return "/" + t
	}
	return ""
}

// cpuModel returns the CPU model name where it is cheap to read (Linux).
func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
