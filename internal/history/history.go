package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

const DefaultRetention = 100

func Append(paths appconfig.Paths, result domain.RunResult, retention int) error {
	if retention <= 0 {
		retention = DefaultRetention
	}
	if err := os.MkdirAll(paths.History, 0o700); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}
	contents, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run result: %w", err)
	}
	contents = append(contents, '\n')
	name := fmt.Sprintf("%020d-%s.json", result.EndedAt.UnixNano(), safeName(result.JobID, result.BindingID))
	destination := filepath.Join(paths.History, name)
	temporary, err := os.CreateTemp(paths.History, ".history-*")
	if err != nil {
		return fmt.Errorf("create history file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set history permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write history: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close history: %w", err)
	}
	if err := os.Rename(temporaryName, destination); err != nil {
		return fmt.Errorf("store history: %w", err)
	}
	return prune(paths, retention)
}

func List(paths appconfig.Paths) ([]domain.RunResult, error) {
	entries, err := os.ReadDir(paths.History)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.RunResult{}, nil
		}
		return nil, fmt.Errorf("read history: %w", err)
	}
	results := make([]domain.RunResult, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join(paths.History, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read history record: %w", readErr)
		}
		var result domain.RunResult
		if decodeErr := json.Unmarshal(contents, &result); decodeErr != nil {
			return nil, fmt.Errorf("decode history record: %w", decodeErr)
		}
		result.NormalizeTelemetry()
		results = append(results, result)
	}
	sort.Slice(results, func(left, right int) bool { return results[left].EndedAt.After(results[right].EndedAt) })
	return results, nil
}

func prune(paths appconfig.Paths, retention int) error {
	entries, err := os.ReadDir(paths.History)
	if err != nil {
		return fmt.Errorf("read history for pruning: %w", err)
	}
	files := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, entry)
		}
	}
	if len(files) <= retention {
		return nil
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Name() < files[right].Name() })
	for _, entry := range files[:len(files)-retention] {
		if err := os.Remove(filepath.Join(paths.History, entry.Name())); err != nil {
			return fmt.Errorf("prune history record: %w", err)
		}
	}
	return nil
}

func safeName(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			value = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(value)
			return value
		}
	}
	return "run"
}
