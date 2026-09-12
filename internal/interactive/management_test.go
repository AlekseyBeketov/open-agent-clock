package interactive

import (
	"bytes"
	"slices"
	"testing"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestScheduleManagementDelegatesEveryLifecycleOperation(t *testing.T) {
	item := domain.Schedule{ID: "window", Name: "Window", TargetID: "native-codex", Provider: domain.ProviderOpenAICodex, Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "5h3m", Timezone: "UTC"}
	prompt := &scriptedPrompt{
		selects: []string{
			"schedules",
			"edit", "window", "native-codex", "interval",
			"toggle", "window",
			"launchd", "window", "preview", "status", "uninstall", "back",
			"remove", "window",
			"back", "exit",
		},
		inputs:   []string{"Updated window", "hello", "6h", "UTC"},
		confirms: []bool{true, true, true},
	}
	operations := &fakeOperations{
		initialized: true,
		config:      appconfig.Config{SchemaVersion: appconfig.SchemaVersion, Language: "en", SetupCompleted: true, Schedules: []domain.Schedule{item}},
		bindings:    []domain.Binding{{ID: "native-codex", Provider: "openai-codex", DisplayName: "Native Codex", AuthMode: "chatgpt", Available: true}},
	}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}

	if err := app.RunUI(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range [][]string{
		{"schedule", "update"},
		{"resume", "--id", "window", "--confirm"},
		{"schedule", "install", "--id", "window", "--dry-run"},
		{"schedule", "status", "--id", "window"},
		{"schedule", "uninstall", "--id", "window", "--confirm"},
		{"schedule", "remove", "--id", "window", "--confirm"},
	} {
		if !containsCommandPrefix(operations.commands, expected) {
			t.Errorf("missing command prefix %#v in %#v", expected, operations.commands)
		}
	}
	update := operations.commands[0]
	if !slices.Contains(update, "--target") || !slices.Contains(update, "native-codex") {
		t.Fatalf("target was not retained in update: %#v", update)
	}
}

func TestExecutionMenuDelegatesPreviewOnceAndTick(t *testing.T) {
	item := domain.Schedule{ID: "window", Name: "Window", TargetID: "native-codex", Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "5h3m", Timezone: "UTC", Enabled: true}
	prompt := &scriptedPrompt{
		selects: []string{
			"run",
			"preview", "native-codex",
			"once", "window",
			"tick", "window",
			"back", "exit",
		},
		inputs:   []string{"hi"},
		confirms: []bool{true, true},
	}
	operations := &fakeOperations{
		initialized: true,
		config:      appconfig.Config{SchemaVersion: appconfig.SchemaVersion, Language: "en", SetupCompleted: true, Schedules: []domain.Schedule{item}},
		bindings:    []domain.Binding{{ID: "native-codex", Provider: "openai-codex", DisplayName: "Native Codex", AuthMode: "chatgpt", Available: true}},
	}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}

	if err := app.RunUI(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range [][]string{
		{"run", "--dry-run", "--target", "native-codex", "--prompt", "hi"},
		{"run", "--once", "--schedule", "window", "--confirm"},
		{"tick", "--schedule", "window", "--confirm"},
	} {
		if !containsCommand(operations.commands, expected) {
			t.Errorf("missing command %#v in %#v", expected, operations.commands)
		}
	}
}

func TestHistoryAndSettingsFlows(t *testing.T) {
	item := domain.Schedule{ID: "window", Name: "Window"}
	prompt := &scriptedPrompt{selects: []string{
		"history", "status", "history", "last", "window", "back",
		"settings", "language", "ru", "paths", "back",
		"exit",
	}}
	operations := &fakeOperations{
		initialized: true,
		config:      appconfig.Config{SchemaVersion: appconfig.SchemaVersion, Language: "en", SetupCompleted: true, Schedules: []domain.Schedule{item}},
	}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}

	if err := app.RunUI(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range [][]string{{"status"}, {"history"}, {"last-run", "--schedule", "window"}, {"config"}} {
		if !containsCommand(operations.commands, expected) {
			t.Errorf("missing command %#v in %#v", expected, operations.commands)
		}
	}
	if len(operations.preferences) == 0 || operations.preferences[len(operations.preferences)-1] != "ru:true" {
		t.Fatalf("preferences = %#v", operations.preferences)
	}
}

func TestAccessibleEnvironmentEnablesAccessiblePrompts(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")
	prompt := NewHuhPrompter(nil, nil)
	if !prompt.Accessible {
		t.Fatal("ACCESSIBLE did not enable accessible prompts")
	}
}

func containsCommandPrefix(commands [][]string, expected []string) bool {
	return slices.ContainsFunc(commands, func(command []string) bool {
		return len(command) >= len(expected) && slices.Equal(command[:len(expected)], expected)
	})
}
