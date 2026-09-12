package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/discovery"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	runner "github.com/AlekseyBeketov/open-agent-clock/internal/exec"
	"github.com/AlekseyBeketov/open-agent-clock/internal/history"
	"github.com/AlekseyBeketov/open-agent-clock/internal/lock"
	"github.com/AlekseyBeketov/open-agent-clock/internal/macos/launchd"
	"github.com/AlekseyBeketov/open-agent-clock/internal/plan"
	"github.com/AlekseyBeketov/open-agent-clock/internal/provider"
	"github.com/AlekseyBeketov/open-agent-clock/internal/redact"
	scheduleengine "github.com/AlekseyBeketov/open-agent-clock/internal/schedule"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		if isInteractiveTerminal(os.Stdin, os.Stdout) {
			return interactiveCommand(false)
		}
		printHelp()
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Println(version)
		return nil
	}

	switch args[0] {
	case "setup":
		return interactiveCommand(true)
	case "ui":
		return interactiveCommand(false)
	case "detect":
		return detectCommand(args[1:])
	case "run":
		return runCommand(args[1:])
	case "history":
		return historyCommand(args[1:])
	case "last-run":
		return lastRunCommand(args[1:])
	case "tick":
		return tickCommand(args[1:])
	case "init":
		return initCommand(args[1:])
	case "config":
		return configCommand(args[1:])
	case "status":
		return statusCommand(args[1:])
	case "pause":
		return setScheduleEnabled(args[1:], false)
	case "resume":
		return setScheduleEnabled(args[1:], true)
	case "schedule":
		return scheduleCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func detectCommand(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	bindings := discovery.Discover()
	if *jsonOutput {
		return renderJSON(bindings)
	}
	for _, binding := range bindings {
		status := "unavailable"
		if binding.Available {
			status = "available"
		}
		fmt.Printf("%s [%s] %s\n", binding.ID, status, binding.DisplayName)
		fmt.Printf("  provider: %s\n", binding.Provider)
		fmt.Printf("  backend: %s\n", binding.Backend)
		fmt.Printf("  executable: %s\n", valueOrUnknown(binding.Executable))
		fmt.Printf("  version: %s\n", valueOrUnknown(binding.Version))
		fmt.Printf("  auth: %s\n", valueOrUnknown(binding.AuthMode))
		fmt.Printf("  auth store: %s\n", valueOrUnknown(binding.AuthStore))
		fmt.Printf("  identity: %s\n", binding.IdentityStatus)
		if binding.IdentityHash != "" {
			fmt.Printf("  identity fingerprint: %s\n", binding.IdentityHash)
		}
		if binding.Reason != "" {
			fmt.Printf("  reason: %s\n", binding.Reason)
		}
	}
	return nil
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "render the execution plan without invoking a provider")
	once := fs.Bool("once", false, "execute one configured schedule")
	scheduleID := fs.String("schedule", "", "schedule id for --once")
	target := fs.String("target", "", "binding id, for example native-codex or hermes-codex")
	prompt := fs.String("prompt", "hi", "minimal provider prompt")
	confirm := fs.Bool("confirm", false, "explicitly confirm a real provider invocation")
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *once {
		if *dryRun {
			return errors.New("--once and --dry-run cannot be used together")
		}
		return runOnceCommand(*scheduleID, *confirm, *jsonOutput)
	}
	if !*dryRun {
		return errors.New("provider invocation is fail-closed; use --dry-run or --once --schedule <id> --confirm")
	}
	if strings.TrimSpace(*target) == "" {
		return errors.New("--target is required")
	}

	var binding domain.Binding
	found := false
	for _, candidate := range discovery.Discover() {
		if candidate.ID == *target {
			binding = candidate
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown target %q", *target)
	}
	planned, err := plan.ForBinding(binding, *prompt)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return renderJSON(planned)
	}
	fmt.Println("DRY RUN — provider will not be invoked")
	fmt.Printf("binding: %s\n", planned.BindingID)
	fmt.Printf("provider: %s\n", planned.Provider)
	fmt.Printf("backend: %s\n", planned.Backend)
	fmt.Printf("executable: %s\n", planned.Executable)
	fmt.Printf("args: %q\n", planned.Args)
	fmt.Printf("working directory: %s\n", planned.WorkingDir)
	fmt.Printf("auth mode: %s\n", valueOrUnknown(planned.AuthMode))
	fmt.Printf("prompt: %q\n", planned.Prompt)
	fmt.Printf("timeout: %s\n", planned.Timeout)
	fmt.Printf("side effect: %s\n", planned.SideEffect)
	return nil
}

