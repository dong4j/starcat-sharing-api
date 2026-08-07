// Package assets 内嵌 sharing-api 运行所需的 HTML 模板与字体。
//
// 为什么用 go:embed：历史实现用 cwd 相对路径 ParseGlob("templates/*.html") /
// ServeFile("static/...")，聚合进程（starcat-api）与 Docker WORKDIR 稍有偏差就会起不来。
// 打进二进制后，任意工作目录启动都一致。
package assets

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/fonts/*.woff2
var fontFS embed.FS

// ParseHTML 解析内嵌 HTML 模板，并挂上 repoOwner / repoName 辅助函数。
// 模板名仍是文件名（share.html / repository.html），与历史 ExecuteTemplate 调用一致。
func ParseHTML(funcs template.FuncMap) (*template.Template, error) {
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse embedded templates: %w", err)
	}
	return tmpl, nil
}

// Font 返回内嵌 woff2 字体字节；未知文件名返回 false。
func Font(fileName string) ([]byte, bool) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" || strings.Contains(fileName, "/") || strings.Contains(fileName, `\`) {
		return nil, false
	}
	data, err := fs.ReadFile(fontFS, "static/fonts/"+fileName)
	if err != nil {
		return nil, false
	}
	return data, true
}
