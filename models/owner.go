package models

type Owner struct {
	Base
	Username string  `gorm:"uniqueIndex;not null" json:"username"`
	Password string  `gorm:"not null" json:"-"` // json:"-" 确保密码不会被返回给前端
	Nickname string  `json:"nickname"`
	Stores   []Store `gorm:"foreignKey:OwnerID" json:"stores"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Nickname string `json:"nickname"`
}