func runOnceCommand(scheduleID string, confirm, jsonOutput bool) error {
	if strings.TrimSpace(scheduleID) == "" {
		return errors.New("--schedule is required with --once")
	}
	paths, cfg, state, err := loadStore()
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, scheduleID)
	if !found {
		return fmt.Errorf("schedule %q was not found", scheduleID)
	}
	if !item.Enabled {
		return errors.New("cannot run a paused schedule")
	}
	if !confirm {
		return errors.New("real provider invocation requires explicit --confirm; no process was started")
	}
	binding, found := findBinding(item.TargetID)
	if !found {
		return fmt.Errorf("unknown target %q", item.TargetID)
	}
	if !binding.Available {
		return fmt.Errorf("target %q is unavailable: %s", item.TargetID, binding.Reason)
	}
	if !domain.IsSubscriptionAuthMode(binding.AuthMode) {
		return errors.New("usage-window schedules require a subscription/OAuth target; API-key targets are not supported")
	}
	adapter, err := provider.ForBinding(binding)
	if err != nil {
		return err
	}
	planned, err := adapter.BuildPlan(binding, item.Prompt)
	if err != nil {
		return err
	}
	printConsentWarning(jsonOutput)
	if jsonOutput {
		fmt.Fprintf(os.Stderr, "confirmed command: %s %q\n", planned.Executable, planned.Args)
	} else {
		fmt.Printf("confirmed command: %s %q\n", planned.Executable, planned.Args)
	}

	startedAt := time.Now().UTC()
	result := domain.RunResult{BindingID: binding.ID, JobID: item.ID, StartedAt: startedAt, Version: binding.Version}
	jobLock, lockErr := lock.Acquire(filepath.Join(paths.Locks, item.ID+".lock"))
	if lockErr != nil {
		if errors.Is(lockErr, lock.ErrAlreadyHeld) {
			result.EndedAt = time.Now().UTC()
			result.Duration = result.EndedAt.Sub(result.StartedAt)
			result.Status = string(provider.StatusSkipped)
			result.Reason = "another invocation for this schedule is already running"
			if persistErr := persistRun(paths, &state, result); persistErr != nil {
				return persistErr
			}
			if jsonOutput {
				if err := renderJSON(result); err != nil {
					return err
				}
			}
			return nil
		}
		return lockErr
	}
	defer jobLock.Release()

	timeout, err := time.ParseDuration(planned.Timeout)
	if err != nil {
		return fmt.Errorf("invalid execution timeout: %w", err)
	}
	execution := runner.Run(context.Background(), planned.Executable, planned.Args, timeout)
	result.EndedAt = time.Now().UTC()
	result.Duration = result.EndedAt.Sub(result.StartedAt)
	status, reason := adapter.Classify(execution.ExitCode, execution.TimedOut, redact.Text(execution.Stdout), redact.Text(execution.Stderr))
	result.Status = string(status)
	result.Reason = redact.Text(reason)
	result.ExitCode = execution.ExitCode
	if persistErr := persistRun(paths, &state, result); persistErr != nil {
		return persistErr
	}
	if jsonOutput {
		if err := renderJSON(result); err != nil {
			return err
		}
	} else {
		fmt.Printf("result: %s\n", result.Status)
		fmt.Printf("duration: %s\n", result.Duration)
		fmt.Printf("reason: %s\n", result.Reason)
	}
	if result.Status != string(provider.StatusSuccess) {
		return fmt.Errorf("provider run classified as %s: %s", result.Status, result.Reason)
	}
	return nil
}

func printConsentWarning(jsonOutput bool) {
	writer := os.Stdout
	if jsonOutput {
		writer = os.Stderr
	}
	fmt.Fprintln(writer, "WARNING: this invokes an official provider CLI and may consume subscription allowance; it does not guarantee a reset, additional capacity, or acceptance by the provider.")
}

func findBinding(id string) (domain.Binding, bool) {
	for _, binding := range discovery.Discover() {
		if binding.ID == id {
			return binding, true
		}
	}
	return domain.Binding{}, false
}

