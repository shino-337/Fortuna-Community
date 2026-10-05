package poddetail

import (
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

// sensitiveName matches flag and variable names whose value is a credential.
var sensitiveName = regexp.MustCompile(`(?i)(pass(word|wd)?|pwd|secret|token|api[-_]?key|access[-_]?key|private[-_]?key|credential|auth)`)

// urlUserinfo matches the password part of scheme://user:password@host.
var urlUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)[^\s@/]+@`)

// redactCommandLine hides credential values in a process command line before it
// leaves the node: --password=x, --token x, -p x (after a password-like
// command), PASSWORD=x and URLs with user:password@. The executable and the
// other arguments are kept so the process stays recognizable.
func redactCommandLine(cmdline string) string {
	if cmdline == "" {
		return cmdline
	}
	fields := strings.Fields(cmdline)
	for i := 0; i < len(fields); i++ {
		f := urlUserinfo.ReplaceAllString(fields[i], "${1}"+redacted+"@")
		if name, _, ok := strings.Cut(f, "="); ok && i > 0 && sensitiveName.MatchString(strings.TrimLeft(name, "-")) {
			f = name + "=" + redacted
		} else if strings.HasPrefix(f, "-") && sensitiveName.MatchString(strings.TrimLeft(f, "-")) && i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
			fields[i] = f
			fields[i+1] = redacted
			i++
			continue
		}
		fields[i] = f
	}
	return strings.Join(fields, " ")
}
