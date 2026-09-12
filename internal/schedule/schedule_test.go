package schedule

import (
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestParseIntervalAcceptsFiveHoursAndThreeMinutes(t *testing.T) {
	value, err := ParseInterval("5h3m")
	if err != nil {
		t.Fatal(err)
	}
	if value != 5*time.Hour+3*time.Minute {
		t.Fatalf("interval = %s", value)
	}
}

func TestNextIntervalUsesLastRunAndSkipsMissedOccurrences(t *testing.T) {
	lastRun := time.Date(2026, time.January, 1, 5, 0, 0, 0, time.UTC)
	now := lastRun.Add(11*time.Hour + 10*time.Minute)
	spec := domain.Schedule{
		ID: "codex", TargetID: "native-codex", Prompt: "hi", Mode: domain.ScheduleInterval,
		Interval: "5h3m", Timezone: "UTC", StartAt: lastRun,
	}
	next, err := NextRun(spec, now, &lastRun)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2026, time.January, 1, 20, 9, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("next = %s, expected %s", next, expected)
	}
}

func TestNextDailyUsesIanaTimezone(t *testing.T) {
	now := time.Date(2026, time.January, 10, 1, 0, 0, 0, time.UTC)
	spec := domain.Schedule{
		ID: "morning", TargetID: "native-codex", Prompt: "hi", Mode: domain.ScheduleDaily,
		Times: []string{"05:00"}, Timezone: "Europe/Moscow",
	}
	next, err := NextRun(spec, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2026, time.January, 10, 5, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	if !next.Equal(expected) || next.Location().String() != "Europe/Moscow" {
		t.Fatalf("next = %s (%s), expected %s", next, next.Location(), expected)
	}
}

func TestNextDailyUsesResolvedDstOffset(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 1, 0, 30, 0, 0, location)
	spec := domain.Schedule{
		ID: "dst", TargetID: "native-codex", Prompt: "hi", Mode: domain.ScheduleDaily,
		Times: []string{"01:30"}, Timezone: "America/New_York",
	}
	next, err := NextRun(spec, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, offset := next.Zone()
	if next.Location().String() != "America/New_York" || name != "EDT" || offset != -4*60*60 {
		t.Fatalf("next = %s, zone=%s offset=%d", next, name, offset)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	cases := []domain.Schedule{
		{ID: "x", TargetID: "target", Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "0s", Timezone: "UTC"},
		{ID: "x", TargetID: "target", Prompt: "hi", Mode: domain.ScheduleDaily, Times: []string{"25:00"}, Timezone: "UTC"},
		{ID: "x", TargetID: "target", Prompt: "hi", Mode: domain.ScheduleDaily, Times: []string{"05:00"}, Timezone: "Not/AZone"},
	}
	for _, value := range cases {
		if err := Validate(value); err == nil {
			t.Fatalf("expected validation error for %+v", value)
		}
	}
}