func persistRun(paths appconfig.Paths, _ *appconfig.State, result domain.RunResult) error {
	stateLock, err := lock.Acquire(filepath.Join(paths.Locks, "state.lock"))
	if err != nil {
		return fmt.Errorf("lock state for update: %w", err)
	}
	defer stateLock.Release()
	state, err := appconfig.LoadState(paths)
	if err != nil {
		return err
	}
	if state.LastRuns == nil {
		state.LastRuns = map[string]domain.RunResult{}
	}
	state.LastRuns[result.JobID] = result
	if err := appconfig.SaveState(paths, state); err != nil {
		return err
	}
	return history.Append(paths, result, history.DefaultRetention)
}

func historyCommand(args []string) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, _, _, err := loadStore()
	if err != nil {
		return err
	}
	results, err := history.List(paths)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return renderJSON(results)
	}
	for _, result := range results {
		fmt.Printf("%s schedule=%s target=%s status=%s duration=%s\n", result.EndedAt.Format(time.RFC3339), result.JobID, result.BindingID, result.Status, result.Duration)
	}
	if len(results) == 0 {
		fmt.Println("no run history")
	}
	return nil
}

func lastRunCommand(args []string) error {
	fs := flag.NewFlagSet("last-run", flag.ContinueOnError)
	id := fs.String("schedule", "", "schedule id")
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--schedule is required")
	}
	_, _, state, err := loadStore()
	if err != nil {
		return err
	}
	result, found := state.LastRuns[*id]
	if !found {
		return fmt.Errorf("no run result for schedule %q", *id)
	}
	if *jsonOutput {
		return renderJSON(result)
	}
	fmt.Printf("schedule: %s\n", result.JobID)
	fmt.Printf("target: %s\n", result.BindingID)
	fmt.Printf("status: %s\n", result.Status)
	fmt.Printf("ended: %s\n", result.EndedAt.Format(time.RFC3339))
	fmt.Printf("duration: %s\n", result.Duration)
	fmt.Printf("reason: %s\n", result.Reason)
	return nil
}
func tickCommand(args []string) error {
	fs := flag.NewFlagSet("tick", flag.ContinueOnError)
	id := fs.String("schedule", "", "schedule id")
	confirm := fs.Bool("confirm", false, "confirm a foreground provider invocation")
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--schedule is required")
	}
	paths, cfg, state, err := loadStore()
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, *id)
	if !found {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	if !item.Enabled {
		return errors.New("cannot tick a paused schedule")
	}
	now := time.Now()
	lastRun, hasLastRun := state.LastRuns[item.ID]
	if !scheduleIsDue(item, now, lastRun, hasLastRun) {
		next, nextErr := scheduleengine.NextRun(item, now, resultTimePointer(lastRun, hasLastRun))
		if nextErr != nil {
			return nextErr
		}
		fmt.Printf("schedule %s is not due; next=%s\n", item.ID, next.Format(time.RFC3339))
		return nil
	}
	_ = paths
	return runOnceCommand(item.ID, *confirm, *jsonOutput)
}

func scheduleIsDue(item domain.Schedule, now time.Time, lastRun domain.RunResult, hasLastRun bool) bool {
	if item.Mode == domain.ScheduleInterval {
		interval, err := scheduleengine.ParseInterval(item.Interval)
		if err != nil {
			return false
		}
		anchor := item.StartAt
		if hasLastRun && !lastRun.EndedAt.IsZero() {
			anchor = lastRun.EndedAt.Add(interval)
		}
		return !anchor.After(now)
	}
	location, err := time.LoadLocation(item.Timezone)
	if err != nil {
		return false
	}
	localNow := now.In(location)
	for _, clock := range item.Times {
		candidate, parseErr := time.ParseInLocation("15:04", strings.TrimSpace(clock), location)
		if parseErr != nil {
			candidate, parseErr = time.ParseInLocation("15:04:05", strings.TrimSpace(clock), location)
		}
		if parseErr != nil {
			continue
		}
		candidate = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), candidate.Hour(), candidate.Minute(), candidate.Second(), 0, location)
		if !candidate.After(localNow) && (!hasLastRun || lastRun.EndedAt.Before(candidate)) {
			return true
		}
	}
	return false
}

func resultTimePointer(result domain.RunResult, exists bool) *time.Time {
	if !exists || result.EndedAt.IsZero() {
		return nil
	}
	value := result.EndedAt
	return &value
}

func initCommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return err
	}
	if err := appconfig.Init(paths); err != nil {
		return err
	}
	fmt.Printf("initialized local storage: %s\n", paths.Root)
	fmt.Printf("config: %s\n", paths.Config)
	fmt.Printf("state: %s\n", paths.State)
	return nil
}

