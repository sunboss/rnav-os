package models

import (
	"encoding/json"

	"gorm.io/gorm"
)

// 卡片图标信息 (V2 card_data.icon)
type CardIconInfo struct {
	Src  string `json:"src"`
	Text string `json:"text"`
	Type int    `json:"type"`
}

// 卡片数据 (V2 item_card.card_data JSON 结构)
type CardData struct {
	Url        string       `json:"url"`
	LanUrl     string       `json:"lanUrl"`
	OpenMethod int          `json:"openMethod"`
	Icon       CardIconInfo `json:"icon"`
	CustomUrls []string     `json:"customUrls"`
	TextColor  string       `json:"textColor"`
}

// V2 item_card 表
type ItemCard struct {
	BaseModel
	Title           string `gorm:"type:varchar(50)" json:"title"`
	Background      string `gorm:"type:varchar(20)" json:"background"`
	OnlyName        string `gorm:"type:varchar(200)" json:"onlyName"`
	Description     string `gorm:"type:varchar(1000)" json:"description"`
	CardSize        int    `gorm:"type:tinyint(1)" json:"cardSize"`
	Sort            int    `gorm:"type:int(11)" json:"sort"`
	ShowTitle       int    `gorm:"type:tinyint(1)" json:"showTitle"`
	Blur            int    `gorm:"type:tinyint(1)" json:"blur"`
	CardData        string `gorm:"type:text" json:"-"`
	CardDataPrivate string `gorm:"type:text" json:"-"`
	MicroAppId      string `gorm:"type:varchar(20)" json:"microAppId"`
	ItemCardGroupId int    `json:"itemCardGroupId"`
	UserId          uint   `json:"userId"`

	// 解析后的卡片数据，不存库
	ParsedCardData CardData `gorm:"-" json:"cardData"`
	// OPT-013：延迟解析标记（解析只在首次序列化/访问时发生，列表查询不再逐张 eager 解析）
	cardDataParsed bool `gorm:"-"`
}

func (ItemCard) TableName() string { return "item_card" }

// R导航优化：确保高频查询字段有索引
// OPT-012：补充复合索引，覆盖最热查询路径
func EnsureItemCardIndexes(db *gorm.DB) {
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_user_group ON item_card(user_id, item_card_group_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_sort ON item_card(sort)")
	// GetGroupList 的 Preload：WHERE item_card_group_id IN (...) ORDER BY sort
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_group_sort ON item_card(item_card_group_id, sort)")
}

// 解析 card_data JSON（幂等：已解析过直接返回）
// OPT-013：延迟解析 —— 查询列表时不再调用，只在序列化输出或修改前调用
func (m *ItemCard) ParseCardData() {
	if m.cardDataParsed {
		return
	}
	m.cardDataParsed = true
	if m.CardData == "" {
		return
	}
	_ = json.Unmarshal([]byte(m.CardData), &m.ParsedCardData)
}

// 打包 card_data JSON
// OPT-013：先确保解析，避免"DB 读出未解析就打包"时用零值覆盖 CardData
func (m *ItemCard) PackCardData() {
	m.ParseCardData()
	if b, err := json.Marshal(m.ParsedCardData); err == nil {
		m.CardData = string(b)
	}
}

// OPT-013：序列化时才解析 card_data，读多写少的列表接口省掉逐张 Unmarshal
// 注意：必须用值接收者 —— API 层多处以值类型（如 SuccessData(c, req)）序列化，
// 指针接收者的 MarshalJSON 对 interface{} 中的值不会被调用，会导致 cardData 为空
func (m ItemCard) MarshalJSON() ([]byte, error) {
	m.ParseCardData()
	type plain ItemCard // 去方法防递归，保留全部 tag，输出形状不变
	return json.Marshal(plain(m))
}

func (m *ItemCard) GetByUserId(db *gorm.DB, userId uint) ([]ItemCard, error) {
	cards := []ItemCard{}
	err := db.Where("user_id=?", userId).Order("sort, created_at").Find(&cards).Error
	// OPT-013：不再 eager 解析，MarshalJSON 在序列化时延迟解析
	return cards, err
}

// V2 item_card_group 表
type ItemCardGroup struct {
	BaseModel
	Title           string `gorm:"type:varchar(50)" json:"title"`
	OnlyName        string `gorm:"type:varchar(50)" json:"onlyName"`
	Description     string `gorm:"type:varchar(1000)" json:"description"`
	Sort            int    `gorm:"type:int(11)" json:"sort"`
	PublicVisitHide int    `gorm:"type:tinyint(1);default:0" json:"publicVisitHide"` // 0.公开可见 1.访客隐藏(需登录)
	UserId          uint   `json:"userId"`
	Style           string `gorm:"type:text" json:"style"`

	Cards []ItemCard `gorm:"foreignKey:ItemCardGroupId" json:"cards"`
}

func (ItemCardGroup) TableName() string { return "item_card_group" }

// R导航优化：确保高频查询字段有索引
func EnsureItemCardGroupIndexes(db *gorm.DB) {
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_group_user ON item_card_group(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_group_hide ON item_card_group(public_visit_hide)")
	// OPT-012：GetList 主查询 WHERE user_id=? AND public_visit_hide=? ORDER BY sort，用复合索引一遍覆盖
	db.Exec("CREATE INDEX IF NOT EXISTS idx_item_card_group_user_hide_sort ON item_card_group(user_id, public_visit_hide, sort)")
}

// OPT-012：用户表高频查询索引（R通行证登录按 rpass_id 查；AutoMigrate 只对新库建索引，老库需显式补）
func EnsureUserIndexes(db *gorm.DB) {
	db.Exec("CREATE INDEX IF NOT EXISTS idx_user_rpass_id ON user(rpass_id)")
}

// 获取分组列表（publicOnly=true 时过滤掉访客隐藏的分组）
func (m *ItemCardGroup) GetList(db *gorm.DB, userId uint, publicOnly bool) ([]ItemCardGroup, error) {
	return m.fetchList(db, userId, publicOnly)
}

func (m *ItemCardGroup) fetchList(db *gorm.DB, userId uint, publicOnly bool) ([]ItemCardGroup, error) {
	groups := []ItemCardGroup{}
	q := db.Where("user_id=?", userId)
	if publicOnly {
		q = q.Where("public_visit_hide=0")
	}
	err := q.Order("sort, created_at").Preload("Cards", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort, created_at")
	}).Find(&groups).Error
	// OPT-013：不再 eager 解析，MarshalJSON 在序列化时延迟解析
	return groups, err
}
