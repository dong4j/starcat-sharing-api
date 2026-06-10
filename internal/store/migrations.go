package store

import (
	"database/sql"
	"log"
)

// createSchema 初始化 sharing-api 全部数据表与索引。
//
// 全新服务,无版本迁移:任何现存 *.db 直接 rm 即可。首启动调一次
// CREATE TABLE IF NOT EXISTS 即可,不做 destructive migration。
func createSchema(db *sql.DB) error {
	log.Println("[migrate] createSchema: shares table")
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS shares (
			id              TEXT PRIMARY KEY,
			repo_json       TEXT NOT NULL,
			ai_summary_json TEXT NOT NULL,
			created_at      TEXT NOT NULL,
			expires_at      TEXT,
			visit_count     INTEGER NOT NULL DEFAULT 0,
			last_visited_at TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_shares_created_at ON shares(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_shares_expires_at ON shares(expires_at) WHERE expires_at IS NOT NULL;
	`)
	return err
}
