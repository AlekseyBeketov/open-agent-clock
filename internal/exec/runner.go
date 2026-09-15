package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	osExec "os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/redact"
)

const OutputLimit = 8 * 1024

const outputTruncationMarker = "\n[output truncated]"

type Result struct {
	ExitCode *int
	Stdout   string
	Stderr   string
	TimedOut bool
	Err      error
}

func Run(ctx context.Context, executable string, args []string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	workingDir, err := os.MkdirTemp("", "open-agent-clock-run-")
	if err != nil {
		return Result{Err: fmt.Errorf("create temporary working directory: %w", err)}
	}
	defer os.RemoveAll(workingDir)

	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := osExec.CommandContext(runContext, executable, args...)
	command.Dir = workingDir
	command.Stdin = nil
	command.Env = safeEnvironment()
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout = &boundedBuffer{limit: OutputLimit}
	command.Stderr = &boundedBuffer{limit: OutputLimit}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = 2 * time.Second

	err = command.Run()
	result := Result{
		Stdout:   redact.Text(command.Stdout.(*boundedBuffer).String()),
		Stderr:   redact.Text(command.Stderr.(*boundedBuffer).String()),
		TimedOut: errors.Is(runContext.Err(), context.DeadlineExceeded),
		Err:      err,
	}
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		result.ExitCode = &code
	}
	return result
}

func safeEnvironment() []string {
	blocked := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "CLAUDE_API_KEY", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"}
	result := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		blockedEntry := false
		for _, prefix := range blocked {
			if name == prefix || strings.HasPrefix(name, prefix+"_") {
				blockedEntry = true
				break
			}
		}
		if !blockedEntry {
			result = append(result, entry)
		}
	}
	return result
}

type boundedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining <= 0 {
		buffer.truncated = true
		return len(value), nil
	}
	if len(value) > remaining {
		_, _ = buffer.buffer.Write(value[:remaining])
		buffer.truncated = true
		return len(value), nil
	}
	_, _ = buffer.buffer.Write(value)
	return len(value), nil
}

func (buffer *boundedBuffer) String() string {
	value := buffer.buffer.String()
	if buffer.truncated {
		available := buffer.limit - len(outputTruncationMarker)
		if available < 0 {
			available = 0
		}
		if len(value) > available {
			value = value[:available]
		}
		value += outputTruncationMarker
	}
	return value
}
