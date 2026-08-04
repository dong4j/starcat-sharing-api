// Package model 定义分享功能的数据模型
//
// R-01 P1-3b 修订（2026-06-10，dong4j 审计）：
// 原所有 JSON tag 走 camelCase（fullName / starsCount / aiSummary / shareUrl 等），
// 与 trending-api / weekly-api 的 snake_case（gh_repo_id / full_name / html_url 等）
// 风格不一致——三仓 API 是同一个客户端消费，命名风格必须统一。
//
// 决策：sharing-api 全量改 snake_case；前端 Starcat ShareAPI 的 JSONEncoder/
// JSONDecoder 同步设 keyEncodingStrategy/keyDecodingStrategy = .convertToSnakeCase /
// .convertFromSnakeCase，Swift 端属性名仍用 camelCase（语言习惯）但传输层走 snake。
//
// 这是 v1.2 schema 内的非破坏性 tag rename：所有 starcat 客户端在 P1 同步发布前都
// 还没用 sharing-api 的真实 endpoint（仍在 R-01 数据层接通中），所以直接 rename
// 不需要 backward compat 的双 tag 兼容期。
package model

import "time"

// ShareRepoDTO 仓库基本信息 DTO
type ShareRepoDTO struct {
	FullName    string   `json:"full_name"`
	Description *string  `json:"description"`
	Language    *string  `json:"language"`
	StarsCount  int      `json:"stars_count"`
	ForksCount  int      `json:"forks_count"`
	// SubscribersCount 是 GitHub 真实 Watch 数（subscribers_count）。
	// 旧分享记录缺省为 0；勿与 list API 的 watchers_count（常等于 stars）混淆。
	SubscribersCount int      `json:"subscribers_count"`
	Topics           []string `json:"topics"`
	Homepage         *string  `json:"homepage"`
	URL              string   `json:"url"`
}

// ShareTagDTO AI 推荐的标签 DTO
type ShareTagDTO struct {
	Name       string   `json:"name"`
	Confidence *float64 `json:"confidence"`
}

// ShareAISummaryDTO AI 摘要 DTO
type ShareAISummaryDTO struct {
	OneLiner      string        `json:"one_liner"`
	Summary       string        `json:"summary"`
	Platforms     []string      `json:"platforms"`
	SuitableFor   []string      `json:"suitable_for"`
	Strengths     []string      `json:"strengths"`
	Risks         []string      `json:"risks"`
	SuggestedTags []ShareTagDTO `json:"suggested_tags"`
}

// ShareRepoRequest 分享请求结构
type ShareRepoRequest struct {
	Repo      ShareRepoDTO      `json:"repo"`
	AISummary ShareAISummaryDTO `json:"ai_summary"`
}

// ShareResponseDTO 分享响应 DTO。
// R-01 v1.2: 扩 share_id + created_at 字段，expires_at 改为 nullable 指针。
type ShareResponseDTO struct {
	ShareURL  string     `json:"share_url"`
	ShareID   string     `json:"share_id"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// ShareCreateResponse 是 POST /api/v1/share 的 data 内 payload。
type ShareCreateResponse struct {
	ShareURL  string     `json:"share_url"`
	ShareID   string     `json:"share_id"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// ShareData 分享数据完整结构
type ShareData struct {
	ID        string           `json:"id"`
	Request   ShareRepoRequest `json:"request"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt time.Time        `json:"expires_at"`
}
