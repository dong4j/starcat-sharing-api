// Package main 是 starcat-sharing-api 的入口。
//
// R-01 v1.2: 升级到 SQLite 持久化 + Bearer Token 鉴权 + /api/v1/* 契约。
package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/dong4j/starcat-sharing-api/internal/handler"
	"github.com/dong4j/starcat-sharing-api/internal/middleware"
	"github.com/dong4j/starcat-sharing-api/internal/store"
)

func main() {
	// 加载 .env 文件（不存在不报错）
	if err := godotenv.Load(); err != nil {
		log.Printf("[env] no .env file found, using OS environment only")
	} else {
		log.Printf("[env] .env loaded")
	}

	// PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = "5001"
	}

	// STORE_FILE: SQLite 数据库路径
	storeFile := os.Getenv("STORE_FILE")
	if storeFile == "" {
		storeFile = "./sharing.db"
	}

	// BASE_URL: 短链 URL 拼接用
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://starcat.ink"
	}

	// API_KEYS: 鉴权白名单（逗号分隔）
	apiKeysStr := os.Getenv("API_KEYS")
	if apiKeysStr == "" {
		log.Fatal("API_KEYS env is required (comma-separated list of valid API keys)")
	}
	apiKeys := strings.Split(apiKeysStr, ",")

	// 初始化 SQLite store
	sqliteStore, err := store.NewSQLiteStore(storeFile)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer sqliteStore.Close()

	// 加载 HTML 模板
	var templates *template.Template
	if tmpl, err := template.ParseGlob("templates/*.html"); err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	} else {
		templates = tmpl
	}

	// 装配鉴权中间件
	authMW := middleware.NewBearerAuth(apiKeys)

	// 装配 handler
	shareHandler := handler.NewShareHandler(sqliteStore, templates, baseURL)

	// 注册路由（Go 1.22+ 风格）
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /s/{id}", shareHandler.HandleRenderShare)
	// R-03 (2026-06-11): /api/v1/ping 专门给 Starcat 客户端「测试连接」按钮用，
	// 在 middleware 后面挂——同时验证服务可达 + Bearer Key 正确。详见 handler/ping.go。
	mux.Handle("GET /api/v1/ping", authMW.Wrap(handler.HandlePingV1("sharing")))
	mux.Handle("POST /api/v1/share", authMW.Wrap(http.HandlerFunc(shareHandler.HandleCreateShareV1)))

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Received shutdown signal, closing service...")
		sqliteStore.Close()
		os.Exit(0)
	}()

	log.Printf("starcat-sharing-api starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /api/v1/ping   - Connectivity probe for Starcat client (auth required)")
	log.Printf("  POST /api/v1/share  - Create share link (auth required)")
	log.Printf("  GET  /s/{id}        - View share page (public)")
	log.Printf("  GET  /healthz       - Health check (public)")
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// healthzHandler Fly.io health check 用。
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
