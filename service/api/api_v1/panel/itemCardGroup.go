package panel

import (
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// 卡片分组管理 (V2 item_card_group 表)
type ItemCardGroup struct {
}

// 新增/编辑分组
func (a *ItemCardGroup) Edit(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	req := models.ItemCardGroup{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	req.UserId = userInfo.ID

	if req.ID != 0 {
		defer clearPublicGroupsCache(userInfo.ID)
		updateFields := []string{"Title", "OnlyName", "Description", "Sort", "PublicVisitHide", "Style"}
		if req.Sort != 0 {
			// sort 始终允许更新
		}
		global.Db.Model(&models.ItemCardGroup{}).
			Select(updateFields).
			Where("id=? AND user_id=?", req.ID, userInfo.ID).Updates(&req)
	} else {
		global.Db.Create(&req)
		clearPublicGroupsCache(userInfo.ID)
	}
	apiReturn.SuccessData(c, req)
}

// 删除分组（含分组下卡片）
func (a *ItemCardGroup) Deletes(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	var req struct {
		Ids []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	// 先删卡片再删分组
	global.Db.Where("item_card_group_id IN ? AND user_id=?", req.Ids, userInfo.ID).Delete(&models.ItemCard{})
	global.Db.Where("id IN ? AND user_id=?", req.Ids, userInfo.ID).Delete(&models.ItemCardGroup{})
	clearPublicGroupsCache(userInfo.ID)
	apiReturn.Success(c)
}

// 保存分组排序
func (a *ItemCardGroup) SaveSort(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	var req []struct {
		Id   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	// R导航优化：批量排序用单事务
	tx := global.Db.Begin()
	for _, item := range req {
		if err := tx.Model(&models.ItemCardGroup{}).
			Where("id=? AND user_id=?", item.Id, userInfo.ID).
			Update("sort", item.Sort).Error; err != nil {
			tx.Rollback()
			apiReturn.ErrorDatabase(c, err.Error())
			return
		}
	}
	tx.Commit()
	clearPublicGroupsCache(userInfo.ID)
	apiReturn.Success(c)
}

// 切换分组访客可见性
func (a *ItemCardGroup) SetPublicVisitHide(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	var req struct {
		Id              uint `json:"id"`
		PublicVisitHide int  `json:"publicVisitHide"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	global.Db.Model(&models.ItemCardGroup{}).
		Where("id=? AND user_id=?", req.Id, userInfo.ID).
		Update("public_visit_hide", req.PublicVisitHide)
	clearPublicGroupsCache(userInfo.ID)
	apiReturn.Success(c)
}