func configCommand(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	value := struct {
		Paths  appconfig.Paths  `json:"paths"`
		Config appconfig.Config `json:"config"`
	}{Paths: paths, Config: cfg}
	if *jsonOutput {
		return renderJSON(value)
	}
	fmt.Printf("root: %s\n", paths.Root)
	fmt.Printf("config: %s\n", paths.Config)
	fmt.Printf("state: %s\n", paths.State)
	fmt.Printf("schedules: %d\n", len(cfg.Schedules))
	fmt.Printf("language: %s\n", cfg.Language)
	fmt.Printf("setup completed: %t\n", cfg.SetupCompleted)
	fmt.Printf("consent acknowledged: %t\n", cfg.ConsentAcknowledged)
	return nil
}

func statusCommand(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, cfg, state, err := loadStore()
	if err != nil {
		return err
	}
	views := make([]scheduleStatus, 0, len(cfg.Schedules))
	now := time.Now()
	for _, item := range cfg.Schedules {
		view := scheduleStatus{ID: item.ID, Name: item.Name, TargetID: item.TargetID, Mode: item.Mode, Timezone: item.Timezone, Enabled: item.Enabled}
		if result, ok := state.LastRuns[item.ID]; ok {
			copy := result
			view.LastResult = &copy
		}
		if !item.Enabled {
			view.NextRun = "paused"
		} else {
			var lastRun *time.Time
			if result, ok := state.LastRuns[item.ID]; ok && !result.EndedAt.IsZero() {
				endedAt := result.EndedAt
				lastRun = &endedAt
			}
			next, nextErr := scheduleengine.NextRun(item, now, lastRun)
			if nextErr != nil {
				view.NextRun = "invalid: " + nextErr.Error()
				view.Error = nextErr.Error()
			} else {
				view.NextRun = next.Format(time.RFC3339)
			}
		}
		views = append(views, view)
	}
	if *jsonOutput {
		return renderJSON(views)
	}
	if len(views) == 0 {
		fmt.Println("no schedules configured")
		return nil
	}
	for _, view := range views {
		stateValue := "paused"
		if view.Enabled {
			stateValue = "enabled"
		}
		fmt.Printf("%s [%s] target=%s mode=%s timezone=%s next=%s\n", view.ID, stateValue, view.TargetID, view.Mode, view.Timezone, view.NextRun)
		if view.Error != "" {
			fmt.Printf("  error: %s\n", view.Error)
		}
		if view.LastResult != nil {
			fmt.Printf("  last result: %s at %s\n", view.LastResult.Status, view.LastResult.EndedAt.Format(time.RFC3339))
		}
	}
	return nil
}

type scheduleStatus struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	TargetID   string              `json:"target_id"`
	Mode       domain.ScheduleMode `json:"mode"`
	Timezone   string              `json:"timezone"`
	Enabled    bool                `json:"enabled"`
	NextRun    string              `json:"next_run"`
	Error      string              `json:"error,omitempty"`
	LastResult *domain.RunResult   `json:"last_result,omitempty"`
}

func setScheduleEnabled(args []string, enabled bool) error {
	fs := flag.NewFlagSet("schedule-state", flag.ContinueOnError)
	id := fs.String("id", "", "schedule id")
	confirm := fs.Bool("confirm", false, "confirm enabling a provider schedule")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	if enabled && !*confirm {
		printConsentWarning(false)
		return errors.New("enabling a provider schedule requires explicit --confirm; nothing was changed")
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	configLock, err := lock.Acquire(filepath.Join(paths.Locks, "config.lock"))
	if err != nil {
		return err
	}
	defer configLock.Release()
	cfg, err = appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, *id)
	if !found {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := launchd.NewManager(home)
	launchStatus := manager.Inspect(context.Background(), item)
	if !enabled && launchStatus.Loaded {
		if err := manager.Unload(context.Background(), item.ID); err != nil {
			return err
		}
	}
	previous := item
	item.Enabled = enabled
	item.UpdatedAt = time.Now().UTC()
	if enabled {
		cfg.ConsentAcknowledged = true
	}
	appconfig.UpsertSchedule(&cfg, item)
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		if !enabled && launchStatus.Loaded {
			_ = manager.Load(context.Background(), item.ID)
		}
		return err
	}
	if enabled && launchStatus.Installed && !launchStatus.Loaded {
		if err := manager.Load(context.Background(), item.ID); err != nil {
			appconfig.UpsertSchedule(&cfg, previous)
			_ = appconfig.SaveConfig(paths, cfg)
			return fmt.Errorf("schedule resumed in config but LaunchAgent could not be loaded: %w", err)
		}
	}
	if enabled {
		fmt.Printf("schedule %s resumed; no provider process was started\n", item.ID)
	} else {
		fmt.Printf("schedule %s paused; no provider process was started\n", item.ID)
	}
	return nil
}

