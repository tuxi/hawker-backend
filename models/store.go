package models

import (
	"time"

	"github.com/google/uuid"
)

// Store 门店模型
type Store struct {
	Base
	OwnerID uuid.UUID `gorm:"type:uuid;index" json:"owner_id"` // 索引提高查询效率
	Name    string    `gorm:"type:varchar(100);not null" json:"name"`
	Address string    `gorm:"type:text" json:"address"`
	// 关联关系（可选，方便 Preload）
	Products []Product `gorm:"foreignKey:StoreID" json:"-"`
}

// SalesRecord 营业额模型
type SalesRecord struct {
	Base
	Date    time.Time `gorm:"index;not null" json:"date"` // 加上日期字段，便于按天查询
	Revenue float64   `gorm:"type:decimal(12,2)" json:"revenue"`
	Notes   string    `gorm:"type:text" json:"notes"`
	StoreID uuid.UUID `gorm:"type:uuid;index;not null" json:"store_id"` // 改为 uuid 类型与 Store.ID 匹配
}

type RevenueDTO struct {
	ID      uuid.UUID `json:"id"`
	Date    time.Time `json:"date"`
	Revenue float64   `json:"revenue"`
	Notes   string    `json:"notes"`
	StoreID uuid.UUID `json:"store_id"`
}

// PromotionSession 促销活动场次
type PromotionSession struct {
	Base
	Title     string               `gorm:"type:varchar(255);not null" json:"title"`
	Date      time.Time            `gorm:"index;not null" json:"date"`
	StartDate *time.Time           `json:"start_date"`
	StoreID   uuid.UUID            `gorm:"type:uuid;index;not null" json:"store_id"`
	Items     []MarketingPromotion `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"items"`
}

// MarketingPromotion 具体的特价商品项
type MarketingPromotion struct {
	Base
	SessionID     uuid.UUID `gorm:"type:uuid;index;not null" json:"session_id"`
	ProductID     uuid.UUID `gorm:"type:uuid;index" json:"product_id"`
	ProductName   string    `gorm:"type:varchar(255)" json:"product_name"`
	OriginalPrice float64   `gorm:"type:decimal(12,2)" json:"original_price"`
	PromoPrice    float64   `gorm:"type:decimal(12,2)" json:"promo_price"`
	PromoUnit     string    `gorm:"type:varchar(50)" json:"promo_unit"`
	PromoTag      string    `gorm:"type:varchar(50)" json:"promo_tag"`
	Remark        string    `gorm:"type:text" json:"remark"`
	SortOrder     int       `gorm:"default:0" json:"sort_order"`
}

type PromotionSessionDTO struct {
	ID        uuid.UUID               `json:"id"`
	Title     string                  `json:"title"`
	Date      time.Time               `json:"date"`
	StartDate *time.Time              `json:"start_date"`
	StoreID   uuid.UUID               `json:"store_id"`
	Items     []MarketingPromotionDTO `json:"items"`
}

type MarketingPromotionDTO struct {
	ID            uuid.UUID `json:"id"`
	ProductID     uuid.UUID `json:"product_id"`
	ProductName   string    `json:"product_name"`
	OriginalPrice float64   `json:"original_price"`
	PromoPrice    float64   `json:"promo_price"`
	PromoUnit     string    `json:"promo_unit"`
	PromoTag      string    `json:"promo_tag"`
	Remark        string    `json:"remark"`
	SortOrder     int       `json:"sort_order"`
	SessionID     uuid.UUID `json:"session_id"`
}
