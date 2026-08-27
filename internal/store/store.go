// Package store 定义分享数据持久化接口。
//
// R-01 v1.2: 从内存 + JSON 文件升级到 SQLite。
// Store 接口用于解耦 handler 与具体存储实现，便于单测 mock。
package store

import (
	"math/rand"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/model"
)

// Store 定义分享数据访问接口。
type Store interface {
	// Upsert 创建或更新分享记录（按 id 幂等）。
	Upsert(data model.ShareData) error

	// Get 按 id 获取分享数据。未找到返回 nil。
	Get(id string) (*model.ShareData, error)

	// CountShares 返回当前 shares 表中的分享记录总数。
	//
	// 本地 admin 面板只需要真实总量，不读取 payload 明细，避免为了统计把
	// repo_json / ai_summary_json 拉出来反序列化。
	CountShares() (int, error)

	// GetShareStats 返回 Admin Console 使用的聚合运营统计，不暴露分享 payload。
	GetShareStats(now time.Time) (ShareStats, error)

	// ListShareActivity 返回有界的最近或热门分享活动。
	ListShareActivity(sort string, limit int) ([]ShareActivity, error)

	// Close 关闭数据库连接。
	Close() error
}

// ShareStats 是分享业务的运营统计快照。
type ShareStats struct {
	TotalShares    int    `json:"total_shares"`
	ActiveShares   int    `json:"active_shares"`
	ExpiredShares  int    `json:"expired_shares"`
	Created24Hours int    `json:"created_24h"`
	Created7Days   int    `json:"created_7d"`
	Created30Days  int    `json:"created_30d"`
	TotalVisits    int64  `json:"total_visits"`
	VisitedShares  int    `json:"visited_shares"`
	LastCreatedAt  string `json:"last_created_at,omitempty"`
	LastVisitedAt  string `json:"last_visited_at,omitempty"`
}

// ShareActivity contains only fields needed for operations; repo payload and AI summary stay private.
type ShareActivity struct {
	ID            string  `json:"id"`
	FullName      string  `json:"full_name"`
	CreatedAt     string  `json:"created_at"`
	ExpiresAt     *string `json:"expires_at"`
	VisitCount    int64   `json:"visit_count"`
	LastVisitedAt *string `json:"last_visited_at"`
}

// NewID 生成指定长度的随机 base62 短链 ID。
func NewID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
