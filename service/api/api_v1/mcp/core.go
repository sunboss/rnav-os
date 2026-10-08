package mcp

import (
	"fmt"

	"sun-panel/api/api_v1/panel"
	"sun-panel/global"
	"sun-panel/models"

	"gorm.io/gorm"
)

// OPT-014：MCP 核心逻辑层——从 gin Handler 中抽离，供 REST 接口与标准 JSON-RPC Server 复用

// 输出结构（REST 与 JSON-RPC 共用）
type mcpGroupOut struct {
	Id              uint   `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Sort            int    `json:"sort"`
	PublicVisitHide int    `json:"publicVisitHide"`
	CardCount       int    `json:"cardCount"`
}

type mcpCardOut struct {
	Id          uint            `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Sort        int             `json:"sort"`
	GroupId     int             `json:"groupId"`
	CardData    models.CardData `json:"cardData"`
}

// coreListGroups 分组列表
func coreListGroups() ([]mcpGroupOut, error) {
	uid := mcpUserId()
	mGroup := models.ItemCardGroup{}
	groups, err := mGroup.GetList(global.Db, uid, false)
	if err != nil {
		return nil, err
	}
	result := []mcpGroupOut{}
	for _, g := range groups {
		result = append(result, mcpGroupOut{
			Id: g.ID, Title: g.Title, Description: g.Description,
			Sort: g.Sort, PublicVisitHide: g.PublicVisitHide, CardCount: len(g.Cards),
		})
	}
	return result, nil
}

// coreListCards 卡片列表（groupTitle 为空=全部）
func coreListCards(groupTitle string) ([]mcpCardOut, error) {
	uid := mcpUserId()
	cards := []models.ItemCard{}
	q := global.Db.Where("user_id=?", uid)
	if groupTitle != "" {
		var group models.ItemCardGroup
		if err := global.Db.Where("user_id=? AND title=?", uid, groupTitle).First(&group).Error; err != nil {
			return []mcpCardOut{}, nil
		}
		q = q.Where("item_card_group_id=?", group.ID)
	}
	if err := q.Order("sort, created_at").Find(&cards).Error; err != nil {
		return nil, err
	}
	result := []mcpCardOut{}
	for i := range cards {
		cards[i].ParseCardData()
		result = append(result, mcpCardOut{
			Id: cards[i].ID, Title: cards[i].Title, Description: cards[i].Description,
			Sort: cards[i].Sort, GroupId: cards[i].ItemCardGroupId, CardData: cards[i].ParsedCardData,
		})
	}
	return result, nil
}

// coreAddGroup 新增分组（已存在直接返回）
func coreAddGroup(title, description string, hide int) (map[string]interface{}, error) {
	if title == "" {
		return nil, fmt.Errorf("title必填")
	}
	uid := mcpUserId()
	var exist models.ItemCardGroup
	if err := global.Db.Where("user_id=? AND title=?", uid, title).First(&exist).Error; err == nil {
		return map[string]interface{}{"id": exist.ID, "title": exist.Title, "exists": true}, nil
	}
	group := models.ItemCardGroup{
		Title: title, Description: description,
		PublicVisitHide: hide, UserId: uid,
	}
	if err := global.Db.Create(&group).Error; err != nil {
		return nil, err
	}
	panel.ClearPublicGroupsCache(uid)
	return map[string]interface{}{"id": group.ID, "title": group.Title}, nil
}

// coreAddCardParams 新增卡片参数
type coreAddCardParams struct {
	Title       string
	Url         string
	Group       string
	Description string
	Icon        string
	OpenMethod  int
}

// coreAddCard 新增卡片（分组不存在自动创建）
func coreAddCard(p coreAddCardParams) (map[string]interface{}, error) {
	if p.Title == "" || p.Url == "" {
		return nil, fmt.Errorf("title和url必填")
	}
	uid := mcpUserId()
	var group models.ItemCardGroup
	groupName := p.Group
	if groupName == "" {
		groupName = "APP"
	}
	if err := global.Db.Where("user_id=? AND title=?", uid, groupName).First(&group).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			group = models.ItemCardGroup{Title: groupName, UserId: uid}
			if err := global.Db.Create(&group).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	icon := p.Icon
	if icon == "" {
		icon = "material-symbols:link"
	}
	openMethod := p.OpenMethod
	if openMethod == 0 {
		openMethod = 2 // 默认新标签页打开
	}
	card := models.ItemCard{
		Title: p.Title, Description: p.Description,
		ItemCardGroupId: int(group.ID), UserId: uid,
		ParsedCardData: models.CardData{
			Url: p.Url, OpenMethod: openMethod,
			Icon: models.CardIconInfo{Text: icon, Type: 3},
		},
	}
	card.PackCardData()
	if err := global.Db.Create(&card).Error; err != nil {
		return nil, err
	}
	panel.ClearPublicGroupsCache(uid)
	return map[string]interface{}{"id": card.ID, "title": card.Title}, nil
}

// coreUpdateCardParams 更新卡片参数
type coreUpdateCardParams struct {
	CardId      uint
	Title       string
	Url         string
	Description string
	Icon        string
	OpenMethod  *int
	Group       string
}

// coreUpdateCard 更新卡片
func coreUpdateCard(p coreUpdateCardParams) (map[string]interface{}, error) {
	if p.CardId == 0 {
		return nil, fmt.Errorf("card_id必填")
	}
	uid := mcpUserId()
	var card models.ItemCard
	if err := global.Db.Where("id=? AND user_id=?", p.CardId, uid).First(&card).Error; err != nil {
		return nil, fmt.Errorf("卡片不存在")
	}
	card.ParseCardData()
	if p.Title != "" {
		card.Title = p.Title
	}
	if p.Url != "" {
		card.ParsedCardData.Url = p.Url
	}
	if p.Description != "" {
		card.Description = p.Description
	}
	if p.Icon != "" {
		card.ParsedCardData.Icon = models.CardIconInfo{Text: p.Icon, Type: 3}
	}
	if p.OpenMethod != nil {
		card.ParsedCardData.OpenMethod = *p.OpenMethod
	}
	if p.Group != "" {
		var group models.ItemCardGroup
		if err := global.Db.Where("user_id=? AND title=?", uid, p.Group).First(&group).Error; err == nil {
			card.ItemCardGroupId = int(group.ID)
		}
	}
	card.PackCardData()
	global.Db.Model(&card).Select("Title", "Description", "CardData", "ItemCardGroupId").Updates(&card)
	panel.ClearPublicGroupsCache(uid)
	return map[string]interface{}{"id": card.ID}, nil
}

// coreDeleteCard 删除卡片
func coreDeleteCard(id uint) error {
	if id == 0 {
		return fmt.Errorf("card_id必填")
	}
	uid := mcpUserId()
	if err := global.Db.Where("id=? AND user_id=?", id, uid).Delete(&models.ItemCard{}).Error; err != nil {
		return err
	}
	panel.ClearPublicGroupsCache(uid)
	return nil
}
