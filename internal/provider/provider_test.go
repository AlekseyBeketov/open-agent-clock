package provider

import (
	"strings"
	"testing"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	runner "github.com/AlekseyBeketov/open-agent-clock/internal/exec"
)

func TestClassifyUsageLimitWithoutRetry(t *testing.T) {
	code := 1
	status, reason := (cliAdapter{}).Classify(&code, false, "", "rate limit reached")
	if status != StatusUsageLimited || reason == "" {
		t.Fatalf("status=%q reason=%q", status, reason)
	}
}

func TestClassifySuccessDoesNotClaimReset(t *testing.T) {
	code := 0
	status, reason := (cliAdapter{}).Classify(&code, false, "ok", "")
	if status != StatusSuccess {
		t.Fatalf("status=%q", status)
	}
	if reason == "" || reason == "reset" {
		t.Fatalf("unsafe success reason=%q", reason)
	}
}

func TestParseUsageAcceptsProviderReportedStructuredEnvelope(t *testing.T) {
	usage := ParseUsage(`{"type":"turn.completed","usage":{"input_tokens":11,"cached_input_tokens":5,"output_tokens":7,"total_tokens":18}}`)
	if usage.Availability != domain.TokenUsageAvailable || usage.Source != "provider-reported" {
		t.Fatalf("usage availability/source = %+v", usage)
	}
	if usage.InputTokens != 11 || usage.CachedInputTokens != 5 || usage.OutputTokens != 7 || usage.TotalTokens != 18 {
		t.Fatalf("usage counters = %+v", usage)
	}
}

func TestParseUsageReturnsUnavailableWithoutStructuredMetadata(t *testing.T) {
	usage := ParseUsage("prompt had 11 words and response had 7 words")
	if usage.Availability != domain.TokenUsageUnavailable {
		t.Fatalf("usage availability = %q", usage.Availability)
	}
	if usage.InputTokens != 0 || usage.OutputTokens != 0 || usage.TotalTokens != 0 {
		t.Fatalf("unavailable usage contained counters = %+v", usage)
	}
}

func TestObserveUsesStableDiagnosticCategoriesAndNeverCopiesProviderOutput(t *testing.T) {
	adapter := cliAdapter{provider: "openai-codex"}
	cases := []struct {
		name     string
		output   string
		category domain.DiagnosticCategory
	}{
		{"auth", "authentication required: token expired", domain.DiagnosticAuth},
		{"quota", "quota exceeded: rate limit reached", domain.DiagnosticQuota},
		{"network", "network connection refused", domain.DiagnosticNetwork},
		{"arguments", "unknown option: --not-a-real-flag", domain.DiagnosticArguments},
		{"provider", "provider internal failure authorization=synthetic-secret", domain.DiagnosticProvider},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			code := 1
			observation := adapter.Observe(runner.Result{ExitCode: &code, Stderr: testCase.output})
			if observation.Diagnostic == nil || observation.Diagnostic.Category != testCase.category {
				t.Fatalf("observation = %+v", observation)
			}
			if strings.Contains(observation.Diagnostic.Detail, "synthetic-secret") || strings.Contains(observation.Diagnostic.Detail, testCase.output) {
				t.Fatalf("diagnostic copied provider output: %+v", observation.Diagnostic)
			}
			if len(observation.Diagnostic.Detail) > diagnosticDetailLimit {
				t.Fatalf("diagnostic detail exceeded bound: %d", len(observation.Diagnostic.Detail))
			}
		})
	}
}
