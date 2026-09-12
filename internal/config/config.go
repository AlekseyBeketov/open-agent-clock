package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

const SchemaVersion = 1

var ErrNotInitialized = errors.New("open-agent-clock is not initialized; run init first")

type Paths struct {
	Root    string
	Config  string
	State   string
	Locks   string
	History string
}

type Config struct {
	SchemaVersion       int               `json:"schema_version"`
	Language            string            `json:"language,omitempty"`
	SetupCompleted      bool              `json:"setup_completed,omitempty"`
	Schedules           []domain.Schedule `json:"schedules"`
	ConsentAcknowledged bool              `json:"consent_acknowledged"`
}

type State struct {
	SchemaVersion int                         `json:"schema_version"`
	LastRuns      map[string]domain.RunResult `json:"last_runs"`
}

func DefaultConfig() Config {
	return Config{SchemaVersion: SchemaVersion, Language: "en", Schedules: []domain.Schedule{}}
}

func DefaultState() State {
	return State{SchemaVersion: SchemaVersion, LastRuns: map[string]domain.RunResult{}}
}

func PathsForHome(home string) Paths {
	home = strings.TrimSpace(home)
	root := filepath.Join(home, ".config", "open-agent-clock")
	if runtime.GOOS == "darwin" {
		root = filepath.Join(home, "Library", "Application Support", "open-agent-clock")
	}
	return Paths{
		Root:    root,
		Config:  filepath.Join(root, "config.json"),
		State:   filepath.Join(root, "state.json"),
		Locks:   filepath.Join(root, "locks"),
		History: filepath.Join(root, "history"),
	}
}

func SystemTimezone() string {
	if value := strings.TrimSpace(os.Getenv("TZ")); value != "" {
		if _, err := time.LoadLocation(value); err == nil {
			return value
		}
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		const marker = "/zoneinfo/"
		if index := strings.Index(target, marker); index >= 0 {
			candidate := strings.TrimPrefix(target[index+len(marker):], "/")
			if _, err := time.LoadLocation(candidate); err == nil {
				return candidate
			}
		}
	}
	return "UTC"
}
func DefaultPaths() (Paths, error) {
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return PathsForHome(home), nil
}

func Init(paths Paths) error {
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	for _, directory := range []string{paths.Locks, paths.History} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create runtime directory: %w", err)
		}
	}
	if _, err := os.Stat(paths.Config); errors.Is(err, os.ErrNotExist) {
		if err := SaveConfig(paths, DefaultConfig()); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("inspect config: %w", err)
	}
	if _, err := os.Stat(paths.State); errors.Is(err, os.ErrNotExist) {
		if err := SaveState(paths, DefaultState()); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("inspect state: %w", err)
	}
	return nil
}

func LoadConfig(paths Paths) (Config, error) {
	var value Config
	if err := readJSON(paths.Config, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrNotInitialized
		}
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	if value.SchemaVersion != SchemaVersion {
		return Config{}, fmt.Errorf("unsupported config schema version %d", value.SchemaVersion)
	}
	if value.Schedules == nil {
		value.Schedules = []domain.Schedule{}
	}
	value.Language = NormalizeLanguage(value.Language)
	return value, nil
}

func LoadState(paths Paths) (State, error) {
	var value State
	if err := readJSON(paths.State, &value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, ErrNotInitialized
		}
		return State{}, fmt.Errorf("load state: %w", err)
	}
	if value.SchemaVersion != SchemaVersion {
		return State{}, fmt.Errorf("unsupported state schema version %d", value.SchemaVersion)
	}
	if value.LastRuns == nil {
		value.LastRuns = map[string]domain.RunResult{}
	}
	return value, nil
}

func SaveConfig(paths Paths, value Config) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = SchemaVersion
	}
	value.Language = NormalizeLanguage(value.Language)
	return writeJSONAtomic(paths, paths.Config, value)
}

func NormalizeLanguage(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "ru") {
		return "ru"
	}
	return "en"
}

func SaveState(paths Paths, value State) error {
	if value.SchemaVersion == 0 {
		value.SchemaVersion = SchemaVersion
	}
	return writeJSONAtomic(paths, paths.State, value)
}

func UpsertSchedule(value *Config, schedule domain.Schedule) {
	for index := range value.Schedules {
		if value.Schedules[index].ID == schedule.ID {
			value.Schedules[index] = schedule
			return
		}
	}
	value.Schedules = append(value.Schedules, schedule)
}

func FindSchedule(value Config, id string) (domain.Schedule, bool) {
	for _, schedule := range value.Schedules {
		if schedule.ID == id {
			return schedule, true
		}
	}
	return domain.Schedule{}, false
}

func RemoveSchedule(value *Config, id string) bool {
	for index, schedule := range value.Schedules {
		if schedule.ID == id {
			value.Schedules = append(value.Schedules[:index], value.Schedules[index+1:]...)
			return true
		}
	}
	return false
}

func writeJSONAtomic(paths Paths, destination string, value any) error {
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(paths.Root, ".atomic-*")
	if err != nil {
		return fmt.Errorf("create atomic file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set file permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write JSON: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync JSON: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close JSON: %w", err)
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		return fmt.Errorf("replace JSON: %w", err)
	}
	return nil
}

func readJSON(path string, destination any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(contents, destination); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}
