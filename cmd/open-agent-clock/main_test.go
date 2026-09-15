package main

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	"github.com/AlekseyBeketov/open-agent-clock/internal/macos/notification"
	"github.com/AlekseyBeketov/open-agent-clock/internal/provider"
	"github.com/AlekseyBeketov/open-agent-clock/internal/updater"
)

func TestIntervalScheduleDueUsesAlignedPollingWindow(t *testing.T) {
	start := time.Date(2026, time.September, 12, 6, 44, 0, 0, time.UTC)
	item := domain.Schedule{Mode: domain.ScheduleInterval, Interval: "5h3m", StartAt: start}

	if scheduleIsDue(item, start.Add(2*time.Hour), domain.RunResult{}, false) {
		t.Fatal("schedule became due before the first interval elapsed")
	}
	if !scheduleIsDue(item, start.Add(5*time.Hour+3*time.Minute+30*time.Second), domain.RunResult{}, false) {
		t.Fatal("schedule was not due inside the aligned polling window")
	}
	if scheduleIsDue(item, start.Add(5*time.Hour+5*time.Minute), domain.RunResult{}, false) {
		t.Fatal("missed interval was caught up outside the polling window")
	}

	lastRun := domain.RunResult{EndedAt: start.Add(10 * time.Minute)}
	if !scheduleIsDue(item, lastRun.EndedAt.Add(5*time.Hour+3*time.Minute+20*time.Second), lastRun, true) {
		t.Fatal("schedule was not due after the interval from the last completed run")
	}
}

func TestDailyScheduleDueUsesPollingWindow(t *testing.T) {
	item := domain.Schedule{
		Mode:     domain.ScheduleDaily,
		Times:    []string{"05:00"},
		Timezone: "UTC",
	}
	candidate := time.Date(2026, time.September, 12, 5, 0, 0, 0, time.UTC)
	if !scheduleIsDue(item, candidate.Add(30*time.Second), domain.RunResult{}, false) {
		t.Fatal("daily schedule was not due inside the polling window")
	}
	if scheduleIsDue(item, candidate.Add(5*time.Minute), domain.RunResult{}, false) {
		t.Fatal("missed daily occurrence was caught up outside the polling window")
	}
}

func TestLaunchdSpecUsesCalendarTriggerForSystemTimezone(t *testing.T) {
	t.Setenv("TZ", "Europe/Moscow")
	item := domain.Schedule{
		ID:       "daily-window",
		TargetID: "native-codex",
		Provider: "openai-codex",
		Prompt:   "hi",
		Mode:     domain.ScheduleDaily,
		Times:    []string{"05:00", "13:30:45"},
		Timezone: "Europe/Moscow",
		Enabled:  true,
	}
	spec, err := launchdSpec(appconfig.Paths{Root: t.TempDir()}, item)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Interval != 0 {
		t.Fatalf("calendar schedule unexpectedly uses polling: %s", spec.Interval)
	}
	if !reflect.DeepEqual(spec.DailyTimes, item.Times) {
		t.Fatalf("calendar times = %#v, want %#v", spec.DailyTimes, item.Times)
	}
}

func TestLaunchdSpecPollsCalendarScheduleOutsideSystemTimezone(t *testing.T) {
	t.Setenv("TZ", "Europe/Moscow")
	item := domain.Schedule{
		ID:       "new-york-daily",
		TargetID: "native-codex",
		Provider: "openai-codex",
		Prompt:   "hi",
		Mode:     domain.ScheduleDaily,
		Times:    []string{"05:00"},
		Timezone: "America/New_York",
		Enabled:  true,
	}
	spec, err := launchdSpec(appconfig.Paths{Root: t.TempDir()}, item)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Interval != time.Minute || len(spec.DailyTimes) != 0 {
		t.Fatalf("timezone-mismatched calendar trigger = interval %s, times %#v", spec.Interval, spec.DailyTimes)
	}
}

