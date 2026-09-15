package interactive

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	scheduleengine "github.com/AlekseyBeketov/open-agent-clock/internal/schedule"
)

type Snapshot struct {
	Initialized bool
	Paths       appconfig.Paths
	Config      appconfig.Config
}

type Operations interface {
	Snapshot() (Snapshot, error)
	Initialize() error
	SavePreferences(language string, setupCompleted bool) error
	Bindings() []domain.Binding
	Execute(args []string) error
}

// UpdateSettingsOperations is optional so existing integrations can keep
// implementing Operations without opting into update configuration prompts.
type UpdateSettingsOperations interface {
	SaveUpdateSettings(enabled bool, scheduleID string) error
}

// NotificationSettingsOperations is optional so existing integrations can keep
// implementing Operations without opting into notification configuration prompts.
type NotificationSettingsOperations interface {
	SaveNotificationSettings(enabled bool) error
}

type App struct {
	Prompt Prompter
	Ops    Operations
	Out    io.Writer
	Lang   string
}

func (app *App) RunUI() error {
	app.Prompt.SetLanguage(app.Lang)
	snapshot, err := app.Ops.Snapshot()
	if err != nil {
		return err
	}
	if snapshot.Initialized {
		app.Lang = normalizeLanguage(snapshot.Config.Language)
		app.Prompt.SetLanguage(app.Lang)
	}
	if !snapshot.Initialized || !snapshot.Config.SetupCompleted {
		if err := app.RunSetup(); err != nil {
			return app.handleCancellation(err)
		}
		snapshot, err = app.Ops.Snapshot()
		if err != nil {
			return err
		}
		if !snapshot.Config.SetupCompleted {
			return nil
		}
	}
	return app.runMainMenu()
}

func (app *App) RunSetup() error {
	snapshot, snapshotErr := app.Ops.Snapshot()
	if snapshotErr != nil {
		return snapshotErr
	}
	wasCompleted := snapshot.Initialized && snapshot.Config.SetupCompleted
	app.Prompt.SetLanguage(app.Lang)
	language, err := app.chooseLanguage(app.Lang)
	if err != nil {
		return app.handleCancellation(err)
	}
	app.Lang = language
	app.Prompt.SetLanguage(app.Lang)
	if err := app.Ops.Initialize(); err != nil {
		return err
	}
	if err := app.Ops.SavePreferences(app.Lang, wasCompleted); err != nil {
		return err
	}

	accepted, err := app.Prompt.Confirm(text(app.Lang, "welcome.title"), text(app.Lang, "welcome.body"), false)
	if err != nil {
		return app.handleCancellation(err)
	}
	if !accepted {
		app.line(text(app.Lang, "common.cancelled"))
		return nil
	}

	binding, ok, err := app.chooseAvailableBinding()
	if err != nil {
		return app.handleCancellation(err)
	}
	if !ok {
		return nil
	}
	scheduleID, err := app.createSchedule(binding)
	if err != nil {
		return app.handleCancellation(err)
	}
	if scheduleID == "" {
		return nil
	}
	notificationsEnabled, err := app.configureNotifications()
	if err != nil {
		return err
	}
	if err := app.configureAutomaticUpdates(scheduleID); err != nil {
		return err
	}
	if err := app.Ops.SavePreferences(app.Lang, true); err != nil {
		return err
	}
	app.line(text(app.Lang, "setup.complete"))
	if _, ok := app.Ops.(NotificationSettingsOperations); ok {
		statusKey := "settings.notifications.disabled"
		if notificationsEnabled {
			statusKey = "settings.notifications.enabled"
		}
		app.line(fmt.Sprintf("%s: %s", text(app.Lang, "settings.notifications"), text(app.Lang, statusKey)))
	}
	return nil
}

