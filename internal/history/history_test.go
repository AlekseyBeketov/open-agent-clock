package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	appconfig "github.com/AlekseyBeketov/open-agent-clock/internal/config"
	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestAppendAndListHistoryWithRetention(t *testing.T) {
	paths := appconfig.PathsForHome(t.TempDir())
	if err := appconfig.Init(paths); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		ended := time.Date(2026, time.January, 1, 0, index, 0, 0, time.UTC)
		result := domain.RunResult{JobID: "job", BindingID: "target", EndedAt: ended, Status: "success"}
		if err := Append(paths, result, 2); err != nil {
			t.Fatal(err)
		}
	}
	results, err := List(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || !results[0].EndedAt.After(results[1].EndedAt) {
		t.Fatalf("results = %+v", results)
	}
}

func TestListLegacyHistoryMakesTokenUsageUnavailable(t *testing.T) {
	paths := appconfig.PathsForHome(t.TempDir())
	if err := os.MkdirAll(paths.History, 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte(`{"binding_id":"legacy","status":"provider-failed","ended_at":"2026-01-01T00:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(paths.History, "legacy.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	results, err := List(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].TokenUsage == nil || results[0].TokenUsage.Availability != domain.TokenUsageUnavailable {
		t.Fatalf("legacy history result = %+v", results)
	}
}
