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

	githubclient "github.com/starcat-app/starcat-sharing-api/internal/github"
	"github.com/starcat-app/starcat-sharing-api/internal/model"
	"github.com/starcat-app/starcat-sharing-api/internal/store"
)

// ShareHandler 处理分享相关的 HTTP 请求。
type ShareHandler struct {
	store     store.Store
	templates *template.Template
	baseURL   string
	// repos 用于补全客户端 stars list 路径缺失的 subscribers_count（WATCH）。
	// 可为 nil（单测 / 无 token 环境）；补全失败不阻断创建与渲染。
	repos githubclient.RepositoryClient
}

// NewShareHandler 创建分享处理器。
func NewShareHandler(s store.Store, t *template.Template, baseURL string, repos githubclient.RepositoryClient) *ShareHandler {
	return &ShareHandler{
		store:     s,
		templates: t,
		baseURL:   baseURL,
		repos:     repos,
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

	// Starcat stars 同步不带 subscribers_count，客户端常传 0；创建前向 GitHub 补全。
	h.enrichSubscribers(r.Context(), &req)

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

	// 旧分享 / 客户端未传 WATCH：渲染期补全并回写，避免永久显示 0。
	if data.Request.Repo.SubscribersCount == 0 {
		h.enrichSubscribers(r.Context(), &data.Request)
		if data.Request.Repo.SubscribersCount > 0 {
			if err := h.store.Upsert(*data); err != nil {
				log.Printf("[share] backfill subscribers for %s failed: %v", id, err)
			}
		}
	}

	err = h.templates.ExecuteTemplate(w, "share.html", data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"template render failed: "+err.Error(), nil)
	}
}

// enrichSubscribers 在 SubscribersCount==0 时用 GitHub /repos 真值覆盖。
// 失败只打日志，不改变请求其它字段。
func (h *ShareHandler) enrichSubscribers(ctx context.Context, req *model.ShareRepoRequest) {
	if req == nil || req.Repo.SubscribersCount > 0 || h.repos == nil {
		return
	}
	owner, name := splitRepoFullName(req.Repo.FullName)
	if owner == "" || name == "" {
		return
	}
	preview, err := h.repos.FetchRepository(ctx, owner, name)
	if err != nil {
		log.Printf("[share] enrich subscribers skipped for %s: %v", req.Repo.FullName, err)
		return
	}
	if preview.Subscribers > 0 {
		req.Repo.SubscribersCount = preview.Subscribers
	}
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
