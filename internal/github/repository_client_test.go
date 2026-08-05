package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientFetchRepositoryMapsPublicMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/starcat-app/Starcat" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Fatalf("missing GitHub Accept header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 99,
			"name": "Starcat",
			"full_name": "starcat-app/Starcat",
			"description": "Knowledge base",
			"stargazers_count": 1234,
			"forks_count": 42,
			"subscribers_count": 21,
			"language": "Swift",
			"topics": ["macos", "github"],
			"html_url": "https://github.com/starcat-app/Starcat",
			"default_branch": "dev",
			"archived": false,
			"is_template": false,
			"updated_at": "2026-07-21T12:00:00Z",
			"owner": {"login":"starcat-app","avatar_url":"https://avatars.githubusercontent.com/u/1"}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	repository, err := client.FetchRepository(context.Background(), "starcat-app", "Starcat")
	if err != nil {
		t.Fatalf("FetchRepository failed: %v", err)
	}
	if repository.ID != 99 || repository.FullName != "starcat-app/Starcat" || repository.Language != "Swift" {
		t.Fatalf("unexpected repository mapping: %+v", repository)
	}
	if repository.Stars != 1234 || repository.Forks != 42 || repository.Subscribers != 21 || len(repository.Topics) != 2 {
		t.Fatalf("repository statistics not mapped: %+v", repository)
	}
}

func TestClientHidesNotFoundAndPrivateDifference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, err := client.FetchRepository(context.Background(), "private", "repo")
	if err != ErrRepositoryUnavailable {
		t.Fatalf("want ErrRepositoryUnavailable, got %v", err)
	}
}

func TestClientReportsAnonymousRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, err := client.FetchRepository(context.Background(), "owner", "repo")
	if err != ErrRateLimited {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
}
