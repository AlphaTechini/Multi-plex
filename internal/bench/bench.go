package bench

import (
	"flag"
	"fmt"
	"io"
)

const usage = `usage: mpx bench <startup|hook|render> [flags]

  startup   time 'mpx version' and the TUI's first frame (cold and warm)
  hook      round trips of an allowed pre-tool hook, in-process and as 'mpx hook' processes
  render    replay a generated terminal stream through the emulator

common flags:
  --json        print the result as JSON instead of a table
  --out PATH    also write the result as JSON (a directory gets <name>.json)
  --strict      exit 1 when a measurement is over its budget (by default budgets are only reported)
`

// common holds the flags every benchmark takes.
type common struct {
	json   bool
	out    string
	strict bool
}

func (c *common) register(fs *flag.FlagSet) {
	fs.BoolVar(&c.json, "json", false, "print JSON instead of a table")
	fs.StringVar(&c.out, "out", "", "write the result as JSON to this file or directory (for example evidence/bench/)")
	fs.BoolVar(&c.strict, "strict", false, "exit 1 when a measurement is over its budget")
}

// Run executes `mpx bench`. args are the arguments after "bench". It returns
// the process exit code: 0 on success, 1 when a measurement could not be made
// (or, with --strict, missed its budget), 2 for bad usage.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var (
		res *Result
		err error
		c   common
	)
	fs := flag.NewFlagSet("bench "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	c.register(fs)
	switch args[0] {
	case "startup":
		o := startupFlags(fs)
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		res, err = runStartup(o)
	case "hook":
		o := hookFlags(fs)
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		res, err = runHook(o)
	case "render":
		o := renderFlags(fs)
		if fs.Parse(args[1:]) != nil {
			return 2
		}
		res, err = runRender(o)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "mpx bench: unknown benchmark %q\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "mpx bench %s: %v\n", args[0], err)
		return 1
	}
	if c.json {
		if err := res.WriteJSON(stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	} else {
		res.WriteTable(stdout)
	}
	if c.out != "" {
		path, err := res.SaveTo(c.out)
		if err != nil {
			fmt.Fprintf(stderr, "mpx bench: write result: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "wrote %s\n", path)
	}
	if c.strict && res.Failed() {
		return 1
	}
	return 0
}
