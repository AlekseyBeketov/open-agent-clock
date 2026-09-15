package notification

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultTimeout = 5 * time.Second
	commandPath    = "/usr/bin/osascript"
)

var ErrUnsupportedPlatform = errors.New("native macOS notifications are supported only on macOS")

// Classification is the deliberately small set of completion outcomes shown to
// the user. Provider diagnostics and output are intentionally not part of it.
type Classification string

const (
	CompletionSuccess Classification = "success"
	CompletionFailure Classification = "failure"
)

// Payload contains only stable identifiers and a completion classification.
// It must not be expanded with prompt text, provider output, or credentials.
type Payload struct {
	ScheduleID     string         `json:"schedule_id"`
	TargetID       string         `json:"target_id"`
	Classification Classification `json:"classification"`
}

// Notifier is the seam used by future scheduled-run integration. Implementations
// should treat delivery as best-effort and must not influence provider results.
type Notifier interface {
	Notify(context.Context, Payload) error
}

// CommandRunner isolates the native command from tests so they never display a
// real notification.
type CommandRunner interface {
	Run(context.Context, string, ...string) error
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

// NativeNotifier delivers a fixed-shape completion notification through the
// built-in macOS osascript command.
type NativeNotifier struct {
	runner  CommandRunner
	timeout time.Duration
	goos    string
}

func NewNative() *NativeNotifier {
	return &NativeNotifier{
		runner:  osCommandRunner{},
		timeout: DefaultTimeout,
		goos:    runtime.GOOS,
	}
}

// NewNativeWithRunner creates a native notifier with an injected command seam.
// The explicit runner selects the process boundary, so this constructor is
// usable by tests on non-macOS hosts without invoking the real UI.
func NewNativeWithRunner(runner CommandRunner, timeout time.Duration) *NativeNotifier {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &NativeNotifier{runner: runner, timeout: timeout, goos: "darwin"}
}

func (notifier *NativeNotifier) Notify(ctx context.Context, payload Payload) error {
	if err := validatePayload(payload); err != nil {
		return err
	}
	if notifier == nil || notifier.goos != "darwin" {
		return ErrUnsupportedPlatform
	}
	if notifier.runner == nil {
		return errors.New("notification command runner is not configured")
	}
	if ctx == nil {
		return errors.New("notification context must not be nil")
	}

	timeout := notifier.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	notifyContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := notifier.runner.Run(notifyContext, commandPath, "-e", appleScript(payload)); err != nil {
		return fmt.Errorf("display macOS notification: %w", err)
	}
	return nil
}

func validatePayload(payload Payload) error {
	if err := validateIdentifier("schedule id", payload.ScheduleID); err != nil {
		return err
	}
	if err := validateIdentifier("target id", payload.TargetID); err != nil {
		return err
	}
	switch payload.Classification {
	case CompletionSuccess, CompletionFailure:
		return nil
	default:
		return fmt.Errorf("unsupported completion classification %q", payload.Classification)
	}
}

func validateIdentifier(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if len(value) > 64 {
		return fmt.Errorf("%s must not exceed 64 characters", name)
	}
	for index, character := range value {
		if index == 0 {
			if !isIdentifierStart(character) {
				return fmt.Errorf("%s must be a stable identifier", name)
			}
			continue
		}
		if !isIdentifierPart(character) {
			return fmt.Errorf("%s must be a stable identifier", name)
		}
	}
	return nil
}

func isIdentifierStart(value rune) bool {
	return value == '_' || value == '-' || unicode.IsLetter(value) || unicode.IsDigit(value)
}

func isIdentifierPart(value rune) bool {
	return isIdentifierStart(value) || value == '.'
}

func appleScript(payload Payload) string {
	classification := "failed"
	if payload.Classification == CompletionSuccess {
		classification = "succeeded"
	}
	message := fmt.Sprintf("Schedule %q for target %q %s.", payload.ScheduleID, payload.TargetID, classification)
	return fmt.Sprintf("display notification %s with title %s", appleScriptString(message), appleScriptString("open-agent-clock"))
}

func appleScriptString(value string) string {
	var builder strings.Builder
	builder.Grow(len(value) + 2)
	builder.WriteByte('"')
	for _, character := range value {
		switch character {
		case '\\':
			builder.WriteString("\\\\")
		case '"':
			builder.WriteString("\\\"")
		case '\n':
			builder.WriteString("\\n")
		case '\r':
			builder.WriteString("\\r")
		case '\t':
			builder.WriteString("\\t")
		default:
			builder.WriteRune(character)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

var _ Notifier = (*NativeNotifier)(nil)
