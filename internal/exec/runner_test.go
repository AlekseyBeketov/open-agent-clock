package exec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestRunUsesAndRemovesTemporaryWorkingDirectory(t *testing.T) {
	result := Run(context.Background(), "/bin/sh", []string{"-c", "pwd"}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}

	workingDir := strings.TrimSpace(result.Stdout)
	if workingDir == "" || !strings.HasPrefix(filepath.Base(workingDir), "open-agent-clock-run-") {
		t.Fatalf("expected isolated temporary cwd, got %q", workingDir)
	}
	if _, err := os.Stat(workingDir); !os.IsNotExist(err) {
		t.Fatalf("temporary cwd still exists or could not be checked: %q, %v", workingDir, err)
	}
}

func TestRunRedactsCapturedStdoutAndStderr(t *testing.T) {
	result := Run(context.Background(), "/bin/sh", []string{"-c", "printf 'access_token=synthetic-stdout-secret bearer synthetic-stdout-bearer'; printf 'refresh_token=synthetic-stderr-secret bearer synthetic-stderr-bearer' >&2"}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, item := range []struct {
		output string
		secret string
	}{
		{result.Stdout, "synthetic-stdout-secret"},
		{result.Stdout, "synthetic-stdout-bearer"},
		{result.Stderr, "synthetic-stderr-secret"},
		{result.Stderr, "synthetic-stderr-bearer"},
	} {
		if strings.Contains(item.output, item.secret) || !strings.Contains(item.output, "[REDACTED]") {
			t.Fatalf("captured output was not redacted")
		}
	}
}

func TestRunRedactsCredentialLikeArgumentDiagnostics(t *testing.T) {
	result := Run(context.Background(), "/bin/sh", []string{"-c", "printf 'argument=access_token=synthetic-argument-secret'"}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if strings.Contains(result.Stdout, "synthetic-argument-secret") || !strings.Contains(result.Stdout, "[REDACTED]") {
		t.Fatalf("argument diagnostic was not redacted")
	}
}

func TestRunBoundsCapturedDiagnostics(t *testing.T) {
	result := Run(context.Background(), "/bin/sh", []string{"-c", "head -c 20000 /dev/zero | tr '\\000' x"}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if len(result.Stdout) > OutputLimit || !strings.Contains(result.Stdout, "[output truncated]") {
		t.Fatalf("stdout length=%d, output=%q", len(result.Stdout), result.Stdout)
	}
}

func TestRunKillsDescendantsOnTimeout(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "descendant-ran")
	command := fmt.Sprintf("sleep 1 & (sleep 0.2; printf late > %s) & wait", marker)
	result := Run(context.Background(), "/bin/sh", []string{"-c", command}, 50*time.Millisecond)
	if !result.TimedOut {
		t.Fatalf("expected timeout: %+v", result)
	}

	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("descendant process survived timeout: %v", err)
	}
}

func TestRunDoesNotPassProviderSecretsToChild(t *testing.T) {
	secrets := map[string]string{
		"OPENAI_API_KEY":              "synthetic-openai-secret",
		"OPENAI_API_KEY_EXTRA":        "synthetic-openai-extra-secret",
		"ANTHROPIC_API_KEY":           "synthetic-anthropic-secret",
		"CLAUDE_API_KEY":              "synthetic-claude-secret",
		"AWS_SECRET_ACCESS_KEY":       "synthetic-aws-secret",
		"AWS_SESSION_TOKEN":           "synthetic-session-secret",
		"OPEN_AGENT_CLOCK_TEST_VALUE": "safe",
	}
	for name, value := range secrets {
		t.Setenv(name, value)
	}
	result := Run(context.Background(), "/bin/sh", []string{"-c", "printf '%s|%s|%s|%s|%s|%s|%s' \"${OPENAI_API_KEY-<unset>}\" \"${OPENAI_API_KEY_EXTRA-<unset>}\" \"${ANTHROPIC_API_KEY-<unset>}\" \"${CLAUDE_API_KEY-<unset>}\" \"${AWS_SECRET_ACCESS_KEY-<unset>}\" \"${AWS_SESSION_TOKEN-<unset>}\" \"$OPEN_AGENT_CLOCK_TEST_VALUE\""}, time.Second)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	for name, value := range secrets {
		if name != "OPEN_AGENT_CLOCK_TEST_VALUE" && strings.Contains(result.Stdout, value) {
			t.Fatalf("provider secret reached environment diagnostics: %s", name)
		}
	}
	if !strings.Contains(result.Stdout, "safe") || strings.Count(result.Stdout, "<unset>") != 6 {
		t.Fatalf("environment filtering result did not match the safe diagnostic contract")
	}
}
