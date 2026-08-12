// Package web embeds the built frontend (frontend/dist, copied here at build
// time as internal/web/dist) and serves it as a single-page app fallback.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var embedded embed.FS

func distFS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// RegisterSPA serves static assets from the embedded frontend build and
// falls back to index.html for any unmatched GET route, so client-side
// routing (react-router) works on a full page load/refresh.
func RegisterSPA(r *gin.Engine) {
	assets := distFS()
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}
		path := trimLeadingSlash(c.Request.URL.Path)
		if path == "" {
			path = "index.html"
		}
		if info, err := fs.Stat(assets, path); err == nil && !info.IsDir() {
			serveFile(c, assets, path)
			return
		}
		serveFile(c, assets, "index.html")
	})
}

func serveFile(c *gin.Context, assets fs.FS, path string) {
	data, err := fs.ReadFile(assets, path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, mimeFor(path), data)
}

func mimeFor(path string) string {
	switch ext(path) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func ext(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}