func scheduleCommand(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("Usage:")
		fmt.Println("  open-agent-clock schedule add --id <id> --target <target> [--mode interval|morning|daily]")
		fmt.Println("  open-agent-clock schedule update --id <id> [options]")
		fmt.Println("  open-agent-clock schedule list [--json]")
		fmt.Println("  open-agent-clock schedule install --id <id> [--dry-run] [--confirm]")
		fmt.Println("  open-agent-clock schedule status [--id <id>] [--json]")
		fmt.Println("  open-agent-clock schedule uninstall --id <id> --confirm")
		fmt.Println("  open-agent-clock schedule remove --id <id> --confirm")
		return nil
	}
	switch args[0] {
	case "add":
		return addScheduleCommand(args[1:])
	case "update":
		return updateScheduleCommand(args[1:])
	case "list":
		return statusCommand(args[1:])
	case "install":
		return installScheduleCommand(args[1:])
	case "status":
		return launchdStatusCommand(args[1:])
	case "uninstall":
		return uninstallScheduleCommand(args[1:])
	case "remove":
		return removeScheduleCommand(args[1:])
	default:
		return fmt.Errorf("unknown schedule command %q", args[0])
	}
}

func addScheduleCommand(args []string) error {
	fs := flag.NewFlagSet("schedule add", flag.ContinueOnError)
	id := fs.String("id", "", "stable schedule id")
	name := fs.String("name", "", "human-readable name")
	target := fs.String("target", "", "execution binding id")
	provider := fs.String("provider", string(domain.ProviderOpenAICodex), "provider name")
	prompt := fs.String("prompt", "hi", "minimal provider prompt")
	mode := fs.String("mode", string(domain.ScheduleInterval), "interval, morning, or daily")
	interval := fs.String("interval", "5h3m", "interval duration")
	times := fs.String("times", "", "comma-separated daily times, for example 05:00,13:00")
	dailyTime := fs.String("time", "", "single daily time alias")
	timezone := fs.String("timezone", appconfig.SystemTimezone(), "IANA timezone")
	enabled := fs.Bool("enabled", false, "create enabled; default is disabled")
	confirm := fs.Bool("confirm", false, "confirm enabling a provider schedule")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	if strings.TrimSpace(*target) == "" {
		return errors.New("--target is required")
	}
	if *enabled && !*confirm {
		printConsentWarning(false)
		return errors.New("enabling a provider schedule requires explicit --confirm; create it disabled instead")
	}
	resolvedMode := domain.ScheduleMode(strings.TrimSpace(*mode))
	if resolvedMode == "morning" {
		resolvedMode = domain.ScheduleDaily
		if strings.TrimSpace(*times) == "" {
			*times = "05:00"
		}
	}
	resolvedTimes := splitCSV(*times)
	if len(resolvedTimes) == 0 && strings.TrimSpace(*dailyTime) != "" {
		resolvedTimes = []string{strings.TrimSpace(*dailyTime)}
	}
	now := time.Now().UTC()
	item := domain.Schedule{
		ID: *id, Name: strings.TrimSpace(*name), TargetID: *target, Provider: domain.Provider(strings.TrimSpace(*provider)), Prompt: *prompt,
		Mode: resolvedMode, Interval: *interval, Times: resolvedTimes, Timezone: *timezone, Enabled: *enabled,
		StartAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if item.Name == "" {
		item.Name = item.ID
	}
	if err := scheduleengine.Validate(item); err != nil {
		return err
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	configLock, err := lock.Acquire(filepath.Join(paths.Locks, "config.lock"))
	if err != nil {
		return err
	}
	defer configLock.Release()
	cfg, err = appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	if _, found := appconfig.FindSchedule(cfg, item.ID); found {
		return fmt.Errorf("schedule %q already exists", item.ID)
	}
	appconfig.UpsertSchedule(&cfg, item)
	if item.Enabled {
		cfg.ConsentAcknowledged = true
	}
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		return err
	}
	fmt.Printf("schedule %s created (%s, enabled=%t); no provider process was started\n", item.ID, item.Mode, item.Enabled)
	return nil
}

func updateScheduleCommand(args []string) error {
	fs := flag.NewFlagSet("schedule update", flag.ContinueOnError)
	id := fs.String("id", "", "schedule id")
	name := fs.String("name", "", "human-readable name")
	target := fs.String("target", "", "execution binding id")
	provider := fs.String("provider", "", "provider name")
	prompt := fs.String("prompt", "", "minimal provider prompt")
	mode := fs.String("mode", "", "interval, morning, or daily")
	interval := fs.String("interval", "", "interval duration")
	times := fs.String("times", "", "comma-separated daily times")
	dailyTime := fs.String("time", "", "single daily time alias")
	timezone := fs.String("timezone", "", "IANA timezone")
	enabled := fs.String("enabled", "", "true or false")
	confirm := fs.Bool("confirm", false, "confirm enabling a provider schedule")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	configLock, err := lock.Acquire(filepath.Join(paths.Locks, "config.lock"))
	if err != nil {
		return err
	}
	defer configLock.Release()
	cfg, err = appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, *id)
	if !found {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	previous := item
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := launchd.NewManager(home)
	launchStatus := manager.Inspect(context.Background(), item)
	if launchStatus.Loaded && !*confirm {
		return errors.New("updating an installed schedule requires explicit --confirm so its LaunchAgent can be refreshed")
	}
	if flagWasSet(fs, "name") {
		item.Name = strings.TrimSpace(*name)
	}
	if flagWasSet(fs, "target") {
		item.TargetID = strings.TrimSpace(*target)
	}
	if flagWasSet(fs, "provider") {
		item.Provider = domain.Provider(strings.TrimSpace(*provider))
	}
	if flagWasSet(fs, "prompt") {
		item.Prompt = *prompt
	}
	if flagWasSet(fs, "mode") {
		item.Mode = domain.ScheduleMode(strings.TrimSpace(*mode))
		if item.Mode == "morning" {
			item.Mode = domain.ScheduleDaily
			if !flagWasSet(fs, "times") && !flagWasSet(fs, "time") {
				item.Times = []string{"05:00"}
			}
		}
	}
	if flagWasSet(fs, "interval") {
		item.Interval = *interval
	}
	if flagWasSet(fs, "times") {
		item.Times = splitCSV(*times)
	}
	if flagWasSet(fs, "time") {
		item.Times = []string{strings.TrimSpace(*dailyTime)}
	}
	if flagWasSet(fs, "timezone") {
		item.Timezone = strings.TrimSpace(*timezone)
	}
	if flagWasSet(fs, "enabled") {
		value, parseErr := strconv.ParseBool(strings.TrimSpace(*enabled))
		if parseErr != nil {
			return fmt.Errorf("invalid --enabled value: %w", parseErr)
		}
		if value && !*confirm {
			printConsentWarning(false)
			return errors.New("enabling a provider schedule requires explicit --confirm; nothing was changed")
		}
		item.Enabled = value
		if value {
			cfg.ConsentAcknowledged = true
		}
	}
	item.UpdatedAt = time.Now().UTC()
	if err := scheduleengine.Validate(item); err != nil {
		return err
	}
	unloaded := false
	if !item.Enabled && launchStatus.Loaded {
		if err := manager.Unload(context.Background(), item.ID); err != nil {
			return err
		}
		unloaded = true
	}
	appconfig.UpsertSchedule(&cfg, item)
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		if unloaded {
			_ = manager.Load(context.Background(), item.ID)
		}
		return err
	}
	if launchStatus.Loaded && item.Enabled {
		spec, specErr := launchdSpec(paths, item)
		if specErr != nil {
			appconfig.UpsertSchedule(&cfg, previous)
			_ = appconfig.SaveConfig(paths, cfg)
			return specErr
		}
		if _, installErr := manager.Install(context.Background(), spec); installErr != nil {
			appconfig.UpsertSchedule(&cfg, previous)
			_ = appconfig.SaveConfig(paths, cfg)
			return fmt.Errorf("schedule updated in config but LaunchAgent refresh failed: %w", installErr)
		}
	}
	fmt.Printf("schedule %s updated; no provider process was started\n", item.ID)
	return nil
}

