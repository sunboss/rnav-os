package system

import (
	"sun-panel/api/api_v1"

	"github.com/gin-gonic/gin"
)

func InitRpass(routerGroup *gin.RouterGroup) {
	rpass := api_v1.ApiGroupApp.ApiSystem.RpassApi
	{
		routerGroup.GET("rpass/auth-url", rpass.AuthUrl)
		routerGroup.POST("rpass/callback", rpass.Callback)
	}
}
