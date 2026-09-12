package launchd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestLabelAndIntervalPlistAreDeterministic(t *testing.T) {
	spec := Spec{ScheduleID: "native/interval", ProgramArguments: []string{"/bin/open-agent-clock", "run", "--once", "--schedule", "native/interval", "--confirm"}, Interval: 5*time.Hour + 3*time.Minute, Environment: map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin:/bin", "HOME": "/Users/test"}}
	first, err := Generate(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(spec)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("plist generation is not deterministic")
	}
	value := string(first)
	for _, expected := range []string{"com.openagentclock.schedule.native-interval", "ProgramArguments", "StartInterval", "18180", "EnvironmentVariables", "/opt/homebrew/bin:/usr/bin:/bin", "/Users/test"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("plist missing %q:\n%s", expected, value)
		}
	}
}

func TestDailyPlistContainsCalendarEntries(t *testing.T) {
	value, err := Generate(Spec{ScheduleID: "morning", ProgramArguments: []string{"app", "run"}, DailyTimes: []string{"05:00", "13:30"}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(value)
	for _, expected := range []string{"StartCalendarInterval", "<key>Hour</key>", "<integer>5</integer>", "<integer>13</integer>"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("plist missing %q:\n%s", expected, text)
		}
	}
}

func TestInspectReportsMissingPlistWithoutCallingLaunchctl(t *testing.T) {
	manager := NewManager(t.TempDir())
	status := manager.Inspect(context.Background(), structSchedule("missing"))
	if status.Installed || status.Loaded || status.Enabled {
		t.Fatalf("status = %+v", status)
	}
}

func TestManagerInstallInspectAndUninstallWithFakeLaunchctl(t *testing.T) {
	root := t.TempDir()
	fakeLaunchctl := filepath.Join(root, "launchctl")
	if err := os.WriteFile(fakeLaunchctl, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := Manager{LaunchAgentsDir: filepath.Join(root, "LaunchAgents"), LaunchctlPath: fakeLaunchctl, UID: "123"}
	spec := Spec{ScheduleID: "job", ProgramArguments: []string{"app", "run"}, Interval: time.Minute}
	status, err := manager.Install(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || !status.Loaded || !fileExists(status.PlistPath) {
		t.Fatalf("install status = %+v", status)
	}
	inspected := manager.Inspect(context.Background(), domain.Schedule{ID: "job", Enabled: true})
	if !inspected.Installed || !inspected.Loaded {
		t.Fatalf("inspect status = %+v", inspected)
	}
	if err := manager.Uninstall(context.Background(), "job"); err != nil {
		t.Fatal(err)
	}
	if fileExists(status.PlistPath) {
		t.Fatal("plist still exists after uninstall")
	}
}

func TestUninstallReportsBootoutFailureAndKeepsPlist(t *testing.T) {
	root := t.TempDir()
	fakeLaunchctl := filepath.Join(root, "launchctl")
	if err := os.WriteFile(fakeLaunchctl, []byte("#!/bin/sh\nif [ \"$1\" = bootout ]; then echo bootout-failed >&2; exit 1; fi\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := Manager{LaunchAgentsDir: filepath.Join(root, "LaunchAgents"), LaunchctlPath: fakeLaunchctl, UID: "123"}
	if err := os.MkdirAll(manager.LaunchAgentsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := manager.PlistPath("job")
	if err := os.WriteFile(path, []byte("plist"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall(context.Background(), "job"); err == nil {
		t.Fatal("expected bootout failure")
	}
	if !fileExists(path) {
		t.Fatal("plist was removed after bootout failure")
	}
}

func structSchedule(id string) domain.Schedule {
	return domain.Schedule{ID: id, Enabled: false}
}
