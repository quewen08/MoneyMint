package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler 托管 Flutter Web 构建产物，并为前端路由提供 SPA 回退。
//
// 行为约定：
//   - 只处理 GET/HEAD；其余方法 405
//   - /api/ 前缀永不落到静态文件，返回 404 JSON（避免接口 404 被 HTML 吞掉）
//   - 命中真实文件 → 直接返回，按路径分级设置缓存头
//   - 未命中且路径「看起来是静态资源」（末段含 .）→ 404，不做 SPA 回退
//   - 未命中且路径「看起来是前端路由」→ 回退 index.html
//   - 目录从不列目录列表
//
// root 不存在时返回一个提示性 handler，而不是让服务启动失败：
// 本地开发常常只跑后端、前端用 flutter run 热重载，此时不该阻断后端。
func spaHandler(root string) http.HandlerFunc {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "接口不存在"})
				return
			}
			http.Error(w,
				"前端未部署：请执行 flutter build web 并将 LEDGER_WEB_ROOT 指向产物目录",
				http.StatusNotFound)
		}
	}

	indexPath := filepath.Join(root, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 接口路径绝不回退到前端页面：否则前端会把 HTML 当作 JSON 解析，
		// 报出难以定位的解析错误。
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "接口不存在"})
			return
		}

		// 归一化请求路径：path.Clean 消除 "./" 与多余的 "/"，并让 ".." 无法逃逸根。
		clean := path.Clean("/" + r.URL.Path)
		full := filepath.Join(root, filepath.FromSlash(clean))
		if !withinRoot(root, full) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// 命中真实文件
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			serveStatic(w, r, full, clean)
			return
		}

		// 请求的是目录：尝试其 index.html（不列目录）
		if st, err := os.Stat(full); err == nil && st.IsDir() {
			dirIndex := filepath.Join(full, "index.html")
			if st2, err := os.Stat(dirIndex); err == nil && !st2.IsDir() {
				serveStatic(w, r, dirIndex, path.Join(clean, "index.html"))
				return
			}
			http.NotFound(w, r)
			return
		}

		// 未命中：带扩展名的视为静态资源缺失，明确 404；
		// 无扩展名的视为前端路由，回退入口页。
		if hasExtension(clean) {
			http.NotFound(w, r)
			return
		}
		setStaticCacheHeaders(w, "/index.html")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, indexPath)
	}
}

// serveStatic 输出静态文件并设置安全头与缓存策略。
func serveStatic(w http.ResponseWriter, r *http.Request, fullPath, cleanPath string) {
	setStaticCacheHeaders(w, cleanPath)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, fullPath)
}

// setStaticCacheHeaders 按资源性质设置缓存策略。
//
// 分两类：
//   - /assets/、/canvaskit/：文件名带内容 hash，内容变则名字变 → 永久缓存
//   - 其余（main.dart.js、flutter.js、index.html、manifest 等）：文件名不带 hash，
//     版本更新后内容变了名字不变 → 一律 no-cache
//
// no-cache 的代价只是每次一个 304 请求（这些文件都很小）；而缓存它们会出新问题：
// 一旦浏览器持有旧 main.dart.js，新部署后用户仍跑旧代码，直到缓存过期
// （实测后果：反代地址修复上线后，用户浏览器在缓存期内仍请求旧 API 地址）。
func setStaticCacheHeaders(w http.ResponseWriter, cleanPath string) {
	switch {
	case strings.HasPrefix(cleanPath, "/assets/"),
		strings.HasPrefix(cleanPath, "/canvaskit/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// hasExtension 判断路径末段是否含扩展名（用于区分静态资源与前端路由）。
func hasExtension(cleanPath string) bool {
	base := path.Base(cleanPath)
	return strings.Contains(base, ".")
}

// withinRoot 校验目标路径确实位于 root 之内，防止路径穿越。
func withinRoot(root, full string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
