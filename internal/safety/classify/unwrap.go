package classify

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

// cmdName reduces a command word to the name rules match on: base name,
// lower-case, without a Windows executable extension.
func cmdName(w string) string {
	if i := strings.LastIndexAny(w, `/\`); i >= 0 {
		w = w[i+1:]
	}
	w = strings.ToLower(w)
	for _, ext := range []string{".exe", ".cmd", ".bat", ".com", ".ps1"} {
		if strings.HasSuffix(w, ext) {
			return strings.TrimSuffix(w, ext)
		}
	}
	return w
}

// isCmdlet reports whether a command name looks like a PowerShell cmdlet
// (Verb-Noun), whose parameters are case-insensitive.
func isCmdlet(name string) bool {
	i := strings.IndexByte(name, '-')
	return i > 0 && i < len(name)-1 && strings.IndexByte(name[i+1:], '-') < 0 && !strings.HasPrefix(name, "git-")
}

var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "ash": true,
	"fish": true, "csh": true, "tcsh": true, "mksh": true,
}

var psNames = map[string]bool{"pwsh": true, "powershell": true, "pwsh-preview": true}

// unwrapped is the result of looking through a wrapper command.
type unwrapped struct {
	rest  []string // the wrapped command's words (prefix wrappers)
	text  string   // a script to analyse (shell -c and friends)
	fl    flavour  // flavour of text
	isTxt bool
}

var (
	sudoValue = setOf("-u", "-g", "-h", "-p", "-C", "-D", "-r", "-t", "-T", "-U",
		"--user", "--group", "--host", "--prompt", "--chdir", "--role", "--type", "--close-from")
	envValue     = setOf("-u", "-C", "--unset", "--chdir")
	niceValue    = setOf("-n", "--adjustment")
	ionicValue   = setOf("-c", "-n", "-p", "-P", "-u")
	timeoutValue = setOf("-s", "-k", "--signal", "--kill-after")
	stdbufValue  = setOf("-i", "-o", "-e", "--input", "--output", "--error")
	timeValue    = setOf("-f", "-o", "--format", "--output")
	watchValue   = setOf("-n", "--interval", "-w")
	execValue    = setOf("-a")
	wslValue     = setOf("-d", "-u", "--distribution", "--user", "--cd")
	xargsValue   = setOf("-I", "-n", "-P", "-d", "-L", "-s", "-E", "-a",
		"--max-args", "--max-procs", "--delimiter", "--max-lines", "--max-chars", "--eof", "--arg-file", "--replace")
	sshValue = setOf("-p", "-i", "-l", "-o", "-F", "-J", "-L", "-R", "-D", "-b", "-c", "-E", "-e",
		"-I", "-m", "-O", "-Q", "-S", "-W", "-w", "-B")
	psValue = setOf("executionpolicy", "ep", "windowstyle", "w", "workingdirectory", "wd",
		"configurationname", "config", "inputformat", "outputformat", "version", "settingsfile", "custompipename")
)

func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

// skipOpts drops leading options (and the values of those in withValue).
func skipOpts(args []string, withValue map[string]bool) []string {
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) > 1 && a[0] == '-' {
			if withValue[a] {
				i++
			}
			i++
			continue
		}
		break
	}
	if i > len(args) {
		i = len(args)
	}
	return args[i:]
}

func dropAssignments(w []string) []string {
	for len(w) > 0 && isAssignment(w[0]) {
		w = w[1:]
	}
	return w
}

func isAssignment(w string) bool {
	i := strings.IndexByte(w, '=')
	if i <= 0 {
		return false
	}
	name := strings.TrimSuffix(w[:i], "+")
	for j := 0; j < len(name); j++ {
		c := name[j]
		if !(isAlpha(c) || c == '_' || (j > 0 && c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// unwrap looks through one wrapper layer. name is cmdName(w[0]).
func unwrap(name string, w []string, fl flavour) (unwrapped, bool) {
	args := w[1:]
	switch {
	case shellNames[name]:
		if t, ok := shellDashC(args); ok {
			return unwrapped{text: t, fl: flBash, isTxt: true}, true
		}
	case psNames[name]:
		if t, ok := powershellCommand(args); ok {
			return unwrapped{text: t, fl: flPowerShell, isTxt: true}, true
		}
	case name == "cmd":
		if t, ok := cmdSlashC(args); ok {
			return unwrapped{text: t, fl: flCmd, isTxt: true}, true
		}
	case name == "eval":
		if len(args) > 0 {
			return unwrapped{text: strings.Join(args, " "), fl: fl, isTxt: true}, true
		}
	case name == "iex" || name == "invoke-expression":
		if len(args) > 0 {
			return unwrapped{text: strings.Join(args, " "), fl: flPowerShell, isTxt: true}, true
		}
	case name == "su":
		for i, a := range args {
			if (a == "-c" || a == "--command") && i+1 < len(args) {
				return unwrapped{text: args[i+1], fl: flBash, isTxt: true}, true
			}
		}
	case name == "ssh":
		rest := skipOpts(args, sshValue)
		if len(rest) > 1 { // host, then the remote command
			return unwrapped{text: strings.Join(rest[1:], " "), fl: flBash, isTxt: true}, true
		}
	case name == "sudo" || name == "doas" || name == "pkexec":
		rest := skipOpts(args, sudoValue)
		return unwrapped{rest: dropAssignments(rest)}, true
	case name == "env":
		rest := args
		for i := 0; i < len(rest); i++ {
			if rest[i] == "-S" || rest[i] == "--split-string" {
				if i+1 < len(rest) {
					return unwrapped{text: rest[i+1], fl: flBash, isTxt: true}, true
				}
			}
		}
		return unwrapped{rest: dropAssignments(skipOpts(args, envValue))}, true
	case name == "command":
		for _, a := range args {
			if a == "-v" || a == "-V" {
				return unwrapped{}, true
			}
		}
		return unwrapped{rest: skipOpts(args, nil)}, true
	case name == "builtin", name == "nohup", name == "setsid", name == "unbuffer", name == "caffeinate", name == "call":
		return unwrapped{rest: skipOpts(args, nil)}, true
	case name == "exec":
		return unwrapped{rest: skipOpts(args, execValue)}, true
	case name == "nice":
		return unwrapped{rest: skipOpts(args, niceValue)}, true
	case name == "ionice":
		return unwrapped{rest: skipOpts(args, ionicValue)}, true
	case name == "time":
		return unwrapped{rest: skipOpts(args, timeValue)}, true
	case name == "stdbuf":
		return unwrapped{rest: skipOpts(args, stdbufValue)}, true
	case name == "watch":
		return unwrapped{rest: skipOpts(args, watchValue)}, true
	case name == "wsl":
		return unwrapped{rest: skipOpts(args, wslValue)}, true
	case name == "timeout":
		rest := skipOpts(args, timeoutValue)
		if len(rest) > 0 { // the duration
			rest = rest[1:]
		}
		return unwrapped{rest: rest}, true
	case name == "xargs":
		return unwrapped{rest: skipOpts(args, xargsValue)}, true
	case name == "npx" || name == "pnpx" || name == "bunx":
		return unwrapped{rest: skipOpts(args, setOf("-p", "--package", "-c", "--call"))}, true
	case (name == "pnpm" || name == "yarn" || name == "npm" || name == "bun") && len(args) > 0 && (args[0] == "dlx" || args[0] == "exec" || args[0] == "x"):
		return unwrapped{rest: skipOpts(args[1:], setOf("-p", "--package", "-c", "--call"))}, true
	case name == "start" && fl == flCmd:
		rest := args
		for len(rest) > 0 && (strings.HasPrefix(rest[0], "/") || rest[0] == "") {
			rest = rest[1:]
		}
		return unwrapped{rest: rest}, true
	}
	return unwrapped{}, false
}

// shellDashC finds the script given to "sh -c script".
func shellDashC(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return "", false
		case strings.HasPrefix(a, "--"):
			if a == "--rcfile" || a == "--init-file" {
				i++
			}
		case len(a) > 1 && (a[0] == '-' || a[0] == '+'):
			if a[0] == '-' && strings.Contains(a[1:], "c") {
				if i+1 < len(args) {
					return args[i+1], true
				}
				return "", false
			}
			if a == "-o" || a == "+o" || a == "-O" || a == "+O" {
				i++
			}
		default:
			return "", false // a script file
		}
	}
	return "", false
}

// hasScriptArg reports whether a shell invocation names a script file, which
// means it does not read its script from standard input.
func hasScriptArg(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 1 && (a[0] == '-' || a[0] == '+') {
			if a == "-o" || a == "+o" {
				i++
			}
			continue
		}
		return true
	}
	return false
}

// powershellCommand extracts the script from powershell/pwsh arguments:
// -Command (any unambiguous prefix), -EncodedCommand, or a bare string.
func powershellCommand(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		a := strings.ToLower(args[i])
		if a == "" || (a[0] != '-' && a[0] != '/') {
			return strings.Join(args[i:], " "), true
		}
		opt := strings.TrimLeft(a, "-/")
		if j := strings.IndexAny(opt, ":="); j >= 0 { // -Command:"..." style
			val := args[i][len(args[i])-len(opt)+j+1:]
			opt = opt[:j]
			if opt != "" && strings.HasPrefix("command", opt) {
				return strings.TrimSpace(val + " " + strings.Join(args[i+1:], " ")), true
			}
			continue
		}
		switch {
		case opt == "":
		case psValue[opt]:
			i++
		case opt == "e" || opt == "ec" || (len(opt) >= 2 && strings.HasPrefix("encodedcommand", opt)):
			if i+1 < len(args) {
				if t, ok := decodeEncodedCommand(args[i+1]); ok {
					return t, true
				}
			}
			return "", false
		case strings.HasPrefix("command", opt):
			return strings.Join(args[i+1:], " "), true
		case strings.HasPrefix("file", opt):
			return "", false
		}
	}
	return "", false
}

func decodeEncodedCommand(s string) (string, bool) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(s)
		if err != nil {
			return "", false
		}
	}
	if len(b)%2 != 0 {
		return "", false
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u)), true
}

// cmdSlashC extracts the command from "cmd /c ...".
func cmdSlashC(args []string) (string, bool) {
	for i, a := range args {
		la := strings.ToLower(a)
		if la == "/c" || la == "/k" || la == "/r" {
			return strings.Join(args[i+1:], " "), true
		}
		if len(la) > 2 && (strings.HasPrefix(la, "/c") || strings.HasPrefix(la, "/k")) && !strings.ContainsRune(la[2:], ':') {
			return strings.TrimSpace(a[2:] + " " + strings.Join(args[i+1:], " ")), true
		}
	}
	return "", false
}
