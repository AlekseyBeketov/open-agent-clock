package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquirePreventsOverlappingJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks", "job.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := Acquire(path); !errors.Is(err, ErrAlreadyHeld) {
		t.Fatalf("second acquire error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
}

func TestAcquireIgnoresStaleLockFileAfterProcessCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks", "job.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale pid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Release(); err != nil {
		t.Fatal(err)
	}
}
