package aiinterpretation

import "regexp"

var (
	emailRE        = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	koreanPhoneRE  = regexp.MustCompile(`\b01[016789]-?\d{3,4}-?\d{4}\b`)
	koreanRRNRE    = regexp.MustCompile(`\b\d{6}-[1-4]\d{6}\b`)
	ipv4RE         = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6RE         = regexp.MustCompile(`\b(?:[A-Fa-f0-9]{1,4}:){2,7}[A-Fa-f0-9]{1,4}\b|\b(?:[A-Fa-f0-9]{1,4}:){1,7}:(?:[A-Fa-f0-9]{1,4})?\b|\b(?:[A-Fa-f0-9]{1,4}:){1,6}:[A-Fa-f0-9]{1,4}\b`)
	urlHostRE      = regexp.MustCompile(`(?i)\b((?:https?|jdbc:[a-z0-9]+)://)([^/\s?#;]+)`)
	hostFieldRE    = regexp.MustCompile(`(?i)\b(host|hostname|server|node|pod|instance)\s*[:=]\s*([A-Za-z0-9][A-Za-z0-9.\-]*\.[A-Za-z]{2,})(?::\d+)?`)
	jwtRE          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	authHeaderRE   = regexp.MustCompile(`(?i)\bauthorization\s*:\s*(?:bearer|basic)\s+[A-Za-z0-9._~+/\-=]+`)
	bearerTokenRE  = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/\-=]+`)
	tokenRE        = regexp.MustCompile(`(?i)\b(token|api[_-]?key|secret|password|passwd|pwd|access[_-]?token|refresh[_-]?token)\s*([:=])\s*([^\s,;&]+)`)
	unixPathRE     = regexp.MustCompile(`(?:/Users|/home|/var|/opt|/tmp|/private|/srv)/[^\s:),]+(?:/[^\s:),]+)*`)
	windowsPathRE  = regexp.MustCompile(`(?i)\b[A-Z]:\\[^\s:),]+(?:\\[^\s:),]+)*`)
	sqlStatementRE = regexp.MustCompile(`(?i)\b(select|insert|update|delete|merge)\b[^\n;]*`)
	sqlLiteralRE   = regexp.MustCompile(`'([^']|'')*'|"([^"]|"")*"`)
)

func RedactSensitiveText(value string) string {
	value = emailRE.ReplaceAllString(value, "[redacted-email]")
	value = koreanPhoneRE.ReplaceAllString(value, "[redacted-phone]")
	value = koreanRRNRE.ReplaceAllString(value, "[redacted-rrn]")
	value = authHeaderRE.ReplaceAllString(value, "Authorization: [redacted-secret]")
	value = bearerTokenRE.ReplaceAllString(value, "Bearer [redacted-secret]")
	value = jwtRE.ReplaceAllString(value, "[redacted-jwt]")
	value = ipv4RE.ReplaceAllString(value, "[redacted-ip]")
	value = ipv6RE.ReplaceAllString(value, "[redacted-ip]")
	value = urlHostRE.ReplaceAllString(value, "${1}[redacted-host]")
	value = hostFieldRE.ReplaceAllString(value, "$1=[redacted-host]")
	value = tokenRE.ReplaceAllString(value, "$1$2[redacted-secret]")
	value = unixPathRE.ReplaceAllString(value, "[redacted-path]")
	value = windowsPathRE.ReplaceAllString(value, "[redacted-path]")
	value = redactSQLLiterals(value)
	return value
}

func redactSQLLiterals(value string) string {
	return sqlStatementRE.ReplaceAllStringFunc(value, func(statement string) string {
		return sqlLiteralRE.ReplaceAllString(statement, "[redacted-sql-literal]")
	})
}
