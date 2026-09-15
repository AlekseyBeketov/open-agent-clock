package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/discovery"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	runner "github.com/AlekseyBeketov/open-agent-clock/internal/exec"
)

func TestNativeCodexAdapterRunsDiscoveredFakeProvider(t *testing.T) {
	bindings := setupFakeProviderBindings(t)
	binding := bindingByID(t, bindings, "native-codex")

	adapter, err := ForBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := adapter.BuildPlan(binding, "native integration prompt")
	if err != nil {
		t.Fatal(err)
	}
	expectedArgs := []string{"--ask-for-approval", "never", "exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "native integration prompt"}
	if !reflect.DeepEqual(planned.Args, expectedArgs) {
		t.Fatalf("native plan args = %#v, want %#v", planned.Args, expectedArgs)
	}

	execution := runner.Run(context.Background(), planned.Executable, planned.Args, time.Second)
	if execution.Err != nil || execution.ExitCode == nil || *execution.ExitCode != 0 {
		t.Fatalf("fake native execution = %+v", execution)
	}
	if execution.Stdout != "fake native Codex completed\n" {
		t.Fatalf("fake native stdout = %q", execution.Stdout)
	}
	status, reason := adapter.Classify(execution.ExitCode, execution.TimedOut, execution.Stdout, execution.Stderr)
	if status != StatusSuccess || reason == "" {
		t.Fatalf("native classification = %q, %q", status, reason)
	}
}

func TestHermesAdapterRunsDiscoveredFakeProvider(t *testing.T) {
	bindings := setupFakeProviderBindings(t)
	binding := bindingByID(t, bindings, "hermes-codex")

	adapter, err := ForBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := adapter.BuildPlan(binding, "Hermes integration prompt")
	if err != nil {
		t.Fatal(err)
	}
	expectedArgs := []string{"chat", "--provider", "openai-codex", "--safe-mode", "--ignore-rules", "--toolsets", "", "--oneshot", "--quiet", "-q", "Hermes integration prompt"}
	if !reflect.DeepEqual(planned.Args, expectedArgs) {
		t.Fatalf("Hermes plan args = %#v, want %#v", planned.Args, expectedArgs)
	}

	execution := runner.Run(context.Background(), planned.Executable, planned.Args, time.Second)
	if execution.Err != nil || execution.ExitCode == nil || *execution.ExitCode != 0 {
		t.Fatalf("fake Hermes execution = %+v", execution)
	}
	if execution.Stdout != "fake Hermes Codex completed\n" {
		t.Fatalf("fake Hermes stdout = %q", execution.Stdout)
	}
	status, reason := adapter.Classify(execution.ExitCode, execution.TimedOut, execution.Stdout, execution.Stderr)
	if status != StatusSuccess || reason == "" {
		t.Fatalf("Hermes classification = %q, %q", status, reason)
	}
}

func TestFakeProviderReportsUsageOnlyThroughExplicitDevJSONMode(t *testing.T) {
	bindings := setupFakeProviderBindings(t)
	binding := bindingByID(t, bindings, "native-codex")
	adapter, err := ForBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	normalPlan, err := adapter.BuildPlan(binding, "telemetry prompt")
	if err != nil {
		t.Fatal(err)
	}
	modeAdapter, ok := adapter.(interface {
		BuildPlanForMode(domain.Binding, string, bool) (domain.ExecutionPlan, error)
	})
	if !ok {
		t.Fatal("native adapter does not expose dev plan seam")
	}
	devPlan, err := modeAdapter.BuildPlanForMode(binding, "telemetry prompt", true)
	if err != nil {
		t.Fatal(err)
	}
	if containsArg(normalPlan.Args, "--json") {
		t.Fatalf("normal plan unexpectedly enabled JSON: %#v", normalPlan.Args)
	}
	if !containsArg(devPlan.Args, "--json") || !containsArg(devPlan.Args, "--skip-git-repo-check") {
		t.Fatalf("dev plan missed JSON/trusted-directory flags: %#v", devPlan.Args)
	}

	t.Setenv("FAKE_PROVIDER_SCENARIO", "success-usage")
	execution := runner.Run(context.Background(), normalPlan.Executable, normalPlan.Args, time.Second)
	if execution.Err != nil {
		t.Fatal(execution.Err)
	}
	observation := adapter.Observe(execution)
	if observation.Usage.Availability != domain.TokenUsageUnavailable {
		t.Fatalf("normal observation unexpectedly reported usage = %+v", observation.Usage)
	}

	execution = runner.Run(context.Background(), devPlan.Executable, devPlan.Args, time.Second)
	if execution.Err != nil {
		t.Fatal(execution.Err)
	}
	observation = adapter.Observe(execution)
	if observation.Status != StatusSuccess || observation.Usage.Availability != domain.TokenUsageAvailable {
		t.Fatalf("dev observation = %+v", observation)
	}
	if observation.Usage.InputTokens != 11 || observation.Usage.CachedInputTokens != 5 || observation.Usage.OutputTokens != 7 || observation.Usage.TotalTokens != 18 {
		t.Fatalf("dev usage = %+v", observation.Usage)
	}
}

func TestFakeProviderFailureDiagnostics(t *testing.T) {
	bindings := setupFakeProviderBindings(t)
	binding := bindingByID(t, bindings, "native-codex")
	adapter, err := ForBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := adapter.BuildPlan(binding, "telemetry prompt")
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		scenario string
		category domain.DiagnosticCategory
	}{
		{"quota", domain.DiagnosticQuota},
		{"auth", domain.DiagnosticAuth},
		{"provider", domain.DiagnosticProvider},
	} {
		t.Run(testCase.scenario, func(t *testing.T) {
			t.Setenv("FAKE_PROVIDER_SCENARIO", testCase.scenario)
			execution := runner.Run(context.Background(), planned.Executable, planned.Args, time.Second)
			if execution.Err == nil || execution.ExitCode == nil || *execution.ExitCode != 1 {
				t.Fatalf("fake failure execution = %+v", execution)
			}
			observation := adapter.Observe(execution)
			if observation.Status != StatusUsageLimited && testCase.category == domain.DiagnosticQuota {
				t.Fatalf("quota status = %q", observation.Status)
			}
			if observation.Diagnostic == nil || observation.Diagnostic.Category != testCase.category {
				t.Fatalf("failure observation = %+v", observation)
			}
			if observation.Usage.Availability != domain.TokenUsageUnavailable {
				t.Fatalf("failure usage = %+v", observation.Usage)
			}
			if strings.Contains(observation.Diagnostic.Detail, "synthetic-provider-secret") {
				t.Fatalf("secret leaked into diagnostic: %+v", observation.Diagnostic)
			}
		})
	}
}

