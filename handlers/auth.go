package handlers

import (
	"hawker-backend/conf"
	"hawker-backend/models"
	"hawker-backend/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB  *gorm.DB
	cfg conf.AuthConfig
}

func NewAuthHandler(DB *gorm.DB, cfg conf.AuthConfig) *AuthHandler {
	return &AuthHandler{DB: DB, cfg: cfg}
}

// Register 注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数校验失败"})
		return
	}

	// 密码哈希
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	owner := models.Owner{
		Username: req.Username,
		Password: string(hashedPassword),
		Nickname: req.Nickname,
	}

	if err := h.DB.Create(&owner).Error; err != nil {
		c.JSON(500, gin.H{"error": "用户名已存在或注册失败"})
		return
	}

	c.JSON(200, gin.H{"message": "注册成功"})
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "输入不合法"})
		return
	}

	var owner models.Owner
	if err := h.DB.Where("username = ?", req.Username).First(&owner).Error; err != nil {
		c.JSON(401, gin.H{"error": "用户不存在"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(owner.Password), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "密码错误"})
		return
	}

	// 签发 Token
	token, _ := utils.GenerateToken(owner.ID, h.cfg)
	c.JSON(200, gin.H{
		"token":    token,
		"owner_id": owner.ID,
		"nickname": owner.Nickname,
	})
}
