package lock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var ErrAlreadyHeld = errors.New("lock is already held")

type File struct {
	path string
	file *os.File
}

func Acquire(path string) (*File, error) {
	if err := os.MkdirAll(filepathDir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrAlreadyHeld
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return &File{path: path, file: file}, nil
}

func (file *File) Release() error {
	if file == nil || file.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(file.file.Fd()), unix.LOCK_UN)
	closeErr := file.file.Close()
	removeErr := os.Remove(file.path)
	file.file = nil
	file.path = ""
	if unlockErr != nil {
		return fmt.Errorf("unlock: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close lock: %w", closeErr)
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("remove lock: %w", removeErr)
	}
	return nil
}

func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' {
			if index == 0 {
				return "/"
			}
			return path[:index]
		}
	}
	return "."
}
