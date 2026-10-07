// Package redact hides credential values in text that leaves the node: process command lines,
// runtime sensor output and event targets. It keeps everything else so the process or event
// stays recognizable.
package redact

import (
	"path"
	"regexp"
	"strings"
)

// Marker replaces a hidden value.
const Marker = "[REDACTED]"

// secretWord matches names whose value is a credential.
const secretWord = `pass(?:word|wd|phrase)?|pwd|secret|token|api[-_]?key|access[-_]?key|private[-_]?key|credential|auth`

var (
	sensitiveName = regexp.MustCompile(`(?i)(` + secretWord + `)`)

	// scheme://user:password@host
	urlUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)[^\s@/]+@`)

	// Authorization: Bearer x, X-Api-Key: x, Cookie: x (curl -H, wget --header, logged requests).
	secretHeader = regexp.MustCompile(`(?i)\b((?:proxy-)?authorization|x-api-key|x-auth-token|api-key|cookie)(\s*:\s*)[^"'\r\n]+`)

	// Bearer x outside a header.
	bearer = regexp.MustCompile(`(?i)\b(bearer|basic)(\s+)[A-Za-z0-9._~+/=-]{8,}`)

	// "password": "x" in JSON arguments (curl -d '{"password":"x"}'), with plain or escaped quotes.
	jsonSecret        = regexp.MustCompile(`(?i)("[^"\s]*(?:` + secretWord + `)[^"\s]*"\s*:\s*")[^"]*(")`)
	escapedJSONSecret = regexp.MustCompile(`(?i)(\\"[^"\s\\]*(?:` + secretWord + `)[^"\s\\]*\\"\s*:\s*\\")(?:[^"\\]|\\[^"])*(\\")`)

	// password=x inside a query string or form body (a=1&password=x), not only at the start of a word.
	formSecret = regexp.MustCompile(`(?i)([?&]\w*(?:` + secretWord + `)\w*=)[^&\s"']+`)
)

// mysqlFamily takes the password glued to -p (mysql -pS3cret).
var mysqlFamily = map[string]bool{
	"mysql": true, "mysqldump": true, "mysqladmin": true, "mysqlimport": true, "mysqlshow": true,
	"mysqlcheck": true, "mariadb": true, "mariadb-dump": true, "mariadb-admin": true,
}

// userFlags take user:password as their value (curl -u, wget --user).
var userFlags = map[string]bool{"-u": true, "--user": true, "--proxy-user": true, "-U": true}

// CommandLine hides credential values in a command line: --password=x, --token x,
// PASSWORD=x, mysql -px, curl -u user:x, -H "Authorization: Bearer x", JSON and form
// bodies with a password field, and URLs with user:password@.
func CommandLine(cmdline string) string {
	if cmdline == "" {
		return cmdline
	}
	s := Text(cmdline)
	fields := strings.Fields(s)
	glued := len(fields) > 0 && mysqlFamily[path.Base(fields[0])]
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if glued && strings.HasPrefix(f, "-p") && len(f) > 2 && f != "-p"+Marker {
			fields[i] = "-p" + Marker
			continue
		}
		if userFlags[f] && i+1 < len(fields) {
			if user, _, ok := strings.Cut(fields[i+1], ":"); ok {
				fields[i+1] = user + ":" + Marker
				i++
				continue
			}
		}
		if name, _, ok := strings.Cut(f, "="); ok && i > 0 && sensitiveName.MatchString(strings.TrimLeft(name, "-")) {
			fields[i] = name + "=" + Marker
		} else if strings.HasPrefix(f, "-") && sensitiveName.MatchString(strings.TrimLeft(f, "-")) && i+1 < len(fields) && !strings.HasPrefix(fields[i+1], "-") {
			fields[i+1] = Marker
			i++
		}
	}
	return strings.Join(fields, " ")
}

// Text hides credential values inside free text such as a sensor's alert output, where a
// command line is embedded among other fields. It does not split on words, so it never
// treats the text's own words as flags.
func Text(s string) string {
	if s == "" {
		return s
	}
	s = urlUserinfo.ReplaceAllString(s, "${1}"+Marker+"@")
	s = secretHeader.ReplaceAllString(s, "${1}${2}"+Marker)
	s = bearer.ReplaceAllString(s, "${1}${2}"+Marker)
	s = jsonSecret.ReplaceAllString(s, "${1}"+Marker+"${2}")
	s = escapedJSONSecret.ReplaceAllString(s, "${1}"+Marker+"${2}")
	s = formSecret.ReplaceAllString(s, "${1}"+Marker)
	return s
}
