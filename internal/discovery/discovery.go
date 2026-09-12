package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	runner "github.com/AlekseyBeketov/open-agent-clock/internal/exec"
)

var discoveryCommandTimeout = 3 * time.Second

type nativeAuth struct {
	AuthMode string `json:"auth_mode"`
	Tokens   struct {
		AccountID string `json:"account_id"`
	} `json:"tokens"`
}

type hermesStore struct {
	CredentialPool map[string][]struct {
		AuthType  string `json:"auth_type"`
		Source    string `json:"source"`
		BaseURL   string `json:"base_url"`
		AccountID string `json:"account_id"`
	} `json:"credential_pool"`
}

func Discover() []domain.Binding {
	return []domain.Binding{discoverNativeCodex(), discoverHermesCodex(), discoverClaude()}
}

func discoverNativeCodex() domain.Binding {
	binding := domain.Binding{
		ID:             "native-codex",
		Provider:       "openai-codex",
		Backend:        domain.NativeCodexCLI,
		DisplayName:    "Native Codex CLI",
		IdentityStatus: domain.IdentityUnknown,
	}

	executable, err := exec.LookPath("codex")
	if err != nil {
		binding.Reason = "codex executable was not found in PATH"
		return binding
	}
	binding.Executable = executable
	binding.Version = commandVersion(executable)

	authPath := filepath.Join(homeDir(), ".codex", "auth.json")
	binding.AuthStore = authPath
	contents, err := os.ReadFile(authPath)
	if err != nil {
		binding.Reason = "native Codex auth store is unavailable"
		return binding
	}
	var auth nativeAuth
	if err := json.Unmarshal(contents, &auth); err != nil {
		binding.Reason = "native Codex auth store is invalid JSON"
		return binding
	}
	binding.AuthMode = auth.AuthMode
	if !domain.IsSubscriptionAuthMode(binding.AuthMode) {
		binding.Reason = "native Codex is not authenticated through a supported subscription path"
		return binding
	}
	if strings.EqualFold(binding.AuthMode, "chatgpt") {
		binding.AuthMode = string(domain.AuthChatGPTSubscription)
	}
	if auth.Tokens.AccountID != "" {
		binding.IdentityStatus = domain.IdentityVerified
		binding.IdentityHash = fingerprint(auth.Tokens.AccountID)
	}
	binding.Available = true
	return binding
}

func discoverClaude() domain.Binding {
	binding := domain.Binding{
		ID:             "claude-subscription",
		Provider:       "claude",
		Backend:        domain.ClaudeCLI,
		DisplayName:    "Claude Code subscription",
		IdentityStatus: domain.IdentityUnknown,
	}
	executable, err := exec.LookPath("claude")
	if err != nil {
		binding.Reason = "claude executable was not found in PATH"
		return binding
	}
	binding.Executable = executable
	binding.Version = commandVersion(executable)
	result := runner.Run(context.Background(), executable, []string{"auth", "status"}, discoveryCommandTimeout)
	if result.Err != nil {
		binding.Reason = "Claude subscription auth status could not be verified"
		return binding
	}
	status := strings.ToLower(result.Stdout + "\n" + result.Stderr)
	if strings.Contains(status, "oauth") || strings.Contains(status, "subscription") {
		binding.AuthMode = "subscription-oauth"
		binding.Available = true
		return binding
	}
	binding.Reason = "Claude subscription OAuth was not confirmed; API-key fallback is disabled"
	return binding
}

func discoverHermesCodex() domain.Binding {
	binding := domain.Binding{
		ID:             "hermes-codex",
		Provider:       "openai-codex",
		Backend:        domain.HermesCLI,
		DisplayName:    "Hermes → OpenAI Codex",
		IdentityStatus: domain.IdentityUnknown,
	}

	executable, err := exec.LookPath("hermes")
	if err != nil {
		binding.Reason = "hermes executable was not found in PATH"
		return binding
	}
	binding.Executable = executable
	binding.Version = commandVersion(executable)

	stores := hermesAuthCandidates()
	for _, authPath := range stores {
		contents, err := os.ReadFile(authPath)
		if err != nil {
			continue
		}
		var store hermesStore
		if json.Unmarshal(contents, &store) != nil {
			continue
		}
		entries := store.CredentialPool["openai-codex"]
		if len(entries) == 0 {
			continue
		}
		entry := entries[0]
		if !domain.IsSubscriptionAuthMode(entry.AuthType) {
			continue
		}
		binding.AuthStore = authPath
		binding.AuthMode = string(domain.AuthSubscriptionOAuth)
		binding.Available = true
		// Hermes does not persist a stable account_id in the active record.
		// Keep identity unknown instead of guessing from token material.
		return binding
	}

	binding.Reason = "Hermes openai-codex OAuth credential was not found"
	return binding
}

func hermesAuthCandidates() []string {
	candidates := []string{}
	if value := strings.TrimSpace(os.Getenv("HERMES_HOME")); value != "" {
		candidates = append(candidates, filepath.Join(value, "auth.json"))
	}
	candidates = append(candidates, filepath.Join(homeDir(), ".hermes", "auth.json"))
	return unique(candidates)
}

func commandVersion(executable string) string {
	result := runner.Run(context.Background(), executable, []string{"--version"}, discoveryCommandTimeout)
	if result.Err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if value := strings.TrimSpace(line); value != "" {
			return value
		}
	}
	return "unknown"
}

func homeDir() string {
	if value := strings.TrimSpace(os.Getenv("HOME")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~"
	}
	return home
}

func fingerprint(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:12]
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func ValidateBinding(binding domain.Binding) error {
	if binding.ID == "" || binding.Provider == "" {
		return fmt.Errorf("binding must have id and provider")
	}
	return nil
}
