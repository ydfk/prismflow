package website

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"strings"
)

const templateHTML = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>body{margin:0;background:#f5f3ee;color:#263c36;font-family:Georgia,'Microsoft YaHei',serif}main{max-width:900px;margin:auto;padding:12vh 8vw}nav{display:flex;justify-content:space-between;border-bottom:1px solid #bbc7bf;padding-bottom:24px;font:14px sans-serif}h1{font-size:clamp(40px,8vw,84px);line-height:1.1;font-weight:400;margin:15vh 0 30px}p{font:18px/1.8 sans-serif;max-width:600px}.landscape{height:180px;margin:60px 0;background:linear-gradient(160deg,#cad8cd,#7e9d88);border-radius:90% 10% 0 0}footer{font:13px sans-serif;color:#53645c}.studio{background:#182f32;color:#f2ede2}.studio .landscape{background:linear-gradient(150deg,#b8c7ac,#587b77);border-radius:0 70px 0 0}</style><body class="{{.Template}}"><main><nav><span>{{.Title}}</span><span>随记 · JOURNAL</span></nav><h1>{{.Title}}</h1><p>{{.Description}}</p><div class="landscape" aria-hidden="true"></div><footer>保持好奇，缓慢生活。</footer></main></body></html>`

var siteTemplate = template.Must(template.New("site").Parse(templateHTML))

func Serve(w http.ResponseWriter, r *http.Request, site model.Website, dir string) {
 if site.Disabled { http.NotFound(w,r);return }
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// 上传网站与管理界面分开监听，且只允许同源静态资源，不执行脚本。
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if site.Bundle != "" {
		root := filepath.Join(dir, site.Bundle)
		clean := filepath.Clean("/" + r.URL.Path)
		if strings.Contains(r.URL.Path, "\\") {
			http.NotFound(w, r)
			return
		}
		target := filepath.Join(root, clean)
		stat, err := os.Stat(target)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if stat.IsDir() {
			if _, err = os.Stat(filepath.Join(target, "index.html")); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = siteTemplate.Execute(w, site)
}
func Extract(data []byte, dir string) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("无法读取 ZIP 文件")
	}
	if len(reader.File) > 500 {
		return "", errors.New("最多允许 500 个文件")
	}
	id := model.ID()
	root := filepath.Join(dir, id)
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(root)
		}
	}()
	var total int64
	hasIndex := false
	for _, f := range reader.File {
		if f.Mode()&os.ModeSymlink != 0 || strings.Contains(f.Name, "\\") || strings.Contains(f.Name, ":") || strings.HasPrefix(f.Name, "/") {
			return "", errors.New("ZIP 包含不安全的路径")
		}
		clean := filepath.Clean(f.Name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", errors.New("ZIP 包含越界路径")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > 32<<20 || total < 0 {
			return "", errors.New("解压后不能超过 32 MiB")
		}
		ext := strings.ToLower(filepath.Ext(clean))
		switch ext {
		case ".html", ".css", ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg", ".ico", ".txt", ".woff2":
		default:
			return "", fmt.Errorf("不支持 %s 文件；网站仅允许静态资源", ext)
		}
		target := filepath.Join(root, clean)
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", err
		}
		src, e := f.Open()
		if e != nil {
			return "", e
		}
		dst, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			src.Close()
			return "", e
		}
		n, e := io.Copy(dst, io.LimitReader(src, 32<<20+1))
		src.Close()
		closeErr := dst.Close()
		if e != nil || n > 32<<20 {
			return "", errors.New("静态文件解压失败")
		}
		if closeErr != nil {
			return "", closeErr
		}
		if clean == "index.html" {
			hasIndex = true
		}
	}
	if !hasIndex {
		return "", errors.New("ZIP 根目录需要包含 index.html")
	}
	ok = true
	return id, nil
}
