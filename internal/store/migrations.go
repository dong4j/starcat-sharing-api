// Package store 提供 SQLite schema 迁移管理。
//
// R-01 v1.2: 用 PRAGMA user_version 管理 schema 版本，避免引入 goose 等外部工具。
package store

import (
	"database/sql"
	"fmt"
	"log"
)

// migrate 执行所有未应用的 schema 迁移。
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	log.Printf("[migrate] current schema version: %d", version)

	if version < 1 {
		if err := migrateV1(db); err != nil {
			return err
		}
	}
	// 后续版本在此追加: if version < 2 { migrateV2(db) }

	return nil
}

// setUserVersion 在事务内更新 schema 版本号。
//
// 关键约束 1:必须接 *sql.Tx 而不是 *sql.DB，否则 setUserVersion 与 migrateV1 的 DDL
// 不在同一事务，rollback 时会留下「DDL 已撤但 user_version 已升」的不一致状态。
//
// 关键约束 2:SQLite 的 PRAGMA 语句不支持 ? 占位符绑定(database/sql 走 prepared
// statement 路径会直接报 `near "?": syntax error`)。所以必须用 fmt.Sprintf
// 把 version(int,无注入风险)拼进 SQL。参考 starcat-trending-api 同位置写法。
func setUserVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

// migrateV1 创建初始 shares 表。
func migrateV1(db *sql.DB) error {
	log.Println("[migrate] running migrateV1: create shares table")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS shares (
			id            TEXT PRIMARY KEY,
			repo_json     TEXT NOT NULL,
			ai_summary_json TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			expires_at    TEXT,
			visit_count   INTEGER NOT NULL DEFAULT 0,
			last_visited_at TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_shares_created_at ON shares(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_shares_expires_at ON shares(expires_at) WHERE expires_at IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	if err := setUserVersion(tx, 1); err != nil {
		return err
	}

	return tx.Commit()
}
