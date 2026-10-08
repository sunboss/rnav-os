package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

// R导航 Docker 管理器路由
// 安全：仅管理员可访问（LoginInterceptor + AdminInterceptor 双拦截）
// R通行证登录的普通用户（role!=1）会被 AdminInterceptor 以 1005 拒绝
func InitDocker(router *gin.RouterGroup) {
	docker := api_v1.ApiGroupApp.ApiPanel.Docker

	// 需要登录 + 管理员
	r := router.Group("", middleware.LoginInterceptor, middleware.AdminInterceptor)
	{
		r.POST("/docker/containers", docker.Containers)
		r.POST("/docker/container/operate", docker.Operate)
		r.POST("/docker/container/logs", docker.Logs)
		r.POST("/docker/container/stats", docker.Stats) // OPT-014：实时状态
	}
}
