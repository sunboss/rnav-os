package system

import "github.com/gin-gonic/gin"

func Init(routerGroup *gin.RouterGroup) {
	InitAbout(routerGroup)
	InitLogin(routerGroup)
	InitRpass(routerGroup) // R导航: R通行证OAuth
	InitUserRouter(routerGroup)
	InitFileRouter(routerGroup)
	InitNoticeRouter(routerGroup)
	InitModuleConfigRouter(routerGroup)
	InitMonitorRouter(routerGroup)
}
