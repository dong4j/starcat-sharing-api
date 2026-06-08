# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

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

[Unreleased]: https://github.com/dong4j/starcat-sharing-api/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/dong4j/starcat-sharing-api/releases/tag/v1.0.0
