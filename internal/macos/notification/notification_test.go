package notification

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	calls  int
	name   string
	args   []string
	ctx    context.Context
	err    error
	called chan struct{}
}

func (runner *fakeCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	runner.calls++
	runner.name = name
	runner.args = append([]string(nil), args...)
	runner.ctx = ctx
	if runner.called != nil {
		runner.called <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}
	return runner.err
}

func TestPayloadContainsOnlySafeCompletionFields(t *testing.T) {
	typeOfPayload := reflect.TypeOf(Payload{})
	if typeOfPayload.NumField() != 3 {
		t.Fatalf("payload fields = %d, want 3", typeOfPayload.NumField())
	}
	for _, forbidden := range []string{"Prompt", "Stdout", "Stderr", "Credentials", "Reason"} {
		if _, ok := typeOfPayload.FieldByName(forbidden); ok {
			t.Fatalf("payload contains forbidden field %q", forbidden)
		}
	}
}

func TestNativeNotifierBuildsBoundedOsascriptCommand(t *testing.T) {
	runner := &fakeCommandRunner{}
	notifier := NewNativeWithRunner(runner, time.Second)
	payload := Payload{ScheduleID: "codex-window", TargetID: "native-codex", Classification: CompletionSuccess}

	if err := notifier.Notify(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || runner.name != commandPath {
		t.Fatalf("command = calls:%d name:%q", runner.calls, runner.name)
	}
	if !reflect.DeepEqual(runner.args[:1], []string{"-e"}) {
		t.Fatalf("osascript arguments = %#v", runner.args)
	}
	script := runner.args[1]
	for _, expected := range []string{"display notification", "codex-window", "native-codex", "succeeded", "open-agent-clock"} {
		if !strings.Contains(script, expected) {
			t.Errorf("script %q does not contain %q", script, expected)
		}
	}
	for _, forbidden := range []string{"prompt", "stdout", "stderr", "credential", "secret"} {
		if strings.Contains(strings.ToLower(script), forbidden) {
			t.Errorf("script contains forbidden content %q: %s", forbidden, script)
		}
	}
	if runner.ctx == nil {
		t.Fatal("runner context was nil")
	}
	if _, ok := runner.ctx.Deadline(); !ok {
		t.Fatal("notification command was not bounded")
	}
}

func TestNativeNotifierBuildsFailureClassification(t *testing.T) {
	runner := &fakeCommandRunner{}
	notifier := NewNativeWithRunner(runner, time.Second)
	payload := Payload{ScheduleID: "codex-window", TargetID: "hermes-codex", Classification: CompletionFailure}

	if err := notifier.Notify(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runner.args[1], "failed") || strings.Contains(runner.args[1], "succeeded") {
		t.Fatalf("failure script = %q", runner.args[1])
	}
}

func TestNativeNotifierRejectsInvalidPayloadWithoutRunningCommand(t *testing.T) {
	runner := &fakeCommandRunner{}
	notifier := NewNativeWithRunner(runner, time.Second)
	cases := []Payload{
		{ScheduleID: "codex/window", TargetID: "native-codex", Classification: CompletionSuccess},
		{ScheduleID: "codex-window", TargetID: "native codex", Classification: CompletionSuccess},
		{ScheduleID: "codex-window", TargetID: "native-codex", Classification: "unknown"},
	}
	for _, payload := range cases {
		if err := notifier.Notify(context.Background(), payload); err == nil {
			t.Fatalf("payload %#v was accepted", payload)
		}
	}
	if runner.calls != 0 {
		t.Fatalf("invalid payloads invoked command %d times", runner.calls)
	}
}

func TestNativeNotifierReturnsCommandError(t *testing.T) {
	expected := errors.New("synthetic command failure")
	runner := &fakeCommandRunner{err: expected}
	notifier := NewNativeWithRunner(runner, time.Second)

	err := notifier.Notify(context.Background(), Payload{ScheduleID: "codex-window", TargetID: "native-codex", Classification: CompletionFailure})
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v, want wrapped command error", err)
	}
}

func TestNativeNotifierBoundsBlockedCommand(t *testing.T) {
	runner := &fakeCommandRunner{called: make(chan struct{}, 1)}
	notifier := NewNativeWithRunner(runner, 10*time.Millisecond)

	err := notifier.Notify(context.Background(), Payload{ScheduleID: "codex-window", TargetID: "native-codex", Classification: CompletionSuccess})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("blocked notification error = %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("blocked notification calls = %d", runner.calls)
	}
}