func TestLaunchdSpecRejectsSubminuteInterval(t *testing.T) {
	item := domain.Schedule{
		ID:       "fast-window",
		TargetID: "native-codex",
		Provider: "openai-codex",
		Prompt:   "hi",
		Mode:     domain.ScheduleInterval,
		Interval: "30s",
		Timezone: "UTC",
		Enabled:  true,
	}
	if _, err := launchdSpec(appconfig.Paths{Root: t.TempDir()}, item); err == nil {
		t.Fatal("sub-minute interval was accepted for launchd materialization")
	}
}

func TestLaunchdSpecPollsIntervalSchedulesEveryMinute(t *testing.T) {
	item := domain.Schedule{
		ID:       "native-codex-window",
		TargetID: "native-codex",
		Provider: "openai-codex",
		Prompt:   "hi",
		Mode:     domain.ScheduleInterval,
		Interval: "5h3m",
		Timezone: "Europe/Moscow",
		Enabled:  true,
		StartAt:  time.Now(),
	}
	spec, err := launchdSpec(appconfig.Paths{Root: t.TempDir()}, item)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Interval != time.Minute {
		t.Fatalf("launchd polling interval = %s, want %s", spec.Interval, time.Minute)
	}
	if len(spec.ProgramArguments) < 2 || spec.ProgramArguments[1] != "tick" {
		t.Fatalf("unexpected program arguments: %#v", spec.ProgramArguments)
	}
	if spec.Environment["HOME"] == "" {
		t.Fatal("launchd HOME environment is empty")
	}
	if spec.Environment["PATH"] != "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" {
		t.Fatalf("unexpected launchd PATH: %q", spec.Environment["PATH"])
	}
}

func TestRedactedExecutionPlanHidesCredentialLikeArguments(t *testing.T) {
	original := domain.ExecutionPlan{
		Executable: "/bin/provider",
		Args:       []string{"--api-key", "synthetic-argument-secret", "--safe", "value"},
		Prompt:     "access_token=synthetic-prompt-secret",
	}
	display := redactedExecutionPlan(original)
	if display.Args[1] != "[REDACTED]" || display.Prompt != "access_token=[REDACTED]" {
		t.Fatalf("execution plan display was not redacted")
	}
	if original.Args[1] != "synthetic-argument-secret" || original.Prompt != "access_token=synthetic-prompt-secret" {
		t.Fatal("execution plan redaction mutated execution data")
	}
}

func TestExecutionPlanKeepsJSONDevOnly(t *testing.T) {
	binding := domain.Binding{
		ID: "native-codex", Provider: "openai-codex", Backend: domain.NativeCodexCLI,
		Executable: "/opt/homebrew/bin/codex", AuthMode: "chatgpt", Available: true,
		Capabilities: []domain.Capability{domain.CapabilityEphemeral},
	}
	adapter, err := provider.ForBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	normal, err := buildExecutionPlan(adapter, binding, "hi", false)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := buildExecutionPlan(adapter, binding, "hi", true)
	if err != nil {
		t.Fatal(err)
	}
	if containsArg(normal.Args, "--json") {
		t.Fatalf("normal execution plan enabled JSON: %#v", normal.Args)
	}
	if !containsArg(normal.Args, "--skip-git-repo-check") || !containsArg(dev.Args, "--skip-git-repo-check") || !containsArg(dev.Args, "--json") {
		t.Fatalf("execution plans missed required flags: normal=%#v dev=%#v", normal.Args, dev.Args)
	}
}

