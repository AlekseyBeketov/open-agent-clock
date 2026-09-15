package provider

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	runner "github.com/AlekseyBeketov/open-agent-clock/internal/exec"
	"github.com/AlekseyBeketov/open-agent-clock/internal/plan"
)

type Status string

const (
	StatusSuccess        Status = "success"
	StatusProviderFailed Status = "provider-failed"
	StatusTimeout        Status = "timeout"
	StatusUsageLimited   Status = "usage-limited"
	StatusUnavailable    Status = "unavailable"
	StatusSkipped        Status = "skipped"
)

// Observation is the adapter-owned interpretation of one bounded provider
// process result. Stdout and stderr never cross this boundary into persistence.
type Observation struct {
	Status     Status
	Reason     string
	Usage      *domain.TokenUsage
	Diagnostic *domain.Diagnostic
}

type Adapter interface {
	BuildPlan(binding domain.Binding, prompt string) (domain.ExecutionPlan, error)
	Classify(exitCode *int, timedOut bool, stdout, stderr string) (Status, string)
	Observe(execution runner.Result) Observation
}

type cliAdapter struct {
	provider string
}

func ForBinding(binding domain.Binding) (Adapter, error) {
	switch binding.Backend {
	case domain.NativeCodexCLI, domain.HermesCLI:
		return cliAdapter{provider: binding.Provider}, nil
	default:
		return nil, fmt.Errorf("unsupported provider backend %q", binding.Backend)
	}
}

func (adapter cliAdapter) BuildPlan(binding domain.Binding, prompt string) (domain.ExecutionPlan, error) {
	return plan.ForBinding(binding, prompt)
}

// BuildPlanForMode keeps the Adapter interface stable while allowing the
// explicit dev path to request provider JSONL telemetry.
func (adapter cliAdapter) BuildPlanForMode(binding domain.Binding, prompt string, devMode bool) (domain.ExecutionPlan, error) {
	return plan.ForBindingWithMode(binding, prompt, devMode)
}

func (adapter cliAdapter) Observe(execution runner.Result) Observation {
	status, reason := adapter.Classify(execution.ExitCode, execution.TimedOut, execution.Stdout, execution.Stderr)
	return Observation{
		Status:     status,
		Reason:     reason,
		Usage:      ParseUsage(execution.Stdout, execution.Stderr),
		Diagnostic: diagnosticFor(status, execution.ExitCode, execution.TimedOut, execution.Stdout, execution.Stderr),
	}
}

func (adapter cliAdapter) Classify(exitCode *int, timedOut bool, stdout, stderr string) (Status, string) {
	if timedOut {
		return StatusTimeout, "provider process exceeded the configured timeout"
	}
	combined := strings.ToLower(stdout + "\n" + stderr)
	for _, marker := range []string{"rate limit", "usage limit", "quota", "too many requests", "limit reached"} {
		if strings.Contains(combined, marker) {
			return StatusUsageLimited, "provider reported a usage or rate limit; no automatic retry was performed"
		}
	}
	if exitCode != nil && *exitCode == 0 {
		return StatusSuccess, "provider exited successfully; server-side usage-window behavior is not inferred"
	}
	if exitCode == nil {
		return StatusProviderFailed, "provider process did not return an exit code"
	}
	return StatusProviderFailed, fmt.Sprintf("provider exited with code %d", *exitCode)
}

const diagnosticDetailLimit = 256

func diagnosticFor(status Status, exitCode *int, timedOut bool, stdout, stderr string) *domain.Diagnostic {
	category := domain.DiagnosticUnknown
	switch {
	case timedOut:
		category = domain.DiagnosticNetwork
	case status == StatusUsageLimited:
		category = domain.DiagnosticQuota
	case containsAny(stdout, stderr, "unauthorized", "authentication", "not logged in", "login required", "token expired", "oauth", "401"):
		category = domain.DiagnosticAuth
	case containsAny(stdout, stderr, "invalid argument", "unknown option", "unrecognized option", "invalid flag", "usage:"):
		category = domain.DiagnosticArguments
	case containsAny(stdout, stderr, "network", "connection", "connect", "dns", "socket", "timed out"):
		category = domain.DiagnosticNetwork
	case status == StatusSuccess || status == StatusSkipped:
		return nil
	case status == StatusProviderFailed:
		category = domain.DiagnosticProvider
	}

	detail := diagnosticDetail(category, exitCode, timedOut)
	if len(detail) > diagnosticDetailLimit {
		detail = detail[:diagnosticDetailLimit]
	}
	return &domain.Diagnostic{Category: category, Detail: detail}
}

