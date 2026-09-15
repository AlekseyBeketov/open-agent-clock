package domain

import (
	"strings"
	"time"
)

type Provider string

const (
	ProviderOpenAICodex Provider = "openai-codex"
	ProviderClaude      Provider = "claude"
)

type AuthMode string

const (
	AuthChatGPTSubscription AuthMode = "chatgpt-subscription"
	AuthSubscriptionOAuth   AuthMode = "subscription-oauth"
	AuthAPIKey              AuthMode = "api-key"
	AuthUnknown             AuthMode = "unknown"
)

func IsSubscriptionAuthMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "chatgpt", "chatgpt-subscription", "subscription", "subscription-oauth", "oauth", "oauth-subscription":
		return true
	default:
		return false
	}
}

type ExecutionBackend string

const (
	NativeCodexCLI ExecutionBackend = "native-codex-cli"
	HermesCLI      ExecutionBackend = "hermes-cli"
	ClaudeCLI      ExecutionBackend = "claude-cli"
)

type IdentityStatus string

const (
	IdentityVerified   IdentityStatus = "verified"
	IdentityUnverified IdentityStatus = "unverified"
	IdentityUnknown    IdentityStatus = "unknown"
)

type Capability string

const (
	CapabilityEphemeral Capability = "ephemeral"
)

type ScheduleMode string

const (
	ScheduleInterval ScheduleMode = "interval"
	ScheduleDaily    ScheduleMode = "daily"
)

type Binding struct {
	ID             string           `json:"id"`
	Provider       string           `json:"provider"`
	Backend        ExecutionBackend `json:"backend"`
	DisplayName    string           `json:"display_name"`
	Executable     string           `json:"executable,omitempty"`
	Version        string           `json:"version,omitempty"`
	AuthMode       string           `json:"auth_mode,omitempty"`
	AuthStore      string           `json:"auth_store,omitempty"`
	IdentityStatus IdentityStatus   `json:"identity_status"`
	IdentityHash   string           `json:"identity_hash,omitempty"`
	Capabilities   []Capability     `json:"capabilities,omitempty"`
	Available      bool             `json:"available"`
	Reason         string           `json:"reason,omitempty"`
}

func (binding Binding) Supports(capability Capability) bool {
	for _, supported := range binding.Capabilities {
		if supported == capability {
			return true
		}
	}
	return false
}

type Schedule struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	TargetID  string       `json:"target_id"`
	Provider  Provider     `json:"provider"`
	Prompt    string       `json:"prompt"`
	Mode      ScheduleMode `json:"mode"`
	Interval  string       `json:"interval,omitempty"`
	Times     []string     `json:"times,omitempty"`
	Timezone  string       `json:"timezone"`
	Enabled   bool         `json:"enabled"`
	StartAt   time.Time    `json:"start_at"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Job struct {
	ID         string    `json:"id"`
	ScheduleID string    `json:"schedule_id"`
	TargetID   string    `json:"target_id"`
	DueAt      time.Time `json:"due_at"`
}

type ExecutionPlan struct {
	BindingID  string   `json:"binding_id"`
	Provider   string   `json:"provider"`
	Backend    string   `json:"backend"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
	WorkingDir string   `json:"working_dir"`
	AuthMode   string   `json:"auth_mode"`
	Prompt     string   `json:"prompt"`
	Timeout    string   `json:"timeout"`
	SideEffect string   `json:"side_effect"`
}

// TokenUsageAvailability says whether the provider supplied structured usage
// counters. The application never estimates these values.
type TokenUsageAvailability string

const (
	TokenUsageAvailable   TokenUsageAvailability = "available"
	TokenUsageUnavailable TokenUsageAvailability = "unavailable"
)

type TokenUsage struct {
	Availability      TokenUsageAvailability `json:"availability"`
	Source            string                 `json:"source,omitempty"`
	InputTokens       int64                  `json:"input_tokens,omitempty"`
	CachedInputTokens int64                  `json:"cached_input_tokens,omitempty"`
	OutputTokens      int64                  `json:"output_tokens,omitempty"`
	TotalTokens       int64                  `json:"total_tokens,omitempty"`
}

func UnavailableTokenUsage() *TokenUsage {
	return &TokenUsage{Availability: TokenUsageUnavailable}
}

func (usage *TokenUsage) Normalize() {
	if usage == nil {
		return
	}
	if usage.Availability != TokenUsageAvailable {
		usage.Availability = TokenUsageUnavailable
		usage.Source = ""
		usage.InputTokens = 0
		usage.CachedInputTokens = 0
		usage.OutputTokens = 0
		usage.TotalTokens = 0
		return
	}
	if usage.Source == "" {
		usage.Source = "provider-reported"
	}
}

type DiagnosticCategory string

const (
	DiagnosticAuth      DiagnosticCategory = "auth"
	DiagnosticQuota     DiagnosticCategory = "quota"
	DiagnosticNetwork   DiagnosticCategory = "network"
	DiagnosticArguments DiagnosticCategory = "arguments"
	DiagnosticProvider  DiagnosticCategory = "provider"
	DiagnosticUnknown   DiagnosticCategory = "unknown"
)

type Diagnostic struct {
	Category DiagnosticCategory `json:"category"`
	Detail   string             `json:"detail,omitempty"`
}

type RunResult struct {
	BindingID  string        `json:"binding_id"`
	JobID      string        `json:"job_id,omitempty"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at"`
	Duration   time.Duration `json:"duration_ns"`
	Status     string        `json:"status"`
	ExitCode   *int          `json:"exit_code,omitempty"`
	Reason     string        `json:"reason,omitempty"`
	Version    string        `json:"provider_version,omitempty"`
	TokenUsage *TokenUsage   `json:"token_usage,omitempty"`
	Diagnostic *Diagnostic   `json:"diagnostic,omitempty"`
}

// NormalizeTelemetry makes legacy records explicit to callers while keeping
// their original JSON shape readable and migration-free.
func (result *RunResult) NormalizeTelemetry() {
	if result.TokenUsage == nil {
		result.TokenUsage = UnavailableTokenUsage()
	} else {
		result.TokenUsage.Normalize()
	}
	if result.Diagnostic != nil {
		switch result.Diagnostic.Category {
		case DiagnosticAuth, DiagnosticQuota, DiagnosticNetwork, DiagnosticArguments, DiagnosticProvider, DiagnosticUnknown:
		default:
			result.Diagnostic.Category = DiagnosticUnknown
		}
	}
}