func TestPrintRunDiagnosticsShowsProviderReportedUsageWithoutRawOutput(t *testing.T) {
	result := domain.RunResult{
		JobID: "codex-window",
		TokenUsage: &domain.TokenUsage{
			Availability: domain.TokenUsageAvailable, Source: "provider-reported",
			InputTokens: 17510, CachedInputTokens: 9984, OutputTokens: 14, TotalTokens: 17524,
		},
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	printRunDiagnostics(result)
	_ = writer.Close()
	os.Stdout = previous
	contents, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	output := string(contents)
	for _, expected := range []string{"token usage: available", "input tokens: 17510", "cached input tokens: 9984", "output tokens: 14", "total tokens: 17524"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("diagnostic output missing %q: %s", expected, output)
		}
	}
	if strings.Contains(output, "prompt") || strings.Contains(output, "response") || strings.Contains(output, "stdout") || strings.Contains(output, "stderr") {
		t.Fatalf("diagnostic output exposed raw provider data: %s", output)
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

type fakeAutomaticUpdater struct {
	installCalls int
	current      string
	confirmed    bool
	bounded      bool
	events       *[]string
	result       updater.Result
	err          error
}

func (fake *fakeAutomaticUpdater) Check(context.Context, string) (updater.Result, error) {
	return updater.Result{}, nil
}

func (fake *fakeAutomaticUpdater) Install(ctx context.Context, current string, confirm bool) (updater.Result, error) {
	fake.installCalls++
	fake.current = current
	fake.confirmed = confirm
	_, fake.bounded = ctx.Deadline()
	if fake.events != nil {
		*fake.events = append(*fake.events, "update")
	}
	return fake.result, fake.err
}

func TestAutomaticUpdatesAreDisabledWithoutCallingUpdater(t *testing.T) {
	paths := appconfig.PathsForHome(t.TempDir())
	if err := appconfig.Init(paths); err != nil {
		t.Fatal(err)
	}
	cfg := appconfig.DefaultConfig()
	item := domain.Schedule{ID: "codex-window", Mode: domain.ScheduleInterval, Interval: "5h3m"}
	cfg.Schedules = []domain.Schedule{item}
	state := appconfig.DefaultState()
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		t.Fatal(err)
	}
	if err := appconfig.SaveState(paths, state); err != nil {
		t.Fatal(err)
	}

	factoryCalls := 0
	previousFactory := newUpdaterService
	newUpdaterService = func() updaterService {
		factoryCalls++
		return &fakeAutomaticUpdater{}
	}
	defer func() { newUpdaterService = previousFactory }()

	maybeAutomaticUpdate(paths, cfg, item, false)
	if factoryCalls != 0 {
		t.Fatalf("disabled automatic updates created updater service %d times", factoryCalls)
	}
	loaded, err := appconfig.LoadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Updates.LastResult != nil {
		t.Fatalf("disabled automatic updates persisted a result: %+v", loaded.Updates.LastResult)
	}
}

func TestDueTickAttemptsOneBoundedUpdateBeforeProviderAndPersistsFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths := appconfig.PathsForHome(home)
	if err := appconfig.Init(paths); err != nil {
		t.Fatal(err)
	}
	item := domain.Schedule{
		ID: "codex-window", Name: "Codex window", TargetID: "fake-provider", Provider: domain.ProviderOpenAICodex,
		Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "1h", Timezone: "UTC", Enabled: true,
		StartAt: time.Now().UTC().Add(-time.Hour - 30*time.Second),
	}
	cfg := appconfig.DefaultConfig()
	cfg.SetupCompleted = true
	cfg.Schedules = []domain.Schedule{item}
	cfg.Updates = appconfig.UpdateConfig{Enabled: true, AlignScheduleID: item.ID}
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		t.Fatal(err)
	}
	if err := appconfig.SaveState(paths, appconfig.DefaultState()); err != nil {
		t.Fatal(err)
	}

	events := []string{}
	fake := &fakeAutomaticUpdater{
		events: &events,
		result: updater.Result{Operation: "install", Status: "failed", CurrentVersion: "dev", Reason: "synthetic network failure", RecordedAt: time.Now().UTC()},
		err:    errors.New("synthetic network failure"),
	}
	previousFactory := newUpdaterService
	previousInvocation := runScheduledInvocation
	newUpdaterService = func() updaterService { return fake }
	runScheduledInvocation = func(string, bool, bool) error {
		events = append(events, "provider")
		return nil
	}
	defer func() {
		newUpdaterService = previousFactory
		runScheduledInvocation = previousInvocation
	}()

	if err := tickCommand([]string{"--schedule", item.ID, "--confirm"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"update", "provider"}) {
		t.Fatalf("tick order = %#v", events)
	}
	if fake.installCalls != 1 || fake.current != version || !fake.confirmed || !fake.bounded {
		t.Fatalf("automatic update invocation = calls:%d current:%q confirmed:%t bounded:%t", fake.installCalls, fake.current, fake.confirmed, fake.bounded)
	}
	state, err := appconfig.LoadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if state.Updates.LastResult == nil || state.Updates.LastResult.Status != "failed" || state.Updates.LastResult.Reason != "synthetic network failure" {
		t.Fatalf("last automatic update result = %+v", state.Updates.LastResult)
	}
}

