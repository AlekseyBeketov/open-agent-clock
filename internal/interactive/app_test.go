package interactive

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

type promptCall struct {
	Kind         string
	Title        string
	Description  string
	DefaultValue string
	Options      []Option
}

type scriptedPrompt struct {
	selects  []string
	inputs   []string
	confirms []bool
	calls    []promptCall
	failAt   int
	count    int
}

func (prompt *scriptedPrompt) SetLanguage(string) {}

func (prompt *scriptedPrompt) Select(title, description string, options []Option, defaultValue string) (string, error) {
	if err := prompt.nextError(); err != nil {
		return "", err
	}
	prompt.calls = append(prompt.calls, promptCall{Kind: "select", Title: title, Description: description, DefaultValue: defaultValue, Options: append([]Option(nil), options...)})
	if len(prompt.selects) == 0 {
		return "", errors.New("unexpected select")
	}
	value := prompt.selects[0]
	prompt.selects = prompt.selects[1:]
	return value, nil
}

func (prompt *scriptedPrompt) Input(title, description, defaultValue string, validate func(string) error) (string, error) {
	if err := prompt.nextError(); err != nil {
		return "", err
	}
	prompt.calls = append(prompt.calls, promptCall{Kind: "input", Title: title, Description: description, DefaultValue: defaultValue})
	if len(prompt.inputs) == 0 {
		return "", errors.New("unexpected input")
	}
	value := prompt.inputs[0]
	prompt.inputs = prompt.inputs[1:]
	if validate != nil {
		if err := validate(value); err != nil {
			return "", fmt.Errorf("scripted value %q failed validation: %w", value, err)
		}
	}
	return value, nil
}

func (prompt *scriptedPrompt) Confirm(title, description string, defaultValue bool) (bool, error) {
	if err := prompt.nextError(); err != nil {
		return false, err
	}
	prompt.calls = append(prompt.calls, promptCall{Kind: "confirm", Title: title, Description: description, DefaultValue: fmt.Sprint(defaultValue)})
	if len(prompt.confirms) == 0 {
		return false, errors.New("unexpected confirm")
	}
	value := prompt.confirms[0]
	prompt.confirms = prompt.confirms[1:]
	return value, nil
}

func (prompt *scriptedPrompt) nextError() error {
	prompt.count++
	if prompt.failAt > 0 && prompt.count == prompt.failAt {
		return ErrCancelled
	}
	return nil
}

type fakeOperations struct {
	initialized bool
	config      appconfig.Config
	bindings    []domain.Binding
	commands    [][]string
	preferences []string
}

func (operations *fakeOperations) Snapshot() (Snapshot, error) {
	return Snapshot{Initialized: operations.initialized, Config: operations.config, Paths: appconfig.Paths{Root: "/tmp/open-agent-clock"}}, nil
}

func (operations *fakeOperations) Initialize() error {
	operations.initialized = true
	if operations.config.SchemaVersion == 0 {
		operations.config = appconfig.DefaultConfig()
	}
	return nil
}

func (operations *fakeOperations) SavePreferences(language string, completed bool) error {
	operations.initialized = true
	operations.config.Language = language
	operations.config.SetupCompleted = completed
	operations.preferences = append(operations.preferences, fmt.Sprintf("%s:%t", language, completed))
	return nil
}

func (operations *fakeOperations) Bindings() []domain.Binding {
	return append([]domain.Binding(nil), operations.bindings...)
}

func (operations *fakeOperations) Execute(args []string) error {
	operations.commands = append(operations.commands, append([]string(nil), args...))
	return nil
}

func TestEnglishSetupCreatesDisabledScheduleBeforeOptionalActivation(t *testing.T) {
	prompt := &scriptedPrompt{
		selects:  []string{"en", "native-codex", "interval"},
		inputs:   []string{"codex-window", "Codex window", "5h3m", "Europe/Moscow", "hi"},
		confirms: []bool{true, true, true, true, true},
	}
	operations := &fakeOperations{bindings: []domain.Binding{{ID: "native-codex", Provider: "openai-codex", DisplayName: "Native Codex", AuthMode: "chatgpt", Available: true}}}
	var output bytes.Buffer
	app := App{Prompt: prompt, Ops: operations, Out: &output, Lang: "en"}

	if err := app.RunSetup(); err != nil {
		t.Fatal(err)
	}
	if prompt.calls[0].DefaultValue != "en" || !strings.Contains(prompt.calls[0].Title, "Выберите язык") {
		t.Fatalf("first prompt = %+v", prompt.calls[0])
	}
	wantPrefixes := [][]string{
		{"schedule", "add"},
		{"run", "--dry-run"},
		{"resume", "--id", "codex-window", "--confirm"},
		{"schedule", "install", "--id", "codex-window", "--confirm"},
	}
	if len(operations.commands) != len(wantPrefixes) {
		t.Fatalf("commands = %#v", operations.commands)
	}
	for index, prefix := range wantPrefixes {
		if !slices.Equal(operations.commands[index][:len(prefix)], prefix) {
			t.Fatalf("command %d = %#v, prefix %#v", index, operations.commands[index], prefix)
		}
	}
	add := operations.commands[0]
	if slices.Contains(add, "--enabled") || slices.Contains(add, "--confirm") {
		t.Fatalf("schedule must be created disabled first: %#v", add)
	}
	if operations.config.Language != "en" || !operations.config.SetupCompleted {
		t.Fatalf("preferences = %+v", operations.config)
	}
}