func (app *App) runMainMenu() error {
	for {
		choice, err := app.Prompt.Select(
			text(app.Lang, "main.title"),
			text(app.Lang, "main.description"),
			[]Option{
				{Label: text(app.Lang, "main.dashboard"), Value: "dashboard"},
				{Label: text(app.Lang, "main.targets"), Value: "targets"},
				{Label: text(app.Lang, "main.schedules"), Value: "schedules"},
				{Label: text(app.Lang, "main.run"), Value: "run"},
				{Label: text(app.Lang, "main.history"), Value: "history"},
				{Label: text(app.Lang, "main.settings"), Value: "settings"},
				{Label: text(app.Lang, "main.setup"), Value: "setup"},
				{Label: text(app.Lang, "common.exit"), Value: "exit"},
			},
			"dashboard",
		)
		if err != nil {
			return app.handleCancellation(err)
		}
		switch choice {
		case "dashboard":
			app.execute("status")
		case "targets":
			app.execute("detect")
		case "schedules":
			if err := app.runScheduleMenu(); err != nil {
				return err
			}
		case "run":
			if err := app.runExecutionMenu(); err != nil {
				return err
			}
		case "history":
			if err := app.runHistoryMenu(); err != nil {
				return err
			}
		case "settings":
			if err := app.runSettingsMenu(); err != nil {
				return err
			}
		case "setup":
			if err := app.RunSetup(); err != nil {
				return err
			}
		case "exit":
			return nil
		}
	}
}

func (app *App) runScheduleMenu() error {
	for {
		choice, err := app.Prompt.Select(text(app.Lang, "schedule.title"), "", []Option{
			{Label: text(app.Lang, "schedule.list"), Value: "list"},
			{Label: text(app.Lang, "schedule.inspect"), Value: "inspect"},
			{Label: text(app.Lang, "schedule.create"), Value: "create"},
			{Label: text(app.Lang, "schedule.edit"), Value: "edit"},
			{Label: text(app.Lang, "schedule.toggle"), Value: "toggle"},
			{Label: text(app.Lang, "schedule.launchd"), Value: "launchd"},
			{Label: text(app.Lang, "schedule.remove"), Value: "remove"},
			{Label: text(app.Lang, "common.back"), Value: "back"},
		}, "list")
		if err != nil {
			return app.handleCancellation(err)
		}
		switch choice {
		case "list":
			app.execute("schedule", "list")
		case "inspect":
			if err := app.inspectSchedule(); err != nil {
				return app.handleCancellation(err)
			}
		case "create":
			binding, ok, chooseErr := app.chooseAvailableBinding()
			if chooseErr != nil {
				return app.handleCancellation(chooseErr)
			}
			if ok {
				_, _ = app.createSchedule(binding)
			}
		case "edit":
			if err := app.editSchedule(); err != nil {
				return app.handleCancellation(err)
			}
		case "toggle":
			if err := app.toggleSchedule(); err != nil {
				return app.handleCancellation(err)
			}
		case "launchd":
			if err := app.runLaunchdMenu(); err != nil {
				return app.handleCancellation(err)
			}
		case "remove":
			if err := app.removeSchedule(); err != nil {
				return app.handleCancellation(err)
			}
		case "back":
			return nil
		}
	}
}

func (app *App) inspectSchedule() error {
	item, ok, err := app.chooseSchedule()
	if err != nil || !ok {
		return err
	}
	app.line(fmt.Sprintf("%s: %s (%s)", text(app.Lang, "schedule.inspect"), item.Name, item.ID))
	app.line(fmt.Sprintf("target: %s", item.TargetID))
	app.line(fmt.Sprintf("provider: %s", item.Provider))
	app.line(fmt.Sprintf("mode: %s", item.Mode))
	if item.Mode == domain.ScheduleInterval {
		app.line(fmt.Sprintf("interval: %s", item.Interval))
	} else {
		app.line(fmt.Sprintf("times: %s", strings.Join(item.Times, ", ")))
	}
	app.line(fmt.Sprintf("timezone: %s", item.Timezone))
	app.line(fmt.Sprintf("prompt: %s", item.Prompt))
	app.line(fmt.Sprintf("enabled: %t", item.Enabled))
	return nil
}

