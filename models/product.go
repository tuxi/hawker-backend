package models

import (
	"time"

	"github.com/google/uuid"
)

// 叫卖模式：0-停止, 1-常规, 2-货源充足(引流), 3-低库存(清货), 4-大促销
type HawkingMode int

const (
	ModeStop      HawkingMode = iota // 停止
	ModeNormal                       // 常规：xx元一斤
	ModeAbundant                     // 货源充足：新鲜到货，快来挑
	ModeLowStock                     // 低库存：最后几斤，便宜卖了
	ModePromotion                    // 促销：限时特价
)

// Product 商品模型
type Product struct {
	Base
	StoreID    uuid.UUID `gorm:"type:uuid;index" json:"store_id"` // 所属门店
	CategoryID uuid.UUID `gorm:"type:uuid;not null" json:"category_id"`
	Category   Category  `gorm:"foreignKey:CategoryID" json:"category"`
	Name       string    `gorm:"not null" json:"name"`
	Unit       string    `json:"unit"`                          // 斤、块、只
	Price      float64   `json:"price"`                         // 默认售价
	IsActive   bool      `gorm:"default:true" json:"is_active"` // 是否上架

	// 手动控制的叫卖参数
	HawkingMode HawkingMode `gorm:"default:0" json:"hawking_mode"`   // 当前模式
	IsHawking   bool        `gorm:"default:false" json:"is_hawking"` // 是否正在叫卖
	//CustomPrice float64     `json:"custom_price"`                    // 叫卖时的临时价格（比如促销价）

	// 调度元数据
	Weight      int `gorm:"default:1" json:"weight"`        // 权重：1-10，决定在轮询中出现的频率
	Priority    int `gorm:"default:0" json:"priority"`      // 优先级：紧急插播使用
	IntervalSec int `gorm:"default:10" json:"interval_sec"` // 喊完此商品后的停顿时间（秒）

	LastHawkedAt *time.Time `json:"last_hawked_at"` // 上次叫卖完成时间

	HawkingStatus string    `gorm:"default:'idle'"` // idle, processing
	LockedAt      time.Time // 用于处理超时锁

	LastScriptHash string `json:"last_script_hash"` // 存储文案的 MD5 或 SHA1，防止重复文案重复合成是纯粹的浪费

	// --- 静态营销属性 ---
	// MarketingLabel: 商品的核心物理特征。
	// 比如：牛肉 -> "新鲜现切的", 猪头肉 -> "半熟的", 鸡爪 -> "个大肥嫩的"
	MarketingLabel string `json:"marketing_label"`
}

type ProductDTO struct {
	ID             uuid.UUID `json:"id"`
	StoreID        uuid.UUID `json:"store_id"` // 所属门店
	Name           string    `json:"name"`
	Unit           string    `json:"unit"`
	Price          float64   `json:"price"`
	CategoryName   string    `json:"category_name"`
	MarketingLabel string    `json:"marketing_label"`

	SafetyStock      int     `json:"safety_stock"`
	WeekendFactor    float64 `json:"weekend_factor"`
	MinOrderQuantity int     `json:"min_order_quantity"`

	MarketingUnit  string  `json:"marketing_unit"`
	MarketingPrice float64 `json:"marketing_price"`
	CurrentStock   int     `json:"current_stock"`

	VendorName string `json:"vendor_name"`
}

// Store 门店模型
type Store struct {
	Base
	OwnerID uuid.UUID `gorm:"type:uuid;index" json:"owner_id"` // 索引提高查询效率
	Name    string    `gorm:"type:varchar(100);not null" json:"name"`
	Address string    `gorm:"type:text" json:"address"`
	// 关联关系（可选，方便 Preload）
	Products []Product `gorm:"foreignKey:StoreID" json:"-"`
}

// ProductDependency 商品依赖模型
type ProductDependency struct {
	Base
	// 关联到父商品（如：整猪）
	ParentID uuid.UUID `gorm:"type:uuid;index;not null" json:"parent_id"`
	// 关联到子商品（如：五花肉）
	ChildID uuid.UUID `gorm:"type:uuid;index;not null" json:"child_id"`

	Ratio          float64 `gorm:"type:decimal(10,4);default:1.0" json:"ratio"` // 比例使用 decimal 保证精度
	Priority       int     `gorm:"default:0" json:"priority"`
	AllowsSeparate bool    `gorm:"default:false" json:"allows_separate"`

	// 物理外键约束（可选，保证数据完整性）
	ParentProduct Product `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	ChildProduct  Product `gorm:"foreignKey:ChildID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// SalesRecord 营业额模型
type SalesRecord struct {
	Base
	Date    time.Time `gorm:"index;not null" json:"date"` // 加上日期字段，便于按天查询
	Revenue float64   `gorm:"type:decimal(12,2)" json:"revenue"`
	Notes   string    `gorm:"type:text" json:"notes"`
	StoreID uuid.UUID `gorm:"type:uuid;index;not null" json:"store_id"` // 改为 uuid 类型与 Store.ID 匹配
}

type DependencyDTO struct {
	ID             uuid.UUID `json:"id"`
	ParentID       uuid.UUID `json:"parent_id"`
	ChildID        uuid.UUID `json:"childID"`
	Ratio          float64   `json:"ratio"`
	Priority       int       `json:"priority"`
	AllowsSeparate bool      `json:"allows_separate"`
}

type RevenueDTO struct {
	ID      uuid.UUID `json:"id"`
	Date    time.Time `json:"date"`
	Revenue float64   `json:"revenue"`
	Notes   string    `json:"notes"`
	StoreID uuid.UUID `json:"store_id"`
}
