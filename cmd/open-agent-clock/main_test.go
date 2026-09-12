package main

import (
	"testing"
	"time"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

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
