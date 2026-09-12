package plan

import (
	"testing"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestNativeCodexPlanIsMinimalAndEphemeral(t *testing.T) {
	binding := domain.Binding{
		ID: "native-codex", Provider: "openai-codex", Backend: domain.NativeCodexCLI,
		Executable: "/opt/homebrew/bin/codex", AuthMode: "chatgpt", Available: true,
	}
	result, err := ForBinding(binding, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if result.Args[0] != "exec" || !contains(result.Args, "--ephemeral") {
		t.Fatalf("unexpected native args: %#v", result.Args)
	}
	if result.WorkingDir != "<temporary-empty-cwd>" {
		t.Fatalf("working dir = %q", result.WorkingDir)
	}
}

func TestHermesPlanDisablesCustomContext(t *testing.T) {
	binding := domain.Binding{
		ID: "hermes-codex", Provider: "openai-codex", Backend: domain.HermesCLI,
		Executable: "/Users/test/bin/hermes", AuthMode: "oauth", Available: true,
	}
	result, err := ForBinding(binding, "hi")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"--safe-mode", "--ignore-rules", "--oneshot", "--quiet"} {
		if !contains(result.Args, expected) {
			t.Fatalf("missing %q in %#v", expected, result.Args)
		}
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