func TestRussianSelectionLocalizesTheNextScreen(t *testing.T) {
	prompt := &scriptedPrompt{selects: []string{"ru", "exit"}, confirms: []bool{true}}
	operations := &fakeOperations{}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}

	if err := app.RunSetup(); err != nil {
		t.Fatal(err)
	}
	if len(prompt.calls) < 2 || prompt.calls[1].Title != "Добро пожаловать в open-agent-clock" {
		t.Fatalf("calls = %+v", prompt.calls)
	}
	if operations.config.Language != "ru" || operations.config.SetupCompleted {
		t.Fatalf("preferences = %+v", operations.config)
	}
}

func TestCancellationBeforeLanguageDoesNotInitialize(t *testing.T) {
	prompt := &scriptedPrompt{failAt: 1}
	operations := &fakeOperations{}
	var output bytes.Buffer
	app := App{Prompt: prompt, Ops: operations, Out: &output, Lang: "en"}

	if err := app.RunSetup(); err != nil {
		t.Fatal(err)
	}
	if operations.initialized || len(operations.commands) != 0 {
		t.Fatalf("operations changed after cancellation: %+v", operations)
	}
	if !strings.Contains(output.String(), "Cancelled") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestManagementMenusExposeAllTopLevelAreas(t *testing.T) {
	prompt := &scriptedPrompt{selects: []string{
		"schedules", "list", "back",
		"run", "back",
		"history", "back",
		"settings", "paths", "back",
		"targets", "dashboard", "exit",
	}}
	operations := &fakeOperations{initialized: true, config: appconfig.Config{SchemaVersion: appconfig.SchemaVersion, Language: "en", SetupCompleted: true}}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}

	if err := app.RunUI(); err != nil {
		t.Fatal(err)
	}
	assertOptions(t, prompt.calls[0].Options, "dashboard", "targets", "schedules", "run", "history", "settings", "setup", "exit")
	assertOptions(t, prompt.calls[1].Options, "list", "inspect", "create", "edit", "toggle", "launchd", "remove", "back")
	for _, expected := range [][]string{{"schedule", "list"}, {"config"}, {"detect"}, {"status"}} {
		if !containsCommand(operations.commands, expected) {
			t.Fatalf("missing command %#v in %#v", expected, operations.commands)
		}
	}
}

func TestCancellationAfterReviewDoesNotCreateSchedule(t *testing.T) {
	prompt := &scriptedPrompt{
		selects:  []string{"en", "native-codex", "interval"},
		inputs:   []string{"codex-window", "Codex window", "5h3m", "UTC", "hi"},
		confirms: []bool{true, false},
	}
	operations := &fakeOperations{bindings: []domain.Binding{{ID: "native-codex", Provider: "openai-codex", DisplayName: "Native Codex", AuthMode: "chatgpt", Available: true}}}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}
	if err := app.RunSetup(); err != nil {
		t.Fatal(err)
	}
	if len(operations.commands) != 0 || operations.config.SetupCompleted {
		t.Fatalf("setup changed state before review confirmation: %+v", operations)
	}
}

func TestNoTargetExitDoesNotCreateSchedule(t *testing.T) {
	prompt := &scriptedPrompt{selects: []string{"en", "exit"}, confirms: []bool{true}}
	operations := &fakeOperations{}
	app := App{Prompt: prompt, Ops: operations, Out: &bytes.Buffer{}, Lang: "en"}
	if err := app.RunSetup(); err != nil {
		t.Fatal(err)
	}
	if len(operations.commands) != 0 || operations.config.SetupCompleted {
		t.Fatalf("no-target exit changed state: %+v", operations)
	}
}

func TestEveryMessageHasEnglishAndRussianText(t *testing.T) {
	for key, value := range messages {
		if strings.TrimSpace(value.EN) == "" || strings.TrimSpace(value.RU) == "" {
			t.Errorf("message %q is incomplete: %+v", key, value)
		}
		if text("en", key) == key || text("ru", key) == key {
			t.Errorf("message %q did not resolve", key)
		}
	}
}

func assertOptions(t *testing.T, options []Option, values ...string) {
	t.Helper()
	for _, expected := range values {
		if !slices.ContainsFunc(options, func(option Option) bool { return option.Value == expected }) {
			t.Errorf("missing option %q in %+v", expected, options)
		}
	}
}

func containsCommand(commands [][]string, expected []string) bool {
	return slices.ContainsFunc(commands, func(command []string) bool { return slices.Equal(command, expected) })
}
