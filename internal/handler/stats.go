// Package handler 中的 stats.go 提供本地 admin 面板使用的分享统计。
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/starcat-app/starcat-sharing-api/internal/store"
)

// SharingStatsResponse 是 GET /internal/stats 的 data 结构。
type SharingStatsResponse = store.ShareStats

// HandleStats 返回 sharing-api 的真实分享记录总数。
func HandleStats(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		stats, err := s.GetShareStats(time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				"failed to aggregate shares: "+err.Error(), nil)
			return
		}
		writeJSON(w, stats)
	}
}

// HandleShareActivity returns a bounded recent/top list for the Admin Console.
func HandleShareActivity(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sortBy := r.URL.Query().Get("sort")
		if sortBy == "" {
			sortBy = "recent"
		}
		if sortBy != "recent" && sortBy != "visits" {
			writeError(w, http.StatusBadRequest, "INVALID_SORT", "sort must be recent or visits", nil)
			return
		}
		limit := 25
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit must be between 1 and 100", nil)
				return
			}
			limit = parsed
		}
		items, err := s.ListShareActivity(sortBy, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list share activity", nil)
			return
		}
		writeJSON(w, items)
	}
}