func (app *App) createSchedule(binding domain.Binding) (string, error) {
	idDefault := strings.TrimSuffix(binding.ID, "-subscription") + "-window"
	id, err := app.Prompt.Input(text(app.Lang, "setup.id"), text(app.Lang, "setup.id_help"), idDefault, app.required)
	if err != nil {
		return "", err
	}
	name, err := app.Prompt.Input(text(app.Lang, "setup.name"), "", id, app.required)
	if err != nil {
		return "", err
	}
	mode, err := app.chooseMode(string(domain.ScheduleInterval))
	if err != nil {
		return "", err
	}
	interval, times, err := app.collectTiming(mode, "5h3m", "05:00")
	if err != nil {
		return "", err
	}
	timezone, err := app.Prompt.Input(text(app.Lang, "setup.timezone"), "", appconfig.SystemTimezone(), func(value string) error {
		return scheduleengine.ValidateTimezone(strings.TrimSpace(value))
	})
	if err != nil {
		return "", err
	}
	prompt, err := app.Prompt.Input(text(app.Lang, "setup.prompt"), "", "hi", app.required)
	if err != nil {
		return "", err
	}

	args := []string{"schedule", "add", "--id", strings.TrimSpace(id), "--name", strings.TrimSpace(name), "--target", binding.ID, "--provider", binding.Provider, "--prompt", prompt, "--mode", mode, "--timezone", strings.TrimSpace(timezone)}
	if mode == string(domain.ScheduleInterval) {
		args = append(args, "--interval", interval)
	} else {
		args = append(args, "--times", times)
	}
	reviewText := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %s",
		text(app.Lang, "setup.review.body"), text(app.Lang, "setup.review.id"), id,
		text(app.Lang, "setup.review.target"), binding.ID, text(app.Lang, "setup.review.mode"), mode,
		text(app.Lang, "setup.review.timing"), timingSummary(mode, interval, times),
		text(app.Lang, "setup.review.timezone"), timezone, text(app.Lang, "setup.review.prompt"), prompt)
	review, err := app.Prompt.Confirm(text(app.Lang, "setup.review.title"), reviewText, true)
	if err != nil {
		return "", err
	}
	if !review {
		return "", nil
	}
	if err := app.Ops.Execute(args); err != nil {
		return "", err
	}

	preview, err := app.Prompt.Confirm(text(app.Lang, "setup.summary"), text(app.Lang, "setup.preview"), true)
	if err != nil {
		return "", err
	}
	if preview {
		app.execute("run", "--dry-run", "--target", binding.ID, "--prompt", prompt)
	}
	activate, err := app.Prompt.Confirm(text(app.Lang, "setup.summary"), text(app.Lang, "setup.activate"), false)
	if err != nil {
		return "", err
	}
	if !activate {
		return id, nil
	}
	if err := app.Ops.Execute([]string{"resume", "--id", id, "--confirm"}); err != nil {
		return id, err
	}
	install, err := app.Prompt.Confirm(text(app.Lang, "launchd.title"), text(app.Lang, "setup.install"), false)
	if err != nil {
		return "", err
	}
	if install {
		if !app.execute("schedule", "install", "--id", id, "--confirm") {
			return id, errors.New("LaunchAgent installation failed; schedule remains active without LaunchAgent")
		}
	}
	return id, nil
}

func timingSummary(mode, interval, times string) string {
	if mode == string(domain.ScheduleInterval) {
		return interval
	}
	return times
}

