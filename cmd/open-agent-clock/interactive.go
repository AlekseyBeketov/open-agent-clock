package main

import (
	"errors"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/discovery"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
	"github.com/AlekseyBeketov/open-agent-clock/internal/interactive"
)

func interactiveCommand(setupOnly bool) error {
	prompt := interactive.NewHuhPrompter(os.Stdin, os.Stderr)
	operations := cliOperations{}
	language := "en"
	if snapshot, err := operations.Snapshot(); err == nil && snapshot.Initialized {
		language = snapshot.Config.Language
	}
	app := interactive.App{
		Prompt: prompt,
		Ops:    operations,
		Out:    os.Stdout,
		Lang:   language,
	}
	if setupOnly {
		return app.RunSetup()
	}
	return app.RunUI()
}

func isInteractiveTerminal(input, output *os.File) bool {
	if input == nil || output == nil {
		return false
	}
	return term.IsTerminal(input.Fd()) && term.IsTerminal(output.Fd())
}

type cliOperations struct{}

func (cliOperations) Snapshot() (interactive.Snapshot, error) {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return interactive.Snapshot{}, err
	}
	cfg, err := appconfig.LoadConfig(paths)
	if errors.Is(err, appconfig.ErrNotInitialized) {
		return interactive.Snapshot{Initialized: false, Paths: paths, Config: appconfig.DefaultConfig()}, nil
	}
	if err != nil {
		return interactive.Snapshot{}, err
	}
	return interactive.Snapshot{Initialized: true, Paths: paths, Config: cfg}, nil
}

func (cliOperations) Initialize() error {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return err
	}
	return appconfig.Init(paths)
}

func (cliOperations) SavePreferences(language string, setupCompleted bool) error {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return err
	}
	if err := appconfig.Init(paths); err != nil {
		return err
	}
	cfg, err := appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	cfg.Language = appconfig.NormalizeLanguage(language)
	cfg.SetupCompleted = setupCompleted
	return appconfig.SaveConfig(paths, cfg)
}

func (cliOperations) SaveUpdateSettings(enabled bool, scheduleID string) error {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return err
	}
	if err := appconfig.Init(paths); err != nil {
		return err
	}
	cfg, err := appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	cfg.Updates.Enabled = enabled
	cfg.Updates.AlignScheduleID = strings.TrimSpace(scheduleID)
	return appconfig.SaveConfig(paths, cfg)
}

func (cliOperations) SaveNotificationSettings(enabled bool) error {
	paths, err := appconfig.DefaultPaths()
	if err != nil {
		return err
	}
	if err := appconfig.Init(paths); err != nil {
		return err
	}
	cfg, err := appconfig.LoadConfig(paths)
	if err != nil {
		return err
	}
	cfg.Notifications.Enabled = enabled
	return appconfig.SaveConfig(paths, cfg)
}

func (cliOperations) Bindings() []domain.Binding {
	return discovery.Discover()
}

func (cliOperations) Execute(args []string) error {
	return run(args)
}