func containsArg(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func setupFakeProviderBindings(t *testing.T) []domain.Binding {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := fakeProviderFixture(t)
	for _, name := range []string{"codex", "hermes"} {
		if err := os.WriteFile(filepath.Join(binDir, name), fake, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", root)
	t.Setenv("HERMES_HOME", filepath.Join(root, "hermes-profile"))
	t.Setenv("PATH", binDir)

	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	contents, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]string{"account_id": "fake-account-id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "auth.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	hermesHome := filepath.Join(root, "hermes-profile")
	if err := os.MkdirAll(hermesHome, 0o700); err != nil {
		t.Fatal(err)
	}
	contents, err = json.Marshal(map[string]any{
		"credential_pool": map[string]any{
			"openai-codex": []map[string]string{{"auth_type": "oauth", "source": "test"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesHome, "auth.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	return discovery.Discover()
}

func fakeProviderFixture(t *testing.T) []byte {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate provider integration test")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(source), "testdata", "fake-provider.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func bindingByID(t *testing.T, bindings []domain.Binding, id string) domain.Binding {
	t.Helper()
	for _, binding := range bindings {
		if binding.ID == id {
			if !binding.Available {
				t.Fatalf("binding %q is unavailable: %s", id, binding.Reason)
			}
			return binding
		}
	}
	t.Fatalf("binding %q was not discovered: %+v", id, bindings)
	return domain.Binding{}
}
