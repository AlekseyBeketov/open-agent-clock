package schedule

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

var scheduleIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

func ParseInterval(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("interval must not be empty")
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: %w", value, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("interval must be greater than zero")
	}
	return duration, nil
}

func ValidateTimezone(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("timezone must not be empty")
	}
	if _, err := time.LoadLocation(value); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", value, err)
	}
	return nil
}

func ValidateClock(value string) error {
	_, _, _, err := parseClock(value)
	return err
}

func Validate(value domain.Schedule) error {
	if !scheduleIDPattern.MatchString(value.ID) || value.ID == "." || value.ID == ".." {
		return fmt.Errorf("schedule id must match %q", scheduleIDPattern.String())
	}
	if strings.TrimSpace(value.TargetID) == "" {
		return fmt.Errorf("target id must not be empty")
	}
	if strings.TrimSpace(value.Prompt) == "" {
		return fmt.Errorf("prompt must not be empty")
	}
	if strings.TrimSpace(value.Timezone) == "" {
		return fmt.Errorf("timezone must not be empty")
	}
	if _, err := time.LoadLocation(value.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", value.Timezone, err)
	}
	switch value.Mode {
	case domain.ScheduleInterval:
		if _, err := ParseInterval(value.Interval); err != nil {
			return err
		}
	case domain.ScheduleDaily:
		if len(value.Times) == 0 {
			return fmt.Errorf("daily schedule requires at least one time")
		}
		for _, clock := range value.Times {
			if _, _, _, err := parseClock(clock); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported schedule mode %q", value.Mode)
	}
	return nil
}

func NextRun(value domain.Schedule, now time.Time, lastRun *time.Time) (time.Time, error) {
	if err := Validate(value); err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(value.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	switch value.Mode {
	case domain.ScheduleInterval:
		return nextInterval(value, now, lastRun)
	case domain.ScheduleDaily:
		return nextDaily(value, now, location)
	default:
		return time.Time{}, fmt.Errorf("unsupported schedule mode %q", value.Mode)
	}
}

func nextInterval(value domain.Schedule, now time.Time, lastRun *time.Time) (time.Time, error) {
	interval, err := ParseInterval(value.Interval)
	if err != nil {
		return time.Time{}, err
	}
	anchor := value.StartAt
	if anchor.IsZero() {
		anchor = now
	}
	candidate := anchor
	if lastRun != nil && !lastRun.IsZero() {
		candidate = lastRun.Add(interval)
	}
	if candidate.After(now) {
		return candidate, nil
	}
	elapsed := now.Sub(candidate)
	steps := elapsed/interval + 1
	return candidate.Add(time.Duration(steps) * interval), nil
}

func nextDaily(value domain.Schedule, now time.Time, location *time.Location) (time.Time, error) {
	localNow := now.In(location)
	for dayOffset := 0; dayOffset <= 1; dayOffset++ {
		date := localNow.AddDate(0, 0, dayOffset)
		var earliest time.Time
		for _, clock := range value.Times {
			hour, minute, second, err := parseClock(clock)
			if err != nil {
				return time.Time{}, err
			}
			candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, 0, location)
			if !candidate.After(localNow) {
				continue
			}
			if earliest.IsZero() || candidate.Before(earliest) {
				earliest = candidate
			}
		}
		if !earliest.IsZero() {
			return earliest, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not calculate next daily run")
}

func parseClock(value string) (int, int, int, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("15:04", value)
	if err == nil {
		return parsed.Hour(), parsed.Minute(), 0, nil
	}
	parsed, err = time.Parse("15:04:05", value)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid daily time %q: expected HH:MM or HH:MM:SS", value)
	}
	return parsed.Hour(), parsed.Minute(), parsed.Second(), nil
}
