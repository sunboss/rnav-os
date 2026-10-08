package models

import (
	"gorm.io/gorm"
)

type ItemIconGroup struct {
	BaseModel
	Icon        string `json:"icon"`
	Title       string `gorm:"type:varchar(50)" json:"title"`
	Description string `gorm:"type:varchar(1000)" json:"description"`
	Sort        int    `gorm:"type:int(11)" json:"sort"`
	Visibility  int    `gorm:"type:tinyint(1);default:0" json:"visibility"` // 可见性 0.公开 1.需登录(R通行证)
	UserId      uint   `json:"userId"`
	User        User   `json:"user"`
}

func (m *ItemIconGroup) DeleteByUserId(db *gorm.DB, userId uint) (err error) {
	err = db.Delete(&ItemIconGroup{}, "user_id = ?", userId).Error
	return
}
