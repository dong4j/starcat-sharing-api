# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [2.0.1] - 2026-06-10

### Changed
- **全新服务语态（dong4j 拍板）**：删除 `migrateV1` / `setUserVersion` / `PRAGMA user_version` 机制，`migrations.go` 改为单文件 `createSchema(db)` 函数。任何现存 `sharing.db` 直接 `rm` 即可，不做 destructive migration。

## [2.0.0] - 2026-06-09

### Added
- Bearer Token 鉴权（`Authorization: Bearer <api-key>`），所有 `/api/v1/*` 端点强制鉴权
- SQLite 持久化存储（替代内存 + JSON 文件），支持 WAL 模式
- `.env` 配置文件支持（godotenv）
- Schema migration 机制（`PRAGMA user_version`，2.0.1 拆除）
- Envelope 统一响应格式（`schema_version` + `data` / `error`）

### Changed
- **Breaking**: `POST /api/share` → `POST /api/v1/share`（旧路径直接删除）
- **Breaking**: 存储从 `data.json` 迁移到 SQLite，旧数据不做迁移
- **Breaking**: 所有 `/api/v1/*` 端点需携带 Bearer Token
- `GET /s/{id}` 仍保留（公开，不鉴权）

### Removed
- `POST /api/share` 旧端点
- `internal/store/memory.go`（内存 + JSON 文件存储）

## [1.0.0] - 2026-06-08

### Added
- 分享链接生成 API（`POST /api/share`）
- 分享页面渲染（`GET /s/{id}`）
- 基于内存 + JSON 文件的数据持久化
- 响应式分享卡片模板（支持 Light 主题）
- GitHub Actions CI 工作流
- Issue / PR 模板
- 贡献指南和变更日志
- 内部版本号包 (`internal/version`, 暴露 `version.Version` 常量)

[Unreleased]: https://github.com/dong4j/starcat-sharing-api/compare/v2.0.1...HEAD
[2.0.1]: https://github.com/dong4j/starcat-sharing-api/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/dong4j/starcat-sharing-api/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/dong4j/starcat-sharing-api/releases/tag/v1.0.0
