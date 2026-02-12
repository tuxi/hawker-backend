package models

import "github.com/google/uuid"

// Category 分类模型
type Category struct {
	Base
	StoreID  uuid.UUID `gorm:"type:uuid;index" json:"store_id"` // 所属门店
	Name     string    `gorm:"uniqueIndex;not null" json:"name"`
	Products []Product `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
}
