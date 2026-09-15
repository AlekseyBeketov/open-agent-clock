package plan

import (
	"reflect"
	"testing"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestNativeCodexPlanIsMinimalAndEphemeral(t *testing.T) {
	binding := domain.Binding{
		ID: "native-codex", Provider: "openai-codex", Backend: domain.NativeCodexCLI,
		Executable: "/opt/homebrew/bin/codex", AuthMode: "chatgpt", Available: true,
		Capabilities: []domain.Capability{domain.CapabilityEphemeral},
	}
	result, err := ForBinding(binding, "hi")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"--ask-for-approval", "never", "exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "hi"}
	if !reflect.DeepEqual(result.Args, expected) {
		t.Fatalf("unexpected native args: %#v", result.Args)
	}
	if result.WorkingDir != "<temporary-empty-cwd>" {
		t.Fatalf("working dir = %q", result.WorkingDir)
	}
}

func TestNativeCodexDevPlanAddsJSONAfterTrustedDirectoryFix(t *testing.T) {
	binding := domain.Binding{
		ID: "native-codex", Provider: "openai-codex", Backend: domain.NativeCodexCLI,
		Executable: "/opt/homebrew/bin/codex", AuthMode: "chatgpt", Available: true,
		Capabilities: []domain.Capability{domain.CapabilityEphemeral},
	}
	result, err := ForBindingWithMode(binding, "hi", true)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"--ask-for-approval", "never", "exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "--json", "hi"}
	if !reflect.DeepEqual(result.Args, expected) {
		t.Fatalf("unexpected native dev args: %#v", result.Args)
	}
}

func TestNativeCodexPlanOmitsUnsupportedEphemeralFlag(t *testing.T) {
	binding := domain.Binding{
		ID: "native-codex", Provider: "openai-codex", Backend: domain.NativeCodexCLI,
		Executable: "/opt/homebrew/bin/codex", Version: "legacy", AuthMode: "chatgpt", Available: true,
	}
	result, err := ForBinding(binding, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if result.Args[0] != "--ask-for-approval" || result.Args[1] != "never" || result.Args[2] != "exec" {
		t.Fatalf("unexpected non-interactive native args: %#v", result.Args)
	}
	if contains(result.Args, "--ephemeral") {
		t.Fatalf("unsupported ephemeral flag was planned: %#v", result.Args)
	}
	for _, expected := range []string{"--sandbox", "read-only", "--skip-git-repo-check", "hi"} {
		if !contains(result.Args, expected) {
			t.Fatalf("missing %q in %#v", expected, result.Args)
		}
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
