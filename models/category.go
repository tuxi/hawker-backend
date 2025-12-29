package models

// Category 分类模型
type Category struct {
	Base
	Name     string    `gorm:"uniqueIndex;not null" json:"name"`
	Products []Product `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
}
