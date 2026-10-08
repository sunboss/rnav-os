package panel

import (
	"fmt"
	"sync"
	"time"

	"sun-panel/global"
	"sun-panel/lib/cache"
	"sun-panel/models"

	"gorm.io/gorm"
)

// R导航优化：公开分组列表缓存
// 最热路径（首页每次加载），缓存5分钟，变更时主动失效

var (
	publicGroupsCache     cache.Cacher[[]models.ItemCardGroup]
	publicGroupsCacheOnce sync.Once
)

func getPublicGroupsCache() cache.Cacher[[]models.ItemCardGroup] {
	// OPT-012：sync.Once 保证并发下只初始化一次，避免重复创建缓存实例
	publicGroupsCacheOnce.Do(func() {
		publicGroupsCache = global.NewCache[[]models.ItemCardGroup](5*time.Minute, 10*time.Minute, "RnavPublicGroups")
	})
	return publicGroupsCache
}

// 获取分组列表（带缓存）
func getCachedGroupList(db *gorm.DB, userId uint, publicOnly bool) ([]models.ItemCardGroup, error) {
	if !publicOnly {
		mGroup := models.ItemCardGroup{}
		return mGroup.GetList(db, userId, false)
	}
	cacheKey := fmt.Sprintf("rnav:public_groups:%d", userId)
	if cached, ok := getPublicGroupsCache().Get(cacheKey); ok {
		return cached, nil
	}
	mGroup := models.ItemCardGroup{}
	groups, err := mGroup.GetList(db, userId, true)
	if err != nil {
		return nil, err
	}
	getPublicGroupsCache().SetDefault(cacheKey, groups)
	return groups, nil
}

// 清除公开列表缓存
func clearPublicGroupsCache(userId uint) {
	cacheKey := fmt.Sprintf("rnav:public_groups:%d", userId)
	getPublicGroupsCache().Delete(cacheKey)
}

// ClearPublicGroupsCache 供 mcp 包调用
func ClearPublicGroupsCache(userId uint) {
	clearPublicGroupsCache(userId)
}
