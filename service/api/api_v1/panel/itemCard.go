package panel

import (
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/global"
	"sun-panel/lib/cmn/systemSetting"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// 卡片管理 (V2 item_card 表)
type ItemCard struct {
}

// 获取分组+卡片列表
// R导航权限规则：隐藏分组（public_visit_hide=1，如"内网"）仅管理员可见
// - 访客：面板所有者的公开分组，备注清空（走缓存）
// - 普通登录用户：面板所有者的公开分组，备注可见（直查DB）
// - 管理员：面板所有者的全部分组
// R导航优化：导航内容固定归属面板所有者（panel_public_user_id），新注册的
// R通行证用户不会看到空面板；公开列表走缓存
func (a *ItemCard) GetGroupList(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	// 是否为公开访问模式（访客）；注意访客模式下 userInfo 是公开账号本身，
	// 不能只看 Role，必须排除访客模式才能认定管理员
	isPublicVisit := false
	if v, exists := c.Get(base.GIN_GET_VISIT_MODE); exists && v == base.VISIT_MODE_PUBLIC {
		isPublicVisit = true
	}
	isAdmin := !isPublicVisit && userInfo.Role == 1

	// 内容归属：优先面板所有者，未配置公开模式时回退到当前用户（兼容单用户场景）
	ownerId := userInfo.ID
	var publicUserId *uint
	if err := global.SystemSetting.GetValueByInterface(systemSetting.PANEL_PUBLIC_USER_ID, &publicUserId); err == nil && publicUserId != nil {
		ownerId = *publicUserId
	}

	publicOnly := !isAdmin
	var groups []models.ItemCardGroup
	var err error
	if isPublicVisit {
		groups, err = getCachedGroupList(global.Db, ownerId, true)
	} else {
		mGroup := models.ItemCardGroup{}
		groups, err = mGroup.GetList(global.Db, ownerId, publicOnly)
	}
	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}
	// R导航 OPT-010：V2特性——卡片备注(description)仅登录可见，访客请求时清空
	// 注意：公开缓存仅服务访客路径，缓存副本同样被清空是安全的（幂等）
	if isPublicVisit {
		for i := range groups {
			for j := range groups[i].Cards {
				groups[i].Cards[j].Description = ""
			}
		}
	}
	apiReturn.SuccessData(c, groups)
}

// 新增/编辑卡片
func (a *ItemCard) Edit(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	req := models.ItemCard{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	req.UserId = userInfo.ID
	req.PackCardData()

	if req.ID != 0 {
		// 修改（只能改自己的）
		updateFields := []string{"Title", "Background", "OnlyName", "Description", "CardSize",
			"Sort", "ShowTitle", "Blur", "CardData", "ItemCardGroupId"}
		global.Db.Model(&models.ItemCard{}).
			Select(updateFields).
			Where("id=? AND user_id=?", req.ID, userInfo.ID).Updates(&req)
	} else {
		global.Db.Create(&req)
	}
	clearPublicGroupsCache(userInfo.ID) // 优化：失效公开列表缓存
	// OPT-013：响应序列化时 MarshalJSON 会延迟解析，无需 eager 解析
	apiReturn.SuccessData(c, req)
}

// 删除卡片
func (a *ItemCard) Deletes(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	var req struct {
		Ids []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	global.Db.Where("id IN ? AND user_id=?", req.Ids, userInfo.ID).Delete(&models.ItemCard{})
	clearPublicGroupsCache(userInfo.ID) // 优化：失效公开列表缓存
	apiReturn.Success(c)
}

// 保存排序
func (a *ItemCard) SaveSort(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)
	var req []struct {
		Id   uint `json:"id"`
		Sort int  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	// R导航优化：批量排序用单事务（减少事务开销，保证原子性）
	tx := global.Db.Begin()
	for _, item := range req {
		if err := tx.Model(&models.ItemCard{}).
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