func (app *App) editSchedule() error {
	item, ok, err := app.chooseSchedule()
	if err != nil || !ok {
		return err
	}
	binding, bindingOK, err := app.chooseBinding(item.TargetID)
	if err != nil || !bindingOK {
		return err
	}
	name, err := app.Prompt.Input(text(app.Lang, "setup.name"), "", item.Name, app.required)
	if err != nil {
		return err
	}
	prompt, err := app.Prompt.Input(text(app.Lang, "setup.prompt"), "", item.Prompt, app.required)
	if err != nil {
		return err
	}
	mode, err := app.chooseMode(string(item.Mode))
	if err != nil {
		return err
	}
	timingDefault := item.Interval
	if mode != string(domain.ScheduleInterval) {
		timingDefault = strings.Join(item.Times, ",")
	}
	interval, times, err := app.collectTiming(mode, timingDefault, timingDefault)
	if err != nil {
		return err
	}
	timezone, err := app.Prompt.Input(text(app.Lang, "setup.timezone"), "", item.Timezone, func(value string) error {
		return scheduleengine.ValidateTimezone(strings.TrimSpace(value))
	})
	if err != nil {
		return err
	}
	args := []string{"schedule", "update", "--id", item.ID, "--name", name, "--target", binding.ID, "--provider", binding.Provider, "--prompt", prompt, "--mode", mode, "--timezone", timezone}
	if mode == string(domain.ScheduleInterval) {
		args = append(args, "--interval", interval, "--times", "")
	} else {
		args = append(args, "--times", times, "--interval", "")
	}
	app.execute(args...)
	return nil
}

func (app *App) toggleSchedule() error {
	item, ok, err := app.chooseSchedule()
	if err != nil || !ok {
		return err
	}
	if item.Enabled {
		confirmed, confirmErr := app.Prompt.Confirm(text(app.Lang, "schedule.pause"), item.Name, true)
		if confirmErr != nil {
			return confirmErr
		}
		if confirmed {
			app.execute("pause", "--id", item.ID)
		}
		return nil
	}
	confirmed, err := app.Prompt.Confirm(text(app.Lang, "schedule.enable"), text(app.Lang, "setup.activate"), false)
	if err != nil {
		return err
	}
	if confirmed {
		app.execute("resume", "--id", item.ID, "--confirm")
	}
	return nil
}

func (app *App) runLaunchdMenu() error {
	item, ok, err := app.chooseSchedule()
	if err != nil || !ok {
		return err
	}
	for {
		choice, selectErr := app.Prompt.Select(text(app.Lang, "launchd.title"), item.Name, []Option{
			{Label: text(app.Lang, "launchd.preview"), Value: "preview"},
			{Label: text(app.Lang, "launchd.install"), Value: "install"},
			{Label: text(app.Lang, "launchd.status"), Value: "status"},
			{Label: text(app.Lang, "launchd.uninstall"), Value: "uninstall"},
			{Label: text(app.Lang, "common.back"), Value: "back"},
		}, "status")
		if selectErr != nil {
			return selectErr
		}
		switch choice {
		case "preview":
			app.execute("schedule", "install", "--id", item.ID, "--dry-run")
		case "install":
			confirmed, confirmErr := app.Prompt.Confirm(text(app.Lang, "launchd.install"), text(app.Lang, "setup.install"), false)
			if confirmErr != nil {
				return confirmErr
			}
			if confirmed {
				app.execute("schedule", "install", "--id", item.ID, "--confirm")
			}
		case "status":
			app.execute("schedule", "status", "--id", item.ID)
		case "uninstall":
			confirmed, confirmErr := app.Prompt.Confirm(text(app.Lang, "launchd.uninstall"), fmt.Sprintf("%s\n%s", item.Name, text(app.Lang, "confirm.uninstall")), false)
			if confirmErr != nil {
				return confirmErr
			}
			if confirmed {
				app.execute("schedule", "uninstall", "--id", item.ID, "--confirm")
			}
		case "back":
			return nil
		}
	}
}

func (app *App) removeSchedule() error {
	item, ok, err := app.chooseSchedule()
	if err != nil || !ok {
		return err
	}
	confirmed, err := app.Prompt.Confirm(text(app.Lang, "schedule.remove"), fmt.Sprintf("%s (%s)\n%s", item.Name, item.ID, text(app.Lang, "confirm.remove_schedule")), false)
	if err != nil {
		return err
	}
	if confirmed {
		app.execute("schedule", "remove", "--id", item.ID, "--confirm")
	}
	return nil
}

