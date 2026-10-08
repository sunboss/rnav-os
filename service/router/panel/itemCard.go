package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

// R导航卡片/分组路由 (V2表结构)
func InitItemCard(router *gin.RouterGroup) {
	itemCard := api_v1.ApiGroupApp.ApiPanel.ItemCard
	itemCardGroup := api_v1.ApiGroupApp.ApiPanel.ItemCardGroup

	// 需要管理员：卡片/分组的增删改、排序、隐藏开关仅管理员可操作
	// R通行证登录的普通用户（role!=1）会被 AdminInterceptor 以 1005 拒绝
	r := router.Group("", middleware.LoginInterceptor, middleware.AdminInterceptor)
	{
		r.POST("/panel/itemCard/edit", itemCard.Edit)
		r.POST("/panel/itemCard/deletes", itemCard.Deletes)
		r.POST("/panel/itemCard/saveSort", itemCard.SaveSort)

		r.POST("/panel/itemCardGroup/edit", itemCardGroup.Edit)
		r.POST("/panel/itemCardGroup/deletes", itemCardGroup.Deletes)
		r.POST("/panel/itemCardGroup/saveSort", itemCardGroup.SaveSort)
		r.POST("/panel/itemCardGroup/setPublicVisitHide", itemCardGroup.SetPublicVisitHide)
	}

	// 公开模式（访客可访问，后端自动过滤隐藏分组）
	rPublic := router.Group("", middleware.PublicModeInterceptor)
	{
		rPublic.POST("/panel/itemCard/getGroupList", itemCard.GetGroupList)
	}
}
