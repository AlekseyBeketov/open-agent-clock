package history

import (
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