func (app *App) runExecutionMenu() error {
	for {
		choice, err := app.Prompt.Select(text(app.Lang, "run.title"), "", []Option{
			{Label: text(app.Lang, "run.preview"), Value: "preview"},
			{Label: text(app.Lang, "run.once"), Value: "once"},
			{Label: text(app.Lang, "run.tick"), Value: "tick"},
			{Label: text(app.Lang, "common.back"), Value: "back"},
		}, "preview")
		if err != nil {
			return app.handleCancellation(err)
		}
		switch choice {
		case "preview":
			binding, ok, chooseErr := app.chooseAvailableBinding()
			if chooseErr != nil {
				return chooseErr
			}
			if !ok {
				continue
			}
			prompt, inputErr := app.Prompt.Input(text(app.Lang, "setup.prompt"), "", "hi", app.required)
			if inputErr != nil {
				return inputErr
			}
			app.execute("run", "--dry-run", "--target", binding.ID, "--prompt", prompt)
		case "once", "tick":
			item, ok, chooseErr := app.chooseSchedule()
			if chooseErr != nil {
				return chooseErr
			}
			if !ok {
				continue
			}
			confirmed, confirmErr := app.Prompt.Confirm(text(app.Lang, "run.title"), text(app.Lang, "run.warning"), false)
			if confirmErr != nil {
				return confirmErr
			}
			if confirmed {
				if choice == "once" {
					app.execute("run", "--once", "--schedule", item.ID, "--confirm")
				} else {
					app.execute("tick", "--schedule", item.ID, "--confirm")
				}
			}
		case "back":
			return nil
		}
	}
}

func (app *App) runHistoryMenu() error {
	for {
		choice, err := app.Prompt.Select(text(app.Lang, "history.title"), "", []Option{
			{Label: text(app.Lang, "history.status"), Value: "status"},
			{Label: text(app.Lang, "history.all"), Value: "history"},
			{Label: text(app.Lang, "history.last"), Value: "last"},
			{Label: text(app.Lang, "common.back"), Value: "back"},
		}, "status")
		if err != nil {
			return app.handleCancellation(err)
		}
		switch choice {
		case "status":
			app.execute("status")
		case "history":
			app.execute("history")
		case "last":
			item, ok, chooseErr := app.chooseSchedule()
			if chooseErr != nil {
				return chooseErr
			}
			if ok {
				app.execute("last-run", "--schedule", item.ID)
			}
		case "back":
			return nil
		}
	}
}

func (app *App) runSettingsMenu() error {
	for {
		choice, err := app.Prompt.Select(text(app.Lang, "settings.title"), "", []Option{
			{Label: text(app.Lang, "settings.language"), Value: "language"},
			{Label: text(app.Lang, "settings.updates"), Value: "updates"},
			{Label: text(app.Lang, "settings.notifications"), Value: "notifications"},
			{Label: text(app.Lang, "settings.paths"), Value: "paths"},
			{Label: text(app.Lang, "main.setup"), Value: "setup"},
			{Label: text(app.Lang, "common.back"), Value: "back"},
		}, "language")
		if err != nil {
			return app.handleCancellation(err)
		}
		switch choice {
		case "language":
			language, chooseErr := app.chooseLanguage(app.Lang)
			if chooseErr != nil {
				return chooseErr
			}
			app.Lang = language
			app.Prompt.SetLanguage(app.Lang)
			if saveErr := app.Ops.SavePreferences(app.Lang, true); saveErr != nil {
				return saveErr
			}
		case "updates":
			if updateErr := app.configureAutomaticUpdates(""); updateErr != nil {
				return updateErr
			}
		case "notifications":
			if _, notificationErr := app.configureNotifications(); notificationErr != nil {
				return notificationErr
			}
		case "paths":
			app.execute("config")
		case "setup":
			if setupErr := app.RunSetup(); setupErr != nil {
				return setupErr
			}
		case "back":
			return nil
		}
	}
}

