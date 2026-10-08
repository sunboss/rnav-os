package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Gzip 压缩中间件（优化：减少 API 响应体积）
// 只压缩 JSON API 响应，不压缩已压缩的静态资源

var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.writer.Write(data)
}

func GzipInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 客户端不支持 gzip 则跳过
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		// 静态资源由 Cloudflare/反向代理压缩，跳过
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") ||
			strings.HasPrefix(c.Request.URL.Path, "/custom/") {
			c.Next()
			return
		}

		w := gzipPool.Get().(*gzip.Writer)
		defer gzipPool.Put(w)
		w.Reset(c.Writer)
		defer w.Close()

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer = &gzipWriter{ResponseWriter: c.Writer, writer: w}
		c.Next()
	}
}

// 兼容 net/http 的 Writer 接口
var _ http.Flusher = (*gzipWriter)(nil)

func (g *gzipWriter) Flush() {
	g.writer.Flush()
	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
