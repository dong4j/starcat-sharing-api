# Starcat Sharing API

<!-- starcat-promo:start -->
<div align="center">
<a href="https://starcat.ink"><img src="https://raw.githubusercontent.com/starcat-app/starcat-pro/main/banner.webp" width="100%" alt="Starcat" /></a>

<p><strong>这是 Starcat 分享页面生成与托管的可自部署支撑服务。</strong></p>
<p>Starcat 是一款原生 macOS 应用，可以把 GitHub Stars 变成可搜索、可整理、可用 AI 追问的本地知识库。当前 1.4.0 支持 README 渲染、知识库 RAG、GitHub 通知、我的项目、全局与仓库洞察、macOS 桌面小组件、标签与私有笔记、Release 追踪、仓库健康度、AI 摘要、语义搜索、浏览器插件，以及 Alfred / uTools / Raycast 外部搜索，并提供多个可自部署 API。</p>

<a href="https://github.com/starcat-app/homebrew-starcat"><img src="https://img.shields.io/badge/Install%20with-Homebrew-FBBF24?style=for-the-badge&logo=homebrew&logoColor=white" width="220" alt="Install with Homebrew"/></a>
<br/>
<sub><a href="./README.md">English</a></sub>
</div>

<div align="center">
<a href="https://starcat.ink"><img src="https://img.shields.io/badge/website-starcat.ink-38BDF8?style=flat&color=blue" alt="website"/></a>
<a href="https://github.com/starcat-app/starcat-pro"><img src="https://img.shields.io/badge/support-starcat--pro-lightgrey.svg?style=flat&color=blue" alt="support"/></a>
<a href="https://github.com/starcat-app/homebrew-starcat"><img src="https://img.shields.io/badge/install-homebrew-lightgrey.svg?style=flat&color=blue" alt="homebrew"/></a>
<a href="https://github.com/starcat-app/starcat-localization"><img src="https://img.shields.io/badge/localization-open-lightgrey.svg?style=flat&color=blue" alt="localization"/></a>
</div>

<div align="center">
<img width="900" src="https://raw.githubusercontent.com/starcat-app/starcat-pro/main/main.webp" alt="Starcat main window"/>
</div>

**首选 Homebrew 安装：**

```bash
brew tap starcat-app/starcat
brew trust starcat-app/starcat
brew install --cask starcat
```

**相关链接：**

