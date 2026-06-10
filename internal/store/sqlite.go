package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"github.com/dong4j/starcat-sharing-api/internal/model"
)

// SQLiteStore 基于 SQLite 的分享数据持久化实现。
//
// R-01 v1.2: 替代旧的 MemoryStore（内存 + JSON 文件）。
// 连接策略与 trending / weekly 一致: WAL + busy_timeout=5000 + MaxOpenConns(1)。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 打开 SQLite 数据库并执行 migration。
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("[store] sqlite opened at %s", dsn)
	return &SQLiteStore{db: db}, nil
}

// Upsert 按 id 幂等创建或更新分享数据。
func (s *SQLiteStore) Upsert(data model.ShareData) error {
	repoJSON, err := json.Marshal(data.Request.Repo)
	if err != nil {
		return err
	}
	aiSummaryJSON, err := json.Marshal(data.Request.AISummary)
	if err != nil {
		return err
	}

	var expiresAt interface{}
	if !data.ExpiresAt.IsZero() {
		expiresAt = data.ExpiresAt.Format(time.RFC3339)
	}

	_, err = s.db.Exec(`
		INSERT INTO shares (id, repo_json, ai_summary_json, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			repo_json = excluded.repo_json,
			ai_summary_json = excluded.ai_summary_json,
			expires_at = excluded.expires_at
	`, data.ID, string(repoJSON), string(aiSummaryJSON), data.CreatedAt.Format(time.RFC3339), expiresAt)
	return err
}

