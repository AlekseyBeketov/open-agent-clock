package launchd

import (
	"context"
	"fmt"
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

func TestGenerateIntervalPlistMatchesGoldenSnapshot(t *testing.T) {
	spec := Spec{
		ScheduleID:        "native/interval",
		ProgramArguments:  []string{"/usr/local/bin/open-agent-clock", "tick", "--schedule", "native/interval", "--confirm"},
		Interval:          5 * time.Hour,
		StandardOutPath:   "/Users/example/Library/Application Support/open-agent-clock/logs/native-interval.out.log",
		StandardErrorPath: "/Users/example/Library/Application Support/open-agent-clock/logs/native-interval.err.log",
		Environment:       map[string]string{"HOME": "/Users/example", "PATH": "/opt/homebrew/bin:/usr/bin:/bin"},
	}
	assertGoldenPlist(t, "interval.plist", spec)
}

func TestGenerateDailyPlistMatchesGoldenSnapshot(t *testing.T) {
	spec := Spec{
		ScheduleID:        "morning",
		ProgramArguments:  []string{"/usr/local/bin/open-agent-clock", "tick", "--schedule", "morning", "--confirm"},
		DailyTimes:        []string{"05:00", "13:30:45"},
		StandardOutPath:   "/Users/example/Library/Application Support/open-agent-clock/logs/morning.out.log",
		StandardErrorPath: "/Users/example/Library/Application Support/open-agent-clock/logs/morning.err.log",
		Environment:       map[string]string{"HOME": "/Users/example", "PATH": "/opt/homebrew/bin:/usr/bin:/bin"},
	}
	assertGoldenPlist(t, "daily.plist", spec)
}

func TestManagerLaunchdLifecycleUsesOnlyFakeLaunchctl(t *testing.T) {
	fakeLaunchctl, logPath, statePath := writeFakeLaunchctl(t)
	root := t.TempDir()
	manager := Manager{LaunchAgentsDir: filepath.Join(root, "LaunchAgents"), LaunchctlPath: fakeLaunchctl, UID: "501"}
	spec := Spec{
		ScheduleID:        "integration/job",
		ProgramArguments:  []string{"/bin/open-agent-clock", "tick", "--schedule", "integration/job", "--confirm"},
		Interval:          time.Minute,
		StandardOutPath:   filepath.Join(root, "logs", "job.out.log"),
		StandardErrorPath: filepath.Join(root, "logs", "job.err.log"),
		Environment:       map[string]string{"HOME": "/Users/example", "PATH": "/usr/bin:/bin"},
	}

	status, err := manager.Install(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || !status.Loaded || !fileExists(status.PlistPath) {
		t.Fatalf("install status = %+v", status)
	}

	inspected := manager.Inspect(context.Background(), domain.Schedule{ID: spec.ScheduleID, Enabled: true})
	if !inspected.Installed || !inspected.Loaded || !inspected.Enabled {
		t.Fatalf("inspect status = %+v", inspected)
	}
	if err := manager.Uninstall(context.Background(), spec.ScheduleID); err != nil {
		t.Fatal(err)
	}
	if fileExists(status.PlistPath) {
		t.Fatal("plist still exists after uninstall")
	}
	if contents, err := os.ReadFile(statePath); err != nil {
		t.Fatal(err)
	} else if strings.TrimSpace(string(contents)) != "" {
		t.Fatalf("fake launchctl still has loaded jobs: %q", contents)
	}

	commands, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantCommands := []string{
		"print gui/501/com.openagentclock.schedule.integration-job",
		fmt.Sprintf("bootstrap gui/501 %s", status.PlistPath),
		"print gui/501/com.openagentclock.schedule.integration-job",
		"bootout gui/501/com.openagentclock.schedule.integration-job",
	}
	if got := strings.Split(strings.TrimSpace(string(commands)), "\n"); !equalStrings(got, wantCommands) {
		t.Fatalf("launchctl commands = %#v, want %#v", got, wantCommands)
	}
}

func TestManagerInstallRemovesNewPlistWhenFakeBootstrapFails(t *testing.T) {
	fakeLaunchctl, _, _ := writeFakeLaunchctl(t)
	t.Setenv("FAKE_LAUNCHCTL_FAIL_BOOTSTRAP", "1")
	manager := Manager{LaunchAgentsDir: filepath.Join(t.TempDir(), "LaunchAgents"), LaunchctlPath: fakeLaunchctl, UID: "501"}
	spec := Spec{ScheduleID: "rollback", ProgramArguments: []string{"app", "tick"}, Interval: time.Minute}
	if _, err := manager.Install(context.Background(), spec); err == nil {
		t.Fatal("expected fake bootstrap failure")
	}
	if fileExists(manager.PlistPath(spec.ScheduleID)) {
		t.Fatal("new plist was left behind after bootstrap failure")
	}
}

func assertGoldenPlist(t *testing.T, name string, spec Spec) {
	t.Helper()
	got, err := Generate(spec)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", name)
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("generated plist %s differs from golden snapshot:\n--- got ---\n%s--- want ---\n%s", name, got, want)
	}
}

func writeFakeLaunchctl(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	scriptPath := filepath.Join(root, "launchctl")
	logPath := filepath.Join(root, "launchctl.log")
	statePath := filepath.Join(root, "launchctl.state")
	const script = `#!/bin/sh
set -eu
log="${FAKE_LAUNCHCTL_LOG}"
state="${FAKE_LAUNCHCTL_STATE}"
printf '%s\n' "$*" >> "$log"
case "${1:-}" in
print)
  if [ -n "${2:-}" ] && grep -F -x "$2" "$state" >/dev/null 2>&1; then
    exit 0
  fi
  printf '%s\n' 'Could not find service' >&2
  exit 1
  ;;
bootstrap)
  if [ "${FAKE_LAUNCHCTL_FAIL_BOOTSTRAP:-}" = "1" ]; then
    printf '%s\n' 'bootstrap-failed' >&2
    exit 1
  fi
  label="${3##*/}"
  label="${label%.plist}"
  service="$2/$label"
  if ! grep -F -x "$service" "$state" >/dev/null 2>&1; then
    printf '%s\n' "$service" >> "$state"
  fi
  exit 0
  ;;
bootout)
  service="$2"
  if ! grep -F -x "$service" "$state" >/dev/null 2>&1; then
    printf '%s\n' 'Could not find service' >&2
    exit 1
  fi
  temporary="${state}.tmp"
  grep -F -x -v "$service" "$state" > "$temporary" || true
  mv "$temporary" "$state"
  exit 0
  ;;
*)
  exit 2
  ;;
esac
`
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_LAUNCHCTL_LOG", logPath)
	t.Setenv("FAKE_LAUNCHCTL_STATE", statePath)
	return scriptPath, logPath, statePath
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
