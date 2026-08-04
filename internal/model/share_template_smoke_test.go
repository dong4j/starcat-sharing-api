package model_test

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

func TestShareHTMLTemplateRendersHardPixel(t *testing.T) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"repoOwner": func(fullName string) string {
			if i := strings.IndexByte(fullName, '/'); i > 0 {
				return fullName[:i]
			}
			return fullName
		},
		"repoName": func(fullName string) string {
			if i := strings.IndexByte(fullName, '/'); i >= 0 && i+1 < len(fullName) {
				return fullName[i+1:]
			}
			return fullName
		},
	}).ParseFiles("../../templates/share.html")
	if err != nil {
		t.Fatalf("parse share.html: %v", err)
	}

	desc := "Self-hostable backend for Starcat similar-repository recommendations."
	lang := "Go"
	home := "https://starcat.ink"
	data := model.ShareData{
		ID:        "demo",
		CreatedAt: time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC),
		// ExpiresAt 零值 = 永不过期，页面不得出现 0001-01-01
		Request: model.ShareRepoRequest{
			Repo: model.ShareRepoDTO{
				FullName:         "starcat-app/starcat-recommend-api",
				Description:      &desc,
				Language:         &lang,
				StarsCount:       2,
				ForksCount:       0,
				SubscribersCount: 0,
				Topics:           []string{"api", "golang"},
				Homepage:         &home,
				URL:              "https://github.com/starcat-app/starcat-recommend-api",
			},
			AISummary: model.ShareAISummaryDTO{
				OneLiner: "**starcat-recommend-api** uses `/api/v1/repos/{repo_id}/recommendations`.",
				Summary:  "## 概述\n\nhello\n\n## 优势\n\n- a\n\n## 风险与限制\n\n- b\n\n## 外部参考来源\n\n- [x](https://example.com)\n",
				SuggestedTags: []model.ShareTagDTO{
					{Name: "Go"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "share.html", data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"starcat-recommend-api",
		"never expires",
		"github.com/starcat-app.png",
		`data-stars="2"`,
		`data-watch="0"`,
		"ONE_LINER",
		"[ WATCH ]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in rendered HTML", want)
		}
	}
	if strings.Contains(out, "0001-01-01") {
		t.Fatal("zero ExpiresAt must not render as 0001-01-01")
	}
}
