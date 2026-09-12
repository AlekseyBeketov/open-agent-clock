package redact

import "testing"

func TestTextRedactsCredentialLikeValues(t *testing.T) {
	value := Text(`access_token="secret-token" Authorization: Bearer abc.def`)
	if value == "" || containsSecret(value) {
		t.Fatalf("redaction failed: %q", value)
	}
}

func containsSecret(value string) bool {
	for _, secret := range []string{"secret-token", "abc.def"} {
		if len(secret) > 0 && stringContains(value, secret) {
			return true
		}
	}
	return false
}

func stringContains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
