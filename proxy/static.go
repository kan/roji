package proxy

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kan/roji/config"
	"github.com/kan/roji/i18n"
)

// StaticBackend represents a static file hosting configuration
type StaticBackend struct {
	Hostname      string            // Full hostname (e.g., "docs.dev.localhost")
	Root          string            // Absolute path to root directory
	Index         bool              // Enable directory listing
	DashboardHost string            // Dashboard hostname for footer link
	BasicAuth     *config.BasicAuth // Basic authentication (optional)
}

// ServeStaticFile serves a static file from the given root directory
func ServeStaticFile(w http.ResponseWriter, r *http.Request, root string, enableIndex bool, dashboardHost string) {
	// Clean the request path to prevent directory traversal
	reqPath := path.Clean(r.URL.Path)
	if reqPath == "" || reqPath == "." {
		reqPath = "/"
	}

	// Ensure the path starts with /
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	// Build the full file path
	filePath := filepath.Join(root, filepath.FromSlash(reqPath))

	// Security: Ensure the resolved path is within the root directory
	absRoot, err := filepath.Abs(root)
	if err != nil {
		slog.Error("failed to get absolute root path", "root", root, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		slog.Error("failed to get absolute file path", "path", filePath, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check that the file path is within the root (prevent directory traversal)
	if !strings.HasPrefix(absFilePath, absRoot) {
		slog.Warn("directory traversal attempt blocked",
			"requested", r.URL.Path,
			"resolved", absFilePath,
			"root", absRoot)
		http.NotFound(w, r)
		return
	}

	// Check if path exists
	info, err := os.Stat(absFilePath)
	if os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to stat file", "path", absFilePath, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If it's a directory, try to serve index.html or directory listing
	if info.IsDir() {
		indexPath := filepath.Join(absFilePath, "index.html")
		indexInfo, err := os.Stat(indexPath)
		if err == nil && !indexInfo.IsDir() {
			// Serve index.html
			absFilePath = indexPath
			info = indexInfo
		} else if enableIndex {
			// Serve directory listing
			serveDirectoryListing(w, r, absFilePath, absRoot, dashboardHost)
			return
		} else {
			// No index.html and directory listing disabled - show helpful page
			serveIndexDisabledPage(w, r, dashboardHost)
			return
		}
	}

	// Open and serve the file
	file, err := os.Open(absFilePath)
	if err != nil {
		slog.Error("failed to open file", "path", absFilePath, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set content type based on extension
	contentType := getContentType(absFilePath)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	// Serve the file
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// DirectoryEntry represents a file or directory in a listing
type DirectoryEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime string
	Icon    string
}

// serveDirectoryListing generates and serves an HTML directory listing
func serveDirectoryListing(w http.ResponseWriter, r *http.Request, dirPath, rootPath, dashboardHost string) {
	// Ensure URL path ends with / for proper relative links
	// Use r.URL.Path (original request) to check, not the cleaned urlPath
	if !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		slog.Error("failed to read directory", "path", dirPath, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Build entry list
	var items []DirectoryEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := DirectoryEntry{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
			Icon:    getFileIcon(entry.Name(), entry.IsDir()),
		}
		items = append(items, item)
	}

	// Sort: directories first, then files, both alphabetically
	sortDirectoryEntries(items)

	// Calculate relative path from root for display
	relPath := strings.TrimPrefix(dirPath, rootPath)
	if relPath == "" {
		relPath = "/"
	}

	// Use the original request path (with trailing slash) for links
	displayPath := r.URL.Path

	// Detect language for i18n
	lang := i18n.DetectHTTP(r.Header.Get("Accept-Language"))

	// Generate HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	generateDirectoryHTML(w, displayPath, relPath, items, dashboardHost, lang)
}

// sortDirectoryEntries sorts entries: directories first, then files, alphabetically
func sortDirectoryEntries(entries []DirectoryEntry) {
	// Simple bubble sort for small lists
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			swap := false
			if entries[i].IsDir != entries[j].IsDir {
				// Directories come first
				swap = !entries[i].IsDir && entries[j].IsDir
			} else {
				// Alphabetical within same type
				swap = strings.ToLower(entries[i].Name) > strings.ToLower(entries[j].Name)
			}
			if swap {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// getFileIcon returns an appropriate emoji icon for a file or directory
func getFileIcon(name string, isDir bool) string {
	if isDir {
		return "📁"
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".html", ".htm":
		return "🌐"
	case ".css":
		return "🎨"
	case ".js", ".ts":
		return "📜"
	case ".json":
		return "📋"
	case ".md":
		return "📝"
	case ".txt":
		return "📄"
	case ".go":
		return "🐹"
	case ".py":
		return "🐍"
	case ".rs":
		return "🦀"
	case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico":
		return "🖼️"
	case ".mp3", ".wav", ".ogg", ".flac":
		return "🎵"
	case ".mp4", ".webm", ".avi", ".mov":
		return "🎬"
	case ".pdf":
		return "📕"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "📦"
	case ".exe", ".bin", ".app":
		return "⚙️"
	case ".yml", ".yaml":
		return "⚙️"
	case ".xml":
		return "📰"
	case ".sh", ".bash":
		return "🖥️"
	default:
		return "📄"
	}
}

// generateDirectoryHTML writes the directory listing HTML to the response
func generateDirectoryHTML(w http.ResponseWriter, urlPath, displayPath string, entries []DirectoryEntry, dashboardHost string, lang string) {
	msgs := i18n.Messages(lang)
	t := func(key string) string {
		if v, ok := msgs[key]; ok {
			return v
		}
		enMsgs := i18n.Messages("en")
		if v, ok := enMsgs[key]; ok {
			return v
		}
		return key
	}

	// HTML template with roji-consistent styling
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + template.HTMLEscapeString(t("static.index_of")) + ` ` + template.HTMLEscapeString(displayPath) + `</title>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-card: #ffffff;
            --bg-hover: #f5f5f5;
            --text-primary: #1a1a1a;
            --text-secondary: #666666;
            --border-color: #e5e5e5;
            --link-color: #0066cc;
            --shadow-color: rgba(0, 0, 0, 0.1);
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg-primary: #1a1a1a;
                --bg-card: #252525;
                --bg-hover: #333333;
                --text-primary: #e5e5e5;
                --text-secondary: #999999;
                --border-color: #404040;
                --link-color: #66b3ff;
                --shadow-color: rgba(0, 0, 0, 0.3);
            }
        }
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            margin: 0;
            padding: 40px 20px;
            max-width: 900px;
            margin: 0 auto;
        }
        h1 {
            font-size: 1.5rem;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .path {
            font-family: monospace;
            background: var(--bg-card);
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 1rem;
        }
        .listing {
            background: var(--bg-card);
            border-radius: 8px;
            box-shadow: 0 1px 3px var(--shadow-color);
            overflow: hidden;
        }
        .entry {
            display: flex;
            align-items: center;
            padding: 12px 16px;
            border-bottom: 1px solid var(--border-color);
            text-decoration: none;
            color: var(--text-primary);
            transition: background 0.2s;
        }
        .entry:last-child { border-bottom: none; }
        .entry:hover { background: var(--bg-hover); }
        .icon { font-size: 1.2rem; margin-right: 12px; min-width: 24px; text-align: center; }
        .name { flex: 1; font-family: monospace; }
        .name.dir { font-weight: 600; }
        a.entry { color: var(--link-color); }
        a.entry .name { color: var(--link-color); }
        .size {
            color: var(--text-secondary);
            font-size: 0.85rem;
            font-family: monospace;
            min-width: 80px;
            text-align: right;
            margin-right: 20px;
        }
        .date {
            color: var(--text-secondary);
            font-size: 0.85rem;
            font-family: monospace;
            min-width: 140px;
        }
        .parent { background: var(--bg-hover); }
        .empty {
            padding: 40px;
            text-align: center;
            color: var(--text-secondary);
        }
        .footer {
            margin-top: 20px;
            text-align: center;
            font-size: 0.8rem;
            color: var(--text-secondary);
        }
        .footer a { color: var(--link-color); text-decoration: none; }
        .footer a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>📂 ` + template.HTMLEscapeString(t("static.index_of")) + ` <span class="path">` + template.HTMLEscapeString(displayPath) + `</span></h1>
    <div class="listing">`

	// Add parent directory link if not at root
	if urlPath != "/" {
		parentPath := path.Dir(strings.TrimSuffix(urlPath, "/"))
		if parentPath == "" {
			parentPath = "/"
		}
		if !strings.HasSuffix(parentPath, "/") {
			parentPath += "/"
		}
		html += `
        <a href="` + template.HTMLEscapeString(parentPath) + `" class="entry parent">
            <span class="icon">⬆️</span>
            <span class="name">..</span>
            <span class="size">-</span>
            <span class="date">` + template.HTMLEscapeString(t("static.parent_directory")) + `</span>
        </a>`
	}

	if len(entries) == 0 && urlPath == "/" {
		html += `
        <div class="empty">📭 ` + template.HTMLEscapeString(t("static.empty_directory")) + `</div>`
	}

	for _, entry := range entries {
		href := template.HTMLEscapeString(entry.Name)
		if entry.IsDir {
			href += "/"
		}
		nameClass := "name"
		if entry.IsDir {
			nameClass += " dir"
		}
		sizeStr := formatSize(entry.Size)
		if entry.IsDir {
			sizeStr = "-"
		}

		html += `
        <a href="` + href + `" class="entry">
            <span class="icon">` + entry.Icon + `</span>
            <span class="` + nameClass + `">` + template.HTMLEscapeString(entry.Name) + `</span>
            <span class="size">` + sizeStr + `</span>
            <span class="date">` + entry.ModTime + `</span>
        </a>`
	}

	html += `
    </div>
    <div class="footer">`

	if dashboardHost != "" {
		html += `<a href="https://` + template.HTMLEscapeString(dashboardHost) + `">` + template.HTMLEscapeString(t("static.dashboard")) + `</a> · `
	}

	html += template.HTMLEscapeString(t("static.served_by")) + ` <a href="https://github.com/kan/roji">roji</a></div>
</body>
</html>`

	w.Write([]byte(html))
}

// serveIndexDisabledPage shows a helpful page when directory listing is disabled
func serveIndexDisabledPage(w http.ResponseWriter, r *http.Request, dashboardHost string) {
	lang := i18n.DetectHTTP(r.Header.Get("Accept-Language"))
	msgs := i18n.Messages(lang)
	t := func(key string) string {
		if v, ok := msgs[key]; ok {
			return v
		}
		enMsgs := i18n.Messages("en")
		if v, ok := enMsgs[key]; ok {
			return v
		}
		return key
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)

	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + template.HTMLEscapeString(t("static.index_disabled.title")) + `</title>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-card: #ffffff;
            --text-primary: #1a1a1a;
            --text-secondary: #666666;
            --border-color: #e5e5e5;
            --link-color: #0066cc;
            --shadow-color: rgba(0, 0, 0, 0.1);
            --code-bg: #f5f5f5;
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg-primary: #1a1a1a;
                --bg-card: #252525;
                --text-primary: #e5e5e5;
                --text-secondary: #999999;
                --border-color: #404040;
                --link-color: #66b3ff;
                --shadow-color: rgba(0, 0, 0, 0.3);
                --code-bg: #333333;
            }
        }
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            margin: 0;
            padding: 40px 20px;
            max-width: 700px;
            margin: 0 auto;
        }
        .card {
            background: var(--bg-card);
            border-radius: 12px;
            box-shadow: 0 2px 8px var(--shadow-color);
            padding: 32px;
            text-align: center;
        }
        .icon { font-size: 3rem; margin-bottom: 16px; }
        h1 {
            font-size: 1.5rem;
            margin: 0 0 12px 0;
            color: var(--text-primary);
        }
        p {
            color: var(--text-secondary);
            margin: 0 0 20px 0;
            line-height: 1.6;
        }
        .config-hint {
            background: var(--code-bg);
            border-radius: 8px;
            padding: 16px;
            text-align: left;
            margin: 20px 0;
        }
        .config-hint h3 {
            font-size: 0.85rem;
            margin: 0 0 12px 0;
            color: var(--text-secondary);
        }
        pre {
            margin: 0;
            font-family: "SF Mono", Monaco, "Cascadia Code", monospace;
            font-size: 0.85rem;
            line-height: 1.5;
            overflow-x: auto;
        }
        code {
            background: var(--code-bg);
            padding: 2px 6px;
            border-radius: 4px;
            font-family: "SF Mono", Monaco, "Cascadia Code", monospace;
            font-size: 0.9em;
        }
        .highlight { color: var(--link-color); }
        .footer {
            margin-top: 24px;
            padding-top: 16px;
            border-top: 1px solid var(--border-color);
        }
        a {
            color: var(--link-color);
            text-decoration: none;
        }
        a:hover { text-decoration: underline; }
        .btn {
            display: inline-block;
            background: var(--link-color);
            color: white;
            padding: 10px 20px;
            border-radius: 6px;
            font-size: 0.9rem;
            margin-top: 8px;
        }
        .btn:hover {
            text-decoration: none;
            opacity: 0.9;
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="icon">🔒</div>
        <h1>` + template.HTMLEscapeString(t("static.index_disabled.title")) + `</h1>
        <p>` + template.HTMLEscapeString(t("static.index_disabled.message")) + `</p>

        <div class="config-hint">
            <h3>` + template.HTMLEscapeString(t("static.index_disabled.config_hint")) + `</h3>
            <pre><span class="highlight"># ~/.config/roji/config.yaml</span>
static_sites:
  - host: ` + template.HTMLEscapeString(r.Host) + `
    root: /path/to/files
    <span class="highlight">index: true</span>  # ` + template.HTMLEscapeString(t("static.index_disabled.enable_comment")) + `</pre>
        </div>

        <p style="font-size: 0.9rem;">` + template.HTMLEscapeString(t("static.index_disabled.reload_hint")) + `</p>

        <div class="footer">`

	if dashboardHost != "" {
		html += `<a href="https://` + template.HTMLEscapeString(dashboardHost) + `" class="btn">` + template.HTMLEscapeString(t("static.index_disabled.go_dashboard")) + `</a><br><br>`
	}

	html += template.HTMLEscapeString(t("static.served_by")) + ` <a href="https://github.com/kan/roji">roji</a>
        </div>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// formatSize formats a file size in human-readable format
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// getContentType returns the content type based on file extension
func getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	default:
		return "" // Let http.ServeContent detect it
	}
}

// fileReadSeeker wraps an os.File to implement io.ReadSeeker
type fileReadSeeker struct {
	file *os.File
}

func (f *fileReadSeeker) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

func (f *fileReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

var _ io.ReadSeeker = (*fileReadSeeker)(nil)
