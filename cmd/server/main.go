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
	"time"

	"github.com/joho/godotenv"

	"github.com/starcat-app/starcat-sharing-api/internal/cache"
	githubclient "github.com/starcat-app/starcat-sharing-api/internal/github"
	"github.com/starcat-app/starcat-sharing-api/internal/handler"
	"github.com/starcat-app/starcat-sharing-api/internal/middleware"
	"github.com/starcat-app/starcat-sharing-api/internal/render"
	"github.com/starcat-app/starcat-sharing-api/internal/store"
	"github.com/starcat-app/starcat-sharing-api/internal/version"
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

	// GITHUB_TOKENS: 公开仓库预览使用。允许本地匿名启动，但生产环境应配置
	// token pool，避免聊天平台 crawler 消耗 GitHub 匿名额度。
	githubTokens := strings.Split(os.Getenv("GITHUB_TOKENS"), ",")

	// 初始化 SQLite store
	sqliteStore, err := store.NewSQLiteStore(storeFile)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer sqliteStore.Close()

	// 加载 HTML 模板（repoOwner / repoName 供 Hard Pixel 分享页拆 owner/repo）
	var templates *template.Template
	if tmpl, err := template.New("").Funcs(template.FuncMap{
		"repoOwner": repoOwner,
		"repoName":  repoName,
	}).ParseGlob("templates/*.html"); err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	} else {
		templates = tmpl
	}

	// 装配鉴权中间件
	authMW := middleware.NewBearerAuth(apiKeys)

	// 装配 handler（share 与 /r 预览共用 GitHub client，补全 WATCH）
	githubRepos := githubclient.NewClient(os.Getenv("GITHUB_API_BASE_URL"), githubTokens)
	shareHandler := handler.NewShareHandler(sqliteStore, templates, baseURL, githubRepos)
	repositoryRenderer, err := render.NewOGRenderer()
	if err != nil {
		log.Fatalf("Failed to initialize repository OG renderer: %v", err)
	}
	repositoryHandler, err := handler.NewRepositoryHandler(
		githubRepos,
		cache.NewRepositoryCache(time.Hour, 512),
		repositoryRenderer,
		templates,
		baseURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize repository preview handler: %v", err)
	}

	// 注册路由（Go 1.22+ 风格）
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /s/{id}", shareHandler.HandleRenderShare)
	// 品牌图标放在已由 Nginx 转发的 /r/ 下，确保官网反代、Fly 直连和本地预览
	// 都由同一服务响应，不依赖外部静态目录。
	mux.HandleFunc("GET /r/starcat-logo.png", starcatLogoHandler)
	mux.HandleFunc("GET /r/{owner}/{repo}", repositoryHandler.HandleRepositoryPage)
	// Go ServeMux wildcard 必须占完整 segment，因此 `.png` 后缀由 handler 校验。
	mux.HandleFunc("GET /og/repo/{owner}/{repo}", repositoryHandler.HandleRepositoryOG)
	// R-03 (2026-06-11): /api/v1/ping 专门给 Starcat 客户端「测试连接」按钮用，
	// 在 middleware 后面挂——同时验证服务可达 + Bearer Key 正确。详见 handler/ping.go。
	mux.Handle("GET /api/v1/ping", authMW.Wrap(handler.HandlePingV1(version.Service, version.Version)))
	mux.Handle("POST /api/v1/share", authMW.Wrap(http.HandlerFunc(shareHandler.HandleCreateShareV1)))
	mux.Handle("GET /internal/stats", authMW.Wrap(handler.HandleStats(sqliteStore)))

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Received shutdown signal, closing service...")
		sqliteStore.Close()
		os.Exit(0)
	}()

	log.Printf("starcat-sharing-api %s starting on port %s", version.Version, port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /api/v1/ping   - Connectivity probe for Starcat client (auth required)")
	log.Printf("  POST /api/v1/share  - Create share link (auth required)")
	log.Printf("  GET  /internal/stats - Share statistics (auth required)")
	log.Printf("  GET  /s/{id}        - View share page (public)")
	log.Printf("  GET  /r/{owner}/{repo} - View public repository preview")
	log.Printf("  GET  /og/repo/{owner}/{repo}.png - Repository Open Graph image")
	log.Printf("  GET  /healthz       - Health check (public)")
	handler := middleware.CORS(mux)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

// healthzHandler Fly.io health check 用。
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// starcatLogoHandler 提供网页与 favicon 共用的正式 Starcat App Icon。
func starcatLogoHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Write(render.StarcatLogoPNG())
}

// repoOwner 从 "owner/name" 取出 owner，供分享页模板使用。
func repoOwner(fullName string) string {
	if i := strings.IndexByte(fullName, '/'); i > 0 {
		return fullName[:i]
	}
	return fullName
}

// repoName 从 "owner/name" 取出 repo name，供分享页模板使用。
func repoName(fullName string) string {
	if i := strings.IndexByte(fullName, '/'); i >= 0 && i+1 < len(fullName) {
		return fullName[i+1:]
	}
	return fullName
}
