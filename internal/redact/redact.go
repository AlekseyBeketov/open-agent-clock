package redact

import "regexp"

var sensitivePatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)(access[_-]?token|refresh[_-]?token|api[_-]?key)(\s*[=:]\s*)("[^"]*"|[^,\s}\"]+)`), `$1$2[REDACTED]`},
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^,\s}\"]+`), `$1[REDACTED]`},
	{regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`), `Bearer [REDACTED]`},
}

func Text(value string) string {
	for _, item := range sensitivePatterns {
		value = item.pattern.ReplaceAllString(value, item.replace)
	}
	return value
}