func (app *App) configureNotifications() (bool, error) {
	operations, ok := app.Ops.(NotificationSettingsOperations)
	if !ok {
		return false, nil
	}
	snapshot, err := app.Ops.Snapshot()
	if err != nil {
		return false, err
	}
	enabled, err := app.Prompt.Confirm(
		text(app.Lang, "settings.notifications.title"),
		text(app.Lang, "settings.notifications.description"),
		snapshot.Config.Notifications.Enabled,
	)
	if err != nil {
		return false, app.handleCancellation(err)
	}
	if err := operations.SaveNotificationSettings(enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func (app *App) configureAutomaticUpdates(preferredScheduleID string) error {
	operations, ok := app.Ops.(UpdateSettingsOperations)
	if !ok {
		return nil
	}
	snapshot, err := app.Ops.Snapshot()
	if err != nil {
		return err
	}
	defaultEnabled := snapshot.Config.Updates.Enabled
	enabled, err := app.Prompt.Confirm(
		text(app.Lang, "settings.updates.title"),
		text(app.Lang, "settings.updates.description"),
		defaultEnabled,
	)
	if err != nil {
		return app.handleCancellation(err)
	}
	if !enabled {
		return operations.SaveUpdateSettings(false, snapshot.Config.Updates.AlignScheduleID)
	}

	intervals := make([]domain.Schedule, 0, len(snapshot.Config.Schedules))
	for _, item := range snapshot.Config.Schedules {
		if item.Mode == domain.ScheduleInterval {
			intervals = append(intervals, item)
		}
	}
	if len(intervals) == 0 {
		app.line(text(app.Lang, "settings.updates.no_interval"))
		return operations.SaveUpdateSettings(false, "")
	}
	options := make([]Option, 0, len(intervals))
	selected := preferredScheduleID
	if selected == "" {
		selected = snapshot.Config.Updates.AlignScheduleID
	}
	for _, item := range intervals {
		options = append(options, Option{Label: fmt.Sprintf("%s — %s", item.Name, item.ID), Value: item.ID})
	}
	if !slices.ContainsFunc(intervals, func(item domain.Schedule) bool { return item.ID == selected }) {
		selected = intervals[0].ID
	}
	selected, err = app.Prompt.Select(
		text(app.Lang, "settings.updates.schedule"),
		text(app.Lang, "settings.updates.schedule_description"),
		options,
		selected,
	)
	if err != nil {
		return app.handleCancellation(err)
	}
	return operations.SaveUpdateSettings(true, selected)
}

func (app *App) chooseLanguage(current string) (string, error) {
	return app.Prompt.Select(
		text("en", "language.title"),
		text(normalizeLanguage(current), "language.description"),
		[]Option{{Label: text("en", "language.english"), Value: "en"}, {Label: text("ru", "language.russian"), Value: "ru"}},
		normalizeLanguage(current),
	)
}

func (app *App) chooseAvailableBinding() (domain.Binding, bool, error) {
	return app.chooseBinding("")
}

func (app *App) chooseBinding(defaultID string) (domain.Binding, bool, error) {
	for {
		bindings := app.Ops.Bindings()
		available := make([]domain.Binding, 0, len(bindings))
		options := make([]Option, 0, len(bindings))
		for _, binding := range bindings {
			if binding.Available && !strings.Contains(strings.ToLower(binding.AuthMode), "api") {
				available = append(available, binding)
				label := fmt.Sprintf("%s — %s (%s)", binding.DisplayName, binding.ID, binding.AuthMode)
				options = append(options, Option{Label: label, Value: binding.ID})
			}
		}
		if len(available) > 0 {
			selected := defaultID
			if selected == "" || !slices.ContainsFunc(available, func(binding domain.Binding) bool { return binding.ID == selected }) {
				selected = available[0].ID
			}
			id, err := app.Prompt.Select(text(app.Lang, "setup.target"), "", options, selected)
			if err != nil {
				return domain.Binding{}, false, err
			}
			for _, binding := range available {
				if binding.ID == id {
					return binding, true, nil
				}
			}
		}
		action, err := app.Prompt.Select(text(app.Lang, "setup.no_targets_action"), text(app.Lang, "setup.no_targets"), []Option{
			{Label: text(app.Lang, "common.retry"), Value: "retry"},
			{Label: text(app.Lang, "common.exit"), Value: "exit"},
		}, "retry")
		if err != nil {
			return domain.Binding{}, false, err
		}
		if action == "exit" {
			return domain.Binding{}, false, nil
		}
	}
}

func (app *App) chooseSchedule() (domain.Schedule, bool, error) {
	snapshot, err := app.Ops.Snapshot()
	if err != nil {
		return domain.Schedule{}, false, err
	}
	if len(snapshot.Config.Schedules) == 0 {
		app.line(text(app.Lang, "schedule.empty"))
		return domain.Schedule{}, false, nil
	}
	options := make([]Option, 0, len(snapshot.Config.Schedules))
	for _, item := range snapshot.Config.Schedules {
		state := text(app.Lang, "schedule.state.paused")
		if item.Enabled {
			state = text(app.Lang, "schedule.state.enabled")
		}
		options = append(options, Option{Label: fmt.Sprintf("%s — %s (%s)", item.Name, item.ID, state), Value: item.ID})
	}
	id, err := app.Prompt.Select(text(app.Lang, "schedule.select"), "", options, options[0].Value)
	if err != nil {
		return domain.Schedule{}, false, err
	}
	for _, item := range snapshot.Config.Schedules {
		if item.ID == id {
			return item, true, nil
		}
	}
	return domain.Schedule{}, false, nil
}

func (app *App) chooseMode(current string) (string, error) {
	if current != string(domain.ScheduleDaily) && current != "morning" {
		current = string(domain.ScheduleInterval)
	}
	return app.Prompt.Select(text(app.Lang, "setup.mode"), "", []Option{
		{Label: text(app.Lang, "setup.mode.interval"), Value: string(domain.ScheduleInterval)},
		{Label: text(app.Lang, "setup.mode.morning"), Value: "morning"},
		{Label: text(app.Lang, "setup.mode.daily"), Value: string(domain.ScheduleDaily)},
	}, current)
}

func (app *App) collectTiming(mode, intervalDefault, timesDefault string) (string, string, error) {
	if mode == string(domain.ScheduleInterval) {
		if strings.TrimSpace(intervalDefault) == "" {
			intervalDefault = "5h3m"
		}
		value, err := app.Prompt.Input(text(app.Lang, "setup.interval"), "", intervalDefault, func(value string) error {
			_, parseErr := scheduleengine.ParseInterval(strings.TrimSpace(value))
			return parseErr
		})
		return strings.TrimSpace(value), "", err
	}
	if strings.TrimSpace(timesDefault) == "" {
		timesDefault = "05:00"
	}
	key := "setup.times"
	if mode == "morning" {
		key = "setup.time"
	}
	value, err := app.Prompt.Input(text(app.Lang, key), "", timesDefault, func(value string) error {
		values := strings.Split(value, ",")
		if mode == "morning" && len(values) != 1 {
			return errors.New(text(app.Lang, "validation.required"))
		}
		for _, clock := range values {
			if validateErr := scheduleengine.ValidateClock(strings.TrimSpace(clock)); validateErr != nil {
				return validateErr
			}
		}
		return nil
	})
	return "", strings.TrimSpace(value), err
}

func (app *App) execute(args ...string) bool {
	if err := app.Ops.Execute(args); err != nil {
		app.operationError(err)
		return false
	}
	return true
}

func (app *App) operationError(err error) {
	app.line(text(app.Lang, "operation.failed", err))
}

func (app *App) required(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(text(app.Lang, "validation.required"))
	}
	return nil
}

func (app *App) handleCancellation(err error) error {
	if errors.Is(err, ErrCancelled) {
		app.line(text(app.Lang, "common.cancelled"))
		return nil
	}
	return err
}

func (app *App) line(value string) {
	if app.Out != nil {
		fmt.Fprintln(app.Out, value)
	}
}