func diagnosticDetail(category domain.DiagnosticCategory, exitCode *int, timedOut bool) string {
	switch category {
	case domain.DiagnosticAuth:
		return "provider authentication was rejected or is unavailable"
	case domain.DiagnosticQuota:
		return "provider reported a usage or rate limit; inspect the provider usage surface"
	case domain.DiagnosticNetwork:
		if timedOut {
			return "provider process exceeded the configured timeout"
		}
		return "provider could not complete a network request"
	case domain.DiagnosticArguments:
		return "provider rejected the invocation arguments"
	case domain.DiagnosticProvider:
		if exitCode != nil {
			return "provider process failed with exit code " + strconv.Itoa(*exitCode)
		}
		return "provider process failed without an exit code"
	default:
		return "provider failure could not be classified"
	}
}

func containsAny(stdout, stderr string, markers ...string) bool {
	combined := strings.ToLower(stdout + "\n" + stderr)
	for _, marker := range markers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

// ParseUsage accepts only structured JSON objects containing a usage or
// token_usage envelope. Textual numbers, prompt lengths, and estimates are
// intentionally ignored.
func ParseUsage(outputs ...string) *domain.TokenUsage {
	for _, output := range outputs {
		if usage, ok := parseUsageOutput(output); ok {
			return usage
		}
	}
	return domain.UnavailableTokenUsage()
}

func parseUsageOutput(output string) (*domain.TokenUsage, bool) {
	candidates := []string{strings.TrimSpace(output)}
	candidates = append(candidates, splitJSONLines(output)...)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(candidate), &value); err != nil {
			continue
		}
		if usage, ok := findUsage(value); ok {
			return usage, true
		}
	}
	return nil, false
}

func splitJSONLines(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func findUsage(value any) (*domain.TokenUsage, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
			if normalized == "usage" || normalized == "tokenusage" {
				if usage, ok := usageFromMap(nested); ok {
					return usage, true
				}
			}
		}
		for _, nested := range typed {
			if usage, ok := findUsage(nested); ok {
				return usage, true
			}
		}
	case []any:
		for _, nested := range typed {
			if usage, ok := findUsage(nested); ok {
				return usage, true
			}
		}
	}
	return nil, false
}

func usageFromMap(value any) (*domain.TokenUsage, bool) {
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	var usage domain.TokenUsage
	usage.Availability = domain.TokenUsageAvailable
	usage.Source = "provider-reported"
	found := false
	for key, raw := range fields {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		counter, valid := tokenCounter(raw)
		if !valid {
			continue
		}
		switch normalized {
		case "inputtokens", "prompttokens", "input":
			usage.InputTokens, found = counter, true
		case "cachedinputtokens", "cachedinput":
			usage.CachedInputTokens, found = counter, true
		case "outputtokens", "completiontokens", "output":
			usage.OutputTokens, found = counter, true
		case "totaltokens", "total":
			usage.TotalTokens, found = counter, true
		}
	}
	if !found {
		return nil, false
	}
	return &usage, true
}

func tokenCounter(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		if typed >= 0 && typed <= math.MaxInt64 && typed == math.Trunc(typed) {
			return int64(typed), true
		}
	case json.Number:
		counter, err := strconv.ParseInt(string(typed), 10, 64)
		if err == nil && counter >= 0 {
			return counter, true
		}
	case int64:
		if typed >= 0 {
			return typed, true
		}
	}
	return 0, false
}
