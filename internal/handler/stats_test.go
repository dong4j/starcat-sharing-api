package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/model"
	"github.com/starcat-app/starcat-sharing-api/internal/store"
)

type fakeStatsStore struct {
	count int
	err   error
}

func (f fakeStatsStore) Upsert(model.ShareData) error { return nil }

func (f fakeStatsStore) Get(string) (*model.ShareData, error) { return nil, nil }

func (f fakeStatsStore) CountShares() (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

func (f fakeStatsStore) GetShareStats(time.Time) (store.ShareStats, error) {
	if f.err != nil {
		return store.ShareStats{}, f.err
	}
	return store.ShareStats{TotalShares: f.count}, nil
}

func (f fakeStatsStore) ListShareActivity(string, int) ([]store.ShareActivity, error) {
	return nil, f.err
}

func (f fakeStatsStore) Close() error { return nil }

func TestHandleStats_ReturnsShareCount(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/stats", nil)

	HandleStats(fakeStatsStore{count: 123})(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var env model.Envelope[SharingStatsResponse]
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if env.Data.TotalShares != 123 {
		t.Fatalf("total_shares: want 123, got %d", env.Data.TotalShares)
	}
}

func TestHandleStats_CountError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/stats", nil)

	HandleStats(fakeStatsStore{err: errors.New("db down")})(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d body=%s", w.Code, w.Body.String())
	}
	var env model.ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if env.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("error code: want INTERNAL_ERROR, got %s", env.Error.Code)
	}
}
