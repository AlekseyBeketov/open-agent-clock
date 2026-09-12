package provider

import (
	"fmt"
	"strings"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
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

type Adapter interface {
	BuildPlan(binding domain.Binding, prompt string) (domain.ExecutionPlan, error)
	Classify(exitCode *int, timedOut bool, stdout, stderr string) (Status, string)
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
