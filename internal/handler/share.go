// Package handler 提供 HTTP 请求处理
package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/dong4j/starcat-sharing-api/internal/model"
	"github.com/dong4j/starcat-sharing-api/internal/store"
)

// ShareHandler 处理分享相关的 HTTP 请求
type ShareHandler struct {
	store     *store.MemoryStore
	templates *template.Template
	baseURL   string
}

// NewShareHandler 创建分享处理器
func NewShareHandler(s *store.MemoryStore, t *template.Template, baseURL string) *ShareHandler {
	return &ShareHandler{
		store:     s,
		templates: t,
		baseURL:   baseURL,
	}
}

// HandleCreateShare POST /api/share - 创建分享链接
func (h *ShareHandler) HandleCreateShare(w http.ResponseWriter, r *http.Request) {
	var req model.ShareRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := store.NewID(8)
	expiresAt := time.Now().AddDate(0, 1, 0) // 1个月有效期

	data := model.ShareData{
		ID:        id,
		Request:   req,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	h.store.Set(data)
	h.store.SaveAsync()

	resp := model.ShareResponseDTO{
		ShareUrl:  fmt.Sprintf("%s/s/%s", h.baseURL, id),
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleViewShare GET /s/{id} - 查看分享页面
func (h *ShareHandler) HandleViewShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	data, ok := h.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	err := h.templates.ExecuteTemplate(w, "share.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