func installScheduleCommand(args []string) error {
	fs := flag.NewFlagSet("schedule install", flag.ContinueOnError)
	id := fs.String("id", "", "schedule id")
	confirm := fs.Bool("confirm", false, "confirm LaunchAgent installation")
	dryRun := fs.Bool("dry-run", false, "render the plist without writing or loading it")
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, *id)
	if !found {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	if !*dryRun && !item.Enabled {
		return errors.New("only enabled schedules can be installed; resume it first")
	}
	binding, found := findBinding(item.TargetID)
	if !found || !binding.Available {
		return fmt.Errorf("target %q is unavailable; schedule was not installed", item.TargetID)
	}
	spec, err := launchdSpec(paths, item)
	if err != nil {
		return err
	}
	plist, err := launchd.Generate(spec)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	manager := launchd.NewManager(home)
	preview := struct {
		Label     string   `json:"label"`
		PlistPath string   `json:"plist_path"`
		Command   []string `json:"command"`
		Plist     string   `json:"plist"`
	}{Label: launchd.Label(item.ID), PlistPath: manager.PlistPath(item.ID), Command: spec.ProgramArguments, Plist: string(plist)}
	if !*dryRun {
		printConsentWarning(*jsonOutput)
	}
	if *dryRun || !*confirm {
		if *jsonOutput {
			return renderJSON(preview)
		}
		fmt.Printf("LAUNCHD DRY RUN — no plist was written or loaded\n")
		fmt.Printf("label: %s\n", preview.Label)
		fmt.Printf("plist: %s\n", preview.PlistPath)
		fmt.Printf("command: %s %q\n", spec.ProgramArguments[0], spec.ProgramArguments[1:])
		if !*dryRun {
			return errors.New("LaunchAgent installation requires explicit --confirm; nothing was changed")
		}
		fmt.Print(preview.Plist)
		return nil
	}
	status, err := manager.Install(context.Background(), spec)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return renderJSON(status)
	}
	fmt.Printf("LaunchAgent installed and loaded: %s\n", status.Label)
	fmt.Printf("plist: %s\n", status.PlistPath)
	return nil
}

