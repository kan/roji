package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeStaticFile(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "roji-static-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	indexContent := "<html><body>Hello World</body></html>"
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("Failed to write index.html: %v", err)
	}

	cssContent := "body { color: red; }"
	if err := os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte(cssContent), 0644); err != nil {
		t.Fatalf("Failed to write style.css: %v", err)
	}

	// Create subdirectory with index.html
	subDir := filepath.Join(tmpDir, "docs")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	docsContent := "<html><body>Documentation</body></html>"
	if err := os.WriteFile(filepath.Join(subDir, "index.html"), []byte(docsContent), 0644); err != nil {
		t.Fatalf("Failed to write docs/index.html: %v", err)
	}

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
		expectedType   string
	}{
		{
			name:           "root serves index.html",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   indexContent,
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "serve CSS file",
			path:           "/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   cssContent,
			expectedType:   "text/css; charset=utf-8",
		},
		{
			name:           "subdirectory serves index.html",
			path:           "/docs/",
			expectedStatus: http.StatusOK,
			expectedBody:   docsContent,
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "not found",
			path:           "/notfound.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "directory traversal blocked",
			path:           "/../etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "encoded traversal blocked",
			path:           "/..%2F..%2Fetc%2Fpasswd",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			ServeStaticFile(w, req, tmpDir, false, "") // index disabled

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}

			if tt.expectedType != "" {
				contentType := w.Header().Get("Content-Type")
				if contentType != tt.expectedType {
					t.Errorf("Expected Content-Type %q, got %q", tt.expectedType, contentType)
				}
			}
		})
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"file.html", "text/html; charset=utf-8"},
		{"file.htm", "text/html; charset=utf-8"},
		{"file.css", "text/css; charset=utf-8"},
		{"file.js", "application/javascript; charset=utf-8"},
		{"file.json", "application/json; charset=utf-8"},
		{"file.svg", "image/svg+xml"},
		{"file.png", "image/png"},
		{"file.jpg", "image/jpeg"},
		{"file.jpeg", "image/jpeg"},
		{"file.gif", "image/gif"},
		{"file.webp", "image/webp"},
		{"file.ico", "image/x-icon"},
		{"file.woff", "font/woff"},
		{"file.woff2", "font/woff2"},
		{"file.ttf", "font/ttf"},
		{"file.pdf", "application/pdf"},
		{"file.txt", "text/plain; charset=utf-8"},
		{"file.md", "text/markdown; charset=utf-8"},
		{"file.unknown", ""}, // Let http.ServeContent detect
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := getContentType(tt.path)
			if got != tt.expected {
				t.Errorf("getContentType(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDirectoryListing(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "roji-dirlist-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644); err != nil {
		t.Fatalf("Failed to write readme.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("js"), 0644); err != nil {
		t.Fatalf("Failed to write app.js: %v", err)
	}

	// Create a subdirectory
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	tests := []struct {
		name           string
		path           string
		enableIndex    bool
		expectedStatus int
		expectListing  bool
	}{
		{
			name:           "directory listing disabled returns 403 with help page",
			path:           "/",
			enableIndex:    false,
			expectedStatus: http.StatusForbidden,
			expectListing:  false,
		},
		{
			name:           "directory listing enabled returns listing",
			path:           "/",
			enableIndex:    true,
			expectedStatus: http.StatusOK,
			expectListing:  true,
		},
		{
			name:           "subdirectory redirects to add trailing slash",
			path:           "/subdir",
			enableIndex:    true,
			expectedStatus: http.StatusMovedPermanently, // redirects to /subdir/
			expectListing:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			ServeStaticFile(w, req, tmpDir, tt.enableIndex, "roji.dev.localhost")

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectListing {
				body := w.Body.String()
				if !strings.Contains(body, "Index of") {
					t.Error("Expected directory listing page")
				}
				// Check that files appear in listing
				if tt.path == "/" {
					if !strings.Contains(body, "readme.txt") {
						t.Error("Expected readme.txt in listing")
					}
					if !strings.Contains(body, "app.js") {
						t.Error("Expected app.js in listing")
					}
					if !strings.Contains(body, "subdir") {
						t.Error("Expected subdir in listing")
					}
				}
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatSize(tt.size)
			if got != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.size, got, tt.expected)
			}
		})
	}
}

func TestGetFileIcon(t *testing.T) {
	tests := []struct {
		name     string
		isDir    bool
		expected string
	}{
		{"folder", true, "📁"},
		{"index.html", false, "🌐"},
		{"style.css", false, "🎨"},
		{"app.js", false, "📜"},
		{"data.json", false, "📋"},
		{"readme.md", false, "📝"},
		{"main.go", false, "🐹"},
		{"script.py", false, "🐍"},
		{"image.png", false, "🖼️"},
		{"video.mp4", false, "🎬"},
		{"archive.zip", false, "📦"},
		{"unknown.xyz", false, "📄"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFileIcon(tt.name, tt.isDir)
			if got != tt.expected {
				t.Errorf("getFileIcon(%q, %v) = %q, want %q", tt.name, tt.isDir, got, tt.expected)
			}
		})
	}
}

func TestRouter_StaticRoutes(t *testing.T) {
	router := NewRouter()

	// Add a static site
	static := &StaticBackend{
		Hostname: "docs.dev.localhost",
		Root:     "/var/www/docs",
	}
	router.AddStaticSite(static)

	// Lookup should find the static route
	route := router.LookupStatic("docs.dev.localhost")
	if route == nil {
		t.Fatal("Expected to find static route")
	}
	if route.StaticBackend.Root != "/var/www/docs" {
		t.Errorf("Expected root /var/www/docs, got %s", route.StaticBackend.Root)
	}

	// ListRoutes should include static routes
	routes := router.ListRoutes()
	found := false
	for _, r := range routes {
		if r.Hostname == "docs.dev.localhost" && r.IsStatic {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected static route in ListRoutes")
	}

	// Remove static site
	router.RemoveStaticSite("docs.dev.localhost")
	route = router.LookupStatic("docs.dev.localhost")
	if route != nil {
		t.Error("Expected static route to be removed")
	}
}
