# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Changed
- **AI 分享页 Hard Pixel 视觉（2026-08-05）**：`/s/{id}` 模板重做为暗色像素/极客风。
  - 修 `ExpiresAt` 零值误显示 `0001-01-01`；`one_liner` / summary 走 Markdown 渲染。
  - 指标区 STARS / FORKS / WATCH；契约新增可选 `repo.subscribers_count`（旧记录缺省 0）。
- **分享页 CSP 兼容（2026-08-05）**：`marked`/`DOMPurify` 改走 `unpkg.com`（starcat.ink CSP 禁 jsdelivr）；ONE_LINER 服务端纯文本兜底，脚本失败不再空白。
- **分享页 WATCH 服务端补全（2026-08-05）**：创建/渲染 `/s/{id}` 时若 `subscribers_count` 为 0，用 GitHub `/repos` 真值回填并落库（兼容 stars list 客户端缺字段）。

### Added
- **Canonical repository sharing (2026-07-21)**：新增公开 `/r/{owner}/{repo}` 服务端落地页和 `/og/repo/{owner}/{repo}.png` 动态 Open Graph 卡片。
  - 复用 quota-aware `GITHUB_TOKENS` pool，并增加有界 TTL cache 与并发请求合并。
  - 基础仓库链接保持无状态，不创建 share ID、不写入现有 `sharing.db`。
  - 仓库不存在、私有无权限和 GitHub 限流均使用不泄密 fallback 页面/图片。
- **R-03 (2026-06-11)**：新增 `GET /api/v1/ping` 端点，专给 Starcat 客户端「测试连接」按钮用。
  - 走 BearerAuth 中间件，鉴权通过返回 200 + envelope `{data: {service: "sharing", ok: true}}`；
    无效 / 缺失 Key → 401；服务故障 → 5xx。
  - 实现：`internal/handler/ping.go` + `internal/handler/ping_test.go`（7 case）。
  - 设计意图：之前客户端 auth probe 用 `GET /api/v1/share` 会触发 405（业务 endpoint 只接受 POST），
    现在统一走 ping，语义更清晰。
  - 跨项目约定：本 `ping.go` 与 trending / weekly / wiki 三个项目「除 import path 外 byte-level 一致」。

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

[Unreleased]: https://github.com/starcat-app/starcat-sharing-api/compare/v2.0.1...HEAD
[2.0.1]: https://github.com/starcat-app/starcat-sharing-api/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/starcat-app/starcat-sharing-api/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/starcat-app/starcat-sharing-api/releases/tag/v1.0.0
