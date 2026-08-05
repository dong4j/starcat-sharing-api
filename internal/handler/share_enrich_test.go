package handler

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/cache"
	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

type fakeRepoClient struct {
	stars       int
	forks       int
	subscribers int
	err         error
	calls       int
}

func (f *fakeRepoClient) FetchRepository(ctx context.Context, owner, name string) (model.RepositoryPreview, error) {
	f.calls++
	if f.err != nil {
		return model.RepositoryPreview{}, f.err
	}
	return model.RepositoryPreview{
		Owner:       owner,
		Name:        name,
		FullName:    owner + "/" + name,
		Stars:       f.stars,
		Forks:       f.forks,
		Subscribers: f.subscribers,
	}, nil
}

type memoryShareStore struct {
	data map[string]model.ShareData
}

func (m *memoryShareStore) Upsert(data model.ShareData) error {
	if m.data == nil {
		m.data = map[string]model.ShareData{}
	}
	m.data[data.ID] = data
	return nil
}

func (m *memoryShareStore) Get(id string) (*model.ShareData, error) {
	d, ok := m.data[id]
	if !ok {
		return nil, errors.New("not found")
	}
	copy := d
	return &copy, nil
}

func (m *memoryShareStore) CountShares() (int, error) { return len(m.data), nil }

func (m *memoryShareStore) Close() error { return nil }

func TestCreateShareRefreshesLiveMetrics(t *testing.T) {
	store := &memoryShareStore{}
	repos := &fakeRepoClient{stars: 900, forks: 60, subscribers: 21}
	h := NewShareHandler(store, template.New("x"), "https://starcat.ink", repos, nil)

	body := `{
		"repo": {
			"full_name": "tjhorner/upsy-desky",
			"stars_count": 817,
			"forks_count": 54,
			"subscribers_count": 0,
			"topics": [],
			"url": "https://github.com/tjhorner/upsy-desky"
		},
		"ai_summary": {
			"one_liner": "desk",
			"summary": "## 概述\n\nok",
			"platforms": [],
			"suitable_for": [],
			"strengths": [],
			"risks": [],
			"suggested_tags": []
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/share", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleCreateShareV1(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if repos.calls != 1 {
		t.Fatalf("expected 1 github fetch, got %d", repos.calls)
	}
	for _, d := range store.data {
		if d.Request.Repo.StarsCount != 900 || d.Request.Repo.ForksCount != 60 || d.Request.Repo.SubscribersCount != 21 {
			t.Fatalf("metrics not refreshed: %+v", d.Request.Repo)
		}
	}
}

func TestRenderShareRefreshesMetricsWithoutWritingStore(t *testing.T) {
	store := &memoryShareStore{data: map[string]model.ShareData{
		"abc12345": {
			ID: "abc12345",
			Request: model.ShareRepoRequest{
				Repo: model.ShareRepoDTO{
					FullName:         "tjhorner/upsy-desky",
					StarsCount:       817,
					ForksCount:       54,
					SubscribersCount: 0,
					URL:              "https://github.com/tjhorner/upsy-desky",
				},
				AISummary: model.ShareAISummaryDTO{OneLiner: "desk", Summary: "ok"},
			},
			CreatedAt: time.Now(),
		},
	}}
	tmpl := template.Must(template.New("share.html").Parse(
		`STARS={{.Request.Repo.StarsCount}} FORKS={{.Request.Repo.ForksCount}} WATCH={{.Request.Repo.SubscribersCount}}`,
	))
	repos := &fakeRepoClient{stars: 901, forks: 61, subscribers: 22}
	h := NewShareHandler(store, tmpl, "https://starcat.ink", repos, cache.NewRepositoryCache(time.Hour, 8))

	req := httptest.NewRequest(http.MethodGet, "/s/abc12345", nil)
	req.SetPathValue("id", "abc12345")
	rr := httptest.NewRecorder()
	h.HandleRenderShare(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "STARS=901") ||
		!strings.Contains(rr.Body.String(), "FORKS=61") ||
		!strings.Contains(rr.Body.String(), "WATCH=22") {
		t.Fatalf("body=%q", rr.Body.String())
	}
	// 方案 A：不回写 DB，快照保持创建时数字。
	stored := store.data["abc12345"].Request.Repo
	if stored.StarsCount != 817 || stored.ForksCount != 54 || stored.SubscribersCount != 0 {
		t.Fatalf("store should stay snapshot: %+v", stored)
	}
}

func TestRenderShareKeepsSnapshotWhenGitHubFails(t *testing.T) {
	store := &memoryShareStore{data: map[string]model.ShareData{
		"abc12345": {
			ID: "abc12345",
			Request: model.ShareRepoRequest{
				Repo: model.ShareRepoDTO{
					FullName:         "tjhorner/upsy-desky",
					StarsCount:       817,
					ForksCount:       54,
					SubscribersCount: 3,
					URL:              "https://github.com/tjhorner/upsy-desky",
				},
				AISummary: model.ShareAISummaryDTO{OneLiner: "desk", Summary: "ok"},
			},
			CreatedAt: time.Now(),
		},
	}}
	tmpl := template.Must(template.New("share.html").Parse(
		`STARS={{.Request.Repo.StarsCount}} WATCH={{.Request.Repo.SubscribersCount}}`,
	))
	repos := &fakeRepoClient{err: errors.New("boom")}
	h := NewShareHandler(store, tmpl, "https://starcat.ink", repos, nil)

	req := httptest.NewRequest(http.MethodGet, "/s/abc12345", nil)
	req.SetPathValue("id", "abc12345")
	rr := httptest.NewRecorder()
	h.HandleRenderShare(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "STARS=817") || !strings.Contains(rr.Body.String(), "WATCH=3") {
		t.Fatalf("body=%q", rr.Body.String())
	}
}

func TestSplitRepoFullName(t *testing.T) {
	o, n := splitRepoFullName("a/b")
	if o != "a" || n != "b" {
		t.Fatalf("got %q %q", o, n)
	}
	o, n = splitRepoFullName("bad")
	if o != "" || n != "" {
		t.Fatalf("want empty, got %q %q", o, n)
	}
}
