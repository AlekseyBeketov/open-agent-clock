package plan

import (
	"fmt"
	"strings"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func ForBinding(binding domain.Binding, prompt string) (domain.ExecutionPlan, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return domain.ExecutionPlan{}, fmt.Errorf("prompt must not be empty")
	}
	if !binding.Available {
		return domain.ExecutionPlan{}, fmt.Errorf("binding %s is unavailable: %s", binding.ID, binding.Reason)
	}

	plan := domain.ExecutionPlan{
		BindingID:  binding.ID,
		Provider:   binding.Provider,
		Backend:    string(binding.Backend),
		WorkingDir: "<temporary-empty-cwd>",
		AuthMode:   binding.AuthMode,
		Prompt:     prompt,
		Timeout:    "120s",
		SideEffect: "provider invocation; not executed in dry-run",
	}

	switch binding.Backend {
	case domain.NativeCodexCLI:
		plan.Executable = binding.Executable
		plan.Args = []string{"exec", "--ephemeral", "--sandbox", "read-only", "--ask-for-approval", "never", prompt}
	case domain.HermesCLI:
		plan.Executable = binding.Executable
		plan.Args = []string{"chat", "--provider", "openai-codex", "--safe-mode", "--ignore-rules", "--toolsets", "", "--oneshot", "--quiet", "-q", prompt}
	case domain.ClaudeCLI:
		return domain.ExecutionPlan{}, fmt.Errorf("Claude subscription minimal local execution is unsupported; API-key fallback is disabled")
	default:
		return domain.ExecutionPlan{}, fmt.Errorf("unsupported execution backend %q", binding.Backend)
	}
	return plan, nil
}