type fakeCompletionNotifier struct {
	payloads []notification.Payload
	err      error
}

func (fake *fakeCompletionNotifier) Notify(ctx context.Context, payload notification.Payload) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("notification context was not bounded")
	}
	fake.payloads = append(fake.payloads, payload)
	return fake.err
}

func TestCliOperationsPersistNotificationPreference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	operations := cliOperations{}
	if err := operations.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := operations.SaveNotificationSettings(true); err != nil {
		t.Fatal(err)
	}
	paths := appconfig.PathsForHome(home)
	cfg, err := appconfig.LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Notifications.Enabled {
		t.Fatal("enabled notification preference was not persisted")
	}
	if err := operations.SaveNotificationSettings(false); err != nil {
		t.Fatal(err)
	}
	cfg, err = appconfig.LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notifications.Enabled {
		t.Fatal("disabled notification preference was not persisted")
	}
}

func TestNotifyRunCompletionClassifiesOnlyProviderOutcome(t *testing.T) {
	fake := &fakeCompletionNotifier{}
	previousFactory := newNotificationNotifier
	newNotificationNotifier = func() notification.Notifier { return fake }
	defer func() { newNotificationNotifier = previousFactory }()

	cfg := appconfig.Config{Notifications: appconfig.NotificationConfig{Enabled: true}}
	item := domain.Schedule{ID: "codex-window", Prompt: "private prompt"}
	binding := domain.Binding{ID: "native-codex"}

	notifyRunCompletion(cfg, item, binding, domain.RunResult{Status: string(provider.StatusSuccess), Reason: "provider output must not be copied"})
	notifyRunCompletion(cfg, item, binding, domain.RunResult{Status: string(provider.StatusProviderFailed), Reason: "private provider reason"})

	if len(fake.payloads) != 2 {
		t.Fatalf("notification calls = %d, want 2", len(fake.payloads))
	}
	want := []notification.Classification{notification.CompletionSuccess, notification.CompletionFailure}
	for index, payload := range fake.payloads {
		if payload.ScheduleID != item.ID || payload.TargetID != binding.ID || payload.Classification != want[index] {
			t.Fatalf("payload[%d] = %+v", index, payload)
		}
	}
}

func TestNotifyRunCompletionIsDisabledAndBestEffort(t *testing.T) {
	factoryCalls := 0
	previousFactory := newNotificationNotifier
	newNotificationNotifier = func() notification.Notifier {
		factoryCalls++
		return &fakeCompletionNotifier{err: errors.New("notification unavailable")}
	}
	defer func() { newNotificationNotifier = previousFactory }()

	notifyRunCompletion(appconfig.Config{}, domain.Schedule{ID: "codex-window"}, domain.Binding{ID: "native-codex"}, domain.RunResult{Status: string(provider.StatusSuccess)})
	if factoryCalls != 0 {
		t.Fatalf("disabled notification factory calls = %d, want 0", factoryCalls)
	}

	cfg := appconfig.Config{Notifications: appconfig.NotificationConfig{Enabled: true}}
	result := domain.RunResult{Status: string(provider.StatusProviderFailed), Reason: "authoritative provider result"}
	before := result
	notifyRunCompletion(cfg, domain.Schedule{ID: "codex-window"}, domain.Binding{ID: "native-codex"}, result)
	if !reflect.DeepEqual(result, before) {
		t.Fatalf("notifier changed provider result: before=%+v after=%+v", before, result)
	}
	if factoryCalls != 1 {
		t.Fatalf("enabled notification factory calls = %d, want 1", factoryCalls)
	}
}
