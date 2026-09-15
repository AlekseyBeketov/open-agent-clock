package plan

import (
	"fmt"
	"strings"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func ForBinding(binding domain.Binding, prompt string) (domain.ExecutionPlan, error) {
	return ForBindingWithMode(binding, prompt, false)
}

// ForBindingWithMode preserves the normal plan seam while allowing an
// explicitly requested development run to ask native Codex for JSONL usage.
func ForBindingWithMode(binding domain.Binding, prompt string, devMode bool) (domain.ExecutionPlan, error) {
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
		// Keep global flags before `exec`; only add `--ephemeral` when discovery
		// confirmed that this installed Codex exposes the flag. The runner uses a
		// temporary empty cwd, so Codex must not require a trusted git directory.
		plan.Args = []string{"--ask-for-approval", "never", "exec"}
		if binding.Supports(domain.CapabilityEphemeral) {
			plan.Args = append(plan.Args, "--ephemeral")
		}
		plan.Args = append(plan.Args, "--sandbox", "read-only", "--skip-git-repo-check")
		if devMode {
			plan.Args = append(plan.Args, "--json")
		}
		plan.Args = append(plan.Args, prompt)
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
