package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

func TestShareStatsAndActivity(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "sharing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	for _, data := range []model.ShareData{
		{ID: "active", Request: model.ShareRepoRequest{Repo: model.ShareRepoDTO{FullName: "starcat/app"}},
			CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)},
		{ID: "expired", Request: model.ShareRepoRequest{Repo: model.ShareRepoDTO{FullName: "starcat/old"}},
			CreatedAt: now.Add(-8 * 24 * time.Hour), ExpiresAt: now.Add(-time.Hour)},
	} {
		if err := store.Upsert(data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec("UPDATE shares SET visit_count=7, last_visited_at=? WHERE id='active'",
		now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	stats, err := store.GetShareStats(now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalShares != 2 || stats.ActiveShares != 1 || stats.ExpiredShares != 1 ||
		stats.Created24Hours != 1 || stats.TotalVisits != 7 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	activity, err := store.ListShareActivity("visits", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(activity) != 2 || activity[0].ID != "active" || activity[0].FullName != "starcat/app" {
		t.Fatalf("unexpected activity: %#v", activity)
	}
}
