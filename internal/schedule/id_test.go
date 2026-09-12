package schedule

import (
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestScheduleIDRejectsPathAndLabelCollisions(t *testing.T) {
	base := domain.Schedule{ID: "valid-id", TargetID: "target", Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "5h3m", Timezone: "UTC", StartAt: time.Now()}
	for _, id := range []string{"", ".", "..", "a/b", "a b", "-starts-with-dash", "a" + string(make([]byte, 65))} {
		base.ID = id
		if err := Validate(base); err == nil {
			t.Errorf("Validate(%q) unexpectedly succeeded", id)
		}
	}
}

func TestScheduleIDAcceptsStableValues(t *testing.T) {
	value := domain.Schedule{ID: "codex-window_01", TargetID: "target", Prompt: "hi", Mode: domain.ScheduleInterval, Interval: "5h3m", Timezone: "UTC", StartAt: time.Now()}
	if err := Validate(value); err != nil {
		t.Fatal(err)
	}
}
