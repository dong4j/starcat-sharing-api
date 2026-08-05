// Package handler 提供分享相关的 HTTP 请求处理。
//
// R-01 v1.2: POST /api/v1/share 包 envelope + 错误响应统一形态;
// GET /s/{id} HTML 渲染不动，改为读 SQLite。
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/cache"
	githubclient "github.com/starcat-app/starcat-sharing-api/internal/github"
	"github.com/starcat-app/starcat-sharing-api/internal/model"
	"github.com/starcat-app/starcat-sharing-api/internal/store"
)

// ShareHandler 处理分享相关的 HTTP 请求。
type ShareHandler struct {
	store     store.Store
	templates *template.Template
	baseURL   string
	// repos + repoCache：打开分享页时刷新 STARS/FORKS/WATCH（方案 A）。
	// 可为 nil；失败回退 DB 快照，不阻断创建与渲染。
	repos     githubclient.RepositoryClient
	repoCache *cache.RepositoryCache
}

// NewShareHandler 创建分享处理器。
func NewShareHandler(
	s store.Store,
	t *template.Template,
	baseURL string,
	repos githubclient.RepositoryClient,
	repoCache *cache.RepositoryCache,
) *ShareHandler {
	return &ShareHandler{
		store:     s,
		templates: t,
		baseURL:   baseURL,
		repos:     repos,
		repoCache: repoCache,
	}
}

// HandleCreateShareV1 POST /api/v1/share - 创建分享链接（v1 envelope 响应）。
//
// 过期策略（R-01 v1.2）：
//   - data.ExpiresAt 留零值 → store.Upsert 写 NULL → 响应 ExpiresAt=nil → 永不过期
//   - 与设计文档 supports/docs/R-01-sharing-api-改造方案.md §3.1 一致
//     （schema 列 `expires_at TEXT NULL`，null 表示永不过期，nullable 设计）
//   - 当前业务无主动过期需求；如未来需要，应在请求体接受 expires_in 参数后再赋值。
func (h *ShareHandler) HandleCreateShareV1(w http.ResponseWriter, r *http.Request) {
	var req model.ShareRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST",
			"request body decode failed: "+err.Error(), nil)
		return
	}

	// 创建时也拉一次指标，避免落库就是过期 stars / 缺失 WATCH。
	h.refreshLiveMetrics(r.Context(), &req)

	id := store.NewID(8)
	now := time.Now()

	// 留 ExpiresAt 零值 → 落库为 NULL，响应中为 nil
	data := model.ShareData{
		ID:        id,
		Request:   req,
		CreatedAt: now,
	}

	if err := h.store.Upsert(data); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"failed to save share: "+err.Error(), nil)
		return
	}

	resp := model.ShareCreateResponse{
		ShareURL:  fmt.Sprintf("%s/s/%s", h.baseURL, id),
		ShareID:   id,
		ExpiresAt: nil, // 永不过期
		CreatedAt: now,
	}

	writeJSON(w, resp)
}

// HandleRenderShare GET /s/{id} - 查看分享页面（HTML 渲染，不鉴权）。
func (h *ShareHandler) HandleRenderShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	data, err := h.store.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// 方案 A：仅刷新本次响应的 STARS/FORKS/WATCH，不回写 DB；AI 正文保持创建快照。
	h.refreshLiveMetrics(r.Context(), &data.Request)

	err = h.templates.ExecuteTemplate(w, "share.html", data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"template render failed: "+err.Error(), nil)
	}
}

// refreshLiveMetrics 用 GitHub /repos（经进程内 TTL cache）覆盖公开指标。
// 不改 description / language / topics / AI 字段；失败保留原值。
func (h *ShareHandler) refreshLiveMetrics(ctx context.Context, req *model.ShareRepoRequest) {
	if req == nil || h.repos == nil {
		return
	}
	owner, name := splitRepoFullName(req.Repo.FullName)
	if owner == "" || name == "" {
		return
	}

	preview, err := h.fetchRepositoryPreview(ctx, owner, name)
	if err != nil {
		log.Printf("[share] live metrics skipped for %s: %v", req.Repo.FullName, err)
		return
	}

	req.Repo.StarsCount = preview.Stars
	req.Repo.ForksCount = preview.Forks
	req.Repo.SubscribersCount = preview.Subscribers
}

func (h *ShareHandler) fetchRepositoryPreview(ctx context.Context, owner, name string) (model.RepositoryPreview, error) {
	if h.repoCache == nil {
		return h.repos.FetchRepository(ctx, owner, name)
	}
	key := strings.ToLower(owner + "/" + name)
	return h.repoCache.GetOrLoad(ctx, key, func(loadContext context.Context) (model.RepositoryPreview, error) {
		return h.repos.FetchRepository(loadContext, owner, name)
	})
}

// splitRepoFullName 解析 "owner/name"；非法格式返回空串。
func splitRepoFullName(fullName string) (owner, name string) {
	fullName = strings.TrimSpace(fullName)
	i := strings.IndexByte(fullName, '/')
	if i <= 0 || i+1 >= len(fullName) {
		return "", ""
	}
	owner = fullName[:i]
	name = fullName[i+1:]
	if strings.Contains(name, "/") || owner == "" || name == "" {
		return "", ""
	}
	return owner, name
}
