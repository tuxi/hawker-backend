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
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Unit         string    `json:"unit"`
	CategoryName string    `json:"category_name"`
}