// Get 按 id 查询分享数据, 同时递增访问计数。
func (s *SQLiteStore) Get(id string) (*model.ShareData, error) {
	var (
		repoJSON, aiSummaryJSON, createdAtStr string
		expiresAtStr                          sql.NullString
	)

	err := s.db.QueryRow(
		`SELECT repo_json, ai_summary_json, created_at, expires_at FROM shares WHERE id = ?`, id,
	).Scan(&repoJSON, &aiSummaryJSON, &createdAtStr, &expiresAtStr)

	if err != nil {
		return nil, err
	}

	// 反序列化业务数据
	//
	// R-01 P1-3b（2026-06-10）：JSON tag 已从 camelCase 改 snake_case，但
	// SQLite 里历史 blob 可能是老 camelCase 格式（v1.2 上线后 → P1 修订前
	// 写入的分享记录）。Go encoding/json 严格匹配 tag，老 blob 用新 model
	// 直接 Unmarshal 会得到全零值（不报错但所有字段空）。
	//
	// 兜底策略：先按 model 解，再做 zero-value 检测；若是老格式，fallback
	// 用 legacy shadow struct（camelCase tag）二次解码。
	repo, err := decodeShareRepoCompat(repoJSON)
	if err != nil {
		return nil, err
	}
	aiSummary, err := decodeAISummaryCompat(aiSummaryJSON)
	if err != nil {
		return nil, err
	}

	createdAt, _ := time.Parse(time.RFC3339, createdAtStr)
	var expiresAt time.Time
	if expiresAtStr.Valid {
		expiresAt, _ = time.Parse(time.RFC3339, expiresAtStr.String)
	}

	// 异步更新访问计数（不阻塞 HTML 渲染）
	go func() {
		s.db.Exec(
			"UPDATE shares SET visit_count = visit_count + 1, last_visited_at = ? WHERE id = ?",
			time.Now().Format(time.RFC3339), id,
		)
	}()

	return &model.ShareData{
		ID: id,
		Request: model.ShareRepoRequest{
			Repo:      repo,
			AISummary: aiSummary,
		},
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

// Close 关闭数据库连接。
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// MARK: - 历史 camelCase JSON blob 兼容（R-01 P1-3b 2026-06-10）

// legacyShareRepoDTO 与 model.ShareRepoDTO 字段一致，但 JSON tag 是 P1 修订前的 camelCase。
// 用于反序列化 SQLite 中历史写入的旧格式 blob。
type legacyShareRepoDTO struct {
	FullName    string   `json:"fullName"`
	Description *string  `json:"description"`
	Language    *string  `json:"language"`
	StarsCount  int      `json:"starsCount"`
	ForksCount  int      `json:"forksCount"`
	Topics      []string `json:"topics"`
	Homepage    *string  `json:"homepage"`
	URL         string   `json:"url"`
}

// legacyAISummaryDTO 与 model.ShareAISummaryDTO 字段一致，但 JSON tag 是 P1 修订前的 camelCase。
type legacyAISummaryDTO struct {
	OneLiner      string              `json:"oneLiner"`
	Summary       string              `json:"summary"`
	Platforms     []string            `json:"platforms"`
	SuitableFor   []string            `json:"suitableFor"`
	Strengths     []string            `json:"strengths"`
	Risks         []string            `json:"risks"`
	SuggestedTags []model.ShareTagDTO `json:"suggestedTags"`
}

// decodeShareRepoCompat 反序列化 ShareRepoDTO，自动兼容新旧格式。
//
// 策略：先按新 model（snake_case）解；若 FullName 为空（高概率是老 camelCase blob
// 用新 tag 解出全零值），fallback 用 legacyShareRepoDTO 重解，复制字段。
//
// 为什么用「FullName 是否为空」做判断而不是先解 legacy 再 fallback：
// 1. FullName 在业务语义里**永不为空**（前端必填 owner/repo），可作为「解析有效性」哨兵
// 2. 新格式优先：避免每次成功路径都解两次 JSON（性能）
func decodeShareRepoCompat(blob string) (model.ShareRepoDTO, error) {
	var repo model.ShareRepoDTO
	if err := json.Unmarshal([]byte(blob), &repo); err != nil {
		return repo, err
	}
	if repo.FullName != "" {
		return repo, nil
	}

	// fallback：尝试 legacy camelCase 解析
	var legacy legacyShareRepoDTO
	if err := json.Unmarshal([]byte(blob), &legacy); err != nil {
		return repo, err
	}
	if legacy.FullName == "" {
		// 都解不到 FullName，可能是真的空数据，按解出的零值返回
		return repo, nil
	}
	log.Printf("[store] decoded legacy camelCase ShareRepoDTO blob (full_name=%s); 建议运行 migration 升级 blob", legacy.FullName)
	return model.ShareRepoDTO{
		FullName:    legacy.FullName,
		Description: legacy.Description,
		Language:    legacy.Language,
		StarsCount:  legacy.StarsCount,
		ForksCount:  legacy.ForksCount,
		Topics:      legacy.Topics,
		Homepage:    legacy.Homepage,
		URL:         legacy.URL,
	}, nil
}

// decodeAISummaryCompat 同 decodeShareRepoCompat，但作用于 ShareAISummaryDTO。
// 哨兵字段用 OneLiner（业务语义必填）。
func decodeAISummaryCompat(blob string) (model.ShareAISummaryDTO, error) {
	var summary model.ShareAISummaryDTO
	if err := json.Unmarshal([]byte(blob), &summary); err != nil {
		return summary, err
	}
	if summary.OneLiner != "" {
		return summary, nil
	}

	var legacy legacyAISummaryDTO
	if err := json.Unmarshal([]byte(blob), &legacy); err != nil {
		return summary, err
	}
	if legacy.OneLiner == "" {
		return summary, nil
	}
	log.Printf("[store] decoded legacy camelCase ShareAISummaryDTO blob; 建议运行 migration 升级 blob")
	return model.ShareAISummaryDTO{
		OneLiner:      legacy.OneLiner,
		Summary:       legacy.Summary,
		Platforms:     legacy.Platforms,
		SuitableFor:   legacy.SuitableFor,
		Strengths:     legacy.Strengths,
		Risks:         legacy.Risks,
		SuggestedTags: legacy.SuggestedTags,
	}, nil
}
