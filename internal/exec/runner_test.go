package exec

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesExitCodeAndOutput(t *testing.T) {
	result := Run(context.Background(), "/bin/sh", []string{"-c", "printf ok"}, time.Second)
	if result.Err != nil || result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("result = %+v", result)
	}
	if result.Stdout != "ok" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestRunTimesOut(t *testing.T) {
	started := time.Now()
	result := Run(context.Background(), "/bin/sh", []string{"-c", "sleep 2"}, 50*time.Millisecond)
	if !result.TimedOut {
		t.Fatalf("expected timeout: %+v", result)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("timeout took too long: %s", time.Since(started))
	}
}

func TestRunDoesNotPassProviderSecretsToChild(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret-openai")
	t.Setenv("ANTHROPIC_API_KEY_EXTRA", "secret-anthropic")
	t.Setenv("OPEN_AGENT_CLOCK_TEST_VALUE", "safe")
	result := Run(context.Background(), "/bin/sh", []string{"-c", "env"}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if strings.Contains(result.Stdout, "secret-openai") || strings.Contains(result.Stdout, "secret-anthropic") {
		t.Fatalf("provider secret leaked to child: %q", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "OPEN_AGENT_CLOCK_TEST_VALUE=safe") {
		t.Fatalf("safe environment variable was unexpectedly removed: %q", result.Stdout)
	}
}