- 官网与下载: https://starcat.ink
- Mac App Store: 搜索 Starcat for GitHub
- 当前 Direct 版本: https://starcat.ink/downloads/Starcat-1.4.0-arm64.dmg
- 公开支持与发布说明: https://github.com/starcat-app/starcat-pro
- Starcat App Homebrew tap: https://github.com/starcat-app/homebrew-starcat
- CLI / MCP: [starcat-cli](https://github.com/starcat-app/starcat-cli) / [Homebrew tap](https://github.com/starcat-app/homebrew-starcat-cli)
- AI Agent Skill: https://github.com/starcat-app/starcat-skill
- 浏览器插件: [Chrome](https://github.com/starcat-app/starcat-chrome-plugin) / [Safari](https://github.com/starcat-app/starcat-safari-plugin)
- 启动器集成: [Alfred](https://github.com/starcat-app/starcat-alfred-workflow) / [uTools](https://github.com/starcat-app/starcat-utools-plugin) / [Raycast](https://github.com/starcat-app/starcat-raycast-extension)
- 官方文档: https://github.com/starcat-app/starcat-docs
- 官网源码: https://github.com/starcat-app/starcat-site
- 本地化: https://github.com/starcat-app/starcat-localization

**可自部署支撑 API：**

- [starcat-sharing-api](https://github.com/starcat-app/starcat-sharing-api)
- [starcat-trending-api](https://github.com/starcat-app/starcat-trending-api)
- [starcat-weekly-api](https://github.com/starcat-app/starcat-weekly-api)
- [starcat-wiki-api](https://github.com/starcat-app/starcat-wiki-api)
- [starcat-recommend-api](https://github.com/starcat-app/starcat-recommend-api)
- [starcat-discovery-api](https://github.com/starcat-app/starcat-discovery-api)

> Starcat 为普通用户提供默认托管服务。这个 API 开源出来，是为了让进阶用户可以审查实现、本地运行，或部署自己的实例。
<!-- starcat-promo:end -->

Starcat 应用的后端服务，提供 AI 分享页面的生成和托管功能。

> **R-01 v1.2**（2026-06-09）：存储从 JSON 文件迁移到 SQLite，加 Bearer Token 鉴权，API 升级到 `/api/v1/*`。

## 功能特性

- **分享链接生成**：接收 Repo 数据和 AI 摘要，生成唯一短链接（`POST /api/v1/share`，需鉴权）
- **分享页面渲染**：通过短链接访问时，Go `html/template` + Tailwind CSS 渲染分享页面（`GET /s/{id}`，公开）
- **基础仓库链接**：按 `owner/repo` 服务端渲染公开仓库落地页（`GET /r/{owner}/{repo}`，公开、无状态）
- **Open Graph 图片**：生成 `1280×640` PNG 仓库卡片（`GET /og/repo/{owner}/{repo}.png`，公开）
- **数据持久化**：SQLite（WAL 模式），替代旧的 `data.json` 文件存储

## 快速开始

### 环境要求

- Go 1.25+

### 本地运行

```bash
cp .env.example .env
# 编辑 .env，填入 API_KEYS（用 ../scripts/gen-api-key.sh 生成）
cd starcat-sharing-api
go run ./cmd/server/
```

默认端口 `5001`。

### .env 配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `5001` |
| `STORE_FILE` | SQLite 数据库路径 | `./sharing.db` |
| `METRICS_STORE_FILE` | 独立请求指标 SQLite 路径 | `./sharing-metrics.db` |
| `BASE_URL` | 短链基础 URL | `http://localhost:5001` |
| `API_KEYS` | Bearer Token 白名单（逗号分隔） | 必填 |
| `GITHUB_TOKENS` | GitHub PAT 池，供公开仓库预览使用 | 本地可空，生产必填 |

## API 接口

所有数据接口需要 `Authorization: Bearer <api-key>` 头。

### `GET /api/v1/ping`（需鉴权）

返回服务标识，以及由发布 tag 注入的构建版本：

```json
{"schema_version":1,"data":{"service":"sharing","version":"1.2.3","ok":true}}
```

### `POST /api/v1/share`（需鉴权）

创建分享链接。

**请求体**：

> R-01 P1-3b（2026-06-10）：所有 JSON 字段名统一为 snake_case，与 trending-api / weekly-api 风格一致。

```json
{
  "repo": {
    "full_name": "owner/repo",
    "description": "项目描述...",
    "language": "Swift",
    "stars_count": 12345,
    "forks_count": 1234,
    "topics": ["swift", "macos"],
    "homepage": "https://example.com",
    "url": "https://github.com/owner/repo"
  },
  "ai_summary": {
    "one_liner": "AI 一句话总结",
    "summary": "详细分析...",
    "platforms": ["macOS", "iOS"],
    "suitable_for": ["适合..."],
    "strengths": ["优势..."],
    "risks": ["风险..."],
    "suggested_tags": [{"name": "SwiftUI", "confidence": 0.95}]
  }
}
```

**响应 200**：

```json
{
  "schema_version": 1,
  "data": {
    "share_url": "https://starcat.ink/s/aBc1d2eF",
    "share_id": "aBc1d2eF",
    "expires_at": null,
    "created_at": "2026-06-09T12:00:00Z"
  }
}
```

### `GET /s/{id}`（公开）

访问分享页面，返回渲染后的 HTML。不存在则返回 404。

### `GET /r/{owner}/{repo}`（公开）

读取 GitHub 公开 metadata，并在首次 HTML 响应中直接输出完整 Open Graph metadata。基础链接不创建 share ID，也不写入 `sharing.db`。

### `GET /og/repo/{owner}/{repo}.png`（公开）

返回 `1280×640 image/png`。仓库不存在、GitHub 限流或头像读取失败时返回不泄密的 Starcat fallback 图片。

### `GET /healthz`（公开）

健康检查，返回 `ok`。

## 鉴权

所有 `/api/v1/*` 端点需要 `Authorization: Bearer <api-key>` 头。API Key 通过 `API_KEYS` 环境变量配置（逗号分隔多个 key）。

生成新 key：

```bash
bash ../scripts/gen-api-key.sh
```

## 部署（Fly.io）

```bash
fly secrets set \
  API_KEYS="sk-starcat-prodKey1,sk-starcat-prodKey2" \
  GITHUB_TOKENS="ghp_token1,ghp_token2" \
  BASE_URL="https://starcat.ink" \
  STORE_FILE="/data/sharing.db" \
  METRICS_STORE_FILE="/data/sharing-metrics.db" \
  -a starcat-sharing-api

fly deploy -a starcat-sharing-api
```
