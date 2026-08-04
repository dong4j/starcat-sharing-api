// Package github 封装公开仓库 metadata 获取。
//
// 客户端只访问固定 GitHub API origin，owner/repo 必须先由 HTTP handler 校验。
// Token 通过 quota-aware pool 轮换，日志中禁止输出 token 原文。
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/model"
	"github.com/starcat-app/starcat-sharing-api/internal/tokenpool"
)

var (
	// ErrRepositoryUnavailable 故意合并 404 与无权限，避免探测私有仓库存在性。
	ErrRepositoryUnavailable = errors.New("repository does not exist or is unavailable")
	// ErrRateLimited 表示当前 token pool 暂时没有可用配额。
	ErrRateLimited = errors.New("GitHub API rate limit exhausted")
)

type repositoryResponse struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	FullName         string    `json:"full_name"`
	Description      *string   `json:"description"`
	StargazersCount  int       `json:"stargazers_count"`
	ForksCount       int       `json:"forks_count"`
	SubscribersCount int       `json:"subscribers_count"`
	Language         *string   `json:"language"`
	Topics           []string  `json:"topics"`
	HTMLURL          string    `json:"html_url"`
	DefaultBranch    string    `json:"default_branch"`
	Archived         bool      `json:"archived"`
	IsTemplate       bool      `json:"is_template"`
	UpdatedAt        time.Time `json:"updated_at"`
	Owner            struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

// RepositoryClient 是 handler 可替换测试的仓库数据源。
type RepositoryClient interface {
	FetchRepository(ctx context.Context, owner, name string) (model.RepositoryPreview, error)
}

// Client 调用 GitHub REST API。
type Client struct {
	baseURL   *url.URL
	http      *http.Client
	pool      *tokenpool.Pool
	hasTokens bool
}

// NewClient 创建 GitHub 仓库客户端。baseURL 仅供测试替换，生产传空字符串。
func NewClient(baseURL string, tokenValues []string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.github.com"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		panic("invalid GitHub API base URL: " + err.Error())
	}
	validTokenCount := 0
	for _, token := range tokenValues {
		if strings.TrimSpace(token) != "" {
			validTokenCount++
		}
	}
	return &Client{
		baseURL:   parsed,
		http:      &http.Client{Timeout: 10 * time.Second},
		pool:      tokenpool.New(tokenValues),
		hasTokens: validTokenCount > 0,
	}
}

// FetchRepository 精确读取公开仓库，最多在 token pool 内切换三次。
func (c *Client) FetchRepository(ctx context.Context, owner, name string) (model.RepositoryPreview, error) {
	attempts := 1
	if c.hasTokens {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		var token *tokenpool.TokenState
		if c.hasTokens {
			token = c.pool.PickBest()
			if token == nil {
				return model.RepositoryPreview{}, ErrRateLimited
			}
		}

		endpoint := c.baseURL.JoinPath("repos", owner, name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return model.RepositoryPreview{}, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "starcat-sharing-api/2.1")
		if token != nil {
			req.Header.Set("Authorization", "Bearer "+token.Value)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return model.RepositoryPreview{}, fmt.Errorf("fetch GitHub repository: %w", err)
		}
		if token != nil {
			c.pool.UpdateFromResponse(token, resp)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			defer resp.Body.Close()
			var payload repositoryResponse
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return model.RepositoryPreview{}, fmt.Errorf("decode GitHub repository: %w", err)
			}
			return makePreview(payload), nil
		case http.StatusNotFound, http.StatusUnauthorized:
			resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized && token != nil {
				continue
			}
			return model.RepositoryPreview{}, ErrRepositoryUnavailable
		case http.StatusForbidden, http.StatusTooManyRequests:
			resetAt := parseReset(resp.Header.Get("X-RateLimit-Reset"))
			if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter.After(resetAt) {
				resetAt = retryAfter
			}
			resp.Body.Close()
			if token != nil {
				c.pool.DisableUntil(token, resetAt, fmt.Sprintf("GitHub status %d", resp.StatusCode))
				continue
			}
			return model.RepositoryPreview{}, ErrRateLimited
		default:
			status := resp.StatusCode
			resp.Body.Close()
			if status >= 500 && token != nil {
				continue
			}
			return model.RepositoryPreview{}, fmt.Errorf("GitHub API returned status %d", status)
		}
	}
	return model.RepositoryPreview{}, ErrRateLimited
}

func makePreview(payload repositoryResponse) model.RepositoryPreview {
	description := ""
	if payload.Description != nil {
		description = strings.TrimSpace(*payload.Description)
	}
	language := ""
	if payload.Language != nil {
		language = strings.TrimSpace(*payload.Language)
	}
	return model.RepositoryPreview{
		ID:            payload.ID,
		Owner:         payload.Owner.Login,
		Name:          payload.Name,
		FullName:      payload.FullName,
		Description:   description,
		Language:      language,
		Stars:         payload.StargazersCount,
		Forks:         payload.ForksCount,
		Subscribers:   payload.SubscribersCount,
		Topics:        payload.Topics,
		AvatarURL:     payload.Owner.AvatarURL,
		HTMLURL:       payload.HTMLURL,
		DefaultBranch: payload.DefaultBranch,
		Archived:      payload.Archived,
		Template:      payload.IsTemplate,
		UpdatedAt:     payload.UpdatedAt,
	}
}

func parseReset(raw string) time.Time {
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Now().Add(time.Minute)
	}
	return time.Unix(seconds, 0)
}

func parseRetryAfter(raw string) time.Time {
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(seconds) * time.Second)
}
