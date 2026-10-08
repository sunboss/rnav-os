package panel

import "github.com/gin-gonic/gin"

func Init(routerGroup *gin.RouterGroup) {
	InitItemIcon(routerGroup)
	InitUserConfig(routerGroup)
	InitUsersRouter(routerGroup)
	InitItemIconGroup(routerGroup)
	InitItemCard(routerGroup) // R导航: V2卡片/分组
	InitDocker(routerGroup)   // R导航: Docker管理器（仅管理员）
}
