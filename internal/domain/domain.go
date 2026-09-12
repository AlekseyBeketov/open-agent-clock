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
	Available      bool             `json:"available"`
	Reason         string           `json:"reason,omitempty"`
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

type RunResult struct {
	BindingID string        `json:"binding_id"`
	JobID     string        `json:"job_id,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Duration  time.Duration `json:"duration_ns"`
	Status    string        `json:"status"`
	ExitCode  *int          `json:"exit_code,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Version   string        `json:"provider_version,omitempty"`
}
