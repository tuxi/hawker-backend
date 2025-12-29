package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base 模型：让所有表都有 UUID 和时间戳
type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 钩子：创建前自动生成 UUID
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
