# AGENTS.md — starcat-sharing-api

> **唯一协作规范源**：本仓库根目录 `AGENTS.md` 是本项目协作规范的唯一正文维护源。
> 开工前还必须阅读并遵守上级 [`../AGENTS.md`](../AGENTS.md) 的跨仓规则。

## 项目概述

Starcat 分享页后端：接收 Repo 数据与 AI 摘要，生成短链（`POST /api/v1/share`）；公开路由渲染 HTML 与 OG 图片。R-01 起存储为 SQLite（WAL），替代旧 JSON 文件。生产环境经 `starcat-api` 聚合部署；本仓 tag 仅驱动 GitHub Release，不独立 Fly 生产部署。

## 技术栈

- Go 1.25.0 · `net/http`（无第三方 HTTP 框架）
- `github.com/joho/godotenv` · `modernc.org/sqlite`
- `github.com/starcat-app/starcat-api-kit` v0.3.0（auth / envelope / metrics 等）
- `golang.org/x/image`（OG PNG 渲染）
- Docker 多阶段构建 · Fly.io（历史独立 App `starcat-sharing-api`）

## 关键目录

```
cmd/server/           # 程序入口
server/               # 可导出装配，供 starcat-api 聚合引用
internal/handler/     # HTTP handlers
internal/store/       # SQLite 持久化
internal/render/      # HTML / OG 渲染
internal/cache/       # 公开仓库预览缓存
internal/github/      # GitHub 公开仓库读取
internal/assets/      # 嵌入模板与静态资源
scripts/deploy.sh     # 发版脚本（PR → merge → tag）
Makefile              # 统一命令入口
```

## 开发与测试命令

```bash
cp .env.example .env          # 填入 API_KEYS；GITHUB_TOKENS 本地可空
make deps                     # go mod download && go mod verify
make run                      # go run ./cmd/server
make build                    # 输出 bin/server
make test                     # go test -v -race -coverprofile=coverage.out ./...
make vet                      # go vet ./...
make fmt                      # go fmt ./...
make check                    # fmt-check + vet + test（PR 前）
make docker-build             # 本地镜像 starcat-sharing-api:latest
```

CI（`.github/workflows/go.yml`）等价检查：`go mod verify` · `gofmt -s` · `go vet ./...` · `docker build` · `go build` · `go test -race ./...`

默认端口 **5001**。环境变量见 `.env.example`：`PORT`、`STORE_FILE`、`METRICS_STORE_FILE`、`BASE_URL`、`API_KEYS`、`GITHUB_TOKENS`。

## 代码与架构约束

- **Module path**：`github.com/starcat-app/starcat-sharing-api`；项目内 import 必须用绝对 module path，禁止相对路径。
- **响应契约**：`/api/v1/*` 使用 envelope（`schema_version` + `data`）；JSON 字段 snake_case。
- **鉴权**：`/api/v1/*` 必须 `Authorization: Bearer <API_KEYS 之一>`；`/healthz`、`GET /s/{id}`、`GET /r/*`、`GET /og/repo/*` 公开，不得加鉴权（health check 与浏览器直访依赖此规则）。
- **共享代码**：auth / envelope / tokenpool 优先用 `starcat-api-kit`，禁止复制通用中间件到其他服务后再分叉维护。
- **GitHub 读取**：`/r/*` 与 OG 走 `GITHUB_TOKENS` 池；日志中 API Key / PAT 必须脱敏。
- 改 Go 代码后至少跑 `make check` 或 `go build ./... && go vet ./...`。

## 安全与数据边界

- **禁止入库**：`.env`、`*.db`、`*.db-wal`、`*.db-shm`、`bin/`、`coverage.out`、`logs/`、编译产物。
- **Secrets**：本地用 `.env`；Fly 用 `fly secrets set`；`.dockerignore` 须排除 `.env`。
- **数据文件**：`sharing.db`、`sharing-metrics.db` 为运行时生成，不可 commit。
- API Key 格式：`sk-starcat-<32 字符 base32 大写>`（43 字符总长）。

## 部署与发布禁令

以下操作 **必须经 dong4j 明确授权**，Agent 不得擅自执行：

- `make release`、`./scripts/deploy.sh`
- `fly deploy`、`fly secrets set`、改 `fly.toml` 的 `app` 字段
- `git push`、`git tag`、创建 GitHub Release
- 任何打包 / 上传 / 生产切流

业务仓 `scripts/deploy.sh` 注释明确：生产 Fly 部署统一由 `starcat-api` 聚合仓完成。
