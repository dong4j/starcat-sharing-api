// Package github 封装公开仓库 metadata 获取。
//
// 客户端只访问固定 GitHub API origin，owner/repo 必须先由 HTTP handler 校验。
// Token 通过 quota-aware pool 轮换；HTTP / 重试实现已收敛到 starcat-api-kit/github。
package github

import (
	"context"
	"errors"
	"strings"
	"time"

	kitgithub "github.com/starcat-app/starcat-api-kit/github"
	"github.com/starcat-app/starcat-api-kit/tokenpool"
	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

var (
	// ErrRepositoryUnavailable 故意合并 404 与无权限，避免探测私有仓库存在性。
	ErrRepositoryUnavailable = errors.New("repository does not exist or is unavailable")
	// ErrRateLimited 表示当前 token pool 暂时没有可用配额。
	ErrRateLimited = errors.New("GitHub API rate limit exhausted")
)

// RepositoryClient 是 handler 可替换测试的仓库数据源。
type RepositoryClient interface {
	FetchRepository(ctx context.Context, owner, name string) (model.RepositoryPreview, error)
}

// Client 调用 GitHub REST API（经 kit）。
type Client struct {
	inner *kitgithub.Client
}

// NewClient 创建 GitHub 仓库客户端。baseURL 仅供测试替换，生产传空字符串。
func NewClient(baseURL string, tokenValues []string) *Client {
	opt := kitgithub.Options{
		UserAgent:      "starcat-sharing-api/2.1",
		Pool:           tokenpool.New(tokenValues),
		Timeout:        10 * time.Second,
		AllowAnonymous: true, // 无 PAT 时仍可预览公开仓库（与历史行为一致）
	}
	if strings.TrimSpace(baseURL) != "" {
		opt.BaseURL = baseURL
	}
	return &Client{
		inner: kitgithub.NewClient(opt),
	}
}

// FetchRepository 精确读取公开仓库；404/401 合并为 ErrRepositoryUnavailable。
func (c *Client) FetchRepository(ctx context.Context, owner, name string) (model.RepositoryPreview, error) {
	repo, err := c.inner.GetRepo(ctx, owner, name)
	if err != nil {
		if errors.Is(err, kitgithub.ErrRepoNotFound) {
			return model.RepositoryPreview{}, ErrRepositoryUnavailable
		}
		if errors.Is(err, kitgithub.ErrRateLimited) {
			return model.RepositoryPreview{}, ErrRateLimited
		}
		var httpErr *kitgithub.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == 401 {
			return model.RepositoryPreview{}, ErrRepositoryUnavailable
		}
		return model.RepositoryPreview{}, err
	}
	return makePreview(repo), nil
}

func makePreview(repo *kitgithub.Repo) model.RepositoryPreview {
	description := ""
	if repo.Description != nil {
		description = strings.TrimSpace(*repo.Description)
	}
	language := ""
	if repo.Language != nil {
		language = strings.TrimSpace(*repo.Language)
	}
	avatar := ""
	if repo.OwnerAvatar != nil {
		avatar = *repo.OwnerAvatar
	}
	htmlURL := repo.HTMLURL
	if htmlURL == "" && repo.FullName != "" {
		htmlURL = "https://github.com/" + repo.FullName
	}
	updatedAt := time.Time{}
	if repo.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, repo.UpdatedAt); err == nil {
			updatedAt = t
		}
	}
	return model.RepositoryPreview{
		ID:            repo.ID,
		Owner:         repo.Owner,
		Name:          repo.Name,
		FullName:      repo.FullName,
		Description:   description,
		Language:      language,
		Stars:         repo.Stars,
		Forks:         repo.Forks,
		Subscribers:   repo.Subscribers,
		Topics:        repo.Topics,
		AvatarURL:     avatar,
		HTMLURL:       htmlURL,
		DefaultBranch: repo.DefaultBranch,
		Archived:      repo.Archived,
		Template:      repo.IsTemplate,
		UpdatedAt:     updatedAt,
	}
}
