package models

import (
	"time"

	"github.com/google/uuid"
)

// DisplayItem 柜台商品模型
type DisplayItem struct {
	Base
	ProductID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID" json:"product"` // 关联基本信息

	// 柜台专属属性
	DisplayStock float64 `gorm:"type:decimal(10,2);default:0" json:"display_stock"`
	Threshold    float64 `gorm:"type:decimal(10,2);default:1" json:"threshold"` // 低于此值触发“最后几斤”喊话

	// 价格策略
	CurrentPrice float64 `gorm:"type:decimal(10,2)" json:"current_price"`
	IsPromoted   bool    `gorm:"default:false" json:"is_promoted"`

	// 叫卖状态
	IsHawkingEnabled bool       `gorm:"default:false" json:"is_hawking_enabled"`
	LastHawkedAt     *time.Time `json:"last_hawked_at"`
}