func launchdStatusCommand(args []string) error {
	fs := flag.NewFlagSet("schedule status", flag.ContinueOnError)
	id := fs.String("id", "", "optional schedule id")
	jsonOutput := fs.Bool("json", false, "render machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := launchd.NewManager(home)
	statuses := make([]launchd.Status, 0, len(cfg.Schedules))
	for _, item := range cfg.Schedules {
		if strings.TrimSpace(*id) != "" && item.ID != *id {
			continue
		}
		status := manager.Inspect(context.Background(), item)
		status.Enabled = item.Enabled
		statuses = append(statuses, status)
	}
	if strings.TrimSpace(*id) != "" && len(statuses) == 0 {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	if *jsonOutput {
		return renderJSON(statuses)
	}
	for _, status := range statuses {
		fmt.Printf("%s installed=%t loaded=%t enabled=%t plist=%s\n", status.Label, status.Installed, status.Loaded, status.Enabled, status.PlistPath)
	}
	if len(statuses) == 0 {
		fmt.Println("no schedules configured")
	}
	_ = paths
	return nil
}

func uninstallScheduleCommand(args []string) error {
	fs := flag.NewFlagSet("schedule uninstall", flag.ContinueOnError)
	id := fs.String("id", "", "schedule id")
	confirm := fs.Bool("confirm", false, "confirm LaunchAgent removal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	if !*confirm {
		return errors.New("LaunchAgent removal requires explicit --confirm; nothing was changed")
	}
	if _, _, _, err := loadStore(); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := launchd.NewManager(home)
	if err := manager.Uninstall(context.Background(), *id); err != nil {
		return err
	}
	fmt.Printf("LaunchAgent removed: %s; provider configuration and credentials were not changed\n", launchd.Label(*id))
	return nil
}

func launchdSpec(paths appconfig.Paths, item domain.Schedule) (launchd.Spec, error) {
	executable, err := os.Executable()
	if err != nil {
		return launchd.Spec{}, fmt.Errorf("resolve executable: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return launchd.Spec{}, fmt.Errorf("resolve home directory: %w", err)
	}
	environment := map[string]string{
		"HOME": home,
		"PATH": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
	}
	if hermesHome := strings.TrimSpace(os.Getenv("HERMES_HOME")); hermesHome != "" {
		environment["HERMES_HOME"] = hermesHome
	}
	spec := launchd.Spec{
		ScheduleID:        item.ID,
		ProgramArguments:  []string{executable, "tick", "--schedule", item.ID, "--confirm"},
		StandardOutPath:   filepath.Join(paths.Root, "logs", item.ID+".out.log"),
		StandardErrorPath: filepath.Join(paths.Root, "logs", item.ID+".err.log"),
		Environment:       environment,
	}
	if item.Mode == domain.ScheduleInterval {
		if _, err := scheduleengine.ParseInterval(item.Interval); err != nil {
			return launchd.Spec{}, err
		}
	}
	// Poll every minute for every mode. Interval schedules are anchored to the
	// last completed provider run, so a launchd trigger equal to the provider
	// interval could fire a few seconds too early and skip an entire window.
	// tickCommand is lightweight and invokes a provider only when the schedule
	// is due according to the persisted state.
	spec.Interval = time.Minute
	return spec, nil
}
func removeScheduleCommand(args []string) error {
	fs := flag.NewFlagSet("schedule remove", flag.ContinueOnError)
	id := fs.String("id", "", "schedule id")
	confirm := fs.Bool("confirm", false, "confirm local schedule removal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required")
	}
	if !*confirm {
		return errors.New("removing a schedule requires --confirm")
	}
	paths, cfg, _, err := loadStore()
	if err != nil {
		return err
	}
	configLock, err := lock.Acquire(filepath.Join(paths.Locks, "config.lock"))
	if err != nil {
		return err
	}
	defer configLock.Release()
	cfg, err = appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	item, found := appconfig.FindSchedule(cfg, *id)
	if !found {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	manager := launchd.NewManager(home)
	launchStatus := manager.Inspect(context.Background(), item)
	if launchStatus.Loaded {
		if err := manager.Unload(context.Background(), item.ID); err != nil {
			return err
		}
	}
	if !appconfig.RemoveSchedule(&cfg, *id) {
		return fmt.Errorf("schedule %q was not found", *id)
	}
	if err := appconfig.SaveConfig(paths, cfg); err != nil {
		if launchStatus.Loaded {
			_ = manager.Load(context.Background(), item.ID)
		}
		return err
	}
	fmt.Printf("schedule %s removed; provider configuration and credentials were not changed\n", *id)
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(item *flag.Flag) {
		if item.Name == name {
			found = true
		}
	})
	return found
}

func loadStore() (appconfig.Paths, appconfig.Config, appconfig.State, error) {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return appconfig.Paths{}, appconfig.Config{}, appconfig.State{}, err
	}
	cfg, err := appconfig.LoadConfig(paths)
	if err != nil {
		return appconfig.Paths{}, appconfig.Config{}, appconfig.State{}, err
	}
	state, err := appconfig.LoadState(paths)
	if err != nil {
		return appconfig.Paths{}, appconfig.Config{}, appconfig.State{}, err
	}
	return paths, cfg, state, nil
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func renderJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func printHelp() {
	fmt.Println("open-agent-clock", version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  open-agent-clock                         # interactive setup or management")
	fmt.Println("  open-agent-clock setup                   # guided first-run setup")
	fmt.Println("  open-agent-clock ui                      # interactive management")
	fmt.Println("  open-agent-clock init")
	fmt.Println("  open-agent-clock detect [--json]")
	fmt.Println("  open-agent-clock config [--json]")
	fmt.Println("  open-agent-clock status [--json]")
	fmt.Println("  open-agent-clock schedule add --id <id> --target <binding> [options]")
	fmt.Println("  open-agent-clock schedule update --id <id> [options]")
	fmt.Println("  open-agent-clock schedule list [--json]")
	fmt.Println("  open-agent-clock schedule install --id <id> [--dry-run] [--confirm]")
	fmt.Println("  open-agent-clock schedule status [--id <id>] [--json]")
	fmt.Println("  open-agent-clock schedule uninstall --id <id> --confirm")
	fmt.Println("  open-agent-clock schedule remove --id <id> --confirm")
	fmt.Println("  open-agent-clock pause --id <id>")
	fmt.Println("  open-agent-clock resume --id <id> --confirm")
	fmt.Println("  open-agent-clock run --dry-run --target <binding> [--prompt hi] [--json]")
	fmt.Println("  open-agent-clock run --once --schedule <id> --confirm [--json]")
	fmt.Println("  open-agent-clock tick --schedule <id> --confirm [--json]")
	fmt.Println("  open-agent-clock history [--json]")
	fmt.Println("  open-agent-clock last-run --schedule <id> [--json]")
	fmt.Println("  open-agent-clock version")
	fmt.Println()
	fmt.Println("Targets are discovered independently: native-codex and hermes-codex.")
	fmt.Println("Scheduled runs are transparent provider invocations, not guaranteed limit resets or bypasses.")
}
