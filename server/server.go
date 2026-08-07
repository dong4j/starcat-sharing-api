// Package server 导出 sharing-api 的可装配 HTTP 服务。
//
// 单仓部署走 cmd/server；聚合部署（starcat-api）import 本包并挂到网关。
// 模板与 static 仍相对进程工作目录解析（与历史 cmd/server 一致）。
package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	kitenv "github.com/starcat-app/starcat-api-kit/env"
	"github.com/starcat-app/starcat-sharing-api/internal/cache"
	githubclient "github.com/starcat-app/starcat-sharing-api/internal/github"
	"github.com/starcat-app/starcat-sharing-api/internal/handler"
	"github.com/starcat-app/starcat-sharing-api/internal/middleware"
	"github.com/starcat-app/starcat-sharing-api/internal/render"
	"github.com/starcat-app/starcat-sharing-api/internal/store"
	"github.com/starcat-app/starcat-sharing-api/internal/version"
)

const (
	defaultPort      = "5001"
	defaultStoreFile = "./sharing.db"
	defaultBaseURL   = "https://starcat.ink"
)

// Options 控制 sharing 服务装配。聚合网关可显式传入，单仓部署通常用 FromEnv。
type Options struct {
	Port                   string
	StoreFile              string
	BaseURL                string
	APIKeys                []string
	GithubTokens           []string
	GithubAPIBaseURL       string
	SkipListenLogEndpoints bool
}

// Service 是已装配的 sharing HTTP 服务。
type Service struct {
	opts        Options
	handler     http.Handler
	sqliteStore *store.SQLiteStore
}

// Name 返回聚合网关识别用的稳定服务名。
func Name() string { return "sharing" }

// DefaultPort 返回单仓默认监听端口。
func DefaultPort() string { return defaultPort }

// FromEnv 从环境变量装配服务（与历史 cmd/server 行为一致）。
func FromEnv() (*Service, error) {
	apiKeys, err := kitenv.RequiredCSV("API_KEYS")
	if err != nil {
		return nil, fmt.Errorf("API_KEYS env is required (comma-separated list of valid API keys)")
	}
	opt := Options{
		Port:             kitenv.OrDefault("PORT", defaultPort),
		StoreFile:        kitenv.OrDefault("STORE_FILE", defaultStoreFile),
		BaseURL:          kitenv.OrDefault("BASE_URL", defaultBaseURL),
		APIKeys:          apiKeys,
		GithubTokens:     kitenv.CSV(os.Getenv("GITHUB_TOKENS")),
		GithubAPIBaseURL: strings.TrimSpace(os.Getenv("GITHUB_API_BASE_URL")),
	}
	return New(opt)
}

// New 按 Options 装配服务。
func New(opt Options) (*Service, error) {
	if strings.TrimSpace(opt.Port) == "" {
		opt.Port = defaultPort
	}
	if strings.TrimSpace(opt.StoreFile) == "" {
		opt.StoreFile = defaultStoreFile
	}
	if strings.TrimSpace(opt.BaseURL) == "" {
		opt.BaseURL = defaultBaseURL
	}
	if len(opt.APIKeys) == 0 {
		return nil, fmt.Errorf("APIKeys is required")
	}

	sqliteStore, err := store.NewSQLiteStore(opt.StoreFile)
	if err != nil {
		return nil, fmt.Errorf("initialize SQLite store: %w", err)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"repoOwner": repoOwner,
		"repoName":  repoName,
	}).ParseGlob("templates/*.html")
	if err != nil {
		sqliteStore.Close()
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	authMW := middleware.NewBearerAuth(opt.APIKeys)

	githubRepos := githubclient.NewClient(opt.GithubAPIBaseURL, opt.GithubTokens)
	repoCache := cache.NewRepositoryCache(time.Hour, 512)
	shareHandler := handler.NewShareHandler(sqliteStore, tmpl, opt.BaseURL, githubRepos, repoCache)
	repositoryRenderer, err := render.NewOGRenderer()
	if err != nil {
		sqliteStore.Close()
		return nil, fmt.Errorf("initialize repository OG renderer: %w", err)
	}
	repositoryHandler, err := handler.NewRepositoryHandler(
		githubRepos,
		repoCache,
		repositoryRenderer,
		tmpl,
		opt.BaseURL,
	)
	if err != nil {
		sqliteStore.Close()
		return nil, fmt.Errorf("initialize repository preview handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /s/{id}", shareHandler.HandleRenderShare)
	mux.HandleFunc("GET /r/starcat-logo.png", starcatLogoHandler)
	mux.HandleFunc("GET /r/fonts/{file}", shareFontHandler)
	mux.HandleFunc("GET /r/{owner}/{repo}", repositoryHandler.HandleRepositoryPage)
	mux.HandleFunc("GET /og/repo/{owner}/{repo}", repositoryHandler.HandleRepositoryOG)
	mux.Handle("GET /api/v1/ping", authMW.Wrap(handler.HandlePingV1(version.Service, version.Version)))
	mux.Handle("POST /api/v1/share", authMW.Wrap(http.HandlerFunc(shareHandler.HandleCreateShareV1)))
	mux.Handle("GET /internal/stats", authMW.Wrap(handler.HandleStats(sqliteStore)))

	if !opt.SkipListenLogEndpoints {
		log.Printf("starcat-sharing-api %s starting on port %s", version.Version, opt.Port)
		log.Printf("Endpoints:")
		log.Printf("  GET  /api/v1/ping   - Connectivity probe for Starcat client (auth required)")
		log.Printf("  POST /api/v1/share  - Create share link (auth required)")
		log.Printf("  GET  /internal/stats - Share statistics (auth required)")
		log.Printf("  GET  /s/{id}        - View share page (public)")
		log.Printf("  GET  /r/{owner}/{repo} - View public repository preview")
		log.Printf("  GET  /og/repo/{owner}/{repo}.png - Repository Open Graph image")
		log.Printf("  GET  /healthz       - Health check (public)")
	}

	return &Service{
		opts:        opt,
		handler:     middleware.CORS(mux),
		sqliteStore: sqliteStore,
	}, nil
}

// Handler 返回已包 CORS 的根 handler，可供聚合网关挂载。
func (s *Service) Handler() http.Handler { return s.handler }

// Addr 返回建议监听地址（":port"）。
func (s *Service) Addr() string { return ":" + s.opts.Port }

// Close 关闭 SQLite 连接。
func (s *Service) Close() error {
	if s.sqliteStore != nil {
		return s.sqliteStore.Close()
	}
	return nil
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func starcatLogoHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Write(render.StarcatLogoPNG())
}

func shareFontHandler(w http.ResponseWriter, r *http.Request) {
	file := path.Base(r.PathValue("file"))
	switch file {
	case "press-start-2p-latin-400-normal.woff2",
		"ibm-plex-mono-latin-400-normal.woff2",
		"ibm-plex-mono-latin-500-normal.woff2",
		"ibm-plex-mono-latin-600-normal.woff2",
		"ibm-plex-mono-latin-700-normal.woff2":
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	http.ServeFile(w, r, filepath.Join("static", "fonts", file))
}

func repoOwner(fullName string) string {
	if i := strings.IndexByte(fullName, '/'); i > 0 {
		return fullName[:i]
	}
	return fullName
}

func repoName(fullName string) string {
	if i := strings.IndexByte(fullName, '/'); i >= 0 && i+1 < len(fullName) {
		return fullName[i+1:]
	}
	return fullName
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
