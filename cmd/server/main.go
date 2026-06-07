// Package main 是 starcat-sharing-api 程序的入口点
package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"

	"starcat-sharing-api/internal/handler"
	"starcat-sharing-api/internal/store"
)

func main() {
	// 初始化配置
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://starcat.ink"
	}

	// 数据文件路径: 优先读取 STORE_FILE, 缺省使用当前目录的 data.json (本地开发默认)
	storeFile := os.Getenv("STORE_FILE")
	if storeFile == "" {
		storeFile = "data.json"
	}

	// 初始化存储
	s, err := store.NewMemoryStore(storeFile)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// 加载模板
	var templates *template.Template
	if tmpl, err := template.ParseGlob("templates/*.html"); err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	} else {
		templates = tmpl
	}

	// 初始化处理器
	shareHandler := handler.NewShareHandler(s, templates, baseURL)

	// 注册路由
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", rootHandler)
	mux.HandleFunc("POST /api/share", shareHandler.HandleCreateShare)
	mux.HandleFunc("GET /s/{id}", shareHandler.HandleViewShare)
	// 健康检查: Fly.io http_service.checks 用, 固定返回 200
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "5001"
	}

	log.Printf("Starting server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// rootHandler 服务首页 / 健康探活
// 风格与 github-trending-api 的 rootHandler 保持一致:
// 1. 路径必须精确是 "/" 否则 404 (避免掩盖未知路由)
// 2. 返回 JSON, 方便浏览器和 curl 都能用
// 附带可用路由列表, 方便手动测试时一眼看清楚能调什么
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{
		"service": "starcat-sharing-api",
		"status":  "ok",
		"endpoints": map[string]string{
			"create_share": "POST /api/share",
			"view_share":   "GET /s/{id}",
			"health":       "GET /healthz",
		},
	})
}
