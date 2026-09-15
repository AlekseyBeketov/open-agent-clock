package redact

import (
	"regexp"
	"strings"
)

var sensitivePatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)(access[_-]?token|refresh[_-]?token|api[_-]?key|secret[_-]?access[_-]?key|session[_-]?token)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^,\s}\"]+)`), `$1$2[REDACTED]`},
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^,\s}\"]+`), `$1[REDACTED]`},
	{regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`), `Bearer [REDACTED]`},
}

func Text(value string) string {
	for _, item := range sensitivePatterns {
		value = item.pattern.ReplaceAllString(value, item.replace)
	}
	return value
}

// Arguments returns a display-safe copy of command-line arguments. It never
// mutates the arguments used for provider execution.
func Arguments(values []string) []string {
	result := append([]string(nil), values...)
	for index := 0; index < len(result); index++ {
		result[index] = Text(result[index])
		if isSensitiveFlag(values[index]) && !strings.Contains(values[index], "=") && index+1 < len(result) {
			result[index+1] = "[REDACTED]"
			index++
		}
	}
	return result
}

func isSensitiveFlag(value string) bool {
	if !strings.HasPrefix(value, "-") {
		return false
	}
	switch strings.ToLower(strings.TrimLeft(value, "-")) {
	case "access-token", "access_token", "refresh-token", "refresh_token", "api-key", "api_key", "authorization":
		return true
	default:
		return false
	}
}
