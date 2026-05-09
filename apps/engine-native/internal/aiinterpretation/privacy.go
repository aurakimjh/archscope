package aiinterpretation

import "regexp"

var (
	emailRE = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	ipv4RE  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	tokenRE = regexp.MustCompile(`(?i)\b(token|api[_-]?key|secret|password)=([^\s,;&]+)`)
)

func RedactSensitiveText(value string) string {
	value = emailRE.ReplaceAllString(value, "[redacted-email]")
	value = ipv4RE.ReplaceAllString(value, "[redacted-ip]")
	value = tokenRE.ReplaceAllString(value, "$1=[redacted-secret]")
	return value
}
