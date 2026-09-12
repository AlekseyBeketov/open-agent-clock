package config

import (
	"os"
	"path/filepath"
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
