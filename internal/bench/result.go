package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

// Metric is one measured quantity of a benchmark.
type Metric struct {
	Name string `json:"name"`
	// Stats holds timing samples; Value holds a single number (with Unit).
	Stats  *Stats  `json:"stats,omitempty"`
	Value  float64 `json:"value,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Budget string  `json:"budget,omitempty"` // human text, for example "< 10ms p95"
	Pass   *bool   `json:"pass,omitempty"`   // nil when there is no budget to judge by
}

// Result is the outcome of one benchmark run.
type Result struct {
	Name    string    `json:"name"`
	At      time.Time `json:"at"`
	Machine Machine   `json:"machine"`
	Metrics []Metric  `json:"metrics"`
	Notes   []string  `json:"notes,omitempty"`
}

func newResult(name string) *Result {
	return &Result{Name: name, At: time.Now().UTC(), Machine: describeMachine()}
}

func (r *Result) addTiming(name string, samples []time.Duration, budget string, limit time.Duration, at func(Stats) float64) {
	st := summarize(samples)
	m := Metric{Name: name, Stats: &st, Budget: budget}
	if limit > 0 && st.N > 0 {
		ok := at(st) <= ms(limit)
		m.Pass = &ok
	}
	r.Metrics = append(r.Metrics, m)
}

func (r *Result) addValue(name string, v float64, unit string) {
	r.Metrics = append(r.Metrics, Metric{Name: name, Value: v, Unit: unit})
}

func (r *Result) note(format string, a ...any) { r.Notes = append(r.Notes, fmt.Sprintf(format, a...)) }

// Failed reports whether any metric with a budget missed it.
func (r *Result) Failed() bool {
	for _, m := range r.Metrics {
		if m.Pass != nil && !*m.Pass {
			return true
		}
	}
	return false
}

// WriteTable prints the result: timing metrics as a table of percentiles, and
// single values as name/value lines.
func (r *Result) WriteTable(w io.Writer) {
	fmt.Fprintf(w, "mpx bench %s  (%s/%s, %d CPUs, %s)\n", r.Name, r.Machine.OS, r.Machine.Arch, r.Machine.CPUs, r.Machine.GoVersion)
	var timings, values []Metric
	for _, m := range r.Metrics {
		if m.Stats != nil {
			timings = append(timings, m)
		} else {
			values = append(values, m)
		}
	}
	if len(timings) > 0 {
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "metric\tn\tmin\tp50\tp95\tp99\tmax\tbudget\t")
		for _, m := range timings {
			status := ""
			if m.Pass != nil {
				status = map[bool]string{true: "ok", false: "OVER"}[*m.Pass]
			}
			s := m.Stats
			fmt.Fprintf(tw, "%s\t%d\t%.2fms\t%.2fms\t%.2fms\t%.2fms\t%.2fms\t%s\t\n",
				m.Name, s.N, s.Min, s.P50, s.P95, s.P99, s.Max, strings.TrimSpace(m.Budget+" "+status))
		}
		_ = tw.Flush()
	}
	if len(values) > 0 {
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		for _, m := range values {
			fmt.Fprintf(tw, "%s\t%s\t\n", m.Name, strings.TrimSpace(fmt.Sprintf("%.2f %s", m.Value, m.Unit)))
		}
		_ = tw.Flush()
	}
	for _, n := range r.Notes {
		fmt.Fprintln(w, "note:", n)
	}
}

// WriteJSON prints the result as indented JSON.
func (r *Result) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// SaveTo writes the result as JSON to path. If path is an existing directory
// or ends in a separator, the file is <path>/<name>.json.
func (r *Result) SaveTo(path string) (string, error) {
	if st, err := os.Stat(path); (err == nil && st.IsDir()) || strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(os.PathSeparator)) {
		path = filepath.Join(path, r.Name+".json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return path, r.WriteJSON(f)
}
