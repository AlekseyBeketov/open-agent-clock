package redact

import "testing"

func TestTextRedactsCredentialLikeValues(t *testing.T) {
	value := Text(`access_token="secret-token" Authorization: Bearer abc.def`)
	if value == "" || containsSecret(value) {
		t.Fatalf("redaction failed: %q", value)
	}
}

func TestTextRedactsEnvironmentDiagnosticValues(t *testing.T) {
	value := Text("OPENAI_API_KEY=synthetic-openai-secret AWS_SECRET_ACCESS_KEY=synthetic-aws-secret AWS_SESSION_TOKEN=synthetic-session-secret")
	for _, secret := range []string{"synthetic-openai-secret", "synthetic-aws-secret", "synthetic-session-secret"} {
		if stringContains(value, secret) {
			t.Fatalf("environment diagnostic leaked a synthetic secret: %q", secret)
		}
	}
}

func TestArgumentsRedactsInlineAndSeparateCredentialValues(t *testing.T) {
	input := []string{"--api-key=synthetic-inline-secret", "--access-token", "synthetic-separate-secret", "--safe", "value"}
	result := Arguments(input)
	if result[0] != "--api-key=[REDACTED]" || result[2] != "[REDACTED]" {
		t.Fatalf("argument redaction result = %#v", result)
	}
	if input[0] != "--api-key=synthetic-inline-secret" || input[2] != "synthetic-separate-secret" {
		t.Fatal("argument redaction mutated execution arguments")
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
