package handler

import (
	"context"
	"html/template"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/cache"
	githubclient "github.com/starcat-app/starcat-sharing-api/internal/github"
	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

type fakeRepositorySource struct {
	repository model.RepositoryPreview
	err        error
}

func (f fakeRepositorySource) FetchRepository(context.Context, string, string) (model.RepositoryPreview, error) {
	return f.repository, f.err
}

type fakeOGRenderer struct{}

func (fakeOGRenderer) Render(model.RepositoryPreview, image.Image) ([]byte, error) {
	return []byte("png-data"), nil
}

func newRepositoryTestHandler(t *testing.T, source fakeRepositorySource) *RepositoryHandler {
	t.Helper()
	templates, err := template.New("").Funcs(template.FuncMap{
		"repoOwner": func(fullName string) string {
			for i := 0; i < len(fullName); i++ {
				if fullName[i] == '/' {
					return fullName[:i]
				}
			}
			return fullName
		},
		"repoName": func(fullName string) string {
			for i := 0; i < len(fullName); i++ {
				if fullName[i] == '/' {
					if i+1 < len(fullName) {
						return fullName[i+1:]
					}
					return ""
				}
			}
			return fullName
		},
	}).ParseGlob("../../templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	handler, err := NewRepositoryHandler(
		source,
		cache.NewRepositoryCache(time.Minute, 8),
		fakeOGRenderer{},
		templates,
		"https://starcat.ink",
	)
	if err != nil {
		t.Fatalf("NewRepositoryHandler: %v", err)
	}
	return handler
}

func TestRepositoryPageContainsServerRenderedOGMetadata(t *testing.T) {
	handler := newRepositoryTestHandler(t, fakeRepositorySource{repository: model.RepositoryPreview{
		ID:          99,
		Owner:       "starcat-app",
		Name:        "Starcat",
		FullName:    "starcat-app/Starcat",
		Description: `Fast <script>alert("x")</script> knowledge base`,
		Language:    "Swift",
		Stars:       1234,
		Forks:       42,
		HTMLURL:     "https://github.com/starcat-app/Starcat",
	}})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /r/{owner}/{repo}", handler.HandleRepositoryPage)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/starcat-app/Starcat", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`property="og:title" content="starcat-app/Starcat"`,
		`property="og:image" content="https://starcat.ink/og/repo/starcat-app/Starcat.png?rev=v3-99"`,
		`href="starcat://repo/starcat-app/Starcat?rid=99&amp;v=1"`,
		`class="brand-logo" src="/r/starcat-logo.png"`,
		`<strong>Swift</strong>`,
		`class="star-icon"`,
		`class="fork-icon"`,
		`<strong>1.2k</strong> stars`,
		`<strong>42</strong> forks`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in HTML", expected)
		}
	}
	if strings.Contains(body, `<script>alert`) {
		t.Fatal("repository description must be HTML escaped")
	}
}

func TestRepositoryPageShowsMissingLanguageWithoutInventingMetadata(t *testing.T) {
	handler := newRepositoryTestHandler(t, fakeRepositorySource{repository: model.RepositoryPreview{
		ID:       100,
		Owner:    "example",
		Name:     "docs-only",
		FullName: "example/docs-only",
		HTMLURL:  "https://github.com/example/docs-only",
	}})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /r/{owner}/{repo}", handler.HandleRepositoryPage)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/example/docs-only", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Language not detected") {
		t.Fatal("missing honest fallback for repositories without a primary language")
	}
}

func TestRepositoryPageUsesNonLeakingUnavailableState(t *testing.T) {
	handler := newRepositoryTestHandler(t, fakeRepositorySource{err: githubclient.ErrRepositoryUnavailable})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /r/{owner}/{repo}", handler.HandleRepositoryPage)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/private/repo", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "does not exist or is not publicly available") {
		t.Fatal("missing non-leaking unavailable message")
	}
}

func TestRepositoryOGRejectsMissingPNGSuffixWithFallback(t *testing.T) {
	handler := newRepositoryTestHandler(t, fakeRepositorySource{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /og/repo/{owner}/{repo}", handler.HandleRepositoryOG)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/og/repo/owner/repo", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("fallback must remain a PNG response: status=%d type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
}
