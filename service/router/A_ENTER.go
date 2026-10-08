package router

import (
	"net/http"
	_ "net/http/pprof" // OPT-014：pprof 端点（路由层限制仅管理员访问）
	"strings"
	"time"

	"sun-panel/api/api_v1/middleware"
	"sun-panel/global"
	// "sun-panel/router/admin"
	"sun-panel/router/mcp"
	"sun-panel/router/openness"
	"sun-panel/router/panel"
	"sun-panel/router/system"

	"github.com/gin-gonic/gin"
)

// 初始化总路由
func InitRouters(addr string) error {
	router := gin.Default()

	// 优化：Gzip 压缩 API 响应
	router.Use(middleware.GzipInterceptor())

	// OPT-014：健康检查（容器自检/监控用，不鉴权）
	router.GET("/api/health", func(c *gin.Context) {
		data := gin.H{"status": "ok", "service": "rnav-os", "time": time.Now().Unix()}
		if sqlDb, err := global.Db.DB(); err != nil || sqlDb.Ping() != nil {
			data["status"] = "error"
			data["db"] = "unreachable"
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 500, "msg": "db unreachable", "data": data})
			return
		}
		data["db"] = "ok"
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
	})

	// OPT-014：pprof 性能分析，仅管理员可访问
	pprofGroup := router.Group("/debug/pprof", middleware.LoginInterceptor, middleware.AdminInterceptor)
	pprofGroup.GET("/*action", gin.WrapH(http.DefaultServeMux))

	rootRouter := router.Group("/")
	routerGroup := rootRouter.Group("api")

	// 接口
	system.Init(routerGroup)
	panel.Init(routerGroup)
	openness.Init(routerGroup)
	mcp.Init(routerGroup)  // R导航内置MCP（REST）
	mcp.InitServer(router) // OPT-014：标准 MCP Server（JSON-RPC 2.0，POST /mcp）

	// R导航品牌
	global.Logger.Info("R导航 is Started.  Listening and serving HTTP on ", addr)

	// WEB文件服务
	{
		// OPT-012：静态资源缓存策略（只影响本块之后注册的静态路由）
		// /assets/* 是带 content hash 的构建产物：长缓存 immutable
		// index.html / SPA fallback / 图标：no-cache，保证发版后即时生效
		router.Use(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
			c.Next()
		})
		webPath := "./web"
		router.StaticFile("/", webPath+"/index.html")
		router.Static("/assets", webPath+"/assets")
		router.Static("/custom", webPath+"/custom")
		router.StaticFile("/favicon.ico", webPath+"/favicon.ico")
		router.StaticFile("/favicon.svg", webPath+"/favicon.svg")
		router.StaticFile("/logo.png", webPath+"/logo.png")
		// SPA fallback：前端路由（/login、/admin、/auth/callback 等）直接访问时返回 index.html
		router.NoRoute(func(c *gin.Context) {
			// API 路径保持 404
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.JSON(404, gin.H{"code": 404, "msg": "not found"})
				return
			}
			c.File(webPath + "/index.html")
		})
	}

	// 上传的文件
	sourcePath := global.Config.GetValueString("base", "source_path")
	router.Static(sourcePath[1:], sourcePath)

	return router.Run(addr)
}
