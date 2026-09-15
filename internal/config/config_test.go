package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestPathsForHomeUsesMacOSApplicationSupportLayout(t *testing.T) {
	paths := PathsForHome("/tmp/example-home")
	if filepath.Base(paths.Root) != "open-agent-clock" {
		t.Fatalf("root = %q", paths.Root)
	}
	if filepath.Base(paths.Config) != "config.json" || filepath.Base(paths.State) != "state.json" {
		t.Fatalf("paths = %+v", paths)
	}
}

func TestInitAndAtomicConfigPersistenceAreUserOnly(t *testing.T) {
	paths := PathsForHome(t.TempDir())
	if err := Init(paths); err != nil {
		t.Fatal(err)
	}
	value := DefaultConfig()
	value.Schedules = append(value.Schedules, domain.Schedule{ID: "morning", TargetID: "native-codex", Prompt: "hi", Mode: domain.ScheduleDaily, Times: []string{"05:00"}, Timezone: "UTC", CreatedAt: time.Now()})
	if err := SaveConfig(paths, value); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Schedules) != 1 || loaded.Schedules[0].ID != "morning" {
		t.Fatalf("loaded = %+v", loaded)
	}
	info, err := os.Stat(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
	stateInfo, err := os.Stat(paths.State)
	if err != nil {
		t.Fatal(err)
	}
	if stateInfo.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions = %o", stateInfo.Mode().Perm())
	}
}

func TestMissingConfigRequiresInit(t *testing.T) {
	_, err := LoadConfig(PathsForHome(t.TempDir()))
	if err != ErrNotInitialized {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigDefaultsMissingLanguageToEnglish(t *testing.T) {
	paths := PathsForHome(t.TempDir())
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"schema_version":1,"schedules":[],"consent_acknowledged":false}`)
	if err := os.WriteFile(paths.Config, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Language != "en" || loaded.SetupCompleted {
		t.Fatalf("loaded preferences = language %q, setup_completed %t", loaded.Language, loaded.SetupCompleted)
	}
}

func TestLegacyConfigAndStateMigrateWithUpdatesDisabled(t *testing.T) {
	paths := PathsForHome(t.TempDir())
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyConfig := struct {
		SchemaVersion int               `json:"schema_version"`
		Language      string            `json:"language"`
		Schedules     []domain.Schedule `json:"schedules"`
	}{
		SchemaVersion: LegacySchemaVersion,
		Language:      "ru",
		Schedules: []domain.Schedule{{
			ID:       "codex",
			TargetID: "native-codex",
			Mode:     domain.ScheduleInterval,
			Interval: "5h3m",
		}},
	}
	configContents, err := json.Marshal(legacyConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Config, configContents, 0o600); err != nil {
		t.Fatal(err)
	}
	legacyState := struct {
		SchemaVersion int                         `json:"schema_version"`
		LastRuns      map[string]domain.RunResult `json:"last_runs"`
	}{
		SchemaVersion: LegacySchemaVersion,
		LastRuns:      map[string]domain.RunResult{"job-1": {Status: "success"}},
	}
	stateContents, err := json.Marshal(legacyState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.State, stateContents, 0o600); err != nil {
		t.Fatal(err)
	}

	loadedConfig, err := LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig.SchemaVersion != SchemaVersion || loadedConfig.Language != "ru" || len(loadedConfig.Schedules) != 1 {
		t.Fatalf("migrated config = %+v", loadedConfig)
	}
	if loadedConfig.Updates.Enabled || loadedConfig.Updates.AlignScheduleID != "" {
		t.Fatalf("legacy config enabled updates: %+v", loadedConfig.Updates)
	}
	if loadedConfig.Notifications.Enabled {
		t.Fatalf("legacy config enabled notifications: %+v", loadedConfig.Notifications)
	}

	loadedState, err := LoadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if loadedState.SchemaVersion != SchemaVersion || loadedState.Updates.LastResult != nil {
		t.Fatalf("migrated state = %+v", loadedState)
	}
	if loadedState.LastRuns["job-1"].Status != "success" {
		t.Fatalf("legacy last runs = %+v", loadedState.LastRuns)
	}
	if usage := loadedState.LastRuns["job-1"].TokenUsage; usage == nil || usage.Availability != domain.TokenUsageUnavailable {
		t.Fatalf("legacy token usage = %+v", usage)
	}
}

func TestUpdateConfigAndStateRoundTrip(t *testing.T) {
	paths := PathsForHome(t.TempDir())
	if err := Init(paths); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	if config.Updates.Enabled || config.Updates.AlignScheduleID != "" {
		t.Fatalf("default update config = %+v", config.Updates)
	}
	if config.Notifications.Enabled {
		t.Fatalf("default notification config = %+v", config.Notifications)
	}
	config.Updates = UpdateConfig{Enabled: true, AlignScheduleID: "codex"}
	config.Notifications = NotificationConfig{Enabled: true}
	state := DefaultState()
	state.Updates.LastResult = &UpdateResult{
		Operation:      "check",
		Status:         "available",
		CurrentVersion: "v0.1.2",
		LatestVersion:  "v0.1.3",
		Reason:         "new release",
		RecordedAt:     time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC),
	}
	if err := SaveConfig(paths, config); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(paths, state); err != nil {
		t.Fatal(err)
	}

	loadedConfig, err := LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	loadedState, err := LoadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedConfig.Updates, config.Updates) {
		t.Fatalf("config updates = %+v, want %+v", loadedConfig.Updates, config.Updates)
	}
	if !reflect.DeepEqual(loadedConfig.Notifications, config.Notifications) {
		t.Fatalf("config notifications = %+v, want %+v", loadedConfig.Notifications, config.Notifications)
	}
	if !reflect.DeepEqual(loadedState.Updates, state.Updates) {
		t.Fatalf("state updates = %+v, want %+v", loadedState.Updates, state.Updates)
	}
}
