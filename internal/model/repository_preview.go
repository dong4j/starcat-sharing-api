// Package model 定义公开仓库预览的数据边界。
//
// 仓库预览只包含 GitHub 公开 metadata，不包含 Starcat 用户身份、Star 状态、
// 笔记、标签或 AI 内容。基础分享链接是无状态能力，禁止写入 sharing.db。
package model

import (
	"html/template"
	"time"
)

// RepositoryPreview 是 HTML 页面和 OG 图片共享的只读仓库模型。
type RepositoryPreview struct {
	ID          int64
	Owner       string
	Name        string
	FullName    string
	Description string
	Language    string
	Stars       int
	Forks       int
	// Subscribers 是 GitHub 真实 Watch 数（subscribers_count），与 watchers_count 不同。
	Subscribers   int
	Topics        []string
	AvatarURL     string
	HTMLURL       string
	DefaultBranch string
	Archived      bool
	Template      bool
	UpdatedAt     time.Time
}

// RepositoryPageData 是 repository.html 的渲染模型。
//
// URL 均由 handler 通过 net/url 构造，模板只负责转义和展示，避免模板内拼接
// 未验证的 owner/repo 输入。
type RepositoryPageData struct {
	Repository   RepositoryPreview
	Available    bool
	StatusCode   int
	CanonicalURL string
	OGImageURL   string
	GitHubURL    string
	// OpenAppURL 是 handler 从已校验 owner/repo 构造的自定义 Scheme。
	// html/template 默认会拒绝非 HTTP scheme，因此这里显式使用 template.URL。
	OpenAppURL  template.URL
	DownloadURL string
	Title       string
	Description string
	StarsText   string
	ForksText   string
}
